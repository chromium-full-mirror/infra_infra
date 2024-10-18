// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"google.golang.org/genproto/protobuf/field_mask"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
	"infra/unifiedfleet/app/model/history"
	"infra/unifiedfleet/app/model/inventory"
)

func mockMachineLSEDeployment(serialNumber string) *ufspb.MachineLSEDeployment {
	return &ufspb.MachineLSEDeployment{
		SerialNumber: serialNumber,
	}
}

func TestUpdateMachineLSEDeployment(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateMachineLSEDeployment", t, func(t *ftt.Test) {
		t.Run("Update MachineLSEDeployment for non-existing MachineLSEDeployment - happy path", func(t *ftt.Test) {
			dr1 := mockMachineLSEDeployment("serial-1")
			resp, err := UpdateMachineLSEDeployment(ctx, dr1, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetHostname(), should.Equal("no-host-yet-serial-1"))
			assert.Loosely(t, resp.GetSerialNumber(), should.Equal("serial-1"))

			resGet, err := inventory.GetMachineLSEDeployment(ctx, "serial-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resGet, should.Match(resp))
		})

		t.Run("Update MachineLSEDeployment for existing MachineLSEDeployment - happy path", func(t *ftt.Test) {
			dr2 := mockMachineLSEDeployment("serial-2")
			_, err := UpdateMachineLSEDeployment(ctx, dr2, nil)
			assert.Loosely(t, err, should.BeNil)

			dr2.Hostname = "hostname-2"
			resp, err := UpdateMachineLSEDeployment(ctx, dr2, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetHostname(), should.Equal("hostname-2"))
			assert.Loosely(t, resp.GetSerialNumber(), should.Equal("serial-2"))

			resGet, err := inventory.GetMachineLSEDeployment(ctx, "serial-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resGet, should.Match(dr2))

			// Verify change events
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machineLSEDeployments/serial-2")
			assert.Loosely(t, err, should.BeNil)
			// Record the first-time registration
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse_deployment"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			// Record the update
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine_lse_deployment.hostname"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("no-host-yet-serial-2"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("hostname-2"))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machineLSEDeployments/serial-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			assert.Loosely(t, msgs[1].Delete, should.BeFalse)
		})

		t.Run("Update MachineLSEDeployment for existing MachineLSEDeployment - partial update hostname & deployment env", func(t *ftt.Test) {
			dr3 := mockMachineLSEDeployment("serial-3")
			_, err := UpdateMachineLSEDeployment(ctx, dr3, nil)
			assert.Loosely(t, err, should.BeNil)

			dr3.Hostname = "hostname-3"
			dr3.DeploymentIdentifier = "identifier-3"
			dr3.DeploymentEnv = ufspb.DeploymentEnv_AUTOPUSH
			resp, err := UpdateMachineLSEDeployment(ctx, dr3, &field_mask.FieldMask{Paths: []string{"hostname", "deployment_env"}})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.GetSerialNumber(), should.Equal("serial-3"))
			assert.Loosely(t, resp.GetHostname(), should.Equal("hostname-3"))
			assert.Loosely(t, resp.GetDeploymentIdentifier(), should.BeEmpty)

			resGet, err := inventory.GetMachineLSEDeployment(ctx, "serial-3")
			assert.Loosely(t, err, should.BeNil)
			dr3.DeploymentIdentifier = ""
			assert.Loosely(t, resGet.GetHostname(), should.Equal("hostname-3"))
			assert.Loosely(t, resGet.GetDeploymentIdentifier(), should.BeEmpty)
			assert.Loosely(t, resGet.GetDeploymentEnv(), should.Equal(ufspb.DeploymentEnv_AUTOPUSH))

			// Verify change events
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "machineLSEDeployments/serial-3")
			assert.Loosely(t, err, should.BeNil)
			// Record the first-time registration
			assert.Loosely(t, changes, should.HaveLength(3))
			assert.Loosely(t, changes[0].GetEventLabel(), should.Equal("machine_lse_deployment"))
			assert.Loosely(t, changes[0].GetOldValue(), should.Equal(LifeCycleRegistration))
			assert.Loosely(t, changes[0].GetNewValue(), should.Equal(LifeCycleRegistration))
			// Record the update
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("machine_lse_deployment.hostname"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("no-host-yet-serial-3"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("hostname-3"))
			assert.Loosely(t, changes[2].GetEventLabel(), should.Equal("machine_lse_deployment.deployment_env"))
			assert.Loosely(t, changes[2].GetOldValue(), should.Equal(ufspb.DeploymentEnv_DEPLOYMENTENV_UNDEFINED.String()))
			assert.Loosely(t, changes[2].GetNewValue(), should.Equal(ufspb.DeploymentEnv_AUTOPUSH.String()))
			msgs, err := history.QuerySnapshotMsgByPropertyName(ctx, "resource_name", "machineLSEDeployments/serial-2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msgs, should.HaveLength(2))
			assert.Loosely(t, msgs[0].Delete, should.BeFalse)
			assert.Loosely(t, msgs[1].Delete, should.BeFalse)
		})
	})
}

func TestListMachineLSEDeployments(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	drs := make([]*ufspb.MachineLSEDeployment, 0, 4)
	for i := 0; i < 4; i++ {
		dr := mockMachineLSEDeployment(fmt.Sprintf("list-dr-%d", i))
		if i%2 == 0 {
			dr.Hostname = fmt.Sprintf("host-%d", i)
		}
		drs = append(drs, dr)
	}
	updatedDrs, _ := inventory.UpdateMachineLSEDeployments(ctx, drs)
	ftt.Run("ListMachineLSEDeployments", t, func(t *ftt.Test) {
		t.Run("List MachineLSEDeployments - filter invalid - error", func(t *ftt.Test) {
			_, _, err := ListMachineLSEDeployments(ctx, 5, "", "invalid=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name invalid"))
		})

		t.Run("List MachineLSEDeployment - filter host - happy path", func(t *ftt.Test) {
			resp, _, err := ListMachineLSEDeployments(ctx, 5, "", "host=host-0", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(1))
			assert.Loosely(t, resp[0], should.Match(updatedDrs[0]))

			resp, _, err = ListMachineLSEDeployments(ctx, 5, "", "host=host-2", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(1))
			assert.Loosely(t, resp[0], should.Match(updatedDrs[2]))
		})

		t.Run("List MachineLSEDeployment - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListMachineLSEDeployments(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(updatedDrs))
		})
	})
}
