//go:build linux
// +build linux

// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/recovery/internal/components/mocks"
	"infra/cros/recovery/logger"
)

func getBaseTestRequest(installThroughServo bool) *InstallFirmwareImageRequest {
	return &InstallFirmwareImageRequest{
		Board:             "my-board",
		Model:             "my-model",
		FlashThroughServo: installThroughServo,
	}
}

func TestExtractECImage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	logger := logger.NewLogger()
	ctrl := gomock.NewController(t)
	tarballPath := "/some/folder/my_folder/tarbar.tr"
	ftt.Run("Happy path", t, func(t *ftt.Test) {
		req := getBaseTestRequest(true)
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Get(ctx, "ec_board").Return(stringValue("s-Board"), nil).Times(1)
		req.Servod = servod
		runRequest := map[string]string{
			"mkdir -p /some/folder/my_folder/EC": "",
			"tar tf /some/folder/my_folder/tarbar.tr s-board/ec.bin ./s-board/ec.bin my-model/ec.bin ./my-model/ec.bin my-board/ec.bin ./my-board/ec.bin ec.bin ./ec.bin": `ec.bin
my-board/ec.bin`,
			"tar xf /some/folder/my_folder/tarbar.tr -C /some/folder/my_folder/EC ec.bin": "",
		}
		req.ServoHostRunner = mockRunner(runRequest)
		image, fwBoard, err := extractECImage(ctx, req, tarballPath, logger)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, image, should.Equal("/some/folder/my_folder/EC/ec.bin"))
		assert.Loosely(t, fwBoard, should.BeEmpty)
	})
	ftt.Run("Happy path with board file", t, func(t *ftt.Test) {
		req := getBaseTestRequest(true)
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Get(ctx, "ec_board").Return(stringValue("s-Board"), nil).Times(1)
		req.Servod = servod
		runRequest := map[string]string{
			"mkdir -p /some/folder/my_folder/EC": "",
			"tar tf /some/folder/my_folder/tarbar.tr s-board/ec.bin ./s-board/ec.bin my-model/ec.bin ./my-model/ec.bin my-board/ec.bin ./my-board/ec.bin ec.bin ./ec.bin": `my-ec.bin
my-board/ec.bin`,
			"tar tf /some/folder/my_folder/tarbar.tr s-board/npcx_monitor.bin ./s-board/npcx_monitor.bin my-model/npcx_monitor.bin ./my-model/npcx_monitor.bin my-board/npcx_monitor.bin ./my-board/npcx_monitor.bin npcx_monitor.bin ./npcx_monitor.bin": ``,
			"tar xf /some/folder/my_folder/tarbar.tr -C /some/folder/my_folder/EC my-board/ec.bin": "",
		}
		req.ServoHostRunner = mockRunner(runRequest)
		image, fwBoard, err := extractECImage(ctx, req, tarballPath, logger)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, image, should.Equal("/some/folder/my_folder/EC/my-board/ec.bin"))
		assert.Loosely(t, fwBoard, should.Equal("my-board"))
	})
	ftt.Run("Happy path with board file with monitor", t, func(t *ftt.Test) {
		req := getBaseTestRequest(true)
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Get(ctx, "ec_board").Return(stringValue("s-Board"), nil).Times(1)
		req.Servod = servod
		runRequest := map[string]string{
			"mkdir -p /some/folder/my_folder/EC": "",
			"tar tf /some/folder/my_folder/tarbar.tr s-board/ec.bin ./s-board/ec.bin my-model/ec.bin ./my-model/ec.bin my-board/ec.bin ./my-board/ec.bin ec.bin ./ec.bin": `my-ec.bin
my-board/ec.bin
npcx_monitor.bin`,
			"tar xf /some/folder/my_folder/tarbar.tr -C /some/folder/my_folder/EC my-board/ec.bin":  "",
			"tar xf /some/folder/my_folder/tarbar.tr -C /some/folder/my_folder/EC npcx_monitor.bin": "",
		}
		req.ServoHostRunner = mockRunner(runRequest)
		image, fwBoard, err := extractECImage(ctx, req, tarballPath, logger)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, image, should.Equal("/some/folder/my_folder/EC/my-board/ec.bin"))
		assert.Loosely(t, fwBoard, should.Equal("my-board"))
	})
	ftt.Run("Happy path without servod", t, func(t *ftt.Test) {
		req := getBaseTestRequest(true)
		runRequest := map[string]string{
			"mkdir -p /some/folder/my_folder/EC": "",
			"tar tf /some/folder/my_folder/tarbar.tr my-model/ec.bin ./my-model/ec.bin my-board/ec.bin ./my-board/ec.bin ec.bin ./ec.bin": `my-ec.bin
my-board/ec.bin
npcx_monitor.bin`,
			"tar xf /some/folder/my_folder/tarbar.tr -C /some/folder/my_folder/EC my-board/ec.bin":  "",
			"tar xf /some/folder/my_folder/tarbar.tr -C /some/folder/my_folder/EC npcx_monitor.bin": "",
		}
		req.ServoHostRunner = mockRunner(runRequest)
		image, fwBoard, err := extractECImage(ctx, req, tarballPath, logger)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, image, should.Equal("/some/folder/my_folder/EC/my-board/ec.bin"))
		assert.Loosely(t, fwBoard, should.Equal("my-board"))
	})
	ftt.Run("Happy path run from DUT", t, func(t *ftt.Test) {
		req := getBaseTestRequest(false)
		runRequest := map[string]string{
			"mkdir -p /some/folder/my_folder/EC": "",
			"tar tf /some/folder/my_folder/tarbar.tr my-model/ec.bin ./my-model/ec.bin my-board/ec.bin ./my-board/ec.bin ec.bin ./ec.bin": `my-ec.bin
my-board/ec.bin
npcx_monitor.bin`,
			"tar xf /some/folder/my_folder/tarbar.tr -C /some/folder/my_folder/EC my-board/ec.bin":  "",
			"tar xf /some/folder/my_folder/tarbar.tr -C /some/folder/my_folder/EC npcx_monitor.bin": "",
		}
		req.DutRunner = mockRunner(runRequest)
		image, fwBoard, err := extractECImage(ctx, req, tarballPath, logger)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, image, should.Equal("/some/folder/my_folder/EC/my-board/ec.bin"))
		assert.Loosely(t, fwBoard, should.Equal("my-board"))
	})
}

func TestExtractAPImage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	logger := logger.NewLogger()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tarballPath := "/some/folder/my_folder/tarbar2.tr"
	ftt.Run("Happy path", t, func(t *ftt.Test) {
		req := getBaseTestRequest(true)
		runRequest := map[string]string{
			"mkdir -p /some/folder/my_folder/AP": "",
			"tar tf /some/folder/my_folder/tarbar2.tr image-s-board.bin ./image-s-board.bin image-my-model.bin ./image-my-model.bin image-my-board.bin ./image-my-board.bin image.bin ./image.bin": `image.bin
image-my-model.bin`,
			"tar xf /some/folder/my_folder/tarbar2.tr -C /some/folder/my_folder/AP image.bin": "",
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Get(ctx, "ec_board").Return(stringValue("s-Board"), nil).Times(1)
		req.Servod = servod
		req.ServoHostRunner = mockRunner(runRequest)
		image, err := extractAPImage(ctx, req, tarballPath, logger)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, image, should.Equal("/some/folder/my_folder/AP/image.bin"))
	})
	ftt.Run("Happy path with board file", t, func(t *ftt.Test) {
		req := getBaseTestRequest(true)
		runRequest := map[string]string{
			"mkdir -p /some/folder/my_folder/AP": "",
			"tar tf /some/folder/my_folder/tarbar2.tr image-s-board.bin ./image-s-board.bin image-my-model.bin ./image-my-model.bin image-my-board.bin ./image-my-board.bin image.bin ./image.bin": `image-my.bin
image-my-model.bin`,
			"tar xf /some/folder/my_folder/tarbar2.tr -C /some/folder/my_folder/AP image-my-model.bin": "",
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Get(ctx, "ec_board").Return(stringValue("S-board"), nil).Times(1)
		req.Servod = servod
		req.ServoHostRunner = mockRunner(runRequest)
		image, err := extractAPImage(ctx, req, tarballPath, logger)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, image, should.Equal("/some/folder/my_folder/AP/image-my-model.bin"))
	})
	ftt.Run("Happy path with serial", t, func(t *ftt.Test) {
		req := getBaseTestRequest(true)
		req.UseSerialTargets = true
		runRequest := map[string]string{
			"mkdir -p /some/folder/my_folder/AP": "",
			"tar tf /some/folder/my_folder/tarbar2.tr image-s-board.serial.bin ./image-s-board.serial.bin image-my-model.serial.bin ./image-my-model.serial.bin image-my-board.serial.bin ./image-my-board.serial.bin image.serial.bin ./image.serial.bin": `image-my.serial.bin
image-my-model.serial.bin`,
			"tar xf /some/folder/my_folder/tarbar2.tr -C /some/folder/my_folder/AP image-my-model.serial.bin": "",
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Get(ctx, "ec_board").Return(stringValue("S-board"), nil).Times(1)
		req.Servod = servod
		req.ServoHostRunner = mockRunner(runRequest)
		image, err := extractAPImage(ctx, req, tarballPath, logger)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, image, should.Equal("/some/folder/my_folder/AP/image-my-model.serial.bin"))
	})
	ftt.Run("Happy path without servod", t, func(t *ftt.Test) {
		req := getBaseTestRequest(true)
		runRequest := map[string]string{
			"mkdir -p /some/folder/my_folder/AP": "",
			"tar tf /some/folder/my_folder/tarbar2.tr image-my-model.bin ./image-my-model.bin image-my-board.bin ./image-my-board.bin image.bin ./image.bin": `image-my.bin
image-my-model.bin`,
			"tar xf /some/folder/my_folder/tarbar2.tr -C /some/folder/my_folder/AP image-my-model.bin": "",
		}
		req.ServoHostRunner = mockRunner(runRequest)
		image, err := extractAPImage(ctx, req, tarballPath, logger)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, image, should.Equal("/some/folder/my_folder/AP/image-my-model.bin"))
	})
	ftt.Run("Happy path run from DUT", t, func(t *ftt.Test) {
		req := getBaseTestRequest(false)
		runRequest := map[string]string{
			"mkdir -p /some/folder/my_folder/AP": "",
			"tar tf /some/folder/my_folder/tarbar2.tr image-my-model.bin ./image-my-model.bin image-my-board.bin ./image-my-board.bin image.bin ./image.bin": `image-my.bin
image-my-model.bin`,
			"tar xf /some/folder/my_folder/tarbar2.tr -C /some/folder/my_folder/AP image-my-model.bin": "",
		}
		req.DutRunner = mockRunner(runRequest)
		image, err := extractAPImage(ctx, req, tarballPath, logger)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, image, should.Equal("/some/folder/my_folder/AP/image-my-model.bin"))
	})
}

// Test cases for TestBoardFromCandidateName
var boardFromCandidateNameCases = []struct {
	in  string
	out string
}{
	{"", ""},
	{"./reef/ec.bin", "reef"},
	{"/reef/ec.bin2", "reef"},
	{"reef/ap.bin5", "reef"},
	{"basking/ec.bin", "basking"},
	{"./basking/ec.bin", "basking"},
}

func TestBoardFromCandidateName(t *testing.T) {
	t.Parallel()
	for _, c := range boardFromCandidateNameCases {
		cs := c
		t.Run(cs.in, func(t *testing.T) {
			got := boardFromCandidateName(cs.in)

			if !cmp.Equal(got, cs.out) {
				t.Errorf("%q ->want: %q\n got: %q", cs.in, cs.out, got)
			}
		})
	}
}
