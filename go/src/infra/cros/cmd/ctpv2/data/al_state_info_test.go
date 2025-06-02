// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"

	androidapi "go.chromium.org/infra/cros/cmd/common_lib/android_api"
	mock_androidapi "go.chromium.org/infra/cros/cmd/common_lib/android_api/mocks"
	"go.chromium.org/infra/cros/cmd/common_lib/ants/androidbuildinternal/v3"
)

func TestSealInvocation(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()
	mockWU := mock_androidapi.NewMockWorkUnitService(mockCtl)
	mockInv := mock_androidapi.NewMockInvocationService(mockCtl)

	inv := &androidbuildinternal.Invocation{
		InvocationId:   "I123",
		SchedulerState: "running",
	}

	wu := &androidbuildinternal.WorkUnit{
		Id:    "WU456",
		Name:  "Test WU",
		State: "running",
	}
	headWU := &androidbuildinternal.WorkUnit{
		Id:    "WU123",
		State: "running",
		Name:  "Head WU",
	}
	a := &AlStateInfo{
		ATPWorkUnit:   wu,
		ATPInvocation: inv,
	}

	ctx := context.Background()
	tree := &androidapi.WorkUnitTree{
		Head: &androidapi.WorkUnitNode{},
	}
	tree.Head.SetWorkUnit(headWU)
	s := &androidapi.Service{
		WorkUnitService:   mockWU,
		InvocationService: mockInv,
	}

	mockWU.EXPECT().Get(wu.Id).Return(wu, nil)
	mockWU.EXPECT().Update(wu.Id, wu).Return(wu, nil)
	mockInv.EXPECT().Get(ctx, inv.InvocationId).Return(inv, nil)
	mockInv.EXPECT().Update(inv.InvocationId, inv).Return(inv, nil)

	err := a.sealInvocation(ctx, tree, s)
	if err != nil {
		t.Errorf("Unexpected err: %v", err)
	}
}

func TestCloseWUTree(t *testing.T) {
	ctx := context.Background()
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()
	mockWU := mock_androidapi.NewMockWorkUnitService(mockCtl)
	mockInv := mock_androidapi.NewMockInvocationService(mockCtl)
	s := &androidapi.Service{
		InvocationService: mockInv,
		WorkUnitService:   mockWU,
	}

	testCases := []struct {
		name   string
		wuErr  error
		invErr error
	}{
		{
			name: "success",
		},
		{
			name:   "multi-err",
			wuErr:  errors.New("wuErr"),
			invErr: errors.New("invErr"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inv := &androidbuildinternal.Invocation{
				InvocationId:   "I123",
				SchedulerState: "running",
			}
			wu := &androidbuildinternal.WorkUnit{
				Id:           "WU456",
				Name:         "Test WU",
				State:        "running",
				InvocationId: "I123",
			}
			wuTree := &androidapi.WorkUnitTree{Head: &androidapi.WorkUnitNode{Service: mockWU}}
			wuTree.Head.SetWorkUnit(wu)
			a := &AlStateInfo{
				WorkUnitTrees: map[string]*androidapi.WorkUnitTree{
					"test": wuTree,
				},
				ATPWorkUnit:   wu,
				ATPInvocation: inv,
			}

			if tc.invErr != nil {
				mockInv.EXPECT().Get(ctx, inv.InvocationId).Return(inv, nil)
			} else {
				mockInv.EXPECT().Get(ctx, inv.InvocationId).Return(inv, nil).Times(2)
				mockInv.EXPECT().Update(inv.InvocationId, gomock.Any()).Return(inv, nil)
			}

			mockWU.EXPECT().Get(gomock.Any()).Return(wu, nil).Times(2)
			mockWU.EXPECT().Update(wu.Id, wu).Return(wu, tc.wuErr)
			mockWU.EXPECT().Update(wu.Id, wu).Return(wu, tc.invErr)

			err := a.CloseWUTree(ctx, s, "error", "CloseWUerr")
			if tc.invErr != nil && !errors.Is(err, tc.invErr) {
				t.Errorf("Unexpected invocation error: %v", err)
			}

			if tc.wuErr != nil && !errors.Is(err, tc.wuErr) {
				t.Errorf("Unexpected workunit error: %v", err)
			}
		})
	}

}
