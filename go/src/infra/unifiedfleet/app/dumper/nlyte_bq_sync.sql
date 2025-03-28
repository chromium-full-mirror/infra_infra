-- Collect individual active state DUT assets from Nlyte BigQuery instance
-- Specifically, look for active assets created from materials given the
-- material_ids parameter, associate these assets to their W-shelf chassis
-- (created from material ID 26095) and cabinet mountings, and return
-- UFS-style information.
WITH
  Assets AS (
    SELECT
      Asset.AssetID,
      Asset.Tag,
      Asset.AssetName,
      Asset.CabinetAssetID,
      Asset.GridReferenceRow AS Row,
      Asset.GridReferenceColumn AS Rack,
      Orientation.Detail AS Face,
    FROM `nlyte-tng-prod.nlyte.dbo_Asset` AS Asset
    LEFT JOIN `nlyte-tng-prod.nlyte.dbo_vwAssetMounting` AS AssetMounting
      ON Asset.AssetID = AssetMounting.MountedAssetID
    LEFT JOIN `nlyte-tng-prod.nlyte.dbo_Orientation` AS Orientation
      USING (OrientationID)
    WHERE
      MaterialID = 25144  -- DUT STD
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
    INNER JOIN `nlyte-tng-prod.nlyte.dbo_vwUMounting` AS UMounting
      USING (CabinetAssetID)
    INNER JOIN `nlyte-tng-prod.nlyte.dbo_Asset` AS ChassisAsset
      USING (UMountingID)
    WHERE ChassisAsset.MaterialID = 26095  -- Enconnex W-Shelf
  )
SELECT
  Assets.AssetID AS ID,
  Assets.Tag,
  Assets.AssetName AS Name,
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
    - MOD(ChassisMountedAssetMap.ColumnPosition, 4) AS Host,
  CustomFieldModel.DataValueString AS Model,
  CustomFieldBoard.DataValueString AS Board,
  CustomFieldZone.DataValueString AS Zone
FROM Assets
INNER JOIN `nlyte-tng-prod.nlyte.dbo_ChassisMountedAssetMap` AS ChassisMountedAssetMap
  ON Assets.AssetID = ChassisMountedAssetMap.MountedAssetID
LEFT JOIN CabinetShelves
  ON
    ChassisMountedAssetMap.ChassisAssetID = CabinetShelves.AssetID
LEFT JOIN `nlyte-tng-prod.nlyte.dbo_vwAssetCustomField` AS CustomFieldModel
  ON
    Assets.AssetID = CustomFieldModel.AssetID
    AND CustomFieldModel.DataLabel = 'Model'
LEFT JOIN `nlyte-tng-prod.nlyte.dbo_vwAssetCustomField` AS CustomFieldBoard
  ON
    Assets.AssetID = CustomFieldBoard.AssetID
    AND CustomFieldBoard.DataLabel = 'Board'
LEFT JOIN `nlyte-tng-prod.nlyte.dbo_vwAssetCustomField` AS CustomFieldZone
  ON
    Assets.CabinetAssetID = CustomFieldZone.AssetID
    AND CustomFieldZone.DataLabel = 'Zone'