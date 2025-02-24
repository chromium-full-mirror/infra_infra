// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestHasServo(t *testing.T) {
	t.Parallel()
	satlabId := "satlab123"
	ftt.Run("Register servo for labstation", t, func(t *ftt.Test) {
		ad := &AddDUT{
			Servo:       "servo_1",
			ServoSerial: "servo_serial",
		}
		if yes := ad.setupServo(satlabId); !yes {
			t.Errorf("Expected servo is not detected but expected!")
		}
		assert.Loosely(t, ad.qualifiedServo, should.Equal("satlab-satlab123-servo_1"))
		assert.Loosely(t, ad.ServoDockerContainerName, should.BeEmpty)
	})
	ftt.Run("Register servo for container", t, func(t *ftt.Test) {
		ad := &AddDUT{
			Servo:       "",
			ServoSerial: "servo_serial",
		}
		if yes := ad.setupServo(satlabId); !yes {
			t.Errorf("Expected servo is not detected but expected!")
		}
		assert.Loosely(t, ad.qualifiedServo, should.Equal("satlab-satlab123--docker_servod:9999"))
		assert.Loosely(t, ad.ServoDockerContainerName, should.Equal("satlab-satlab123--docker_servod"))
	})
	ftt.Run("Servo-less setup", t, func(t *ftt.Test) {
		ad := &AddDUT{
			Servo:       "",
			ServoSerial: "",
		}
		if yes := ad.setupServo(satlabId); yes {
			t.Errorf("Expected servo is detected but not expected!")
		}
		assert.Loosely(t, ad.qualifiedServo, should.BeEmpty)
		assert.Loosely(t, ad.ServoDockerContainerName, should.BeEmpty)
	})
}

// TestValidateHostname tests hostname validation.
func TestValidateHostname(t *testing.T) {
	tests := []struct {
		testname string
		hostname string
		wantErr  bool
	}{
		{
			testname: "valid",
			hostname: "eli-123",
			wantErr:  false,
		},
		{
			testname: "uppercase",
			hostname: "ELI-123",
			wantErr:  true,
		},
		{
			testname: "nonalphanumeric",
			hostname: "eli-123!",
			wantErr:  true,
		},
		{
			testname: "too long",
			hostname: "eli-123-eli-123-eli-123-eli-123-eli-123-eli-123-",
			wantErr:  true,
		},
		{
			testname: "empty string",
			hostname: "",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			t.Parallel()

			if err := validateHostname(tt.hostname); (err != nil) != tt.wantErr {
				t.Errorf("validateHostname() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
