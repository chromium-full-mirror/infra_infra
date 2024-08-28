// Copyright (c) 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package docker

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	dockerContainerTypes "github.com/docker/docker/api/types/container"

	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/common/tsmon"
)

type noOpCloser struct {
	io.Reader
}

func (noOpCloser) Close() (err error) {
	return nil
}

func TestMetrics(t *testing.T) {
	now := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC) // Unix timestamp 946782245
	nowMinus10s := now.Add(-10 * time.Second)
	c := context.Background()
	c, _ = testclock.UseTime(c, now)
	c, _ = tsmon.WithDummyInMemory(c)

	container := dockerTypes.Container{
		Names: []string{"/container_name"},
		State: "running",
	}
	containerState := dockerTypes.ContainerState{
		StartedAt: nowMinus10s.Format(time.RFC3339Nano),
	}
	containerConfig := dockerContainerTypes.Config{
		Hostname: "hostname123",
	}
	containerInfoBase := dockerTypes.ContainerJSONBase{
		State: &containerState,
	}
	containerInfo := dockerTypes.ContainerJSON{
		Config:            &containerConfig,
		ContainerJSONBase: &containerInfoBase,
	}
	containerStatsJSON := dockerTypes.ContainerStats{
		Body: noOpCloser{bytes.NewReader([]byte(`{` +
			`"name": "container1", ` +
			`"memory_stats": {"usage": 1111, "limit": 9999}, ` +
			`"networks": {"eth0": {"rx_bytes": 987, "tx_bytes": 123}}}`))},
	}

	ftt.Run("Test All Metrics", t, func(t *ftt.Test) {
		err := updateContainerMetrics(c, container, containerInfo,
			containerStatsJSON)
		assert.Loosely(t, err, should.BeNil)

		assert.Loosely(t, statusMetric.Get(c, "container_name", "hostname123"), should.Equal("running"))
		assert.Loosely(t, uptimeMetric.Get(c, "container_name"), should.Equal(10.))
		assert.Loosely(t, memUsedMetric.Get(c, "container_name"), should.Equal(1111))
		assert.Loosely(t, memTotalMetric.Get(c, "container_name"), should.Equal(9999))
		assert.Loosely(t, netUpMetric.Get(c, "container_name"), should.Equal(123))
		assert.Loosely(t, netDownMetric.Get(c, "container_name"), should.Equal(987))
	})

	ftt.Run("Test Broken JSON", t, func(t *ftt.Test) {
		bodyJSON := "omg this isn't json"
		reader := bytes.NewReader([]byte(bodyJSON))
		readCloser := noOpCloser{reader}
		containerStatsJSON.Body = readCloser

		err := updateContainerMetrics(c, container, containerInfo,
			containerStatsJSON)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("Test Broken Time Format", t, func(t *ftt.Test) {
		containerState = dockerTypes.ContainerState{StartedAt: "omg this isn't a timestamp"}

		err := updateContainerMetrics(c, container, containerInfo,
			containerStatsJSON)
		assert.Loosely(t, err, should.NotBeNil)
	})
}
