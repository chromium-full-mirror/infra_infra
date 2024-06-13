// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uefi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindDeviceNumInOutput(t *testing.T) {
	output := `BootCurrent: 0000
Timeout: 1 seconds
BootOrder: 0000,0008,0006,0007
Boot0000* ChromeOS Flex	HD(12,GPT,d2a46c0d-b476-494c-a68d-2782b85d7975,0x89000,0x20000)/File(\EFI\BOOT\BOOTX64.EFI)
Boot0003* UEFI: PXE IPv4 Intel(R) Ethernet Controller (3) I225-LM	PciRoot(0x0)/Pci(0x1d,0x0)/Pci(0x0,0x0)/MAC(48210b510365,1)/IPv4(0.0.0.00.0.0.0,0,0)0000424f
Boot0004* UEFI: PXE IPv6 Intel(R) Ethernet Controller (3) I225-LM	PciRoot(0x0)/Pci(0x1d,0x0)/Pci(0x0,0x0)/MAC(48210b510365,1)/IPv6([::]:<->[::]:,0,0)0000424f
Boot0006* UEFI: PXE IPv4 Intel(R) Ethernet Controller (3) I225-LM	PciRoot(0x0)/Pci(0x1d,0x0)/Pci(0x0,0x0)/MAC(48210b510365,1)/IPv4(0.0.0.00.0.0.0,0,0)0000424f
Boot0007* UEFI: PXE IPv6 Intel(R) Ethernet Controller (3) I225-LM	PciRoot(0x0)/Pci(0x1d,0x0)/Pci(0x0,0x0)/MAC(48210b510365,1)/IPv6([::]:<->[::]:,0,0)0000424f
Boot0008* UEFI OS	HD(12,GPT,71789068-5f71-d049-8105-1facf2d87e94,0x89000,0x20000)/File(\EFI\BOOT\BOOTX64.EFI)0000424f`
	devnum, _ := findDeviceNumInOutput(output)

	assert.Equal(t, "0008", devnum)
}
