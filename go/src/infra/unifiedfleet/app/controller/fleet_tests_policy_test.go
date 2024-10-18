// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"testing"
	"time"

	"go.chromium.org/luci/auth/identity"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	ufspb "infra/unifiedfleet/api/v1/models"
	api "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/model/configuration"
)

const (
	LAUNCHED_BOARD                = "launched_board"
	LAUNCHED_BOARD_NO_MODELS      = "launched_board_no_models"
	UNLAUNCHED_BOARD              = "unlaunched_board"
	LAUNCHED_BOARD_PRIVATE_MODELS = "launched_board_private_models"
	VALID_IMAGE_EVE               = "eve-public/R120-xyz" // Any image with a prefix eve-public/R is a valid image
)

func TestIsValidPublicChromiumTest(t *testing.T) {
	t.Parallel()
	ctx := auth.WithState(testingContext(), &authtest.FakeState{
		Identity:       "user:abc@def.com",
		IdentityGroups: []string{"public-chromium-in-chromeos-builders"},
	})
	configuration.AddPublicBoardModelData(ctx, "eve", []string{"eve"}, false)
	ftt.Run("Is Valid Public Chromium Test", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName:  "tast.lacros",
				Board:     "eve",
				Model:     "eve",
				Image:     VALID_IMAGE_EVE,
				QsAccount: "chromium",
			}

			err := IsValidTest(ctx, req)

			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Private test name and public auth group member", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "private",
				Board:    "eve",
				Model:    "eve",
				Image:    VALID_IMAGE_EVE,
			}

			err := IsValidTest(ctx, req)
			err, ok := err.(*InvalidTestError)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, ok, should.BeTrue)
		})
		t.Run("Public test name and not a public auth group member", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "eve",
				Image:    VALID_IMAGE_EVE,
			}
			newCtx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:abc@def.com",
			})

			err := IsValidTest(newCtx, req)

			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Private test name and not a public auth group member", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "private",
				Board:    "eve",
				Model:    "eve",
				Image:    VALID_IMAGE_EVE,
			}
			ctx := auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:abc@def.com",
			})

			err := IsValidTest(ctx, req)

			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Public test and private board", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "private",
				Model:    "eve",
				Image:    VALID_IMAGE_EVE,
			}

			err := IsValidTest(ctx, req)
			err, ok := err.(*InvalidBoardError)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, ok, should.BeTrue)
		})
		t.Run("Public test and private model", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "private",
				Image:    VALID_IMAGE_EVE,
			}

			err := IsValidTest(ctx, req)
			err, ok := err.(*InvalidModelError)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, ok, should.BeTrue)
		})
		t.Run("Public test and incorrect image", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "eve",
				Image:    "chromiumos-image-archive/eve-public/LATEST-14695.113.3",
			}

			err := IsValidTest(ctx, req)
			err, ok := err.(*InvalidImageError)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, ok, should.BeTrue)
		})
		t.Run("Missing Test names", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "",
				Board:    "eve",
				Model:    "eve",
				Image:    VALID_IMAGE_EVE,
			}

			err := IsValidTest(ctx, req)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Test name cannot be empty"))
		})
		t.Run("Missing Board", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Model:    "eve",
				Image:    VALID_IMAGE_EVE,
			}

			err := IsValidTest(ctx, req)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Board cannot be empty"))
		})
		t.Run("Missing Models - returns error if board has private models", func(t *ftt.Test) {
			configuration.AddPublicBoardModelData(ctx, "fakePrivateBoard", []string{"fakeModelLaunched"}, true)
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "fakePrivateBoard",
				Image:    VALID_IMAGE_EVE,
			}

			err := IsValidTest(ctx, req)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Model cannot be empty as the specified board has unlaunched models"))
		})
		t.Run("Public Model and Public Board With Private Model - succeeds", func(t *ftt.Test) {
			configuration.AddPublicBoardModelData(ctx, "fakePrivateBoard", []string{"fakeModelLaunched"}, true)
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "fakePrivateBoard",
				Model:    "fakeModelLaunched",
				Image:    VALID_IMAGE_EVE,
			}

			err := IsValidTest(ctx, req)

			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Private Model and Public Board With Public Models - returns error", func(t *ftt.Test) {
			configuration.AddPublicBoardModelData(ctx, "fakePrivateBoard", []string{"fakeModelLaunched"}, true)
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "fakePrivateBoard",
				Model:    "fakeModelUnLaunched",
				Image:    VALID_IMAGE_EVE,
			}

			err := IsValidTest(ctx, req)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("private model"))
		})
		t.Run("Missing Models - ok for public boards with only public models", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Image:    VALID_IMAGE_EVE,
			}

			err := IsValidTest(ctx, req)

			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Missing Image", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "eve",
			}

			err := IsValidTest(ctx, req)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Image cannot be empty"))
		})
		t.Run("Invalid QsAccount and public auth group member", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName:  "tast.lacros",
				Board:     "eve",
				Model:     "eve",
				Image:     VALID_IMAGE_EVE,
				QsAccount: "invalid",
			}

			err := IsValidTest(ctx, req)
			err, ok := err.(*InvalidQsAccountError)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, ok, should.BeTrue)
		})
		t.Run("Valid Image for Public Board", func(t *ftt.Test) {
			configuration.AddPublicBoardModelData(ctx, "hana", []string{"hana"}, false)
			req := &api.CheckFleetTestsPolicyRequest{
				TestName:  "tast.lacros",
				Board:     "hana",
				Model:     "hana",
				Image:     "hana-public/R123",
				QsAccount: "chromium",
			}

			err := IsValidTest(ctx, req)

			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Invalid Image for Public Board", func(t *ftt.Test) {
			configuration.AddPublicBoardModelData(ctx, "hana", []string{"hana"}, false)
			req := &api.CheckFleetTestsPolicyRequest{
				TestName:  "tast.lacros",
				Board:     "hana",
				Model:     "hana",
				Image:     "eve-public/R123",
				QsAccount: "chromium",
			}

			err := IsValidTest(ctx, req)

			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestIsPublicGroupMember(t *testing.T) {
	t.Parallel()
	ftt.Run("Is Public Group Member", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "eve",
				Image:    VALID_IMAGE_EVE,
			}
			ctx := auth.WithState(testingContext(), &authtest.FakeState{
				Identity:       "user:abc@def.com",
				IdentityGroups: []string{"public-chromium-in-chromeos-builders"},
			})

			publicGroupMember, err := isPublicGroupMember(ctx, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, publicGroupMember, should.BeTrue)
		})
		t.Run("happy path - request with test service account", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName:           "tast.lacros",
				Board:              "eve",
				Model:              "eve",
				Image:              VALID_IMAGE_EVE,
				TestServiceAccount: "user:abc@def.com",
			}
			ctx := auth.WithState(testingContext(), &authtest.FakeState{
				Identity: identity.AnonymousIdentity,
				FakeDB: authtest.NewFakeDB(
					authtest.MockMembership("user:abc@def.com", PublicUsersToChromeOSAuthGroup),
				),
			})

			publicGroupMember, err := isPublicGroupMember(ctx, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, publicGroupMember, should.BeTrue)
		})
		t.Run("Test service account not a public auth group member - Returns false", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName:           "tast.lacros",
				Board:              "eve",
				Model:              "eve",
				Image:              VALID_IMAGE_EVE,
				TestServiceAccount: "abc@def.com",
			}
			ctx := auth.WithState(testingContext(), &authtest.FakeState{
				Identity: identity.AnonymousIdentity,
			})

			publicGroupMember, err := isPublicGroupMember(ctx, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, publicGroupMember, should.BeFalse)
		})
		t.Run("No Test service account and empty context - Returns false", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "eve",
				Image:    VALID_IMAGE_EVE,
			}
			ctx := auth.WithState(testingContext(), &authtest.FakeState{
				Identity: identity.AnonymousIdentity,
			})

			publicGroupMember, err := isPublicGroupMember(ctx, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, publicGroupMember, should.BeFalse)
		})
		t.Run("Not a public group member", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "eve",
				Image:    VALID_IMAGE_EVE,
			}
			ctx := auth.WithState(testingContext(), &authtest.FakeState{
				Identity: "user:abc@def.com",
			})

			publicGroupMember, err := isPublicGroupMember(ctx, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, publicGroupMember, should.BeFalse)
		})
		t.Run("Nil State - Returns false", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "eve",
				Image:    VALID_IMAGE_EVE,
			}
			ctx := auth.WithState(testingContext(), nil)

			publicGroupMember, err := isPublicGroupMember(ctx, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, publicGroupMember, should.BeFalse)
		})
		t.Run("Nil State DB - Returns false", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "eve",
				Image:    VALID_IMAGE_EVE,
			}
			ctx := auth.WithState(testingContext(), &authtest.FakeState{
				FakeDB: nil,
			})
			publicGroupMember, err := isPublicGroupMember(ctx, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, publicGroupMember, should.BeFalse)
		})
		t.Run("Anonymous Identity - Returns true", func(t *ftt.Test) {
			req := &api.CheckFleetTestsPolicyRequest{
				TestName: "tast.lacros",
				Board:    "eve",
				Model:    "eve",
				Image:    VALID_IMAGE_EVE,
			}
			ctx := auth.WithState(testingContext(), &authtest.FakeState{
				IdentityGroups: []string{"public-chromium-in-chromeos-builders"},
			})
			publicGroupMember, err := isPublicGroupMember(ctx, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, publicGroupMember, should.BeTrue)
		})
	})
}

func TestImportPublicBoardsAndModels(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("Import Public Boards and Models", t, func(t *ftt.Test) {
		t.Run("Happy Path", func(t *ftt.Test) {
			mockDevice := mockDevices()

			err := ImportPublicBoardsAndModels(ctx, mockDevice)

			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Happy Path Check DataStore", func(t *ftt.Test) {
			mockDevice := mockDevices()

			dataerr := ImportPublicBoardsAndModels(ctx, mockDevice)
			entity, err := configuration.GetPublicBoardModelData(ctx, LAUNCHED_BOARD)

			assert.Loosely(t, dataerr, should.BeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, entity.Board, should.Equal(LAUNCHED_BOARD))
			assert.Loosely(t, len(entity.Models), should.Equal(4))
			assert.Loosely(t, entity.BoardHasPrivateModels, should.BeFalse)
		})
		t.Run("Empty Input", func(t *ftt.Test) {
			mockDevice := &ufspb.GoldenEyeDevices{}

			dataerr := ImportPublicBoardsAndModels(ctx, mockDevice)
			entity, err := configuration.GetPublicBoardModelData(ctx, "test")

			assert.Loosely(t, dataerr, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, entity, should.BeNil)
		})
		t.Run("Unlaunched Devices not saved to DataStore", func(t *ftt.Test) {
			mockDevice := mockDevices()

			dataerr := ImportPublicBoardsAndModels(ctx, mockDevice)
			entity, err := configuration.GetPublicBoardModelData(ctx, UNLAUNCHED_BOARD)

			assert.Loosely(t, dataerr, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, entity, should.BeNil)
		})
		t.Run("Unlaunched Models not saved to DataStore", func(t *ftt.Test) {
			mockDevice := mockDevices()

			dataerr := ImportPublicBoardsAndModels(ctx, mockDevice)
			entity, err := configuration.GetPublicBoardModelData(ctx, LAUNCHED_BOARD_PRIVATE_MODELS)

			assert.Loosely(t, dataerr, should.BeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, entity, should.NotBeNil)
			assert.Loosely(t, len(entity.Models), should.Equal(2))
			assert.Loosely(t, entity.BoardHasPrivateModels, should.BeTrue)
		})
		t.Run("Launched board with no models", func(t *ftt.Test) {
			mockDevice := mockDevices()

			dataerr := ImportPublicBoardsAndModels(ctx, mockDevice)
			entity, err := configuration.GetPublicBoardModelData(ctx, LAUNCHED_BOARD_NO_MODELS)

			assert.Loosely(t, dataerr, should.BeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, entity.Board, should.Equal(LAUNCHED_BOARD_NO_MODELS))
			assert.Loosely(t, len(entity.Models), should.Equal(1))
			assert.Loosely(t, entity.Models, should.Match([]string{LAUNCHED_BOARD_NO_MODELS}))
			assert.Loosely(t, entity.BoardHasPrivateModels, should.BeFalse)
		})
	})
}

func TestValidatePublicBoardModel(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	configuration.AddPublicBoardModelData(ctx, "eve", []string{"eve"}, false)
	ftt.Run("Validate Board and Model", t, func(t *ftt.Test) {
		t.Run("Happy Path", func(t *ftt.Test) {
			err := validatePublicBoardModel(ctx, "eve", "eve")
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Private Board", func(t *ftt.Test) {
			err := validatePublicBoardModel(ctx, "board", "eve")
			err, ok := err.(*InvalidBoardError)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, ok, should.BeTrue)
		})
		t.Run("Private Model", func(t *ftt.Test) {
			err := validatePublicBoardModel(ctx, "eve", "model")
			err, ok := err.(*InvalidModelError)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, ok, should.BeTrue)
		})
	})
}

func mockDevices() *ufspb.GoldenEyeDevices {
	return &ufspb.GoldenEyeDevices{
		Devices: []*ufspb.GoldenEyeDevice{
			{
				LaunchDate: "2022-05-01",
				Boards: []*ufspb.Board{
					{
						PublicCodename: LAUNCHED_BOARD,
						Models: []*ufspb.Model{
							{Name: "model1"},
							{Name: "model2"},
						},
					},
				},
			},
			{
				LaunchDate: "2022-05-01",
				Boards: []*ufspb.Board{
					{
						PublicCodename: LAUNCHED_BOARD_PRIVATE_MODELS,
						Models: []*ufspb.Model{
							{Name: "modelOld1"},
							{Name: "modelOld2"},
						},
					},
				},
			},
			{
				LaunchDate: "2021-01-01",
				Boards: []*ufspb.Board{
					{
						PublicCodename: LAUNCHED_BOARD,
						Models: []*ufspb.Model{
							{Name: "model3"},
							{Name: "model4"},
						},
					},
				},
			},
			{
				LaunchDate: time.Now().Add(time.Hour * 10000).Format(DateFormat),
				Boards: []*ufspb.Board{
					{
						PublicCodename: LAUNCHED_BOARD_PRIVATE_MODELS,
						Models: []*ufspb.Model{
							{Name: "modelNew1"},
							{Name: "modelNew2"},
						},
					},
				},
			},
			{
				LaunchDate: time.Now().Add(time.Hour * 10000).Format(DateFormat),
				Boards: []*ufspb.Board{
					{
						PublicCodename: UNLAUNCHED_BOARD,
						Models: []*ufspb.Model{
							{Name: "model2New1"},
							{Name: "model2New2"},
						},
					},
				},
			},
			{
				LaunchDate: "2021-01-01",
				Boards: []*ufspb.Board{
					{
						PublicCodename: LAUNCHED_BOARD_NO_MODELS,
						Models:         []*ufspb.Model{},
					},
				},
			},
		},
	}
}

func getModelNamesFromMockBoard(board *ufspb.Board) (models []string) {
	for _, model := range board.Models {
		models = append(models, model.Name)
	}
	return models
}
