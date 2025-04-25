SELECT DISTINCT
    device.deviceId AS device_id,
    IFNULL(device.platform, 'NONE') AS soc,
    IFNULL(device.deviceFormFactor, 'NONE') AS form_factor,
    IFNULL(device.platformVendorName, 'NONE') AS soc_vendor,
    IFNULL(device.architecture, 'NONE') AS architecture,
    IFNULL(deviceType, 'NONE') AS device_type,
    IFNULL(buildTargets, 'NONE') AS build_targets,
    IFNULL(device.googleCodeName, 'NONE') AS model,
FROM `cros-device-lifecycle-manager.prod.devices` AS device
WHERE
  -- AUE Dates that are currently active
  IFNULL(device.aueDate > CURRENT_DATETIME(), TRUE)
  AND device.deviceType IN ('DEVICE', 'REFERENCE_BOARD');