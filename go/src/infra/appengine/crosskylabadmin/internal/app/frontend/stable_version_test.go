// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/luci/appengine/gaetesting"
	. "go.chromium.org/luci/common/testing/assertions"
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
	Convey("Test GetStableVersion RPC -- stable versions exist", t, func() {
		ctx := testingContext()
		datastore.GetTestable(ctx)
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		err := dssv.PutSingleCrosStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-cros-version")
		So(err, ShouldBeNil)
		err = dssv.PutSingleFaftStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-faft-version")
		So(err, ShouldBeNil)
		err = dssv.PutSingleFirmwareStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-firmware-version")
		So(err, ShouldBeNil)
		resp, err := tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				BuildTarget: "xxx-build-target",
				Model:       "xxx-model",
			},
		)
		So(err, ShouldBeNil)
		So(resp.CrosVersion, ShouldEqual, "xxx-cros-version")
		So(resp.FaftVersion, ShouldEqual, "xxx-faft-version")
		So(resp.FirmwareVersion, ShouldEqual, "xxx-firmware-version")
	})

	Convey("Test GetStableVersion RPC -- look up by hostname labstation", t, func() {
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
		So(err, ShouldBeNil)
		err = dssv.PutSingleCrosStableVersion(ctx, "xxx-labstation-board", "xxx-labstation-model", "xxx-labstation-cros-version")
		So(err, ShouldBeNil)
		err = dssv.PutSingleFaftStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-faft-version")
		So(err, ShouldBeNil)
		err = dssv.PutSingleFirmwareStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-firmware-version")
		So(err, ShouldBeNil)

		resp, err := tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				Hostname: "xxx-hostname",
			},
		)

		So(err, ShouldBeNil)
		So(resp.CrosVersion, ShouldEqual, "xxx-cros-version")
		So(resp.FaftVersion, ShouldEqual, "xxx-faft-version")
		So(resp.FirmwareVersion, ShouldEqual, "xxx-firmware-version")
		So(resp.ServoCrosVersion, ShouldEqual, "")
	})

	Convey("Test GetStableVersion RPC -- look up labstation proper", t, func() {
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
		So(err, ShouldBeNil)
		err = dssv.PutSingleCrosStableVersion(ctx, "xxx-labstation-board", "xxx-labstation-model", "xxx-labstation-cros-version")
		So(err, ShouldBeNil)
		err = dssv.PutSingleFirmwareStableVersion(ctx, "xxx-labstation-board", "xxx-labstation-model", "xxx-labstation-firmware-version")
		So(err, ShouldBeNil)
		err = dssv.PutSingleFaftStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-faft-version")
		So(err, ShouldBeNil)
		err = dssv.PutSingleFirmwareStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-firmware-version")
		So(err, ShouldBeNil)

		resp, err := tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				Hostname: "xxx-labstation",
			},
		)

		So(err, ShouldBeNil)
		So(resp.CrosVersion, ShouldEqual, "xxx-labstation-cros-version")
		So(resp.FaftVersion, ShouldEqual, "")
		So(resp.FirmwareVersion, ShouldEqual, "") // not expected as faft version is empty.
		So(resp.ServoCrosVersion, ShouldEqual, "")
		So(resp.Reason, ShouldContainSubstring, "looked up non-satlab device hostname")
	})

	Convey("Test GetStableVersion RPC -- hostname with dummy_host", t, func() {
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
		So(err, ShouldBeNil)
		err = dssv.PutSingleCrosStableVersion(ctx, "xxx-labstation-board", "xxx-labstation-model", "xxx-labstation-cros-version")
		So(err, ShouldBeNil)
		err = dssv.PutSingleFaftStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-faft-version")
		So(err, ShouldBeNil)
		err = dssv.PutSingleFirmwareStableVersion(ctx, "xxx-build-target", "xxx-model", "xxx-firmware-version")
		So(err, ShouldBeNil)

		resp, err := tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				Hostname: "xxx-hostname",
			},
		)

		So(err, ShouldBeNil)
		So(resp.CrosVersion, ShouldEqual, "xxx-cros-version")
		So(resp.FaftVersion, ShouldEqual, "xxx-faft-version")
		So(resp.FirmwareVersion, ShouldEqual, "xxx-firmware-version")
		So(resp.ServoCrosVersion, ShouldEqual, "")
		So(resp.Reason, ShouldContainSubstring, "looked up non-satlab device hostname")
	})

	Convey("Test GetStableVersion RPC -- no stable versions exist", t, func() {
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
		So(err, ShouldNotBeNil)
		So(resp, ShouldBeNil)
	})

	// This test creates a fake eve device that is a satlab device, and looks up its stable version.
	// Then we create a hostname-specific stable version and check to make sure that that version overrides the real one.
	Convey("Satlab DUT by model and then by hostname", t, func() {
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
		So(err, ShouldBeNil)

		err = dssv.PutSingleFaftStableVersion(ctx, "eve", "eve", "FAKE-FAFT")
		So(err, ShouldBeNil)

		err = dssv.PutSingleFirmwareStableVersion(ctx, "eve", "eve", "FAKE-FIRMWARE")
		So(err, ShouldBeNil)

		resp, err := tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				Hostname: "satlab-hi-host1",
			},
		)
		So(err, ShouldBeNil)
		So(resp, ShouldNotBeNil)
		So(resp.GetCrosVersion(), ShouldEqual, "FAKE-CROS")
		So(resp.GetFirmwareVersion(), ShouldEqual, "FAKE-FIRMWARE")
		So(resp.GetFaftVersion(), ShouldEqual, "FAKE-FAFT")
		So(resp.GetServoCrosVersion(), ShouldBeEmpty)
		So(resp.GetReason(), ShouldContainSubstring, "falling back")

		err = satlab.PutSatlabStableVersionEntry(
			ctx,
			&satlab.SatlabStableVersionEntry{
				ID:      "satlab-hi-host1",
				OS:      "OVERRIDE-CROS",
				FW:      "OVERRIDE-FIRMWARE",
				FWImage: "OVERRIDE-FAFT",
			},
		)
		So(err, ShouldBeNil)

		resp, err = tf.Inventory.GetStableVersion(
			ctx,
			&fleet.GetStableVersionRequest{
				Hostname: "satlab-hi-host1",
			},
		)
		So(err, ShouldBeNil)
		So(resp, ShouldNotBeNil)
		So(resp.GetCrosVersion(), ShouldEqual, "OVERRIDE-CROS")
		So(resp.GetFirmwareVersion(), ShouldEqual, "OVERRIDE-FIRMWARE")
		So(resp.GetFaftVersion(), ShouldEqual, "OVERRIDE-FAFT")
		So(resp.GetServoCrosVersion(), ShouldBeEmpty)
		So(resp.GetReason(), ShouldContainSubstring, "looked up satlab device using id")
	})
}

func TestDumpStableVersionToDatastore(t *testing.T) {
	Convey("Dump Stable version smoke test", t, func() {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		tf.setStableVersionFactory("{}")
		is := tf.Inventory
		resp, err := is.DumpStableVersionToDatastore(ctx, nil)
		So(err, ShouldBeNil)
		So(resp, ShouldNotBeNil)
	})
	Convey("Update Datastore from empty stableversions file", t, func() {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		tf.setStableVersionFactory(emptyStableVersions)
		_, err := tf.Inventory.DumpStableVersionToDatastore(ctx, nil)
		So(err, ShouldBeNil)
	})
	Convey("Update Datastore from non-empty stableversions file", t, func() {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t)
		defer validate()
		tf.setStableVersionFactory(stableVersions)
		_, err := tf.Inventory.DumpStableVersionToDatastore(ctx, nil)
		So(err, ShouldBeNil)
		cros, err := dssv.GetCrosStableVersion(ctx, "auron_paine", "auron_paine")
		So(err, ShouldBeNil)
		So(cros, ShouldEqual, "R78-12499.40.0")
		firmware, err := dssv.GetFirmwareStableVersion(ctx, "auron_paine", "auron_paine")
		So(err, ShouldBeNil)
		So(firmware, ShouldEqual, "Google_Auron_paine.6301.58.98")
		faft, err := dssv.GetFaftStableVersion(ctx, "auron_paine", "auron_paine")
		So(err, ShouldBeNil)
		So(faft, ShouldEqual, "auron_paine-firmware/R39-6301.58.98")
	})
	Convey("skip entries with empty version strings", t, func() {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t)
		tf.setStableVersionFactory(stableVersionWithEmptyVersions)
		defer validate()
		resp, err := tf.Inventory.DumpStableVersionToDatastore(ctx, nil)
		So(err, ShouldBeNil)
		So(resp, ShouldNotBeNil)
		_, err = dssv.GetCrosStableVersion(ctx, "auron_paine", "auron_paine")
		So(err, ShouldNotBeNil)
		_, err = dssv.GetFirmwareStableVersion(ctx, "auron_paine", "auron_paine")
		So(err, ShouldNotBeNil)
		_, err = dssv.GetFaftStableVersion(ctx, "auron_paine", "auron_paine")
		So(err, ShouldNotBeNil)
	})
}

func TestStableVersionFileParsing(t *testing.T) {
	Convey("Parse non-empty stableversions", t, func() {
		ctx := testingContext()
		parsed, err := parseStableVersions(stableVersions)
		So(err, ShouldBeNil)
		So(parsed, ShouldNotBeNil)
		So(len(parsed.GetCros()), ShouldEqual, 1)
		So(parsed.GetCros()[0].GetVersion(), ShouldEqual, "R78-12499.40.0")
		So(parsed.GetCros()[0].GetKey(), ShouldNotBeNil)
		So(parsed.GetCros()[0].GetKey().GetBuildTarget(), ShouldNotBeNil)
		So(parsed.GetCros()[0].GetKey().GetBuildTarget().GetName(), ShouldEqual, "auron_paine")
		records := getStableVersionRecords(ctx, parsed)
		So(len(records.cros), ShouldEqual, 1)
		So(len(records.firmware), ShouldEqual, 1)
		So(len(records.faft), ShouldEqual, 1)
	})
}

// TestGetAllBoardModels tests getting all board;models out of datastore.
// This test just checks that we can read everything back; it does not use realistic data.
func TestGetAllBoardModels(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContext()
	datastore.GetTestable(ctx).Consistent(true)
	Convey("test get all board models", t, func() {
		So(datastore.Put(ctx, &dssv.CrosStableVersionEntity{
			ID:   "a",
			Cros: "a",
		}), ShouldBeNil)
		So(datastore.Put(ctx, &dssv.FirmwareStableVersionEntity{
			ID:       "b",
			Firmware: "b",
		}), ShouldBeNil)
		So(datastore.Put(ctx, &dssv.FaftStableVersionEntity{
			ID:   "c",
			Faft: "c",
		}), ShouldBeNil)
		out, err := getAllBoardModels(ctx)
		So(err, ShouldBeNil)
		So(out, ShouldResemble, map[string]bool{
			"a": true,
			"b": true,
			"c": true,
		})
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
	Convey("test can clear datastore with empty file", t, func() {
		// 1. Preamble
		So(datastore.Put(ctx, &dssv.CrosStableVersionEntity{
			ID:   "a",
			Cros: "a",
		}), ShouldBeNil)
		So(datastore.Put(ctx, &dssv.FirmwareStableVersionEntity{
			ID:       "b",
			Firmware: "b",
		}), ShouldBeNil)
		So(datastore.Put(ctx, &dssv.FaftStableVersionEntity{
			ID:   "c",
			Faft: "c",
		}), ShouldBeNil)
		out, err := getAllBoardModels(ctx)
		So(err, ShouldBeNil)
		So(len(out), ShouldEqual, 3)
		// 2. Simulate dumping an empty file
		resp, err := dumpStableVersionToDatastoreImpl(ctx, func(_ context.Context, _ string) (string, error) {
			return `{}`, nil
		})
		So(err, ShouldBeNil)
		So(resp, ShouldNotBeNil)
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
	Convey("test can clear datastore with empty file", t, func() {
		// (1/3) Preamble
		So(datastore.Put(ctx, &dssv.CrosStableVersionEntity{
			ID:   "a;a",
			Cros: "a",
		}), ShouldBeNil)
		So(datastore.Put(ctx, &dssv.FirmwareStableVersionEntity{
			ID:       "b;b",
			Firmware: "b",
		}), ShouldBeNil)
		So(datastore.Put(ctx, &dssv.FaftStableVersionEntity{
			ID:   "c;c",
			Faft: "c",
		}), ShouldBeNil)
		out, err := getAllBoardModels(ctx)
		So(err, ShouldBeNil)
		So(len(out), ShouldEqual, 3)
		// (2/3) Simulate reading a file.
		resp, err := dumpStableVersionToDatastoreImpl(ctx, func(_ context.Context, _ string) (string, error) {
			return stableVersions, nil
		})
		So(err, ShouldBeNil)
		So(resp, ShouldNotBeNil)
		// (3/3) Check the contents of datastore post-read.
		cros, err := dssv.GetCrosStableVersion(ctx, "auron_paine", "auron_paine")
		So(err, ShouldBeNil)
		So(cros, ShouldEqual, "R78-12499.40.0")
		firmware, err := dssv.GetFirmwareStableVersion(ctx, "auron_paine", "auron_paine")
		So(err, ShouldBeNil)
		So(firmware, ShouldEqual, "Google_Auron_paine.6301.58.98")
		faft, err := dssv.GetFaftStableVersion(ctx, "auron_paine", "auron_paine")
		So(err, ShouldBeNil)
		So(faft, ShouldEqual, "auron_paine-firmware/R39-6301.58.98")
		// Be extra thorough and check that there are no cros, firmware, or faft versions for any of the fake names
		// used in the preamble.
		for _, name := range []string{"a", "b", "c"} {
			val, err := dssv.GetCrosStableVersion(ctx, name, name)
			So(err, ShouldErrLike, "Entity not found")
			So(val, ShouldEqual, "")
		}
	})
}

func strptr(x string) *string {
	return &x
}
