// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package totmanager encapsulates all the required functions for ensuring tot
// mapping rules are followed.
package totmanager

import (
	"fmt"

	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"

	"go.chromium.org/infra/cros/cmd/kron/common"
	"go.chromium.org/infra/cros/internal/chromeosversion"
)

var (
	tot = chromeosversion.VersionInfo{}
)

// InitTotManager fetch the chromeos version info from the overlays repo.
func InitTotManager() error {
	fileData, err := common.FetchFileFromURL(common.TotFileURL)
	if err != nil {
		return err
	}

	tot, err = chromeosversion.ParseVersionInfo(fileData)

	return err
}

// GetTot returns the calculated ToT version. If the value is 0 then that means the ToT
// info was never fetched and the nil int value is being returned.
//
// NOTE: Canary is ToT.
func GetTot() int {
	return tot.ChromeBranch
}

func GetDev() int {
	return tot.ChromeBranch - 1
}

func GetBeta() int {
	return tot.ChromeBranch - 2
}

func GetStable() int {
	return tot.ChromeBranch - 3
}

func isCanary(milestone int) bool {
	return milestone == GetTot()
}

func isDev(milestone int) bool {
	return milestone == GetDev()
}

func isBeta(milestone int) bool {
	return milestone == GetBeta()
}

func isStable(milestone int) bool {
	return milestone > 0 && milestone <= GetStable()
}

func isLTS(milestone int) bool {
	return milestone > 0 && milestone < GetTot()-3
}

// IsTargetedBranch checks to see if the given milestone is targeted by the
// passed in branch target list.
func IsTargetedBranch(milestone int, branches []suschpb.Branch) (bool, suschpb.Branch, error) {
	if len(branches) == 0 {
		return false, suschpb.Branch_BRANCH_UNSPECIFIED, fmt.Errorf("empty branch target list passed in to IsTargetedBranch")
	}

	for _, branch := range branches {
		isTargeted := false
		targetBranch := suschpb.Branch_BRANCH_UNSPECIFIED

		switch branch {
		case suschpb.Branch_CANARY:
			isTargeted = isCanary(milestone)
			targetBranch = suschpb.Branch_CANARY
		case suschpb.Branch_DEV:
			isTargeted = isDev(milestone)
			targetBranch = suschpb.Branch_DEV
		case suschpb.Branch_BETA:
			isTargeted = isBeta(milestone)
			targetBranch = suschpb.Branch_BETA
		case suschpb.Branch_STABLE:
			isTargeted = isStable(milestone)
			targetBranch = suschpb.Branch_STABLE
		case suschpb.Branch_LTS:
			isTargeted = isLTS(milestone)
			targetBranch = suschpb.Branch_LTS
		case suschpb.Branch_BRANCH_UNSPECIFIED:
			return false, targetBranch, fmt.Errorf("branch unspecified not supported")
		default:
			return false, targetBranch, fmt.Errorf("unknown branch enum value received")
		}

		if isTargeted {
			return true, targetBranch, nil
		}
	}

	return false, suschpb.Branch_BRANCH_UNSPECIFIED, nil
}

func BranchesToMilestones(branches []suschpb.Branch) ([]int, error) {
	if tot.ChromeBranch == 0 {
		return nil, fmt.Errorf("totManager not instantiated")
	}

	if len(branches) == 0 {
		return nil, fmt.Errorf("empty branch target list passed in to BranchesToMilestones")
	}

	milestones := []int{}
	for _, branch := range branches {
		switch branch {
		case suschpb.Branch_CANARY:
			milestones = append(milestones, GetTot())
		case suschpb.Branch_DEV:
			milestones = append(milestones, GetDev())
		case suschpb.Branch_BETA:
			milestones = append(milestones, GetBeta())
		case suschpb.Branch_STABLE:
			milestones = append(milestones, GetStable())
		case suschpb.Branch_BRANCH_UNSPECIFIED:
			return nil, fmt.Errorf("branch unspecified not supported")
		default:
			return nil, fmt.Errorf("unknown branch enum value received")
		}

	}

	return milestones, nil
}

// IdentifyBranch returns the branch which the given milestone maps to based on the current ToT.
func IdentifyBranch(milestone int) (suschpb.Branch, error) {
	switch {
	case isCanary(milestone):
		return suschpb.Branch_CANARY, nil
	case isDev(milestone):
		return suschpb.Branch_DEV, nil
	case isBeta(milestone):
		return suschpb.Branch_BETA, nil
	case isStable(milestone):
		return suschpb.Branch_STABLE, nil
	default:
		return suschpb.Branch_BRANCH_UNSPECIFIED, fmt.Errorf("branch unspecified not supported")
	}
}
