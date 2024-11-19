// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dumper

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/sync/parallel"

	ufspb "infra/unifiedfleet/api/v1/models"
	"infra/unifiedfleet/app/controller"
	"infra/unifiedfleet/app/util"
)

const (
	// Nlyte "DUT STD" Material ID
	nlyteMaterialIDChromeOSAsset = 25144
	// Collect individual active state DUT assets from Nlyte BigQuery instance
	// Specifically, look for active assets created from materials given the
	// material_ids parameter, associate these assets to their W-shelf chassis
	// (created from material ID 26095) and cabinet mountings, and return
	// UFS-style information.
	// TODO(b/378926955): Remove restriction to cabinet ID 580836
	nlyteQueryGetMountedDuts = `WITH
	Assets AS (
	  SELECT
		Asset.AssetID,
		Asset.Tag,
		Asset.AssetName,
		Asset.CabinetAssetID,
		Asset.GridReferenceRow AS Row,
		Asset.GridReferenceColumn AS Rack,
		Orientation.Detail AS Face,
	  FROM ` + "`nlyte-tng-prod.nlyte.dbo_Asset`" + ` AS Asset
	  LEFT JOIN ` + "`nlyte-tng-prod.nlyte.dbo_vwAssetMounting`" + ` AS AssetMounting
		ON Asset.AssetID = AssetMounting.MountedAssetID
	  LEFT JOIN ` + "`nlyte-tng-prod.nlyte.dbo_Orientation`" + ` AS Orientation
		USING (OrientationID)
	  WHERE
		MaterialID IN UNNEST(@material_ids)
		AND RecordStatus = 2  -- Active
	),
	Cabinets AS (
	  SELECT DISTINCT CabinetAssetID FROM Assets
	),
	CabinetShelves AS (
	  SELECT
		ChassisAsset.AssetID,
		Cabinets.CabinetAssetID,
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
Assets.AssetID AS ID,
Assets.Tag,
FORMAT('%02d', SAFE_CAST(Assets.Row AS INT)) AS Row,
FORMAT('%02d-%02d', SAFE_CAST(Assets.Row AS INT), SAFE_CAST(Assets.Rack AS INT)) AS Rack,
-- Chassis Host # Calculation: Offset by 0 if front mounted, or total shelf count * 2 if back
-- mounted, add index of shelf asset is attached to * 2, then finally determine if host is odd or
-- even based on chassis column, subtracting 1 if odd.
(
	CASE
	WHEN Assets.Face = 'Back'
		THEN
		(
			SELECT COUNT(*) * 2
			FROM CabinetShelves
			WHERE CabinetShelves.CabinetAssetID = Assets.CabinetAssetID
		)
	ELSE 0
	END)
	+ (CabinetShelves.ShelfIndex * 2)
	- MOD(ChassisMountedAssetMap.ColumnPosition, 4) AS ChassisHostNumber,
CustomFieldModel.DataValueString AS Model,
CustomFieldBoard.DataValueString AS Board,
CustomFieldZone.DataValueString AS Zone
FROM Assets
INNER JOIN ` + "`nlyte-tng-prod.nlyte.dbo_ChassisMountedAssetMap`" + ` AS ChassisMountedAssetMap
ON Assets.AssetID = ChassisMountedAssetMap.MountedAssetID
LEFT JOIN CabinetShelves
ON
	ChassisMountedAssetMap.ChassisAssetID = CabinetShelves.AssetID
LEFT JOIN ` + "`nlyte-tng-prod.nlyte.dbo_vwAssetCustomField`" + ` AS CustomFieldModel
ON
	Assets.AssetID = CustomFieldModel.AssetID
	AND CustomFieldModel.DataLabel = 'Model'
LEFT JOIN ` + "`nlyte-tng-prod.nlyte.dbo_vwAssetCustomField`" + ` AS CustomFieldBoard
ON
	Assets.AssetID = CustomFieldBoard.AssetID
	AND CustomFieldBoard.DataLabel = 'Board'
LEFT JOIN ` + "`nlyte-tng-prod.nlyte.dbo_vwAssetCustomField`" + ` AS CustomFieldZone
ON
	Assets.CabinetAssetID = CustomFieldZone.AssetID
	AND CustomFieldZone.DataLabel = 'Zone'
WHERE Assets.CabinetAssetID = 580836;`
)

type assetResult struct {
	ID                 int
	Tag                string
	Row, Rack          string
	Host               int
	Model, Board, Zone bigquery.NullString
}

func (a assetResult) toAssetProto() (*ufspb.Asset, error) {
	var merr []error

	// TODO(b/378926955): Fill out additional data about asset (board, model)
	// if !a.ModelBoard.Valid {
	// 	merr = append(merr, errors.New("unset model/board"))
	// }
	zoneString := strings.ToUpper(a.Zone.StringVal)
	zoneInt, ok := ufspb.Zone_value[fmt.Sprintf("ZONE_%s", zoneString)]
	if !ok {
		merr = append(merr, errors.New(fmt.Sprintf("unknown zone %q", zoneString)))
	}

	return &ufspb.Asset{
		Name: a.Tag,
		Type: ufspb.AssetType_DUT,
		// Model: a.Model.StringVal,
		Info: &ufspb.AssetInfo{
			AssetTag: a.Tag,
			// BuildTarget: a.Board.StringVal,
			// Model:       a.Model.StringVal,
		},
		Location: &ufspb.Location{
			Zone:     ufspb.Zone(zoneInt),
			Row:      a.Row,
			Rack:     a.Rack,
			Position: strconv.Itoa(a.Host),
		},
	}, errors.Join(merr...)
}

func fetchNlyteBigQueryData(ctx context.Context) (rErr error) {
	logging.Debugf(ctx, "Entering Nlyte Sync")
	defer logging.Debugf(ctx, "Exiting Nlyte sync")
	defer func() {
		fetchNlyteBigQueryDataTick.Add(ctx, 1, rErr == nil)
	}()
	client, err := bigquery.NewClient(ctx, "nlyte-tng-prod")
	if err != nil {
		return fmt.Errorf("bigquery.NewClient: %w", err)
	}
	defer client.Close()

	// Location must match that of the dataset(s) referenced in the query.
	client.Location = "US"
	q := client.Query(nlyteQueryGetMountedDuts)
	q.Parameters = []bigquery.QueryParameter{
		{
			Name:  "material_ids",
			Value: []int{nlyteMaterialIDChromeOSAsset},
		},
	}
	it, err := q.Read(ctx)
	if err != nil {
		return errors.Annotate(err, "executing nlyte query").Err()
	}

	return parallel.WorkPool(16, func(c chan<- func() error) {
		for {
			var row assetResult
			if err := it.Next(&row); err != nil {
				if !errors.Is(err, iterator.Done) {
					c <- func() error {
						return errors.Annotate(err, "reading nlyte query results").Err()
					}
				}
				break
			} else {
				logging.Debugf(ctx, "Nlyte sync attempt got data: %+v", row)
				c <- func() error {
					na, err := row.toAssetProto()
					if err != nil {
						return err
					}
					return nlyteUpcertAsset(ctx, na)
				}
			}
		}
	})
}

// TODO(b/378926955): Remove nlyte prefix once prod data is ready
// Create a new "nlyte-" prefix asset if one does not exist, and then update the prefix asset with info from the given asset
func nlyteUpcertAsset(ctx context.Context, nlyteAsset *ufspb.Asset) error {
	// Attempt to pull existing asset data (if any)
	baseAsset, err := controller.GetAsset(ctx, nlyteAsset.Name)
	if util.IsNotFoundError(err) {
		logging.Debugf(ctx, "No existing asset %s found in Datastore", nlyteAsset.Name)
		baseAsset = &ufspb.Asset{}
		proto.Merge(baseAsset, nlyteAsset)
	} else if err != nil {
		return err
	}

	nlyteAsset.Name = "nlyte-" + nlyteAsset.Name
	baseAsset.Name = "nlyte-" + baseAsset.Name
	_, err = controller.GetAsset(ctx, baseAsset.Name)
	if util.IsNotFoundError(err) {
		// Create new "nlyte-" asset
		logging.Debugf(ctx, "Creating asset %s", baseAsset.Name)
		_, err = controller.AssetRegistration(ctx, baseAsset)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	// Update existing "nlyte-" asset
	logging.Debugf(ctx, "Updating asset %s", baseAsset.Name)
	_, err = controller.UpdateAsset(ctx, nlyteAsset, &field_mask.FieldMask{Paths: []string{"location.zone", "location.row", "location.rack", "location.position", "info.build_target", "model"}})

	return err
}
