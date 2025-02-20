// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package try

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.chromium.org/infra/cros/internal/assert"
	"go.chromium.org/infra/cros/internal/gerrit"
)

// TestParseEmailFromAuthInfo tests parseEmailFromAuthInfo.
func TestParseEmailFromAuthInfo(t *testing.T) {
	t.Parallel()

	email, err := parseEmailFromAuthInfo("Logged in as sundar@google.com.\n\nfoo...")
	assert.NilError(t, err)
	assert.StringsEqual(t, email, "sundar@google.com")

	email, err = parseEmailFromAuthInfo("Logged in as sundar@subdomain.google.com.\n\nfoo...")
	assert.NilError(t, err)
	assert.StringsEqual(t, email, "sundar@subdomain.google.com")

	email, err = parseEmailFromAuthInfo("Logged in as sundar.pichai@google.com.\n\nfoo...")
	assert.NilError(t, err)
	assert.StringsEqual(t, email, "sundar.pichai@google.com")

	email, err = parseEmailFromAuthInfo("Logged in as sundar+spam@google.com.\n\nfoo...")
	assert.NilError(t, err)
	assert.StringsEqual(t, email, "sundar+spam@google.com")

	_, err = parseEmailFromAuthInfo("\n\nfoo\nLogged in as sundar@google.com.\n\nfoo...")
	assert.NonNilError(t, err)

	_, err = parseEmailFromAuthInfo("Logged in as sundar!!\n\nfoo...")
	assert.NonNilError(t, err)

	_, err = parseEmailFromAuthInfo("Logged in as sundar@.\n\nfoo...")
	assert.NonNilError(t, err)
}

// TestPatchListToBBAddArgs tests patchListToBBAddArgs
func TestPatchListToBBAddArgs(t *testing.T) {
	t.Parallel()

	patchSets := []string{"crrev.com/c/1234567"}
	expectedBBAddArgs := []string{"-cl", "crrev.com/c/1234567"}
	bbAddArgs := patchListToBBAddArgs(patchSets)
	assert.StringArrsEqual(t, bbAddArgs, expectedBBAddArgs)

	patchSets = []string{"crrev.com/c/1234567", "crrev.com/c/8675309"}
	expectedBBAddArgs = []string{"-cl", "crrev.com/c/1234567", "-cl", "crrev.com/c/8675309"}
	bbAddArgs = patchListToBBAddArgs(patchSets)
	assert.StringArrsEqual(t, bbAddArgs, expectedBBAddArgs)

	patchSets = []string{}
	expectedBBAddArgs = []string{}
	bbAddArgs = patchListToBBAddArgs(patchSets)
	assert.StringArrsEqual(t, bbAddArgs, expectedBBAddArgs)
}

func TestIncludeAllAncestors(t *testing.T) {
	/*
		Using ["crrev.com/c/4279213"] to includeAncestors should return ["crrev.com/c/4279210", "crrev.com/c/4279211", "crrev.com/c/4279212", "crrev.com/c/4279213"]
		Using ["4279210", "4279210"] is the same as above because they are in the same relation chain
		Using ["crrev.com/c/4279212", "crrev.com/i/5279212"] will provide a list of 6 elements
		  - ["crrev.com/c/4279210", "crrev.com/c/4279211", "crrev.com/c/4279212", "crrev.com/i/5279210", "crrev.com/i/5279211", "crrev.com/i/5279212"] is valid
		  - ["crrev.com/i/5279210", "crrev.com/c/4279210", "crrev.com/c/4279211", "crrev.com/c/4279212", "crrev.com/i/5279211", "crrev.com/i/5279212"] is also valid
		  - ["crrev.com/i/5279211", "crrev.com/i/5279210", "crrev.com/c/4279210", "crrev.com/c/4279211", "crrev.com/c/4279212", "crrev.com/i/5279212"] is not valid
		    crrev.com/i/5279211 is newer than crrev.com/c/4279210 in the chain and this ordering must be maintained in the output.
	*/
	emptyChain := []gerrit.Change{}
	externalChain := []gerrit.Change{
		{ChangeNumber: 4279218},
		{ChangeNumber: 4279217},
		{ChangeNumber: 4279216},
		{ChangeNumber: 4279215},
		{ChangeNumber: 4279214},
		{ChangeNumber: 4279213},
		{ChangeNumber: 4279212},
		{ChangeNumber: 4279211},
		{ChangeNumber: 4279210},
	}
	internalChain := []gerrit.Change{
		{ChangeNumber: 5279218},
		{ChangeNumber: 5279217},
		{ChangeNumber: 5279216},
		{ChangeNumber: 5279215},
		{ChangeNumber: 5279214},
		{ChangeNumber: 5279213},
		{ChangeNumber: 5279212},
		{ChangeNumber: 5279211},
		{ChangeNumber: 5279210},
	}
	externalChangeMap := map[int][]gerrit.Change{
		4279218: externalChain,
		4279212: externalChain,
		4279217: externalChain,
		4279210: externalChain,
		4273260: emptyChain,
	}
	internalChangeMap := map[int][]gerrit.Change{
		5279218: internalChain,
		5279212: internalChain,
		5279217: internalChain,
		5279210: internalChain,
		5273260: emptyChain,
	}
	patchChains := map[string]map[int][]gerrit.Change{
		"https://chromium-review.googlesource.com":        externalChangeMap,
		"https://chrome-internal-review.googlesource.com": internalChangeMap,
	}
	mockClient := &gerrit.MockClient{
		T:                      t,
		ExpectedRelatedChanges: patchChains,
	}
	ctx := context.Background()
	t.Run("GetRelated error", func(t *testing.T) {
		t.Parallel()
		// An error getting related changes for any patch returns an empty list and error.
		patchesWithAncestors, err := includeAllAncestors(ctx, mockClient, []string{"crrev.com/i/5273269"})
		assert.NonNilError(t, err)
		assert.IntsEqual(t, 0, len(patchesWithAncestors))
		patchesWithAncestors, err = includeAllAncestors(ctx, mockClient, []string{"crrev.com/i/5279210", "crrev.com/i/5273269"})
		assert.NonNilError(t, err)
		assert.IntsEqual(t, 0, len(patchesWithAncestors))
	})
	t.Run("GetRelated empty list", func(t *testing.T) {
		t.Parallel()
		// Patches with no related changes return the specified patch itself.
		patchesWithAncestors, err := includeAllAncestors(ctx, mockClient, []string{"crrev.com/c/4273260"})
		assert.NilError(t, err)
		assert.IntsEqual(t, 1, len(patchesWithAncestors))
		assert.StringArrsEqual(t, []string{"crrev.com/c/4273260"}, patchesWithAncestors)
		patchesWithAncestors, _ = includeAllAncestors(ctx, mockClient, []string{"crrev.com/c/4273260", "crrev.com/i/5273260"})
		assert.IntsEqual(t, 2, len(patchesWithAncestors))
		assert.StringArrsEqual(t, []string{"crrev.com/c/4273260", "crrev.com/i/5273260"}, patchesWithAncestors)
	})
	t.Run("CrosTry duplicate input", func(t *testing.T) {
		t.Parallel()
		// Duplicated inputs in the PatchList count as one.
		patchesWithAncestors, _ := includeAllAncestors(ctx, mockClient, []string{"crrev.com/c/4273260", "crrev.com/i/5273260", "crrev.com/c/4273260"})
		assert.IntsEqual(t, 2, len(patchesWithAncestors))
		assert.StringArrsEqual(t, []string{"crrev.com/c/4273260", "crrev.com/i/5273260"}, patchesWithAncestors)
		patchesWithAncestors, _ = includeAllAncestors(ctx, mockClient, []string{"crrev.com/i/5279212", "crrev.com/i/5279210", "crrev.com/i/5279212"})
		assert.IntsEqual(t, 3, len(patchesWithAncestors))
		for index := 8; index > 5; index-- {
			assert.StringsEqual(t, fmt.Sprintf("crrev.com/i/%d", internalChain[index].ChangeNumber), patchesWithAncestors[8-index])
		}
	})
	t.Run("CrosTry single patch", func(t *testing.T) {
		t.Parallel()
		// Only required patches from a chain.
		patchesWithAncestors, _ := includeAllAncestors(ctx, mockClient, []string{"crrev.com/i/5279210"})
		assert.StringArrsEqual(t, []string{"crrev.com/i/5279210"}, patchesWithAncestors)
		patchesWithAncestors, _ = includeAllAncestors(ctx, mockClient, []string{"crrev.com/i/5279218"})
		assert.IntsEqual(t, len(internalChain), len(patchesWithAncestors))
		for index := 8; index >= 0; index-- {
			assert.StringsEqual(t, fmt.Sprintf("crrev.com/i/%d", internalChain[index].ChangeNumber), patchesWithAncestors[8-index])
		}
		patchesWithAncestors, _ = includeAllAncestors(ctx, mockClient, []string{"crrev.com/i/5279212"})
		assert.IntsEqual(t, 3, len(patchesWithAncestors))
		for index := 8; index > 5; index-- {
			assert.StringsEqual(t, fmt.Sprintf("crrev.com/i/%d", internalChain[index].ChangeNumber), patchesWithAncestors[8-index])
		}
		patchesWithAncestors, _ = includeAllAncestors(ctx, mockClient, []string{"crrev.com/i/5279212", "crrev.com/i/5279210"})
		assert.IntsEqual(t, 3, len(patchesWithAncestors))
		for index := 8; index > 5; index-- {
			assert.StringsEqual(t, fmt.Sprintf("crrev.com/i/%d", internalChain[index].ChangeNumber), patchesWithAncestors[8-index])
		}
		patchesWithAncestors, _ = includeAllAncestors(ctx, mockClient, []string{"crrev.com/i/5279212", "crrev.com/i/5279217"})
		assert.IntsEqual(t, 8, len(patchesWithAncestors))
		for index := 8; index > 0; index-- {
			assert.StringsEqual(t, fmt.Sprintf("crrev.com/i/%d", internalChain[index].ChangeNumber), patchesWithAncestors[8-index])
		}
	})
	t.Run("CrosTry patch ordering", func(t *testing.T) {
		t.Parallel()
		// Maintaining patch ordering in relation chain.
		patchesWithAncestors, _ := includeAllAncestors(ctx, mockClient, []string{"crrev.com/i/5279212", "crrev.com/c/4279217", "crrev.com/c/4273260", "crrev.com/i/5273260", "crrev.com/i/5279217"})
		// Patches "crrev.com/c/4279211" and "crrev.com/i/5279211" will each have 8 related changes in the output.
		// Patches "crrev.com/c/4273260" and "crrev.com/i/5273260" have no related changes so they are each included once.
		// Patch "crrev.com/i/5279216" counts towards patches needed for "crrev.com/i/5279211".
		assert.IntsEqual(t, 8+8+2, len(patchesWithAncestors))
		// The maps are used to make sure patches are ordered according to the relation chain.
		expectedFromInternal := make(map[string]int)
		expectedFromExternal := make(map[string]int)
		for index := 8; index >= 0; index-- {
			expectedFromInternal[fmt.Sprintf("crrev.com/i/%d", internalChain[index].ChangeNumber)] = 8 - index
			expectedFromExternal[fmt.Sprintf("crrev.com/c/%d", externalChain[index].ChangeNumber)] = 8 - index
		}
		lastInternalVisited := 0
		lastExternalVisited := 0
		// These flags mark "crrev.com/i/5273260" and "crrev.com/c/4273260" in the output.
		var expectSingleInternal bool
		var expectSingleExternal bool
		for _, patch := range patchesWithAncestors {
			if patch == "crrev.com/i/5273260" {
				expectSingleInternal = true
				continue
			}
			if patch == "crrev.com/c/4273260" {
				expectSingleExternal = true
				continue
			}
			if strings.Contains(patch, "crrev.com/i/") {
				if expectedFromInternal[patch] < lastInternalVisited {
					t.Errorf("Unexpected order for patch %s", patch)
				} else {
					// The index of this patch in the chain has been visited.
					lastInternalVisited = expectedFromInternal[patch]
				}
			} else {
				if expectedFromExternal[patch] < lastExternalVisited {
					t.Errorf("Unexpected order for patch %s", patch)
				} else {
					lastExternalVisited = expectedFromExternal[patch]
				}
			}
		}
		assert.Assert(t, expectSingleInternal)
		assert.Assert(t, expectSingleExternal)
	})
}
