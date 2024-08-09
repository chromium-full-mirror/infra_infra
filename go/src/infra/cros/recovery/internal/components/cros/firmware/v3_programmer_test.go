// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/recovery/internal/components/mocks"
	"infra/cros/recovery/logger"
)

func TestProgrammerV3ProgramEC(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	logger := logger.NewLogger()
	imagePath := "ec_image.bin"
	fwBoard := ""
	ftt.Run("Happy path for stm32 chip", t, func(t *ftt.Test) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		runRequest := map[string]RunResponse{
			"which flash_ec": {},
			"flash_ec --chip=stm32 --image=ec_image.bin --port=95 --bitbang_rate=57600 --verify --verbose": {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Has(ctx, "cpu_fw_spi").Return(nil).Times(1)
		servod.EXPECT().Has(ctx, "ccd_cpu_fw_spi").Return(nil).Times(1)
		servod.EXPECT().Get(ctx, "cpu_fw_spi_depends_on_ec_fw").Return(stringValue("no"), nil).Times(1)
		servod.EXPECT().Get(ctx, "ccd_cpu_fw_spi_depends_on_ec_fw").Return(stringValue("no"), nil).Times(1)
		servod.EXPECT().Get(ctx, "ec_chip").Return(stringValue("stm32"), nil).Times(1)
		servod.EXPECT().Get(ctx, "servo_type").Return(stringValue("servo_v4_with_servo_micro_and_ccd_cr50"), nil).Times(1)
		servod.EXPECT().Port().Return(95).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programEC(ctx, fwBoard, imagePath)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
	ftt.Run("Happy path for other chips", t, func(t *ftt.Test) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		runRequest := map[string]RunResponse{
			"which flash_ec": {},
			"flash_ec --chip=some_chip --image=ec_image.bin --port=96 --verify --verbose": {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Has(ctx, "cpu_fw_spi").Return(nil).Times(1)
		servod.EXPECT().Has(ctx, "ccd_cpu_fw_spi").Return(nil).Times(1)
		servod.EXPECT().Get(ctx, "cpu_fw_spi_depends_on_ec_fw").Return(stringValue("no"), nil).Times(1)
		servod.EXPECT().Get(ctx, "ccd_cpu_fw_spi_depends_on_ec_fw").Return(stringValue("no"), nil).Times(1)
		servod.EXPECT().Get(ctx, "ec_chip").Return(stringValue("some_chip"), nil).Times(1)
		servod.EXPECT().Get(ctx, "servo_type").Return(stringValue("servo_v4_with_servo_micro_and_ccd_cr50"), nil).Times(1)
		servod.EXPECT().Port().Return(96).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programEC(ctx, fwBoard, imagePath)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
	ftt.Run("Happy path for ite chip on ccd", t, func(t *ftt.Test) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		runRequest := map[string]RunResponse{
			"which flash_ec": {},
			"flash_ec --chip=it8XXXX --image=ec_image.bin --port=95 --verify --verbose --nouse_i2c_pseudo": {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Has(ctx, "cpu_fw_spi").Return(nil).Times(1)
		servod.EXPECT().Has(ctx, "ccd_cpu_fw_spi").Return(nil).Times(1)
		servod.EXPECT().Get(ctx, "cpu_fw_spi_depends_on_ec_fw").Return(stringValue("no"), nil).Times(1)
		servod.EXPECT().Get(ctx, "ccd_cpu_fw_spi_depends_on_ec_fw").Return(stringValue("no"), nil).Times(1)
		servod.EXPECT().Get(ctx, "ec_chip").Return(stringValue("it8XXXX"), nil).Times(1)
		servod.EXPECT().Get(ctx, "servo_type").Return(stringValue("servo_v4_with_ccd_cr50"), nil).Times(1)
		servod.EXPECT().Port().Return(95).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programEC(ctx, fwBoard, imagePath)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
	ftt.Run("Happy path for ite chip on servo_micro", t, func(t *ftt.Test) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		runRequest := map[string]RunResponse{
			"which flash_ec": {},
			"flash_ec --chip=it8yyyyy --image=ec_image.bin --port=96 --verify --verbose": {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Has(ctx, "cpu_fw_spi").Return(nil).Times(1)
		servod.EXPECT().Has(ctx, "ccd_cpu_fw_spi").Return(nil).Times(1)
		servod.EXPECT().Get(ctx, "cpu_fw_spi_depends_on_ec_fw").Return(stringValue("no"), nil).Times(1)
		servod.EXPECT().Get(ctx, "ccd_cpu_fw_spi_depends_on_ec_fw").Return(stringValue("no"), nil).Times(1)
		servod.EXPECT().Get(ctx, "ec_chip").Return(stringValue("it8yyyyy"), nil).Times(1)
		servod.EXPECT().Get(ctx, "servo_type").Return(stringValue("servo_v4_with_servo_micro_and_ccd_cr50"), nil).Times(1)
		servod.EXPECT().Port().Return(96).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programEC(ctx, fwBoard, imagePath)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
	ftt.Run("use try_apshutdown is expected for ccd_cpu_fw_spi_depends_on_ec_fw:yes (1)", t, func(t *ftt.Test) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		runRequest := map[string]RunResponse{
			"which flash_ec": {},
			"flash_ec --chip=just_chip --image=ec_image.bin --port=96 --verify --verbose --try_apshutdown": {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Has(ctx, "cpu_fw_spi").Return(nil).Times(1)
		servod.EXPECT().Has(ctx, "ccd_cpu_fw_spi").Return(nil).Times(1)
		servod.EXPECT().Get(ctx, "cpu_fw_spi_depends_on_ec_fw").Return(stringValue("no"), nil).Times(1)
		servod.EXPECT().Get(ctx, "ccd_cpu_fw_spi_depends_on_ec_fw").Return(stringValue("yes"), nil).Times(1)
		servod.EXPECT().Get(ctx, "ec_chip").Return(stringValue("just_chip"), nil).Times(1)
		servod.EXPECT().Get(ctx, "servo_type").Return(stringValue("servo_v4_with_servo_micro_and_ccd_cr50"), nil).Times(1)
		servod.EXPECT().Port().Return(96).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programEC(ctx, fwBoard, imagePath)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
	ftt.Run("use try_apshutdown is expected for cpu_fw_spi_depends_on_ec_fw:yes (2)", t, func(t *ftt.Test) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		runRequest := map[string]RunResponse{
			"which flash_ec": {},
			"flash_ec --chip=just_chip --image=ec_image.bin --port=96 --verify --verbose --try_apshutdown": {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Has(ctx, "cpu_fw_spi").Return(nil).Times(1)
		servod.EXPECT().Get(ctx, "cpu_fw_spi_depends_on_ec_fw").Return(stringValue("yes"), nil).Times(1)
		// not called as above already told yes.
		servod.EXPECT().Has(ctx, "ccd_cpu_fw_spi").Return(nil).Times(0)
		servod.EXPECT().Get(ctx, "ccd_cpu_fw_spi_depends_on_ec_fw").Return(stringValue("no"), nil).Times(0)
		servod.EXPECT().Get(ctx, "ec_chip").Return(stringValue("just_chip"), nil).Times(1)
		servod.EXPECT().Get(ctx, "servo_type").Return(stringValue("servo_v4_with_servo_micro_and_ccd_cr50"), nil).Times(1)
		servod.EXPECT().Port().Return(96).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programEC(ctx, fwBoard, imagePath)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
	ftt.Run("do not use try_apshutdown if controls are not present", t, func(t *ftt.Test) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		runRequest := map[string]RunResponse{
			"which flash_ec": {},
			"flash_ec --chip=just_chip --image=ec_image.bin --port=96 --verify --verbose": {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Has(ctx, "cpu_fw_spi").Return(errors.Reason("Not present").Err()).Times(1)
		servod.EXPECT().Has(ctx, "ccd_cpu_fw_spi").Return(errors.Reason("Not present").Err()).Times(1)
		servod.EXPECT().Get(ctx, "cpu_fw_spi_depends_on_ec_fw").Return(stringValue("yes"), nil).Times(0)
		servod.EXPECT().Get(ctx, "ccd_cpu_fw_spi_depends_on_ec_fw").Return(stringValue("no"), nil).Times(0)
		servod.EXPECT().Get(ctx, "ec_chip").Return(stringValue("just_chip"), nil).Times(1)
		servod.EXPECT().Get(ctx, "servo_type").Return(stringValue("servo_v4_with_servo_micro_and_ccd_cr50"), nil).Times(1)
		servod.EXPECT().Port().Return(96).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programEC(ctx, fwBoard, imagePath)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
	ftt.Run("set try_apshutdown and board when fun flash", t, func(t *ftt.Test) {
		fwBoard = "reef"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		runRequest := map[string]RunResponse{
			"which flash_ec": {},
			"flash_ec --chip=just_chip --image=ec_image.bin --port=96 --verify --verbose --board=reef --try_apshutdown": {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Has(ctx, "cpu_fw_spi").Return(nil).Times(1)
		servod.EXPECT().Get(ctx, "cpu_fw_spi_depends_on_ec_fw").Return(stringValue("yes"), nil).Times(1)
		// not called as above already told yes.
		servod.EXPECT().Has(ctx, "ccd_cpu_fw_spi").Return(nil).Times(0)
		servod.EXPECT().Get(ctx, "ccd_cpu_fw_spi_depends_on_ec_fw").Return(stringValue("no"), nil).Times(0)
		servod.EXPECT().Get(ctx, "ec_chip").Return(stringValue("just_chip"), nil).Times(1)
		servod.EXPECT().Get(ctx, "servo_type").Return(stringValue("servo_v4_with_servo_micro_and_ccd_cr50"), nil).Times(1)
		servod.EXPECT().Port().Return(96).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programEC(ctx, fwBoard, imagePath)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
}

func TestProgrammerV3ProgramAP(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	logger := logger.NewLogger()
	imagePath := "image-board.bin"
	ftt.Run("Happy path", t, func(t *ftt.Test) {
		runRequest := map[string]RunResponse{
			"which futility": {},
			"futility read --emulate='image-board.bin' -r SI_ME /dev/null || futility read --emulate='image-board.bin' -r SI_DESC /dev/null": {Err: errors.Reason("error code 1").Err()},
			"futility update -i image-board.bin --servo_port=97":                                                                             {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Port().Return(97).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programAP(ctx, imagePath, "", false)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
	ftt.Run("Happy path with CSME", t, func(t *ftt.Test) {
		runRequest := map[string]RunResponse{
			"which futility": {},
			"futility read --emulate='image-board.bin' -r SI_ME /dev/null || futility read --emulate='image-board.bin' -r SI_DESC /dev/null": {},
			"futility update -i image-board.bin --servo_port=97 --quirks unlock_csme":                                                        {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Port().Return(97).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programAP(ctx, imagePath, "", false)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
	ftt.Run("Happy path with GBB 0x18", t, func(t *ftt.Test) {
		runRequest := map[string]RunResponse{
			"which futility": {},
			"futility read --emulate='image-board.bin' -r SI_ME /dev/null || futility read --emulate='image-board.bin' -r SI_DESC /dev/null": {Err: errors.Reason("error code 1").Err()},
			"futility update -i image-board.bin --servo_port=91 --gbb_flags=0x18":                                                            {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Port().Return(91).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programAP(ctx, imagePath, "0x18", false)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
	ftt.Run("Happy path with GBB 0x18 and CSME", t, func(t *ftt.Test) {
		runRequest := map[string]RunResponse{
			"which futility": {},
			"futility read --emulate='image-board.bin' -r SI_ME /dev/null || futility read --emulate='image-board.bin' -r SI_DESC /dev/null": {},
			"futility update -i image-board.bin --servo_port=91 --gbb_flags=0x18 --quirks unlock_csme":                                       {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Port().Return(91).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programAP(ctx, imagePath, "0x18", false)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
	ftt.Run("Happy path with force update", t, func(t *ftt.Test) {
		runRequest := map[string]RunResponse{
			"which futility": {},
			"futility read --emulate='image-board.bin' -r SI_ME /dev/null || futility read --emulate='image-board.bin' -r SI_DESC /dev/null": {Err: errors.Reason("error code 1").Err()},
			"futility update -i image-board.bin --servo_port=97 --force":                                                                     {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Port().Return(97).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programAP(ctx, imagePath, "", true)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
	ftt.Run("Happy path with force update and CSME", t, func(t *ftt.Test) {
		runRequest := map[string]RunResponse{
			"which futility": {},
			"futility read --emulate='image-board.bin' -r SI_ME /dev/null || futility read --emulate='image-board.bin' -r SI_DESC /dev/null": {},
			"futility update -i image-board.bin --servo_port=97 --force --quirks unlock_csme":                                                {},
		}
		servod := mocks.NewMockServod(ctrl)
		servod.EXPECT().Port().Return(97).Times(1)
		run, runCounter := mockRunnerWithCheck(runRequest)
		p := &v3Programmer{
			run:    run,
			servod: servod,
			log:    logger,
		}

		err := p.programAP(ctx, imagePath, "", true)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, runCounter(), should.Equal(len(runRequest)))
	})
}

var gbbToIntCases = []struct {
	name string
	in   string
	out  int
	fail bool
}{
	{"Empty value", "", 0, true},
	{"Incorrect value", "raw", 0, true},
	{"GBB 0", "0", 0, false},
	{"GBB 0x0", "0x0", 0, false},
	{"GBB 0x1", "0x1", 1, false},
	{"GBB 0x8", "0x8", 8, false},
	{"GBB 0x18", "0x18", 24, false},
	{"GBB 0x24", "0x24", 36, false},
	{"GBB 0x39", "0x39", 57, false},
	{"GBB 0x00000039", "0x00000039", 57, false},
}

func TestGbbToInt(t *testing.T) {
	t.Parallel()
	for _, c := range gbbToIntCases {
		cs := c
		t.Run(cs.name, func(t *testing.T) {
			got, err := gbbToInt(cs.in)
			if cs.fail {
				if err == nil {
					t.Errorf("%q -> expected to fail but passed", cs.name)
				}
			} else {
				if err != nil {
					t.Errorf("%q -> expected to pass by fail %s", cs.name, err)
				} else if got != cs.out {
					t.Errorf("%q -> wanted: %v => got: %v", cs.name, cs.out, got)
				}
			}
		})
	}
}
