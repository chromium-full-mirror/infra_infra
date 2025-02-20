// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ctr

import (
	"go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/recovery/tlw"
)

func toLabDut(d *tlw.Dut, cacheAddr *api.IpEndpoint) *api.Dut {
	if d == nil {
		return nil
	}
	chromeos := &api.Dut_ChromeOS{
		Ssh: &api.IpEndpoint{
			Address: d.Name,
			Port:    22,
		},
		DutModel: &api.DutModel{
			BuildTarget: d.GetChromeos().GetBoard(),
			ModelName:   d.GetChromeos().GetModel(),
		},
		Servo: &api.Servo{},
	}
	if sh := d.GetChromeos().GetServo(); sh != nil {
		chromeos.Servo.Present = true
		chromeos.Servo.Serial = sh.GetSerialNumber()
		chromeos.Servo.ServodAddress = &api.IpEndpoint{
			Address: sh.GetName(),
			Port:    sh.GetServodPort(),
		}
		if n := sh.GetContainerName(); n != "" {
			chromeos.Servo.ServodAddress.Address = n
		}
	}
	return &api.Dut{
		Id: &api.Dut_Id{
			Value: d.Name,
		},
		DutType: &api.Dut_Chromeos{
			Chromeos: chromeos,
		},
		CacheServer: &api.CacheServer{
			Address: cacheAddr,
		},
	}
}
