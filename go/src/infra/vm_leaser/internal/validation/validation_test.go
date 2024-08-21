// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package validation

import (
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestValidateLeaseVM(t *testing.T) {
	ftt.Run("Test ValidateLeaseVMRequest", t, func(t *ftt.Test) {
		t.Run("Valid request - successful path", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:       "test-image",
					GceRegion:      "test-region",
					GceProject:     "test-project",
					GceMachineType: "test-machine-type",
					GceDiskSize:    100,
				},
			}
			err := ValidateLeaseVMRequest(req)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Invalid request - missing host requirements", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{}
			err := ValidateLeaseVMRequest(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("VM requirements must be set."))
		})
		t.Run("Invalid request - missing image", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceRegion:      "test-region",
					GceProject:     "test-project",
					GceMachineType: "test-machine-type",
					GceDiskSize:    100,
				},
			}
			err := ValidateLeaseVMRequest(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("GCE image must be set."))
		})
		t.Run("Invalid request - missing region", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:       "test-image",
					GceProject:     "test-project",
					GceMachineType: "test-machine-type",
					GceDiskSize:    100,
				},
			}
			err := ValidateLeaseVMRequest(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("GCE region (zone) must be set."))
		})
		t.Run("Invalid request - missing project", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:       "test-image",
					GceRegion:      "test-region",
					GceMachineType: "test-machine-type",
					GceDiskSize:    100,
				},
			}
			err := ValidateLeaseVMRequest(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("GCE project must be set."))
		})
		t.Run("Invalid request - missing machine type", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:    "test-image",
					GceRegion:   "test-region",
					GceProject:  "test-project",
					GceDiskSize: 100,
				},
			}
			err := ValidateLeaseVMRequest(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("GCE machine type must be set."))
		})
		t.Run("Invalid request - missing disk size", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:       "test-image",
					GceRegion:      "test-region",
					GceProject:     "test-project",
					GceMachineType: "test-machine-type",
				},
			}
			err := ValidateLeaseVMRequest(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("GCE machine disk size must be set (in GB)."))
		})
		t.Run("Valid request - successful path for crosfleet", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:       "test-image",
					GceRegion:      "test-region",
					GceProject:     "test-project",
					GceMachineType: "test-machine-type",
					GceDiskSize:    100,
				},
				TestingClient: api.VMTestingClient_VM_TESTING_CLIENT_CROSFLEET,
				Labels: map[string]string{
					"client":    "crosfleet",
					"leased-by": "example_google_com",
				},
			}
			err := ValidateLeaseVMRequest(req)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Invalid request - missing labels for crosfleet", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:       "test-image",
					GceRegion:      "test-region",
					GceProject:     "test-project",
					GceMachineType: "test-machine-type",
					GceDiskSize:    100,
				},
				TestingClient: api.VMTestingClient_VM_TESTING_CLIENT_CROSFLEET,
			}
			err := ValidateLeaseVMRequest(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Labels should not be nil"))
		})
		t.Run("Invalid request - missing client label for crosfleet", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:       "test-image",
					GceRegion:      "test-region",
					GceProject:     "test-project",
					GceMachineType: "test-machine-type",
					GceDiskSize:    100,
				},
				TestingClient: api.VMTestingClient_VM_TESTING_CLIENT_CROSFLEET,
				Labels: map[string]string{
					"leased-by": "example_google_com",
				},
			}
			err := ValidateLeaseVMRequest(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Labels should contain \"client\"=\"crosfleet\""))
		})
		t.Run("Invalid request - missing leased-by label for crosfleet", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:       "test-image",
					GceRegion:      "test-region",
					GceProject:     "test-project",
					GceMachineType: "test-machine-type",
					GceDiskSize:    100,
				},
				TestingClient: api.VMTestingClient_VM_TESTING_CLIENT_CROSFLEET,
				Labels: map[string]string{
					"client": "crosfleet",
				},
			}
			err := ValidateLeaseVMRequest(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Labels should contain \"leased-by\"={email}"))
		})
	})
}

func TestValidateReleaseVM(t *testing.T) {
	ftt.Run("Test ValidateReleaseVMRequest", t, func(t *ftt.Test) {
		t.Run("Valid request - successful path", func(t *ftt.Test) {
			req := &api.ReleaseVMRequest{
				LeaseId:    "test-lease-id",
				GceProject: "test-project",
				GceRegion:  "test-region",
			}
			err := ValidateReleaseVMRequest(req)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Invalid request - missing lease id", func(t *ftt.Test) {
			req := &api.ReleaseVMRequest{
				GceProject: "test-project",
				GceRegion:  "test-region",
			}
			err := ValidateReleaseVMRequest(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Lease ID must be set."))
		})
		t.Run("Invalid request - missing project", func(t *ftt.Test) {
			req := &api.ReleaseVMRequest{
				LeaseId:   "test-lease-id",
				GceRegion: "test-region",
			}
			err := ValidateReleaseVMRequest(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("GCE project must be set."))
		})
		t.Run("Invalid request - missing region", func(t *ftt.Test) {
			req := &api.ReleaseVMRequest{
				LeaseId:    "test-lease-id",
				GceProject: "test-project",
			}
			err := ValidateReleaseVMRequest(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("GCE region (zone) must be set."))
		})
	})
}

func TestValidateVMRequirements(t *testing.T) {
	ftt.Run("Test ValidateVMRequirements", t, func(t *ftt.Test) {
		t.Run("Valid request - successful path", func(t *ftt.Test) {
			req := &api.VMRequirements{
				GceImage:       "test-image",
				GceRegion:      "test-region",
				GceProject:     "test-project",
				GceMachineType: "test-machine-type",
				GceDiskSize:    100,
			}
			err := ValidateVMRequirements(req)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Invalid request - missing image", func(t *ftt.Test) {
			req := &api.VMRequirements{
				GceRegion:      "test-region",
				GceProject:     "test-project",
				GceMachineType: "test-machine-type",
				GceDiskSize:    100,
			}
			err := ValidateVMRequirements(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("GCE image must be set."))
		})
		t.Run("Invalid request - missing region", func(t *ftt.Test) {
			req := &api.VMRequirements{
				GceImage:       "test-image",
				GceProject:     "test-project",
				GceMachineType: "test-machine-type",
				GceDiskSize:    100,
			}
			err := ValidateVMRequirements(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("GCE region (zone) must be set."))
		})
		t.Run("Invalid request - missing project", func(t *ftt.Test) {
			req := &api.VMRequirements{
				GceImage:       "test-image",
				GceRegion:      "test-region",
				GceMachineType: "test-machine-type",
				GceDiskSize:    100,
			}
			err := ValidateVMRequirements(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("GCE project must be set."))
		})
		t.Run("Invalid request - missing machine type", func(t *ftt.Test) {
			req := &api.VMRequirements{
				GceImage:    "test-image",
				GceRegion:   "test-region",
				GceProject:  "test-project",
				GceDiskSize: 100,
			}
			err := ValidateVMRequirements(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("GCE machine type must be set."))
		})
		t.Run("Invalid request - missing disk size", func(t *ftt.Test) {
			req := &api.VMRequirements{
				GceImage:       "test-image",
				GceRegion:      "test-region",
				GceProject:     "test-project",
				GceMachineType: "test-machine-type",
			}
			err := ValidateVMRequirements(req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("GCE machine disk size must be set (in GB)."))
		})
	})
}

func TestValidateLeaseParent(t *testing.T) {
	ftt.Run("Test ValidateLeaseParent", t, func(t *ftt.Test) {
		t.Run("Valid regex - successful path; only project", func(t *ftt.Test) {
			err := ValidateLeaseParent("projects/test-project")
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Valid regex - successful path; project and zone", func(t *ftt.Test) {
			err := ValidateLeaseParent("projects/test-project/zones/test-zone")
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Valid regex - error; no project", func(t *ftt.Test) {
			err := ValidateLeaseParent("projects/")
			assert.Loosely(t, err, should.ErrLike("parent must be in the format `projects/${project}` or `projects/${project}/zones/${zone}`"))
		})
		t.Run("Valid regex - error; extra string", func(t *ftt.Test) {
			err := ValidateLeaseParent("projects/test-project/123")
			assert.Loosely(t, err, should.ErrLike("parent must be in the format `projects/${project}` or `projects/${project}/zones/${zone}`"))
		})
		t.Run("Valid regex - error; typo in zone", func(t *ftt.Test) {
			err := ValidateLeaseParent("projects/test-project/zone/fail-zone")
			assert.Loosely(t, err, should.ErrLike("parent must be in the format `projects/${project}` or `projects/${project}/zones/${zone}`"))
		})
	})
}
