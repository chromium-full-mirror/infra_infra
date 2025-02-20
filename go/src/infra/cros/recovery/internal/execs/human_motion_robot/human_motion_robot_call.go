// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package human_motion_robot

import (
	"context"

	"go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// Call calls XMLRPC on touchhost.
func Call(ctx context.Context, in tlw.Access, host *tlw.HumanMotionRobot, method string) (*xmlrpc.Value, error) {
	if method == "" {
		return nil, errors.Reason("HMR TouchHost call: method name is empty").Err()
	}
	res := in.CallTouchHostd(ctx, &tlw.CallTouchHostdRequest{
		Resource: host.GetTouchhost(),
		Method:   method,
	})
	log.Debugf(ctx, "HMR TouchHost call %q with hostname %q: received %q", method, host.GetTouchhost(), res.GetValue().GetString_())
	if res.GetFault() {
		return nil, errors.Reason("unable to make HMR TouchHost call %q with hostname %q: %q", method, host.GetTouchhost(), res.GetValue().GetString_()).Err()
	}
	return res.GetValue(), nil
}
