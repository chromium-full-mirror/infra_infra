SELECT DISTINCT 
(SELECT val as pool FROM UNNEST(labels) WHERE label = 'label-pool' LIMIT 1) as pool 
FROM `chromeos-test-platform-data.analytics.swarming_labels_last14days`