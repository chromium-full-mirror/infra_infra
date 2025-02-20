// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/components/servo"
	"go.chromium.org/infra/cros/recovery/logger"
)

// Programmer represent interface to flash EC/AP to the ChromeOS devices by servo.
type Programmer interface {
	// Prepare programmer for actions.
	Prepare(ctx context.Context) error
	// ProgramEC programs EC firmware to devices by servo.
	ProgramEC(ctx context.Context, fwBoard, imagePath string) error
	// ProgramAP programs AP firmware to devices by servo.
	ProgramAP(ctx context.Context, imagePath, gbbHex string, force bool) error
	// Close closes programming resources.
	Close(ctx context.Context) error
}

// NewProgrammer creates programmer to flash device firmware by servo.
func NewProgrammer(ctx context.Context, run components.Runner, servod components.Servod, log logger.Logger) (Programmer, error) {
	if run == nil {
		return nil, errors.Reason("new programmer: runner is not provided").Err()
	} else if servod == nil {
		return nil, errors.Reason("new programmer: servod is not provided").Err()
	} else if log == nil {
		return nil, errors.Reason("new programmer: logger is not provided").Err()
	}
	var st *servo.ServoType
	if stv, err := servod.Get(ctx, "servo_type"); err != nil {
		return nil, errors.Annotate(err, "new programmer").Err()
	} else {
		st = servo.NewServoType(stv.GetString_())
	}
	if st.IsV3() || st.IsV4() {
		p := &v3Programmer{
			st:     st,
			run:    run,
			servod: servod,
			log:    log,
		}
		return p, nil
	}
	return nil, errors.Reason("new programmer: servo-type %s not supported", st).Err()
}
