// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package try

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/internal/gerrit"
)

// interfaceSlicetoStr converts a slice of interface{}s to a slice of strings.
func interfaceSliceToStr(s []any) []string {
	ret := make([]string, len(s))
	for i := range s {
		ret[i] = s[i].(string)
	}
	return ret
}

// parseEmailFromAuthInfo parses an email from a `led auth-info` invocation.
func parseEmailFromAuthInfo(stdout string) (string, error) {
	reAuthUser := regexp.MustCompile(`^Logged in as ([A-Za-z0-9\-_.+]+@[A-Za-z0-9\-_.+]+\.\w+)\.(\s|$)`)
	submatch := reAuthUser.FindStringSubmatch(stdout)
	if len(submatch) == 0 {
		return "", fmt.Errorf("Could not find username in `luci auth-info` output:\n%s", stdout)
	}
	return strings.TrimSpace(submatch[1]), nil
}

// sliceContainsStr checks whether a []string contains a string.
func sliceContainsStr(slice []string, s string) bool {
	for _, x := range slice {
		if x == s {
			return true
		}
	}
	return false
}

// patchListToBBAddArgs converts a []string of patches to []string formatting expected by bb add (like ["crrev.com/c/1234567"] -> ["-cl", "crrev.com/c/123456"])
func patchListToBBAddArgs(patches []string) []string {
	bbAddArgs := make([]string, 0)
	for _, patch := range patches {
		bbAddArgs = append(bbAddArgs, []string{"-cl", patch}...)
	}
	return bbAddArgs
}

// formatPatchURL formats a changeNumber and gerritInstance into a patch string like "crrev.com/c/1234567".
func formatPatchURL(gerritInstance string, changeNumber int) string {
	return fmt.Sprintf("crrev.com/%s/%d", gerritInstance, changeNumber)
}

/*
In includeAllAncestors, patchInfo allows for two things

1. Marking a visited index in an array representing a RelatedChanges chain
2. Keep a record of the changeNumber at a given index in the RelatedChanes chain

consider a chain [A, B, C, D] and a patches list [D, B].
after evaluating required changes for D,
once we get to B and realize that it was visited as part of the alignment for D,
we stop and go no further.
*/
type patchInfo struct {
	changeNumber int
	visited      bool
}

// includeAllAncestors includes all ancestors of a patch so that the cherry-pick steps don't fail due to merge conflict.
func includeAllAncestors(ctx context.Context, client gerrit.Client, patches []string) ([]string, error) {
	hostMap := map[string]string{
		"c": "https://chromium-review.googlesource.com",
		"i": "https://chrome-internal-review.googlesource.com",
	}
	patchSpec := regexp.MustCompile(PatchRegexpPattern)
	var patchesWithAncestors []string
	// the `^crrev\.com\/([ci])\/(\d{7,8})` component that indicates whether to use gerrit internal or external.
	// Example: c for the chromium instance for i for internal.
	// rootChangeMap keeps a map of each gerritInstance to a map of parent changeNumbers to their RelatedChanges.
	rootChangeMap := make(map[string]map[int][]patchInfo)
	for gerritInstance := range hostMap {
		rootChangeMap[gerritInstance] = make(map[int][]patchInfo)
	}
	for _, patch := range patches {
		// r.validate() already ensures all patches match the expected regex pattern.
		regexMatch := patchSpec.FindStringSubmatch(patch)
		gerritInstance := regexMatch[1]
		host := hostMap[gerritInstance]
		changeNumber, _ := strconv.Atoi(regexMatch[2])
		// Get the list of relatedChanges for this given patch.
		// Example for crrev.com/c/4279215: [4279218, 4279217, 4279216, 4279215, 4279214, 4279213, 4279212, 4279211, 4279210]}}.
		relatedChanges, err := client.GetRelatedChanges(ctx, host, changeNumber)
		if err != nil {
			return []string{}, errors.Annotate(err, "GetRelatedChanges(ctx, %s, %d):", host, changeNumber).Err()
		}
		numberOfRelatedChanges := len(relatedChanges)
		if numberOfRelatedChanges == 0 {
			if _, ok := rootChangeMap[gerritInstance][changeNumber]; !ok {
				// For a singleton patch with no related changes, the patch alone must be returned.
				rootChangeMap[gerritInstance][changeNumber] = []patchInfo{{changeNumber, true}}
				patchesWithAncestors = append(patchesWithAncestors, formatPatchURL(gerritInstance, changeNumber))
			}
		} else {
			// The rootChange is the oldest in the list of relatedChanges and is at the last position in the slice.
			rootChange := relatedChanges[numberOfRelatedChanges-1].ChangeNumber
			if _, ok := rootChangeMap[gerritInstance][rootChange]; !ok {
				rootChangeMap[gerritInstance][rootChange] = make([]patchInfo, numberOfRelatedChanges)
				// Reverse the list of relatedChanges from oldest to newest when mapped to the root (oldest).
				// Example: {'c': {4279210:[{4279210}, {4279211}, {4279212}, {4279213}, {4279214}, {4279215}, {4279216}, {4279217}, {4279218}]}}.
				for i := numberOfRelatedChanges - 1; i >= 0; i-- {
					rootChangeMap[gerritInstance][rootChange][numberOfRelatedChanges-i-1] = patchInfo{relatedChanges[i].ChangeNumber, false}
				}
			}
			for i, change := range rootChangeMap[gerritInstance][rootChange] {
				if !change.visited {
					patchesWithAncestors = append(patchesWithAncestors, formatPatchURL(gerritInstance, change.changeNumber))
					rootChangeMap[gerritInstance][rootChange][i] = patchInfo{change.changeNumber, true}
				}
				if change.changeNumber == changeNumber {
					break
				}
			}
		}
	}
	return patchesWithAncestors, nil
}
