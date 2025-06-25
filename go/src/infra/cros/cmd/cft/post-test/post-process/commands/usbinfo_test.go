// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"io"
	"log"
	"testing"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/artifact"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

const UsbInfoFile = "test_data/usb_info.json"

func TestUsbInfo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	test := "firmware.ECPDCCD.normal"
	emptyLogger := log.New(io.Discard, "", 0)

	ftt.Run(`usbInfos works`, t, func(t *ftt.Test) {
		usbFiles := map[string]string{
			test: UsbInfoFile,
		}
		got, _ := usbInfos(ctx, emptyLogger, usbFiles)
		want, _ := anypb.New(&artifact.DutInfo_UsbInfo{
			PowerDeliveryPortServo: "0",
			PowerDeliveryPortCount: 2,
		})

		assert.Loosely(t, got, should.Match(want))
	})

	ftt.Run(`Return nil if no USB info file is found`, t, func(t *ftt.Test) {
		usbFiles := map[string]string{
			test: "",
		}
		got, _ := usbInfos(ctx, emptyLogger, usbFiles)
		want := (*anypb.Any)(nil)

		assert.Loosely(t, got, should.Match(want))
	})
}
