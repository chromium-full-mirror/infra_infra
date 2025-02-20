// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configuration

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/chromiumos/config/go/api"
	"go.chromium.org/chromiumos/config/go/payload"
	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

func mockConfigBundle(id string, programId string, name string) *payload.ConfigBundle {
	return &payload.ConfigBundle{
		DesignList: []*api.Design{
			{
				Id: &api.DesignId{
					Value: id,
				},
				ProgramId: &api.ProgramId{
					Value: programId,
				},
				Name: name,
			},
		},
	}
}

func mockChromeOSMachine(id, buildTarget, model, sku string) *ufspb.Machine {
	return &ufspb.Machine{
		Name: id,
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				BuildTarget: buildTarget,
				Model:       model,
				Sku:         sku,
			},
		},
	}
}

func TestUpdateConfigBundle(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	t.Run("update non-existent ConfigBundle", func(t *testing.T) {
		want := mockConfigBundle("design1", "program1", "name1")
		got, err := UpdateConfigBundle(ctx, want)
		if err != nil {
			t.Fatalf("UpdateConfigBundle failed: %s", err)
		}
		if !proto.Equal(want, got) {
			t.Errorf("UpdateConfigBundle returned unexpected diff (-want +got):\n%s\n%s", want, got)
		}
	})

	t.Run("update existent ConfigBundle", func(t *testing.T) {
		cb2 := mockConfigBundle("design2", "program2", "name2")
		cb2update := mockConfigBundle("design2", "program2", "name2update")

		// Insert cb2 into datastore
		_, _ = UpdateConfigBundle(ctx, cb2)

		// Update cb2
		got, err := UpdateConfigBundle(ctx, cb2update)
		if err != nil {
			t.Fatalf("UpdateConfigBundle failed: %s", err)
		}
		if !proto.Equal(cb2update, got) {
			t.Errorf("UpdateConfigBundle returned unexpected diff (-want +got):\n%s\n%s", cb2update, got)
		}
	})

	t.Run("update ConfigBundle with invalid IDs", func(t *testing.T) {
		cb3 := mockConfigBundle("", "", "")
		got, err := UpdateConfigBundle(ctx, cb3)
		if err == nil {
			t.Errorf("UpdateConfigBundle succeeded with empty IDs")
		}
		if c := status.Code(err); c != codes.Internal {
			t.Errorf("Unexpected error when calling UpdateConfigBundle: %s", err)
		}

		var cbNil *payload.ConfigBundle = nil
		if !proto.Equal(cbNil, got) {
			t.Errorf("UpdateConfigBundle returned unexpected diff (-want +got):\n%s\n%s", cbNil, got)
		}
	})
}

func TestGetConfigBundle(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	t.Run("get ConfigBundle by existing ID", func(t *testing.T) {
		want := mockConfigBundle("design1", "program1", "name1")
		_, err := UpdateConfigBundle(ctx, want)
		if err != nil {
			t.Fatalf("UpdateConfigBundle failed: %s", err)
		}

		got, err := GetConfigBundle(ctx, "program1-design1")
		if err != nil {
			t.Fatalf("GetConfigBundle failed: %s", err)
		}
		if !proto.Equal(want, got) {
			t.Errorf("GetConfigBundle returned unexpected diff (-want +got):\n%s\n%s", want, got)
		}
	})

	t.Run("get ConfigBundle by non-existent ID", func(t *testing.T) {
		id := "program2-design2"
		_, err := GetConfigBundle(ctx, id)
		if err == nil {
			t.Errorf("GetConfigBundle succeeded with non-existent ID: %s", id)
		}
		if c := status.Code(err); c != codes.NotFound {
			t.Errorf("Unexpected error when calling GetConfigBundle: %s", err)
		}
	})

	t.Run("get ConfigBundle by invalid ID", func(t *testing.T) {
		id := "program3-design3-extraid3"
		_, err := GetConfigBundle(ctx, id)
		if err == nil {
			t.Errorf("GetConfigBundle succeeded with invalid ID: %s", id)
		}
		if c := status.Code(err); c != codes.InvalidArgument {
			t.Errorf("Unexpected error when calling GetConfigBundle: %s", err)
		}
	})
}

func mockFlatConfig(id string, programId string, name string) *payload.FlatConfig {
	return &payload.FlatConfig{
		HwDesign: &api.Design{
			Id: &api.DesignId{
				Value: id,
			},
			ProgramId: &api.ProgramId{
				Value: programId,
			},
			Name: name,
		},
		HwDesignConfig: &api.Design_Config{
			Id: &api.DesignConfigId{
				Value: name + ":100",
			},
		},
	}
}

func TestUpdateFlatConfig(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	t.Run("update non-existent FlatConfig", func(t *testing.T) {
		want := mockFlatConfig("design1", "program1", "name1")
		got, err := UpdateFlatConfig(ctx, want)
		if err != nil {
			t.Fatalf("UpdateFlatConfig failed: %s", err)
		}
		if !proto.Equal(want, got) {
			t.Errorf("UpdateFlatConfig returned unexpected diff (-want +got):\n%s\n%s", want, got)
		}
	})

	t.Run("update existent FlatConfig", func(t *testing.T) {
		cb2 := mockFlatConfig("design2", "program2", "name2")
		cb2update := mockFlatConfig("design2", "program2", "name2update")

		// Insert cb2 into datastore
		_, _ = UpdateFlatConfig(ctx, cb2)

		// Update cb2
		got, err := UpdateFlatConfig(ctx, cb2update)
		if err != nil {
			t.Fatalf("UpdateFlatConfig failed: %s", err)
		}
		if !proto.Equal(cb2update, got) {
			t.Errorf("UpdateFlatConfig returned unexpected diff (-want +got):\n%s\n%s", cb2update, got)
		}
	})

	t.Run("update FlatConfig with invalid IDs", func(t *testing.T) {
		cb3 := mockFlatConfig("", "", "")
		got, err := UpdateFlatConfig(ctx, cb3)
		if err == nil {
			t.Errorf("UpdateFlatConfig succeeded with empty IDs")
		}
		if c := status.Code(err); c != codes.Internal {
			t.Errorf("Unexpected error when calling UpdateFlatConfig: %s", err)
		}

		var cbNil *payload.FlatConfig = nil
		if !proto.Equal(cbNil, got) {
			t.Errorf("UpdateFlatConfig returned unexpected diff (-want +got):\n%s\n%s", cbNil, got)
		}
	})
}

func TestGetFlatConfig(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	t.Run("get FlatConfig by existing ID", func(t *testing.T) {
		want := mockFlatConfig("design1", "program1", "name1")
		_, err := UpdateFlatConfig(ctx, want)
		if err != nil {
			t.Fatalf("UpdateFlatConfig failed: %s", err)
		}

		got, err := GetFlatConfig(ctx, "program1-design1-name1:100")
		if err != nil {
			t.Fatalf("GetFlatConfig failed: %s", err)
		}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("GetFlatConfig returned unexpected diff (-want +got):\n%s", diff)
		}
	})

	t.Run("get FlatConfig by non-existent ID", func(t *testing.T) {
		id := "program2-design2-name2:100"
		_, err := GetFlatConfig(ctx, id)
		if err == nil {
			t.Errorf("GetFlatConfig succeeded with non-existent ID: %s", id)
		}
		if c := status.Code(err); c != codes.NotFound {
			t.Errorf("Unexpected error when calling GetFlatConfig: %s", err)
		}
	})

	t.Run("get FlatConfig by invalid ID", func(t *testing.T) {
		id := "program3-design3-name3:100-extraid4"
		_, err := GetFlatConfig(ctx, id)
		if err == nil {
			t.Errorf("GetFlatConfig succeeded with invalid ID: %s", id)
		}
		if c := status.Code(err); c != codes.InvalidArgument {
			t.Errorf("Unexpected error when calling GetFlatConfig: %s", err)
		}
	})
}

func TestGenerateFCIdFromCrosMachine(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	t.Run("generate id from fully described cros machine", func(t *testing.T) {
		machine := mockChromeOSMachine("chromeos-asset-1", "bt", "model", "1")
		want := "bt-model-model:1"
		got, err := GenerateFCIdFromCrosMachine(machine)

		if err != nil {
			t.Fatalf("TestGenerateFCIdFromCrosMachine failed: %s", err)
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Unexpected error when calling GenerateFCIdFromCrosMachine: %s", err)
		}
	})

	t.Run("generate id from cros machine with no sku", func(t *testing.T) {
		machine := mockChromeOSMachine("chromeos-asset-2", "bt", "model", "")
		want := "bt-model"
		got, err := GenerateFCIdFromCrosMachine(machine)

		if err != nil {
			t.Fatalf("TestGenerateFCIdFromCrosMachine failed: %s", err)
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("TestGenerateFCIdFromCrosMachine returned unexpected diff (-want +got):\n%s", diff)
		}
	})

	t.Run("generate id from cros machine with no board", func(t *testing.T) {
		machine := mockChromeOSMachine("chromeos-asset-3", "", "model", "")
		const wantErrMsg = "empty board value"
		const want = ""
		got, gotErr := GenerateFCIdFromCrosMachine(machine)

		if gotErr == nil {
			t.Fatal("GenerateFCIdFromCrosMachine succeeded with no board value")
		}

		if gotErr.Error() != wantErrMsg {
			t.Errorf(`Error message diff got "%v", want "%v"`, gotErr.Error(), wantErrMsg)
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("TestGenerateFCIdFromCrosMachine returned unexpected diff (-want +got):\n%s", diff)
		}
	})

	t.Run("generate id from cros machine with no model", func(t *testing.T) {
		machine := mockChromeOSMachine("chromeos-asset-4", "bt", "", "")
		const wantErrMsg = "empty model value"
		const want = ""
		got, gotErr := GenerateFCIdFromCrosMachine(machine)

		if gotErr == nil {
			t.Fatal("GenerateFCIdFromCrosMachine succeeded with no model value")
		}

		if gotErr.Error() != wantErrMsg {
			t.Errorf(`Error message diff got "%v", want "%v"`, gotErr.Error(), wantErrMsg)
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("TestGenerateFCIdFromCrosMachine returned unexpected diff (-want +got):\n%s", diff)
		}
	})

	t.Run("generate id from cros machine with no board or model", func(t *testing.T) {
		machine := mockChromeOSMachine("chromeos-asset-5", "", "", "")
		const wantErrMsg = "empty board value"
		const want = ""
		got, gotErr := GenerateFCIdFromCrosMachine(machine)

		if gotErr == nil {
			t.Fatal("GenerateFCIdFromCrosMachine succeeded with no board value")
		}

		if gotErr.Error() != wantErrMsg {
			t.Errorf(`Error message diff got "%v", want "%v"`, gotErr.Error(), wantErrMsg)
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("TestGenerateFCIdFromCrosMachine returned unexpected diff (-want +got):\n%s", diff)
		}
	})
}
