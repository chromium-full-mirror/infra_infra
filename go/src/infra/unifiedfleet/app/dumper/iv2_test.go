// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
)

func TestCompare(t *testing.T) {
	t.Parallel()

	t1, _ := time.Parse(time.RFC1123Z, "Mon, 12 Jan 2019 15:04:05 -0800")
	tp1, _ := ptypes.TimestampProto(t1)
	t2, _ := time.Parse(time.RFC1123Z, "Mon, 12 Jan 2019 15:04:06 -0800")
	tp2, _ := ptypes.TimestampProto(t2)
	t3, _ := time.Parse(time.RFC1123Z, "Mon, 12 Jan 2020 15:04:05 -0800")
	tp3, _ := ptypes.TimestampProto(t3)
	// Random machine proto
	machine1 := &ufspb.Machine{
		Name:         "Machine-1",
		SerialNumber: "299-792-458",
		Location: &ufspb.Location{
			Zone:        ufspb.Zone_ZONE_CHROMEOS1,
			Aisle:       "70",
			Row:         "117",
			Rack:        "99",
			Shelf:       "107",
			Position:    "89",
			BarcodeName: "chromimuos111-row117-rack99-host89",
		},
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				ReferenceBoard: "oak",
				BuildTarget:    "teak",
				Model:          "Y",
				GoogleCodeName: "47",
				MacAddress:     "6f:59:6b:63:75:46",
				Sku:            "Sku-2",
				Phase:          "EOLVT",
				CostCenter:     "Steam",
				DeviceType:     ufspb.ChromeOSDeviceType_DEVICE_CHROMEBOOK,
			},
		},
		UpdateTime: tp1,
	}

	// Same as machine1 but with a second forward timestamp
	machine2 := &ufspb.Machine{
		Name:         "Machine-1",
		SerialNumber: "299-792-458",
		Location: &ufspb.Location{
			Zone:        ufspb.Zone_ZONE_CHROMEOS1,
			Aisle:       "70",
			Row:         "117",
			Rack:        "99",
			Shelf:       "107",
			Position:    "89",
			BarcodeName: "chromimuos111-row117-rack99-host89",
		},
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				ReferenceBoard: "oak",
				BuildTarget:    "teak",
				Model:          "Y",
				GoogleCodeName: "47",
				MacAddress:     "6f:59:6b:63:75:46",
				Sku:            "Sku-2",
				Phase:          "EOLVT",
				CostCenter:     "Steam",
				DeviceType:     ufspb.ChromeOSDeviceType_DEVICE_CHROMEBOOK,
			},
		},
		UpdateTime: tp2,
	}

	// Ramdom machine different from machine1
	machine3 := &ufspb.Machine{
		Name:         "Machine-2",
		SerialNumber: "299-792-458",
		Location: &ufspb.Location{
			Zone:        ufspb.Zone_ZONE_CHROMEOS1,
			Aisle:       "78",
			Row:         "119",
			Rack:        "98",
			Shelf:       "117",
			Position:    "99",
			BarcodeName: "chromimuos111-row119-rack98-host99",
		},
		Device: &ufspb.Machine_ChromeosMachine{
			ChromeosMachine: &ufspb.ChromeOSMachine{
				ReferenceBoard: "oak",
				BuildTarget:    "teak",
				Model:          "X",
				GoogleCodeName: "47",
				MacAddress:     "6f:59:6b:63:75:46",
				Sku:            "Sku-9",
				Phase:          "EOLVT",
				CostCenter:     "Charlies",
				DeviceType:     ufspb.ChromeOSDeviceType_DEVICE_CHROMEBOOK,
			},
		},
		UpdateTime: tp3,
	}

	ftt.Run("Compare Machine", t, func(t *ftt.Test) {
		t.Run("Comparing same machine", func(t *ftt.Test) {
			res := Compare(machine1, machine1)
			assert.Loosely(t, res, should.Equal(true))
		})
		t.Run("Comparing same machine with diff timestamp", func(t *ftt.Test) {
			res := Compare(machine1, machine2)
			assert.Loosely(t, res, should.Equal(true))
		})
		t.Run("Comparing different machines", func(t *ftt.Test) {
			res := Compare(machine1, machine3)
			assert.Loosely(t, res, should.Equal(false))
		})
	})
}
