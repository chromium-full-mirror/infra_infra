// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"testing"

	"infra/libs/skylab/inventory"
)

func TestCreateLabels(t *testing.T) {
	testBoard := "board1"
	testModel := "model3"
	testPool := "swimming"
	hwTarget := &HwTarget{board: testBoard, model: testModel}
	trHelper := &TrV2ReqHelper{
		pool:          testPool,
		primaryTarget: hwTarget,
	}
	foo, err := createLabels(trHelper)
	if err != nil {
		t.Fatalf("error building labels")
	}
	if foo.GetBoard() != testBoard {
		t.Fatalf("incorrect test board. got %s, expected %s", foo.GetBoard(), testBoard)
	}
	if foo.GetModel() != testModel {
		t.Fatalf("incorrect test model. got %s, expected %s", foo.GetModel(), testModel)
	}
	if foo.GetSelfServePools() == nil {
		t.Fatalf("incorrect pool; got nil when should have had value")
	}
	if len(foo.GetSelfServePools()) != 1 {
		t.Fatalf("incorrect pool length. Expected 1, got %v", len(foo.GetSelfServePools()))
	}
	if foo.GetSelfServePools()[0] != testPool {
		t.Fatalf("incorrect pool. got %s, expected %s", foo.GetSelfServePools()[0], testPool)
	}

	// Test no pool
	trHelper.pool = ""
	foo, err = createLabels(trHelper)
	if err != nil {
		t.Fatalf("error building labels")
	}
	if foo.GetCriticalPools() == nil {
		t.Fatalf("incorrect pool; got nil when should have had value")
	}
	if len(foo.GetCriticalPools()) != 1 {
		t.Fatalf("incorrect pool length. Expected 1, got %v", len(foo.GetSelfServePools()))
	}
	if foo.GetCriticalPools()[0] != inventory.SchedulableLabels_DUT_POOL_QUOTA {
		t.Fatalf("incorrect pool. got %s, expected %s", foo.GetSelfServePools()[0], inventory.SchedulableLabels_DUT_POOL_QUOTA)
	}

	// Test setting quota.
	trHelper.pool = DutPoolQuota
	foo, err = createLabels(trHelper)
	if err != nil {
		t.Fatalf("error building labels")
	}
	if foo.GetCriticalPools() == nil {
		t.Fatalf("incorrect pool; got nil when should have had value")
	}
	if len(foo.GetCriticalPools()) != 1 {
		t.Fatalf("incorrect pool length. Expected 1, got %v", len(foo.GetSelfServePools()))
	}
	if foo.GetCriticalPools()[0] != inventory.SchedulableLabels_DUT_POOL_QUOTA {
		t.Fatalf("incorrect pool. got %s, expected %s", foo.GetSelfServePools()[0], inventory.SchedulableLabels_DUT_POOL_QUOTA)
	}

}
