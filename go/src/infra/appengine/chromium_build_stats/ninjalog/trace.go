// Copyright 2014 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ninjalog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	trace "cloud.google.com/go/trace/apiv2"
	"cloud.google.com/go/trace/apiv2/tracepb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/logging"
)

// Trace is an entry of trace format.
// https://code.google.com/p/trace-viewer/
type Trace struct {
	Name      string         `json:"name"`
	Category  string         `json:"cat"`
	EventType string         `json:"ph"`
	Timestamp int            `json:"ts"`  // microsecond
	Duration  int            `json:"dur"` // microsecond
	ProcessID int            `json:"pid"`
	ThreadID  int            `json:"tid"`
	Args      map[string]any `json:"args"`
}

type traceByStart []Trace

func (t traceByStart) Len() int           { return len(t) }
func (t traceByStart) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t traceByStart) Less(i, j int) bool { return t[i].Timestamp < t[j].Timestamp }

func toTrace(step Step, pid int, tid int) Trace {
	return Trace{
		Name:      step.Out,
		Category:  "target",
		EventType: "X",
		Timestamp: int(step.Start.Nanoseconds() / 1000),
		Duration:  int(step.Duration().Nanoseconds() / 1000),
		ProcessID: pid,
		ThreadID:  tid,
		Args:      make(map[string]any),
	}
}

// ToTraces converts Flow outputs into trace log.
func ToTraces(steps [][]Step, pid int) []Trace {
	traceNum := 0
	for _, thread := range steps {
		traceNum += len(thread)
	}

	traces := make([]Trace, 0, traceNum)
	for tid, thread := range steps {
		for _, step := range thread {
			// thread id should start from 1
			// https://buganizer.corp.google.com/issues/178753925#comment5
			traces = append(traces, toTrace(step, pid, tid+1))
		}
	}
	sort.Sort(traceByStart(traces))
	return traces
}

func mustHexID(size int) string {
	buf := make([]byte, size)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

// UploadTraceOnCriticalPath uploads build actions included in critical path of build in ninja log to Cloud Trace.
func UploadTraceOnCriticalPath(ctx context.Context, projectID, traceName string, nlog *NinjaLog, invocationID uuid.UUID) (rerr error) {
	if len(nlog.Steps) == 0 {
		logging.Debugf(ctx, "no steps to process")
		return nil
	}
	nlog.Steps = Dedup(nlog.Steps)
	flow := Flow(nlog.Steps, true)
	if len(flow) == 0 {
		logging.Errorf(ctx, "no steps after calling Dedup() and Flow()")
		return errors.New("no steps")
	}
	criticalPath := flow[0]

	c, err := trace.NewClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := c.Close()
		if rerr == nil {
			rerr = err
		}
	}()

	request := &tracepb.BatchWriteSpansRequest{
		Name: "projects/" + projectID,
	}
	traceID := hex.EncodeToString(invocationID[:])

	now := time.Now()

	rootSpanID := mustHexID(8)
	attributeMap := map[string]*tracepb.AttributeValue{}

	targets := nlog.Metadata.getTargets()

	for _, target := range targets {
		attributeMap["build_targets."+target] = &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_BoolValue{BoolValue: true},
		}
	}
	if len(targets) == 0 {
		attributeMap["build_targets.all"] = &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_BoolValue{BoolValue: true},
		}
	}

	for key, value := range nlog.Metadata.BuildConfigs {
		attributeMap["build_configs."+key] = &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_StringValue{
				StringValue: &tracepb.TruncatableString{
					Value: value,
				},
			},
		}
	}

	request.Spans = append(request.Spans, &tracepb.Span{
		Name:   "projects/" + projectID + "/traces/" + traceID + "/spans/" + rootSpanID,
		SpanId: rootSpanID,
		DisplayName: &tracepb.TruncatableString{
			Value: traceName,
		},
		StartTime: timestamppb.New(now),
		EndTime:   timestamppb.New(now.Add(criticalPath[len(criticalPath)-1].End)),
		Attributes: &tracepb.Span_Attributes{
			AttributeMap: attributeMap,
		},
	})

	for _, step := range criticalPath {
		spanID := mustHexID(8)
		request.Spans = append(request.Spans, &tracepb.Span{
			Name:         "projects/" + projectID + "/traces/" + traceID + "/spans/" + spanID,
			SpanId:       spanID,
			ParentSpanId: rootSpanID,
			DisplayName: &tracepb.TruncatableString{
				Value: step.Out,
			},
			StartTime: timestamppb.New(now.Add(step.Start)),
			EndTime:   timestamppb.New(now.Add(step.End)),
		})
	}

	err = c.BatchWriteSpans(ctx, request)
	if err != nil {
		return err
	}
	fmt.Printf("https://console.cloud.google.com/traces/list?project=%s&tid=%s\n", projectID, traceID)
	return nil
}
