// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execs

import (
	"time"

	"go.chromium.org/infra/cros/recovery/logger/metrics"
)

// AddObservation adds observation to the metric assigned to the current exec.
func (ei *ExecInfo) AddObservation(observation *metrics.Observation) {
	logger := ei.NewLogger()
	if ei.metric == nil {
		logger.Debugf("Metric is not specified for the action.")
		return
	}
	logger.Infof("Add observation: %#v", observation)
	ei.metric.Observations = append(ei.metric.Observations, observation)
}

// NewMetric creates a new custom metric.
func (ei *ExecInfo) NewMetric(kind string) *metrics.Action {
	// We do not check kind here as if it is empty then it will be rejected before saving.
	metric := ei.runArgs.NewMetricsAction(kind)
	ei.NewLogger().Debugf("Created new metrics for exec %q: %#v", ei.name, metric)
	ei.additionalMetrics = append(ei.additionalMetrics, metric)
	return metric
}

// NewMetricsAction creates a new metric.
func (a *RunArgs) NewMetricsAction(kind string) *metrics.Action {
	metric := &metrics.Action{
		ActionKind:     kind,
		StartTime:      time.Now(),
		SwarmingTaskID: a.SwarmingTaskID,
		BuildbucketID:  a.BuildbucketID,
		Status:         metrics.ActionStatusUnspecified,
	}
	if a.DUT != nil {
		// TODO(b/248635230): Set asset tag instead of hostname.
		metric.Hostname = a.DUT.Name
	}
	return metric
}

// GetAdditionalMetrics returns additional metrics created by execs.
func (ei *ExecInfo) GetAdditionalMetrics() []*metrics.Action {
	return ei.additionalMetrics
}
