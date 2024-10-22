// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dumper

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
)

const (
	nlyteQueryGetMountedDuts = `WITH
Assets AS (
	SELECT
	AssetID, Tag, AssetName, CabinetAssetID, GridReferenceRow AS Row, GridReferenceColumn AS Rack
	FROM ` + "`nlyte-tng-prod.nlyte.dbo_Asset`" + `
	WHERE MaterialID = 25144  -- DUT STD
	  AND RecordStatus = 2  -- Active
),
Cabinets AS (
	SELECT DISTINCT CabinetAssetID FROM Assets
),
CabinetShelves AS (
	SELECT
	ChassisAsset.AssetID,
	RANK()
		OVER (PARTITION BY UMounting.CabinetAssetID ORDER BY UMounting.CabinetUNumber ASC)
		AS ShelfIndex
	FROM Cabinets
	INNER JOIN ` + "`nlyte-tng-prod.nlyte.dbo_vwUMounting`" + ` AS UMounting
	USING (CabinetAssetID)
	INNER JOIN ` + "`nlyte-tng-prod.nlyte.dbo_Asset`" + ` AS ChassisAsset
	USING (UMountingID)
	WHERE ChassisAsset.MaterialID = 26095  -- Enconnex W-Shelf
)
SELECT
Assets.AssetID,
Assets.Tag,
Assets.AssetName,
FORMAT('%02d', SAFE_CAST(Assets.Row AS INT)) AS Row,
FORMAT('%02d-%02d', SAFE_CAST(Assets.Row AS INT), SAFE_CAST(Assets.Rack AS INT)) AS Rack,
(CabinetShelves.ShelfIndex * 2) - MOD(ChassisMountedAssetMap.ColumnPosition, 4) AS Host,
CustomFieldModelBoard.DataValueString AS ModelBoard,
CustomFieldZone.DataValueString AS Zone
FROM Assets
INNER JOIN ` + "`nlyte-tng-prod.nlyte.dbo_ChassisMountedAssetMap`" + ` AS ChassisMountedAssetMap
ON Assets.AssetID = ChassisMountedAssetMap.MountedAssetID
LEFT JOIN CabinetShelves
ON
	ChassisMountedAssetMap.ChassisAssetID = CabinetShelves.AssetID
LEFT JOIN ` + "`nlyte-tng-prod.nlyte.dbo_vwAssetCustomField`" + ` AS CustomFieldModelBoard
ON
	Assets.AssetID = CustomFieldModelBoard.AssetID
	AND CustomFieldModelBoard.DataLabel = 'Model and Board'
LEFT JOIN ` + "`nlyte-tng-prod.nlyte.dbo_vwAssetCustomField`" + ` AS CustomFieldZone
ON
	ChassisMountedAssetMap.ChassisAssetID = CustomFieldZone.AssetID
	AND CustomFieldZone.DataLabel = 'Zone'`
)

type assetResult struct {
	ID               int
	Tag              string
	Row, Rack        string
	Host             int
	ModelBoard, Zone bigquery.NullString
}

func fetchNlyteBigQueryData(ctx context.Context) (err error) {
	logging.Debugf(ctx, "Entering Nlyte Sync")
	defer logging.Debugf(ctx, "Exiting Nlyte sync")
	defer fetchNlyteBigQueryDataTick.Add(ctx, 1, err == nil)
	client, err := bigquery.NewClient(ctx, "nlyte-tng-prod")
	if err != nil {
		return fmt.Errorf("bigquery.NewClient: %w", err)
	}
	defer client.Close()

	// Location must match that of the dataset(s) referenced in the query.
	client.Location = "US"
	q := client.Query(nlyteQueryGetMountedDuts)
	it, err := q.Read(ctx)
	if err != nil {
		err = errors.Annotate(err, "executing nlyte query").Err()
		return
	}
	for {
		var row assetResult
		err = it.Next(&row)
		if errors.Is(err, iterator.Done) {
			err = nil
			break
		}
		if err != nil {
			err = errors.Annotate(err, "reading nlyte query results").Err()
			return
		}
		logging.Infof(ctx, "Nlyte sync attempt got data: %+v", row)
	}
	return
}
