// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rdblib

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/chromiumos/config/go/test/artifact"
	labpb "go.chromium.org/chromiumos/config/go/test/lab/api"
)

func TestBoardModelRealm(t *testing.T) {
	t.Parallel()

	board := "eve"
	dut := &labpb.Dut{
		DutType: &labpb.Dut_Chromeos{
			Chromeos: &labpb.Dut_ChromeOS{
				DutModel: &labpb.DutModel{
					ModelName: "eve",
				},
			},
		},
	}
	validRealms := map[string]bool{
		"eve-eve": true,
	}

	Convey("Get board-model realm for a base build", t, func() {
		testResult := &artifact.TestResult{
			TestInvocation: &artifact.TestInvocation{
				PrimaryExecutionInfo: &artifact.ExecutionInfo{
					BuildInfo: &artifact.BuildInfo{
						Board: board,
						Name:  "eve-cq/R100.0.0",
					},
					DutInfo: &artifact.DutInfo{
						Dut: dut,
					},
				},
			},
		}
		realm := boardModelRealm(testResult, validRealms)
		So(realm, ShouldResemble, "chromeos:eve-eve")
	})

	Convey("Get board-model realm for an allowlisted variant build", t, func() {
		testResult := &artifact.TestResult{
			TestInvocation: &artifact.TestInvocation{
				PrimaryExecutionInfo: &artifact.ExecutionInfo{
					BuildInfo: &artifact.BuildInfo{
						Board: board,
						Name:  "eve64-cq/R100.0.0",
					},
					DutInfo: &artifact.DutInfo{
						Dut: dut,
					},
				},
			},
		}
		realm := boardModelRealm(testResult, validRealms)
		So(realm, ShouldResemble, "chromeos:eve-eve")
	})

	Convey("Get board-model realm for a non-allowlisted variant build", t, func() {
		testResult := &artifact.TestResult{
			TestInvocation: &artifact.TestInvocation{
				PrimaryExecutionInfo: &artifact.ExecutionInfo{
					BuildInfo: &artifact.BuildInfo{
						Board: board,
						Name:  "eve-foo-cq/R100.0.0",
					},
					DutInfo: &artifact.DutInfo{
						Dut: dut,
					},
				},
			},
		}
		realm := boardModelRealm(testResult, validRealms)
		So(realm, ShouldBeEmpty)
	})

	Convey("Get board-model realm for a multi-DUT test", t, func() {
		testResult := &artifact.TestResult{
			TestInvocation: &artifact.TestInvocation{
				PrimaryExecutionInfo: &artifact.ExecutionInfo{
					BuildInfo: &artifact.BuildInfo{
						Board: board,
						Name:  "eve-cq/R100.0.0",
					},
					DutInfo: &artifact.DutInfo{
						Dut: dut,
					},
				},
				SecondaryExecutionsInfo: []*artifact.ExecutionInfo{
					{
						BuildInfo: &artifact.BuildInfo{
							Board: board,
							Name:  "eve-cq/R100.0.0",
						},
						DutInfo: &artifact.DutInfo{
							Dut: dut,
						},
					},
				},
			},
		}
		realm := boardModelRealm(testResult, validRealms)
		So(realm, ShouldBeEmpty)
	})

	Convey("Get board-model realm for a test result with missing board, model, or image", t, func() {
		testResult := &artifact.TestResult{
			TestInvocation: &artifact.TestInvocation{
				PrimaryExecutionInfo: &artifact.ExecutionInfo{
					BuildInfo: &artifact.BuildInfo{
						Board: "",
						Name:  "eve-cq/R100.0.0",
					},
					DutInfo: &artifact.DutInfo{
						Dut: dut,
					},
				},
			},
		}
		realm := boardModelRealm(testResult, validRealms)
		So(realm, ShouldBeEmpty)
	})
}

func TestIsBuildPartnerVisible(t *testing.T) {
	t.Parallel()

	board := "eve"

	Convey("Is build partner visible for a base build", t, func() {
		isPartnerVisible := isBuildPartnerVisible(board, "eve-cq/R100.0.0")
		So(isPartnerVisible, ShouldBeTrue)
	})

	Convey("Is build partner visible for an allowlisted variant build", t, func() {
		for variant := range boardVariantAllowlist {
			boardVariant := board + variant
			image := boardVariant + "-cq/R100.0.0"
			isPartnerVisible := isBuildPartnerVisible(board, image)
			So(isPartnerVisible, ShouldBeTrue)
		}
	})

	Convey("Is build partner visible for a non-allowlisted variant build", t, func() {
		isPartnerVisible := isBuildPartnerVisible(board, "eve-foo-cq/R100.0.0")
		So(isPartnerVisible, ShouldBeFalse)
	})

	Convey("Is build partner visible for a build with no variant", t, func() {
		isPartnerVisible := isBuildPartnerVisible(board, "eve-cq/R100.0.0")
		So(isPartnerVisible, ShouldBeTrue)
	})

	Convey("Is build partner visible for a build with an invalid image", t, func() {
		isPartnerVisible := isBuildPartnerVisible(board, "eve-cq/R100.0.0-invalid")
		So(isPartnerVisible, ShouldBeFalse)
	})
}

func TestPartnerVMRealm(t *testing.T) {
	t.Parallel()

	Convey("Get partner vm realm for a base vm build", t, func() {
		testResult := &artifact.TestResult{
			TestInvocation: &artifact.TestInvocation{
				PrimaryExecutionInfo: &artifact.ExecutionInfo{
					BuildInfo: &artifact.BuildInfo{
						Board: "betty",
						Name:  "betty-release/R100.0.0",
					},
					DutInfo: &artifact.DutInfo{
						Dut: &labpb.Dut{
							DutType: &labpb.Dut_Chromeos{
								Chromeos: &labpb.Dut_ChromeOS{},
							},
						},
					},
				},
			},
		}
		realm := partnerVMRealm(testResult)
		So(realm, ShouldResemble, ChromeOSPartnerVMRealm)
	})

	Convey("Get partner vm realm for an allowlisted vm variant build", t, func() {
		testResult := &artifact.TestResult{
			TestInvocation: &artifact.TestInvocation{
				PrimaryExecutionInfo: &artifact.ExecutionInfo{
					BuildInfo: &artifact.BuildInfo{
						Board: "betty-arc-r",
						Name:  "betty-arc-r/R100.0.0",
					},
					DutInfo: &artifact.DutInfo{
						Dut: &labpb.Dut{
							DutType: &labpb.Dut_Chromeos{
								Chromeos: &labpb.Dut_ChromeOS{},
							},
						},
					},
				},
			},
		}
		realm := partnerVMRealm(testResult)
		So(realm, ShouldResemble, ChromeOSPartnerVMRealm)
	})

	Convey("Get partner vm realm for a non-allowlisted vm variant build", t, func() {
		testResult := &artifact.TestResult{
			TestInvocation: &artifact.TestInvocation{
				PrimaryExecutionInfo: &artifact.ExecutionInfo{
					BuildInfo: &artifact.BuildInfo{
						// "foo" is not in the allowlist.
						Board: "foo",
						Name:  "foo/R100.0.0",
					},
					DutInfo: &artifact.DutInfo{
						Dut: &labpb.Dut{
							DutType: &labpb.Dut_Chromeos{
								Chromeos: &labpb.Dut_ChromeOS{},
							},
						},
					},
				},
			},
		}
		realm := partnerVMRealm(testResult)
		So(realm, ShouldBeEmpty)
	})

	Convey("Get partner vm realm for a staging vm build", t, func() {
		testResult := &artifact.TestResult{
			TestInvocation: &artifact.TestInvocation{
				PrimaryExecutionInfo: &artifact.ExecutionInfo{
					BuildInfo: &artifact.BuildInfo{
						Board: "betty",
						Name:  "staging-betty/R100.0.0",
					},
					DutInfo: &artifact.DutInfo{
						Dut: &labpb.Dut{
							DutType: &labpb.Dut_Chromeos{
								Chromeos: &labpb.Dut_ChromeOS{},
							},
						},
					},
				},
			},
		}
		realm := partnerVMRealm(testResult)
		So(realm, ShouldBeEmpty)
	})

	Convey("Get partner vm realm for a test result with missing board or image", t, func() {
		testResult := &artifact.TestResult{
			TestInvocation: &artifact.TestInvocation{
				PrimaryExecutionInfo: &artifact.ExecutionInfo{
					BuildInfo: &artifact.BuildInfo{
						Board: "",
						Name:  "betty-release/R100.0.0",
					},
					DutInfo: &artifact.DutInfo{
						Dut: &labpb.Dut{
							DutType: &labpb.Dut_Chromeos{
								Chromeos: &labpb.Dut_ChromeOS{},
							},
						},
					},
				},
			},
		}
		realm := partnerVMRealm(testResult)
		So(realm, ShouldBeEmpty)
	})
}
