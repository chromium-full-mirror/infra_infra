// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	fleet "infra/appengine/crosskylabadmin/api/fleet/v1"
	dssv "infra/appengine/crosskylabadmin/internal/app/frontend/datastore/stableversion"
	"infra/appengine/crosskylabadmin/internal/app/frontend/datastore/stableversion/satlab"
	"infra/libs/skylab/inventory"
)

const (
	emptyStableVersions = `{
	"cros": [],
	"faft": [],
	"firmware": []
}`

	stableVersions = `{
    "cros":[
        {
            "key":{
                "buildTarget":{
                    "name":"auron_paine"
                },
                "modelId":{
                    "value":"auron_paine"
                }
            },
            "version":"R78-12499.40.0"
        }
    ],
    "faft":[
        {
            "key": {
                "buildTarget": {
                    "name": "auron_paine"
                },
                "modelId": {
                    "value": "auron_paine"
                }
            },
            "version": "auron_paine-firmware/R39-6301.58.98"
        }
    ],
    "firmware":[
        {
            "key": {
                "buildTarget": {
                    "name": "auron_paine"
                },
                "modelId": {
                    "value": "auron_paine"
                }
            },
            "version": "Google_Auron_paine.6301.58.98"
        }
    ]
}`

	stableVersionWithEmptyVersions = `{
    "cros":[
        {
            "key":{
                "buildTarget":{
                    "name":"auron_paine"
                },
                "modelId":{
                    "value":"auron_paine"
                }
            },
            "version":""
        }
    ],
    "faft":[
        {
            "key": {
                "buildTarget": {
                    "name": "auron_paine"
                },
                "modelId": {
                    "value": "auron_paine"
                }
            },
            "version": ""
        }
    ],
    "firmware":[
        {
            "key": {
                "buildTarget": {
                    "name": "auron_paine"
                },
                "modelId": {
                    "value": "auron_paine"
                }
            },
            "version": ""
        }
    ]
}`
)

// TestGetStableVersion tests the GetStableVersion RPC.
//
// We use test fixtures to set up a fake environment and we override getDUT in a hacky way
// to stub out calls to UFS.
//
// We sometimes set up an environment to test by adding records to the a testing datastore instance,
// which bypasses integerity checks and sometimes call RPCs.
func TestGetStableVersion(t *testing.T) {
	// t.Parallel(). These tests modify the getDUT test override and therefore can't be parallel.
	ftt.Run("Test GetStableVersion RPC -- stable versions exist", t, func(t *ftt.Test) {
		ctx := testingContext()
		datastore.GetTestable(ctx)
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		err := dssv.PutSingleCrosStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-cros-version")
		assert.Loosely(t, err, should.BeNil)
		err = dssv.PutSingleFaftStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-faft-version")
		assert.Loosely(t, err, should.BeNil)
		err = dssv.PutSingleFirmwareStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-firmware-version")
		assert.Loosely(t, err, should.BeNil)
		resp, err := tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				BuildTarget: "xxx-build-target",
				Model:       "xxx-model",
			},
		)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp.CrosVersion, should.Equal("xxx-cros-version"))
		assert.Loosely(t, resp.FaftVersion, should.Equal("xxx-faft-version"))
		assert.Loosely(t, resp.FirmwareVersion, should.Equal("xxx-firmware-version"))
	})

	ftt.Run("Test GetStableVersion RPC -- look up by hostname labstation", t, func(t *ftt.Test) {
		ctx := testingContext()
		datastore.GetTestable(ctx)
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()

		oldGetDUTOverrideForTests := getDUTOverrideForTests
		getDUTOverrideForTests = func(_ context.Context, hostname string) (*inventory.DeviceUnderTest, error) {
			if hostname == "xxx-hostname" {
				return &inventory.DeviceUnderTest{
					Common: &inventory.CommonDeviceSpecs{
						Attributes: []*inventory.KeyValue{
							{
								Key:   strptr("servo_host"),
								Value: strptr("xxx-labstation"),
							},
						},
						Id:       strptr("xxx-id"),
						Hostname: strptr("xxx-hostname"),
						Labels: &inventory.SchedulableLabels{
							Model: strptr("xxx-model"),
							Board: strptr("xxx-build-target"),
						},
					},
				}, nil
			}
			if hostname == "xxx-labstation" {
				return &inventory.DeviceUnderTest{
					Common: &inventory.CommonDeviceSpecs{
						Id:       strptr("xxx-labstation-id"),
						Hostname: strptr("xxx-labstation"),
						Labels: &inventory.SchedulableLabels{
							Model: strptr("xxx-labstation-model"),
							Board: strptr("xxx-labstation-board"),
						},
					},
				}, nil
			}
			return nil, nil
		}
		defer func() {
			getDUTOverrideForTests = oldGetDUTOverrideForTests
		}()

		err := dssv.PutSingleCrosStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-cros-version")
		assert.Loosely(t, err, should.BeNil)
		err = dssv.PutSingleCrosStableVersion(ctx, "xxx-labstation-board", "xxx-labstation-model", "xxx-labstation-cros-version")
		assert.Loosely(t, err, should.BeNil)
		err = dssv.PutSingleFaftStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-faft-version")
		assert.Loosely(t, err, should.BeNil)
		err = dssv.PutSingleFirmwareStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-firmware-version")
		assert.Loosely(t, err, should.BeNil)

		resp, err := tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				Hostname: "xxx-hostname",
			},
		)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp.CrosVersion, should.Equal("xxx-cros-version"))
		assert.Loosely(t, resp.FaftVersion, should.Equal("xxx-faft-version"))
		assert.Loosely(t, resp.FirmwareVersion, should.Equal("xxx-firmware-version"))
		assert.Loosely(t, resp.ServoCrosVersion, should.BeEmpty)
	})

	ftt.Run("Test GetStableVersion RPC -- look up labstation proper", t, func(t *ftt.Test) {
		ctx := testingContext()
		datastore.GetTestable(ctx)
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()

		oldGetDUTOverrideForTests := getDUTOverrideForTests
		getDUTOverrideForTests = func(_ context.Context, hostname string) (*inventory.DeviceUnderTest, error) {
			if hostname == "xxx-hostname" {
				return &inventory.DeviceUnderTest{
					Common: &inventory.CommonDeviceSpecs{
						Attributes: []*inventory.KeyValue{
							{
								Key:   strptr("servo_host"),
								Value: strptr("xxx-labstation"),
							},
						},
						Id:       strptr("xxx-id"),
						Hostname: strptr("xxx-hostname"),
						Labels: &inventory.SchedulableLabels{
							Model: strptr("xxx-model"),
							Board: strptr("xxx-build-target"),
						},
					},
				}, nil
			}
			if hostname == "xxx-labstation" {
				return &inventory.DeviceUnderTest{
					Common: &inventory.CommonDeviceSpecs{
						Id:       strptr("xxx-labstation-id"),
						Hostname: strptr("xxx-labstation"),
						Labels: &inventory.SchedulableLabels{
							Model: strptr("xxx-labstation-model"),
							Board: strptr("xxx-labstation-board"),
						},
					},
				}, nil
			}
			return nil, nil
		}
		defer func() {
			getDUTOverrideForTests = oldGetDUTOverrideForTests
		}()

		err := dssv.PutSingleCrosStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-cros-version")
		assert.Loosely(t, err, should.BeNil)
		err = dssv.PutSingleCrosStableVersion(ctx, "xxx-labstation-board", "xxx-labstation-model", "xxx-labstation-cros-version")
		assert.Loosely(t, err, should.BeNil)
		err = dssv.PutSingleFirmwareStableVersion(ctx, "xxx-labstation-board", "xxx-labstation-model", "xxx-labstation-firmware-version")
		assert.Loosely(t, err, should.BeNil)
		err = dssv.PutSingleFaftStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-faft-version")
		assert.Loosely(t, err, should.BeNil)
		err = dssv.PutSingleFirmwareStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-firmware-version")
		assert.Loosely(t, err, should.BeNil)

		resp, err := tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				Hostname: "xxx-labstation",
			},
		)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp.CrosVersion, should.Equal("xxx-labstation-cros-version"))
		assert.Loosely(t, resp.FaftVersion, should.BeEmpty)
		assert.Loosely(t, resp.FirmwareVersion, should.BeEmpty) // not expected as faft version is empty.
		assert.Loosely(t, resp.ServoCrosVersion, should.BeEmpty)
		assert.Loosely(t, resp.Reason, should.ContainSubstring("looked up non-satlab device hostname"))
	})

	ftt.Run("Test GetStableVersion RPC -- hostname with dummy_host", t, func(t *ftt.Test) {
		ctx := testingContext()
		datastore.GetTestable(ctx)
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()

		oldGetDUTOverrideForTests := getDUTOverrideForTests
		getDUTOverrideForTests = func(_ context.Context, hostname string) (*inventory.DeviceUnderTest, error) {
			return &inventory.DeviceUnderTest{
				Common: &inventory.CommonDeviceSpecs{
					Attributes: []*inventory.KeyValue{
						{
							Key:   strptr("servo_host"),
							Value: strptr("dummy_host"),
						},
					},
					Id:       strptr("xxx-id"),
					Hostname: strptr("xxx-hostname"),
					Labels: &inventory.SchedulableLabels{
						Model: strptr("xxx-model"),
						Board: strptr("xxx-build-target"),
					},
				},
			}, nil
		}
		defer func() {
			getDUTOverrideForTests = oldGetDUTOverrideForTests
		}()

		err := dssv.PutSingleCrosStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-cros-version")
		assert.Loosely(t, err, should.BeNil)
		err = dssv.PutSingleCrosStableVersion(ctx, "xxx-labstation-board", "xxx-labstation-model", "xxx-labstation-cros-version")
		assert.Loosely(t, err, should.BeNil)
		err = dssv.PutSingleFaftStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-faft-version")
		assert.Loosely(t, err, should.BeNil)
		err = dssv.PutSingleFirmwareStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-firmware-version")
		assert.Loosely(t, err, should.BeNil)

		resp, err := tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				Hostname: "xxx-hostname",
			},
		)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp.CrosVersion, should.Equal("xxx-cros-version"))
		assert.Loosely(t, resp.FaftVersion, should.Equal("xxx-faft-version"))
		assert.Loosely(t, resp.FirmwareVersion, should.Equal("xxx-firmware-version"))
		assert.Loosely(t, resp.ServoCrosVersion, should.BeEmpty)
		assert.Loosely(t, resp.Reason, should.ContainSubstring("looked up non-satlab device hostname"))
	})

	ftt.Run("Test GetStableVersion RPC -- no stable versions exist", t, func(t *ftt.Test) {
		ctx := testingContext()
		datastore.GetTestable(ctx)
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		resp, err := tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				BuildTarget: "xxx-build-target",
				Model:       "xxx-model",
			},
		)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
	})

	// This test creates a fake eve device that is a satlab device, and looks up its stable version.
	// Then we create a hostname-specific stable version and check to make sure that that version overrides the real one.
	ftt.Run("Satlab DUT by model and then by hostname", t, func(t *ftt.Test) {
		oldGetDUTOverrideForTests := getDUTOverrideForTests
		getDUTOverrideForTests = func(_ context.Context, hostname string) (*inventory.DeviceUnderTest, error) {
			return &inventory.DeviceUnderTest{
				Common: &inventory.CommonDeviceSpecs{
					Id:       strptr("satlab-hi-host1"),
					Hostname: strptr("satlab-hi-host1"),
					Labels: &inventory.SchedulableLabels{
						Model: strptr("eve"),
						Board: strptr("eve"),
					},
				},
			}, nil
		}
		defer func() {
			getDUTOverrideForTests = oldGetDUTOverrideForTests
		}()

		ctx := testingContext()
		datastore.GetTestable(ctx)
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()

		err := dssv.PutSingleCrosStableVersion(ctx, "eve", "eve", "FAKE-CROS")
		assert.Loosely(t, err, should.BeNil)

		err = dssv.PutSingleFaftStableVersion(ctx, "eve", "eve", "FAKE-FAFT")
		assert.Loosely(t, err, should.BeNil)

		err = dssv.PutSingleFirmwareStableVersion(ctx, "eve", "eve", "FAKE-FIRMWARE")
		assert.Loosely(t, err, should.BeNil)

		resp, err := tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				Hostname: "satlab-hi-host1",
			},
		)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
		assert.Loosely(t, resp.GetCrosVersion(), should.Equal("FAKE-CROS"))
		assert.Loosely(t, resp.GetFirmwareVersion(), should.Equal("FAKE-FIRMWARE"))
		assert.Loosely(t, resp.GetFaftVersion(), should.Equal("FAKE-FAFT"))
		assert.Loosely(t, resp.GetServoCrosVersion(), should.BeEmpty)
		assert.Loosely(t, resp.GetReason(), should.ContainSubstring("falling back"))

		err = satlab.PutSatlabStableVersionEntry(
			ctx,
			&satlab.SatlabStableVersionEntry{
				ID:      "satlab-hi-host1",
				OS:      "OVERRIDE-CROS",
				FW:      "OVERRIDE-FIRMWARE",
				FWImage: "OVERRIDE-FAFT",
			},
		)
		assert.Loosely(t, err, should.BeNil)

		resp, err = tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				Hostname: "satlab-hi-host1",
			},
		)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
		assert.Loosely(t, resp.GetCrosVersion(), should.Equal("OVERRIDE-CROS"))
		assert.Loosely(t, resp.GetFirmwareVersion(), should.Equal("OVERRIDE-FIRMWARE"))
		assert.Loosely(t, resp.GetFaftVersion(), should.Equal("OVERRIDE-FAFT"))
		assert.Loosely(t, resp.GetServoCrosVersion(), should.BeEmpty)
		assert.Loosely(t, resp.GetReason(), should.ContainSubstring("looked up satlab device using id"))
	})
}

func TestDumpStableVersionToDatastore(t *testing.T) {
	ftt.Run("Dump Stable version smoke test", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		tf.setStableVersionFactory("{}")
		is := tf.Inventory
		resp, err := is.DumpStableVersionToDatastore(ctx, nil)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
	})
	ftt.Run("Update Datastore from empty stableversions file", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		tf.setStableVersionFactory(emptyStableVersions)
		_, err := tf.Inventory.DumpStableVersionToDatastore(ctx, nil)
		assert.Loosely(t, err, should.BeNil)
	})
	ftt.Run("Update Datastore from non-empty stableversions file", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		tf.setStableVersionFactory(stableVersions)
		_, err := tf.Inventory.DumpStableVersionToDatastore(ctx, nil)
		assert.Loosely(t, err, should.BeNil)
		cros, err := dssv.GetCrosStableVersion(ctx, "auron_paine", "auron_paine")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cros, should.Equal("R78-12499.40.0"))
		firmware, err := dssv.GetFirmwareStableVersion(ctx, "auron_paine", "auron_paine")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, firmware, should.Equal("Google_Auron_paine.6301.58.98"))
		faft, err := dssv.GetFaftStableVersion(ctx, "auron_paine", "auron_paine")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, faft, should.Equal("auron_paine-firmware/R39-6301.58.98"))
	})
	ftt.Run("skip entries with empty version strings", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t)
		tf.setStableVersionFactory(stableVersionWithEmptyVersions)
		defer validate()
		resp, err := tf.Inventory.DumpStableVersionToDatastore(ctx, nil)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
		_, err = dssv.GetCrosStableVersion(ctx, "auron_paine", "auron_paine")
		assert.Loosely(t, err, should.NotBeNil)
		_, err = dssv.GetFirmwareStableVersion(ctx, "auron_paine", "auron_paine")
		assert.Loosely(t, err, should.NotBeNil)
		_, err = dssv.GetFaftStableVersion(ctx, "auron_paine", "auron_paine")
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestStableVersionFileParsing(t *testing.T) {
	ftt.Run("Parse non-empty stableversions", t, func(t *ftt.Test) {
		ctx := testingContext()
		parsed, err := parseStableVersions(stableVersions)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, parsed, should.NotBeNil)
		assert.Loosely(t, len(parsed.GetCros()), should.Equal(1))
		assert.Loosely(t, parsed.GetCros()[0].GetVersion(), should.Equal("R78-12499.40.0"))
		assert.Loosely(t, parsed.GetCros()[0].GetKey(), should.NotBeNil)
		assert.Loosely(t, parsed.GetCros()[0].GetKey().GetBuildTarget(), should.NotBeNil)
		assert.Loosely(t, parsed.GetCros()[0].GetKey().GetBuildTarget().GetName(), should.Equal("auron_paine"))
		records := getStableVersionRecords(ctx, parsed)
		assert.Loosely(t, len(records.cros), should.Equal(1))
		assert.Loosely(t, len(records.firmware), should.Equal(1))
		assert.Loosely(t, len(records.faft), should.Equal(1))
	})
}

// TestGetAllBoardModels tests getting all board;models out of datastore.
// This test just checks that we can read everything back; it does not use realistic data.
func TestGetAllBoardModels(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContext()
	datastore.GetTestable(ctx).Consistent(true)
	ftt.Run("test get all board models", t, func(t *ftt.Test) {
		assert.Loosely(t, datastore.Put(ctx, &dssv.CrosStableVersionEntity{
			ID:   "a",
			Cros: "a",
		}), should.BeNil)
		assert.Loosely(t, datastore.Put(ctx, &dssv.FirmwareStableVersionEntity{
			ID:       "b",
			Firmware: "b",
		}), should.BeNil)
		assert.Loosely(t, datastore.Put(ctx, &dssv.FaftStableVersionEntity{
			ID:   "c",
			Faft: "c",
		}), should.BeNil)
		out, err := getAllBoardModels(ctx)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, out, should.Resemble(map[string]bool{
			"a": true,
			"b": true,
			"c": true,
		}))
	})
}

// TestCanClearDatastoreWithZeroRecords tests that we can clear datastore by reading in an empty file.
//
// The new behavior created in response to b:250665959 should, among other things, cause there to be zero
// entities of kind *StableVersionKind when the stable version file is `{}`. This behavior is completely
// different than what the behavior would have been before, which would be to leave the table alone and not
// change any records if the stable version file happened to be empty.
func TestCanClearDatastoreWithZeroRecords(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	datastore.GetTestable(ctx).Consistent(true)
	ftt.Run("test can clear datastore with empty file", t, func(t *ftt.Test) {
		// 1. Preamble
		assert.Loosely(t, datastore.Put(ctx, &dssv.CrosStableVersionEntity{
			ID:   "a",
			Cros: "a",
		}), should.BeNil)
		assert.Loosely(t, datastore.Put(ctx, &dssv.FirmwareStableVersionEntity{
			ID:       "b",
			Firmware: "b",
		}), should.BeNil)
		assert.Loosely(t, datastore.Put(ctx, &dssv.FaftStableVersionEntity{
			ID:   "c",
			Faft: "c",
		}), should.BeNil)
		out, err := getAllBoardModels(ctx)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(out), should.Equal(3))
		// 2. Simulate dumping an empty file
		resp, err := dumpStableVersionToDatastoreImpl(ctx, func(_ context.Context, _ string) (string, error) {
			return `{}`, nil
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
	})
}

// TestReplaceDatastoreContents tests replacing the stable-version-related datastore contents with a new file.
//
// The old file has fake data like a,b,c ... The new file has realistic fake data.
// Check to make sure that the new data is present and that the old data is completely gone.
func TestReplaceDatastoreContents(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	datastore.GetTestable(ctx).Consistent(true)
	ftt.Run("test can clear datastore with empty file", t, func(t *ftt.Test) {
		// (1/3) Preamble
		assert.Loosely(t, datastore.Put(ctx, &dssv.CrosStableVersionEntity{
			ID:   "a;a",
			Cros: "a",
		}), should.BeNil)
		assert.Loosely(t, datastore.Put(ctx, &dssv.FirmwareStableVersionEntity{
			ID:       "b;b",
			Firmware: "b",
		}), should.BeNil)
		assert.Loosely(t, datastore.Put(ctx, &dssv.FaftStableVersionEntity{
			ID:   "c;c",
			Faft: "c",
		}), should.BeNil)
		out, err := getAllBoardModels(ctx)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(out), should.Equal(3))
		// (2/3) Simulate reading a file.
		resp, err := dumpStableVersionToDatastoreImpl(ctx, func(_ context.Context, _ string) (string, error) {
			return stableVersions, nil
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.NotBeNil)
		// (3/3) Check the contents of datastore post-read.
		cros, err := dssv.GetCrosStableVersion(ctx, "auron_paine", "auron_paine")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cros, should.Equal("R78-12499.40.0"))
		firmware, err := dssv.GetFirmwareStableVersion(ctx, "auron_paine", "auron_paine")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, firmware, should.Equal("Google_Auron_paine.6301.58.98"))
		faft, err := dssv.GetFaftStableVersion(ctx, "auron_paine", "auron_paine")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, faft, should.Equal("auron_paine-firmware/R39-6301.58.98"))
		// Be extra thorough and check that there are no cros, firmware, or faft versions for any of the fake names
		// used in the preamble.
		for _, name := range []string{"a", "b", "c"} {
			val, err := dssv.GetCrosStableVersion(ctx, name, name)
			assert.Loosely(t, err, should.ErrLike("Entity not found"))
			assert.Loosely(t, val, should.BeEmpty)
		}
	})
}

func strptr(x string) *string {
	return &x
}
