// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rdblib

import (
	"testing"

	"go.chromium.org/chromiumos/config/go/test/artifact"
	labpb "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
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

	ftt.Run("Get board-model realm for a base build", t, func(t *ftt.Test) {
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
		assert.Loosely(t, realm, should.Match("chromeos:eve-eve"))
	})

	ftt.Run("Get board-model realm for an allowlisted variant build", t, func(t *ftt.Test) {
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
		assert.Loosely(t, realm, should.Match("chromeos:eve-eve"))
	})

	ftt.Run("Get board-model realm for a non-allowlisted variant build", t, func(t *ftt.Test) {
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
		assert.Loosely(t, realm, should.BeEmpty)
	})

	ftt.Run("Get board-model realm for a multi-DUT test", t, func(t *ftt.Test) {
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
		assert.Loosely(t, realm, should.BeEmpty)
	})

	ftt.Run("Get board-model realm for a test result with missing board, model, or image", t, func(t *ftt.Test) {
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
		assert.Loosely(t, realm, should.BeEmpty)
	})
}

func TestIsBuildPartnerVisible(t *testing.T) {
	t.Parallel()

	board := "eve"

	ftt.Run("Is build partner visible for a base build", t, func(t *ftt.Test) {
		isPartnerVisible := isBuildPartnerVisible(board, "eve-cq/R100.0.0")
		assert.Loosely(t, isPartnerVisible, should.BeTrue)
	})

	ftt.Run("Is build partner visible for an allowlisted variant build", t, func(t *ftt.Test) {
		for variant := range boardVariantAllowlist {
			boardVariant := board + variant
			image := boardVariant + "-cq/R100.0.0"
			isPartnerVisible := isBuildPartnerVisible(board, image)
			assert.Loosely(t, isPartnerVisible, should.BeTrue)
		}
	})

	ftt.Run("Is build partner visible for a non-allowlisted variant build", t, func(t *ftt.Test) {
		isPartnerVisible := isBuildPartnerVisible(board, "eve-foo-cq/R100.0.0")
		assert.Loosely(t, isPartnerVisible, should.BeFalse)
	})

	ftt.Run("Is build partner visible for a build with no variant", t, func(t *ftt.Test) {
		isPartnerVisible := isBuildPartnerVisible(board, "eve-cq/R100.0.0")
		assert.Loosely(t, isPartnerVisible, should.BeTrue)
	})

	ftt.Run("Is build partner visible for a build with an invalid image", t, func(t *ftt.Test) {
		isPartnerVisible := isBuildPartnerVisible(board, "eve-cq/R100.0.0-invalid")
		assert.Loosely(t, isPartnerVisible, should.BeFalse)
	})
}

func TestPartnerVMRealm(t *testing.T) {
	t.Parallel()

	ftt.Run("Get partner vm realm for a base vm build", t, func(t *ftt.Test) {
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
		assert.Loosely(t, realm, should.Resemble(ChromeOSPartnerVMRealm))
	})

	ftt.Run("Get partner vm realm for an allowlisted vm variant build", t, func(t *ftt.Test) {
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
		assert.Loosely(t, realm, should.Resemble(ChromeOSPartnerVMRealm))
	})

	ftt.Run("Get partner vm realm for a non-allowlisted vm variant build", t, func(t *ftt.Test) {
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
		assert.Loosely(t, realm, should.BeEmpty)
	})

	ftt.Run("Get partner vm realm for a staging vm build", t, func(t *ftt.Test) {
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
		assert.Loosely(t, realm, should.BeEmpty)
	})

	ftt.Run("Get partner vm realm for a test result with missing board or image", t, func(t *ftt.Test) {
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
		assert.Loosely(t, realm, should.BeEmpty)
	})
}
