// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package topology

import (
	"context"
	"testing"

	"go.chromium.org/infra/cros/satlab/common/paths"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

func TestDeleteTopology_validateArgs(t *testing.T) {
	type fields struct {
		SatlabID string
		Hostname string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"all fields passed", fields{"123", "satlab-123-host"}, false},
		{"empty satlab-id", fields{"", "satlab-123-host"}, false},
		{"empty hostname", fields{"123", ""}, true},
		{"empty hostname and satlab id", fields{"", ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &DeleteTopology{
				SatlabID: tt.fields.SatlabID,
				Hostname: tt.fields.Hostname,
			}
			if err := c.validateArgs(); (err != nil) != tt.wantErr {
				t.Errorf("DeleteTopology.validateArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteTopology_TriggerRun(t *testing.T) {
	type fields struct {
		SatlabID string
		Hostname string
	}
	type args struct {
		ctx      context.Context
		executor executor.IExecCommander
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"should work", fields{"", "satlab-123-host1"}, args{context.Background(), FakeCommanderWithCommandError("")}, false},
		{"should fail, invalid args", fields{"", ""}, args{context.Background(), FakeCommanderWithCommandError("")}, true},
		{"should fail, get host id", fields{"", "satlab-123-host1"}, args{context.Background(), FakeCommanderWithCommandError(paths.GetHostIdentifierScript)}, true},
		{"should fail, delete topology via shivas", fields{"", "satlab-123-host1"}, args{context.Background(), FakeCommanderWithCommandError(paths.ShivasCLI)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &DeleteTopology{
				SatlabID: tt.fields.SatlabID,
				Hostname: tt.fields.Hostname,
			}
			if err := c.TriggerRun(tt.args.ctx, tt.args.executor); (err != nil) != tt.wantErr {
				t.Errorf("DeleteTopology.TriggerRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
