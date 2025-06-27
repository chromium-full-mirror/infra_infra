// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package croslab

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets"
	buildmetadata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/buildmetadata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/dlmmetadata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/hwid/db"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/hwid/decoder"
	swarmingdata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/swarmingdata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/targetproperties"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/inventory/deviceinfo"
)

// TODO(b:275572044) The inventory is currently retreived from bigquery. In the
//                  future it should be retreived from databake.

var (
	GetInventory = getSwarmingInventory

	// Add inventory cache to minimize cost on inventory data access requests
	// Type: map[string]*swarmingdata.SwarmDataResources{}
	inventoryCache = sync.Map{}
	// Lock the inventoryCache down by pool. Saves on resources when pulling from bigquery.
	// Type: map[string]sync.Mutex{}
	inventoryCacheLocksByPool = sync.Map{}
	// Track the expiration of the cache per pool.
	// Type: time.Time
	inventoryCacheExpiration = sync.Map{}

	// Type: map[string][]*deviceinfo.TargetVariant
	deviceInfosCache = sync.Map{}
	// Lock the deviceInfosCache down by pool.
	// Type: map[string]sync.Mutex{}
	deviceInfosCacheLocksByPool = sync.Map{}
	// Track the expiration of the cache per pool.
	// Type: time.Time
	deviceInfosCacheExpiration = sync.Map{}
)

func GenerateAvailableDevicesInfo(resc *datasets.AllDatasetsResources, pool string, logger *log.Logger, clientOpts ...option.ClientOption) []*deviceinfo.TargetVariant {
	swarmResc, err := GetInventory(pool, logger, clientOpts...)
	if err != nil {
		if logger != nil {
			logger.Println("Could not retrieve swarming inventory:", err)
		}
		log.Fatal("Could not retrieve swarming inventory:", err)
	}
	return GeneratePropertiesFromInventory(pool, swarmResc, resc, logger)
}

func GeneratePropertiesFromInventory(pool string, inventory *swarmingdata.SwarmDataResources, resc *datasets.AllDatasetsResources, logger *log.Logger) []*deviceinfo.TargetVariant {
	lock, _ := deviceInfosCacheLocksByPool.LoadOrStore(pool, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()
	deviceInfos, ok := deviceInfosCache.Load(pool)
	deviceInfosExpiration, _ := deviceInfosCacheExpiration.LoadOrStore(pool, time.Now().Add(8*time.Hour))
	if ok && time.Now().Before(deviceInfosExpiration.(time.Time)) {
		return deviceInfos.([]*deviceinfo.TargetVariant)
	}

	fleetSet, err := inventory.ParseAsList()
	if err != nil {
		if logger != nil {
			logger.Println("Could not parse swarming inventory:", err)
		}
		log.Fatal("Could not parse swarming inventory:", err)
	}
	hwids := fleetSet.GetUniqueHwids()
	allDeviceProperties := HwidToProperties(hwids, resc)
	mergeSwarmPropsToHwid(inventory, allDeviceProperties)
	deviceInfosCache.Store(pool, allDeviceProperties)
	deviceInfosCacheExpiration.Store(pool, time.Now().Add(8*time.Hour))
	return allDeviceProperties
}

// todo needs to return error
func HwidToProperties(hwids []string, resc *datasets.AllDatasetsResources) []*deviceinfo.TargetVariant {
	// add the Hwid properties
	hwidDb := db.InitializeHwidDb(resc.HwidDB)
	allDeviceProperties := []*deviceinfo.TargetVariant{}
	for _, h := range hwids {
		hwid, err := decoder.DecodeHwid(h, hwidDb)
		if err != nil {
			log.Printf("  ERROR: Could not decode hwid:%s   error: %s\n", h, err)
			continue
		}
		deviceProperties, err := hwid.ToPropertiesBag()
		if err != nil {
			log.Printf("  ERROR: Could retrieve properties of hwid:%s   error: %s\n", h, err)
			continue
		}
		allDeviceProperties = append(allDeviceProperties, &deviceinfo.TargetVariant{
			DeviceId:   h,
			Properties: deviceProperties,
		})
	}
	allDeviceProperties = mergeImagePropsToHwid(resc.Buildmetadata, allDeviceProperties)
	mergeDlmDevicePropsToHwid(resc.Dlmmetadata, allDeviceProperties)
	return allDeviceProperties
}

// mergeImagePropsToHwid returns a slice containing all of the device variants merged with build image properties.
func mergeImagePropsToHwid(resc *buildmetadata.BuildMetadataResources, hwidTargetVariantProps []*deviceinfo.TargetVariant) []*deviceinfo.TargetVariant {
	//Join the devices with their available cros images properties.
	buildData, err := buildmetadata.PaseAsMapPerBoard(resc)
	if err != nil {
		log.Fatal(errors.ApiError(err))
	}
	results := []*deviceinfo.TargetVariant{}
	for _, target := range hwidTargetVariantProps {
		boardProp := target.Properties.PropertiesDetails["board"]
		board, err := boardProp.GetSingleStringValue()
		if err != nil {
			log.Fatal(errors.ApiError(err))
		}
		images := buildData[board]
		for _, img := range images {
			deviceImageTarget := target.Clone()
			deviceImageTarget.BuildTarget = img.BuildTarget

			imgProps, err := img.GetProperties()

			if err != nil {
				log.Fatal(errors.ApiError(err))
			}
			deviceImageTarget.Properties.Merge(imgProps)
			results = append(results, deviceImageTarget)
		}
	}
	return results
}

// mergeDlmDevicePropsToHwid returns a slice containing all of the device variants merged with Dlm Device properties.
func mergeDlmDevicePropsToHwid(resc *dlmmetadata.DlmResources, hwidTargetVariantProps []*deviceinfo.TargetVariant) {
	//Join the devices with their available cros dlm device properties.
	dlmData, err := dlmmetadata.PaseAsMapPerModel(resc)
	if err != nil {
		log.Fatal(errors.ApiError(err))
	}
	for _, target := range hwidTargetVariantProps {
		modelPropVal, err := getModelPropertyValue(target)
		if err != nil {
			log.Fatal(errors.ApiError(err))
		}
		modelVal, err := modelPropVal.GetSingleStringValue()
		if err != nil {
			log.Fatal(errors.ApiError(err))
		}
		models := dlmData[strings.ToUpper(modelVal)]
		for _, model := range models {
			modelProps, err := model.GetProperties()
			if err != nil {
				log.Fatal(errors.ApiError(err))
			}
			target.Properties.Merge(modelProps)
		}
	}
}

// mergeSwarmPropsToHwid returns a slice containing all of the device variants merged with Swarming Bot properties.
func mergeSwarmPropsToHwid(resc *swarmingdata.SwarmDataResources, hwidTargetVariantProps []*deviceinfo.TargetVariant) {
	//Join the devices with their available swarming bot properties.
	swarmData, err := resc.ParseAsMapPerHwid()
	if err != nil {
		log.Fatal(errors.ApiError(err))
	}
	for _, target := range hwidTargetVariantProps {
		deviceId := target.DeviceId
		if err != nil {
			log.Fatal(errors.ApiError(err))
		}
		swarmDevices := swarmData[deviceId]
		for _, swarmDevice := range swarmDevices {
			swarmLabelProps, err := swarmDevice.GetProperties()
			if err != nil {
				log.Fatal(errors.ApiError(err))
			}
			target.Properties.Merge(swarmLabelProps)
		}
	}
}

// getSwarmingInventory returns a slice contain all the different HWIDs that are present in the fleet.
// Note: getSwarmingInventory when running locally on a developer station requires
// to first run: gcloud auth application-default login
// The command will install the necessairy certificates so getSwarmingInventory
// can access Bigquery.
func getSwarmingInventory(pool string, logger *log.Logger, clientOpts ...option.ClientOption) (*swarmingdata.SwarmDataResources, error) {
	// The body of this function to temporary. It is only here to check
	// that the access to bigquery from golang was functional as it required
	// a lot of changes in the build files.
	ctx := context.Background()
	projectID := "chromeos-test-platform-data"
	var c *bigquery.Client
	var err error
	swarmingResource := &swarmingdata.SwarmDataResources{}

	// Return inventory data if already queried
	lock, _ := inventoryCacheLocksByPool.LoadOrStore(pool, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()
	inventoryData, ok := inventoryCache.Load(pool)
	inventoryExpiration, _ := inventoryCacheExpiration.LoadOrStore(pool, time.Now().Add(8*time.Hour))
	if ok && time.Now().Before(inventoryExpiration.(time.Time)) {
		return inventoryData.(*swarmingdata.SwarmDataResources), nil
	}
	c, err = bigquery.NewClient(ctx, projectID, clientOpts...)
	if err != nil {
		if logger != nil {
			logger.Println("Could not connect to bigquery:", err)
		}
		return swarmingResource, err
	} else {
		if logger != nil {
			logger.Println("Connected to bigquery")
		}
	}
	defer c.Close()
	poolConstraint := ""
	if pool != "" {
		if logger != nil {
			logger.Println("inventory restricted to pool:", pool)
		}
		poolConstraint = "\nWHERE " +
			"(SELECT labels.val FROM UNNEST(labels) AS labels" +
			" WHERE labels.label = 'label-pool' LIMIT 1) ='" + pool + "'"
	}

	// Construct a query.
	// TODO(b:275572044) when we will retreive the data from databake, this
	//                   query should be reworked in order to not be based in
	//                   inline embedded string.

	// this query needs to be extended to get additional schedulalble lable other
	// than hwid
	q := c.Query(`
     SELECT dut_id, labels
     FROM analytics.swarming_labels_last14days` + poolConstraint + ``)

	// Execute the query.
	it, err := q.Read(ctx)
	if err != nil {
		log.Println("Could not execute query:", err)
		return swarmingResource, err
	} else {
		if logger != nil {
			logger.Println("swarming data retreived from BQ")
		}
	}
	fleetData := []*swarmingdata.SwarmingBot{}
	// Iterate through the results.
	for {
		values := &swarmingdata.SwarmingBot{}
		err := it.Next(values)
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatal("Reading row:", err)
			return swarmingResource, err
		}
		if logger != nil {
			// labelsStr := ""
			// for _, value := range values.Values {
			// 	labelsStr += fmt.Sprintf("%s=%s ", value.Label, value.Val)
			// }
			// logger.Printf("swarming row: {dut_id: %s, labels: [%s]}", values.DutId, labelsStr)
		}
		fleetData = append(fleetData, values)
	}
	swarmingResource.SwarmDb = fleetData
	// Cache for future inventory data requests
	inventoryCache.Store(pool, swarmingResource)
	inventoryCacheExpiration.Store(pool, time.Now().Add(8*time.Hour))
	return swarmingResource, nil
}

func getModelPropertyValue(target *deviceinfo.TargetVariant) (*targetproperties.PropertyValue, error) {
	exceptionChassis := getChassisExceptionMap()

	model, ok := target.Properties.PropertiesDetails["project"]
	if !ok {
		return model, errors.NewErrorf("The device is missing the property \"project\". DeviceProperties:%v", target)
	}
	modelVal, err := model.GetSingleStringValue()
	if err != nil {
		return &targetproperties.PropertyValue{}, err
	}
	if _, ok = exceptionChassis[strings.ToLower(modelVal)]; !ok {
		if chassis, ok := target.Properties.PropertiesDetails["chassis_field"]; ok {
			if len(chassis.Values) > 0 && chassis.Values[0].(string) != "" {
				model = chassis
			}
		}
	}
	return model, nil
}

func getChassisExceptionMap() map[string]bool {
	exceptMap := make(map[string]bool)
	exceptionModels := []string{"hayato", "arcada", "drallion", "drallion360", "sarien"}

	for _, model := range exceptionModels {
		exceptMap[model] = true
	}
	return exceptMap
}
