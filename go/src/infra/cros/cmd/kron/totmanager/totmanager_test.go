// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package totmanager

import (
	"testing"

	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"

	"go.chromium.org/infra/cros/internal/chromeosversion"
)

func TestInitTotManager(t *testing.T) {
	err := InitTotManager()
	if err != nil {
		t.Error(err)
	}

	if tot.ChromeBranch == 0 {
		t.Errorf("tot not properly fetched")
	}
}

func TestIsTargetedBranch(t *testing.T) {
	tot = chromeosversion.VersionInfo{
		ChromeBranch: 100,
	}

	totTest, branch, err := IsTargetedBranch(100, []suschpb.Branch{suschpb.Branch_CANARY})
	if err != nil {
		t.Error(err)
		return
	}
	if !totTest {
		t.Errorf("expected %t got %t for TotTest", true, totTest)
		return
	}
	if branch != suschpb.Branch_CANARY {
		t.Errorf("expected %s got %s", suschpb.Branch_name[int32(suschpb.Branch_CANARY)], suschpb.Branch_name[int32(branch)])
	}

	devTest, branch, err := IsTargetedBranch(99, []suschpb.Branch{suschpb.Branch_DEV})
	if err != nil {
		t.Error(err)
		return
	}
	if !devTest {
		t.Errorf("expected %t got %t for devTest", true, devTest)
		return
	}
	if branch != suschpb.Branch_DEV {
		t.Errorf("expected %s got %s", suschpb.Branch_name[int32(suschpb.Branch_DEV)], suschpb.Branch_name[int32(branch)])
	}

	betaTest, branch, err := IsTargetedBranch(98, []suschpb.Branch{suschpb.Branch_BETA})
	if err != nil {
		t.Error(err)
		return
	}
	if !betaTest {
		t.Errorf("expected %t got %t for betaTest", true, betaTest)
		return
	}
	if branch != suschpb.Branch_BETA {
		t.Errorf("expected %s got %s", suschpb.Branch_name[int32(suschpb.Branch_BETA)], suschpb.Branch_name[int32(branch)])
	}

	stableTest, branch, err := IsTargetedBranch(97, []suschpb.Branch{suschpb.Branch_STABLE})
	if err != nil {
		t.Error(err)
		return
	}
	if !stableTest {
		t.Errorf("expected %t got %t for stableTest", true, stableTest)
		return
	}
	if branch != suschpb.Branch_STABLE {
		t.Errorf("expected %s got %s", suschpb.Branch_name[int32(suschpb.Branch_STABLE)], suschpb.Branch_name[int32(branch)])
	}

	ltsTest, branch, err := IsTargetedBranch(94, []suschpb.Branch{suschpb.Branch_STABLE})
	if err != nil {
		t.Error(err)
		return
	}
	if !ltsTest {
		t.Errorf("expected %t got %t for stableTest", true, stableTest)
		return
	}
	if branch != suschpb.Branch_STABLE {
		t.Errorf("expected %s got %s", suschpb.Branch_name[int32(suschpb.Branch_STABLE)], suschpb.Branch_name[int32(branch)])
	}

	multiTest1, branch, err := IsTargetedBranch(100, []suschpb.Branch{suschpb.Branch_CANARY, suschpb.Branch_STABLE})
	if err != nil {
		t.Error(err)
		return
	}
	if !multiTest1 {
		t.Errorf("expected %t got %t for multiTest1", true, multiTest1)
		return
	}
	if branch != suschpb.Branch_CANARY {
		t.Errorf("expected %s got %s", suschpb.Branch_name[int32(suschpb.Branch_CANARY)], suschpb.Branch_name[int32(branch)])
	}

	multiTest2, branch, err := IsTargetedBranch(97, []suschpb.Branch{suschpb.Branch_CANARY, suschpb.Branch_STABLE})
	if err != nil {
		t.Error(err)
		return
	}
	if !multiTest2 {
		t.Errorf("expected %t got %t for multiTest2", true, multiTest2)
		return
	}
	if branch != suschpb.Branch_STABLE {
		t.Errorf("expected %s got %s", suschpb.Branch_name[int32(suschpb.Branch_STABLE)], suschpb.Branch_name[int32(branch)])
	}

	multiTest3, branch, err := IsTargetedBranch(98, []suschpb.Branch{suschpb.Branch_CANARY, suschpb.Branch_STABLE})
	if err != nil {
		t.Error(err)
		return
	}
	if multiTest3 {
		t.Errorf("expected %t got %t for multiTest3", false, multiTest3)
		return
	}
	if branch != suschpb.Branch_BRANCH_UNSPECIFIED {
		t.Errorf("expected %s got %s", suschpb.Branch_name[int32(suschpb.Branch_STABLE)], suschpb.Branch_name[int32(branch)])
	}

}
