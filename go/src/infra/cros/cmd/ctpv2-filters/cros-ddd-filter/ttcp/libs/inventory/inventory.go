// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"google.golang.org/api/option"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/inventory/croslab"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/inventory/deviceinfo"
)

func RetreiveInventoryProperties(useSwarmingInventory bool, inventoryFile string, resc *datasets.AllDatasetsResources, pool string, logger *log.Logger, clientOpts ...option.ClientOption) []*deviceinfo.TargetVariant {
	var inventoryInfo []*deviceinfo.TargetVariant
	if useSwarmingInventory {
		inventoryInfo = croslab.GenerateAvailableDevicesInfo(resc, pool, logger, clientOpts...)

	} else if inventoryFile != "" {
		fileContent, err := ioutil.ReadFile(inventoryFile)
		if err != nil {
			log.Fatal(errors.JoinError(
				fmt.Sprintf("Error while reading the file %s specified via the parameter --inventoryfile", inventoryFile),
				err))
		}
		hwids := strings.Split(string(fileContent), "\n")
		inventoryInfo = croslab.HwidToProperties(hwids, resc)
	} else {
		log.Fatal("Error: no inventory was provided")
	}
	return inventoryInfo
}
