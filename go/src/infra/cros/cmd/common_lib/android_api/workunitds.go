// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapi

import (
	"context"
	"fmt"
	"sort"

	atp "go.chromium.org/infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

// WULayer is and enum signifying what WU layer type the node represents
type WULayer int

func (w WULayer) String() string {
	switch w {
	case WULayerUnknown:
		return "UNKNOWN"
	case WULayerTestJob:
		return "TEST_JOB"
	case WULayerRun:
		return "RUN"
	case WULayerShard:
		return "SHARD"
	case WULayerAttempt:
		return "ATTEMPT"
	case WULayerCTP:
		return "CTP"
	default:
		return ""
	}
}

const (
	WULayerUnknown WULayer = iota
	WULayerTestJob
	WULayerRun
	WULayerShard
	WULayerAttempt
	WULayerCTP
)

type WorkUnitTree struct {
	Head *WorkUnitNode

	// ShardsByKey will be used to map shard nodes by the BuildsMap keys. This
	// will allow us to remove the requirement of including the shards as
	// arguments to scheduleAndMonitor().
	ShardsByKey map[string]*WorkUnitNode
}

// WorkUnitNode encapsulates an ATP WorkUnit and associated metadata to keep the
// tree in memory in a sensible way. Struct fields are not exposed to keep
// access of the tree limited to the exposed functions.
type WorkUnitNode struct {
	// workUnit contains the actual ATP Work Unit of the node.
	workUnit *atp.WorkUnit

	// layer describes the which stage in the tree this work unit represents
	layer WULayer

	// index is used to delineate which attempt the node represents of similar
	// tree level. E.g. Attempt #1 or #2
	index int

	// parent is a pointer to the direct ancestor node.
	parent *WorkUnitNode

	// children is a list of all nodes that are descendants of this node.
	children ChildNodes

	Service WorkUnitService
}

func (w *WorkUnitNode) GetWorkUnit() *atp.WorkUnit {
	return w.workUnit
}

func (w *WorkUnitNode) SetWorkUnit(newWU *atp.WorkUnit) {
	w.workUnit = newWU
}

func (w *WorkUnitNode) GetLayer() WULayer {
	return w.layer
}

func (w *WorkUnitNode) GetIndex() int {
	return w.index
}

func (w *WorkUnitNode) GetParent() *WorkUnitNode {
	return w.parent
}

// GetChildren returns the sorted list of children.
func (w *WorkUnitNode) GetChildren() ChildNodes {
	sort.Sort(w.children)

	return w.children
}

func (w *WorkUnitNode) AddChild(node *WorkUnitNode) error {
	// Enforce the layer hierarchy rules
	switch w.layer {
	case WULayerCTP:
		if node.layer != WULayerTestJob {
			return fmt.Errorf("only TEST_JOB type nodes can be added under CTP")
		}
	case WULayerTestJob:
		if node.layer != WULayerRun {
			return fmt.Errorf("only RUN type nodes can be added under TEST_JOB")
		}
	case WULayerRun:
		if node.layer != WULayerShard {
			return fmt.Errorf("only SHARD type nodes can be added under RUN")
		}
	case WULayerShard:
		if node.layer != WULayerAttempt {
			return fmt.Errorf("only ATTEMPT type nodes can be added under SHARD")
		}
	case WULayerAttempt:
		return fmt.Errorf("nodes cannot be added under ATTEMPT nodes")
	default:
		return fmt.Errorf("unexpected node type received")
	}

	// Insert node into the list of child nodes
	w.children = append(w.children, node)

	// Set the index of the inserted node to the current length of the list post
	// insertion. This will allow us differentiate which nodes came in what
	// order.
	node.index = w.children.Len()

	return nil
}

func (w *WorkUnitNode) FetchHead() (*WorkUnitNode, error) {
	cycleChecker := map[*WorkUnitNode]struct{}{}

	top := w
	for {
		// Check to see if we are stuck in a cycle.
		if _, ok := cycleChecker[top]; ok {
			return nil, fmt.Errorf("a cycle has been detected in the node graph")
		} else {
			cycleChecker[top] = struct{}{}
		}

		if top.parent == nil {
			return top, nil
		}

		top = top.parent
	}
}

func (w *WorkUnitNode) FetchRunLayer() ([]*WorkUnitNode, error) {
	topNode, err := w.FetchHead()
	if err != nil {
		return nil, err
	}

	return topNode.GetChildren(), nil
}

func (w *WorkUnitNode) FetchShardLayer() ([][]*WorkUnitNode, error) {
	runLayer, err := w.FetchRunLayer()
	if err != nil {
		return nil, err
	}

	shardLayer := [][]*WorkUnitNode{}
	for _, runNode := range runLayer {
		shardLayer = append(shardLayer, runNode.children)
	}

	return shardLayer, nil
}

func (w *WorkUnitNode) FetchAttemptLayer() ([][]*WorkUnitNode, error) {
	shardLayer, err := w.FetchShardLayer()
	if err != nil {
		return nil, err
	}

	attemptLayer := [][]*WorkUnitNode{}
	for _, runNodes := range shardLayer {
		for _, runNode := range runNodes {
			attemptLayer = append(attemptLayer, runNode.children)
		}
	}

	return attemptLayer, nil
}

// NewWorkUnitNode Creates and registers a Work Unit using the ATP API and
// inserts it into the local Work Unit tree.
func NewWorkUnitNode(ctx context.Context, parentWUId, invocationID string, nodeType WULayer, parent *WorkUnitNode, env common.Environment) (*WorkUnitNode, error) {
	// TODO: Pass this in rather than create a new one each time
	service, err := NewAndroidBuildService(ctx, ServiceAccount, env)
	if err != nil {
		return nil, err
	}

	// Create the node that will represent the work unit in the cached tree.
	newWU := &WorkUnitNode{
		layer: nodeType,
		// Default to 0. If it is added to a ChildNodes list then get the index
		// based on the order of insertion.
		index:    0,
		parent:   parent,
		children: ChildNodes{},
		Service:  service.WorkUnitService,
	}

	// If parent is nil then that means we are at the top node and do not need
	// to add this node to a ChildNodes list.
	//
	// NOTE: This will only be used by a TEST_JOB type node.
	if parent != nil {
		if err := parent.AddChild(newWU); err != nil {
			return nil, err
		}
	}

	// Set the layer corresponding child number.
	childRunNumber, childShardNumber, childAttemptNumber := 0, 0, 0
	runNumber := newWU.index - 1
	if runNumber < 0 {
		runNumber = 0
	}
	switch nodeType {
	case WULayerRun:
		childRunNumber = runNumber
	case WULayerShard:
		childShardNumber = runNumber
	case WULayerAttempt:
		childAttemptNumber = runNumber
	}

	var name string
	if nodeType == WULayerCTP || nodeType == WULayerTestJob {
		name = nodeType.String()
	} else {
		name = fmt.Sprintf("%s #%d", nodeType.String(), runNumber)
	}

	// Create the work unit "request" then insert it using the ATP API. The API
	// will return a WU that has a registered WUID. We do not set that in code
	// here.
	workUnit := NewWorkUnit(parentWUId, invocationID, name, childRunNumber, childShardNumber, childAttemptNumber)
	workUnit, err = service.WorkUnitService.Insert(ctx, workUnit)
	if err != nil {
		return nil, err
	}

	// Place the registered WU inside the node structure.
	newWU.workUnit = workUnit

	return newWU, nil
}

// ChildNodes implements sort.Interface so that we can iterate according to the
// node's index.
type ChildNodes []*WorkUnitNode

func (c ChildNodes) Len() int {
	return len(c)
}

func (c ChildNodes) Less(i, j int) bool {
	return c[i].index > c[j].index
}

func (c ChildNodes) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}
