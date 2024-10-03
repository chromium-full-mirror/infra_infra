// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package androidapi

import (
	"fmt"
	"infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
	"sort"
)

// WULayer is and enum signifying what WU layer type the node represents
type WULayer int

const (
	TestJob WULayer = iota
	Run
	Shard
	Attempt
)

// WorkUnitNode encapsulates an ATP WorkUnit and associated metadata to keep the
// tree in memory in a sensible way. Struct fields are not exposed to keep
// access of the tree limited to the exposed functions.
type WorkUnitNode struct {
	// workUnit contains the actual ATP Work Unit of the node.
	workUnit *androidbuildinternal.WorkUnit

	// layer describes the which stage in the tree this work unit represents
	layer WULayer

	// index is used to delineate which attempt the node represents of similar
	// tree level. E.g. Attempt #1 or #2
	index int

	// parent is a pointer to the direct ancestor node.
	parent *WorkUnitNode

	// children is a list of all nodes that are descendants of this node.
	children ChildNodes
}

func (w *WorkUnitNode) GetWorkUnit() *androidbuildinternal.WorkUnit {
	return w.workUnit
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
	case TestJob:
		if node.layer != Run {
			return fmt.Errorf("only RUN type nodes can be added under TEST_JOB")
		}
	case Run:
		if node.layer != Shard {
			return fmt.Errorf("only SHARD type nodes can be added under RUN")
		}
	case Shard:
		if node.layer != Attempt {
			return fmt.Errorf("only ATTEMPT type nodes can be added under SHARD")
		}
	case Attempt:
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

func NewWorkUnitNode(workUnit *androidbuildinternal.WorkUnit, nodeType WULayer, parent *WorkUnitNode) *WorkUnitNode {
	return &WorkUnitNode{
		workUnit: workUnit,
		layer:    nodeType,
		// Default to 0. If it is added to a ChildNodes list then get the index
		// based on the order of insertion.
		index:    0,
		parent:   parent,
		children: ChildNodes{},
	}
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
