  -- This materialized view is not created by create_views.sh because:
  --   - Updating or replacing materialized views is not supported (they must be deleted and then recreated)
  --   - Deleting and recreating materialized views can be expensive as it re-queries the entire history of the underlying table
  --   - A single materialized view is used for both staging and production
  --
  -- Thus, if this needs to be updated, please do it manually through pantheon.
  --
  -- Note that materialized views do not support a lot of SQL features, so we are limited in what we can do here.
  -- The main goal is to cut down the amount of data required to be queried by pre-filtering and clustering (BigQuery pre-sorting) data.
  -- https://cloud.google.com/bigquery/docs/materialized-views-create#query_limitations
CREATE MATERIALIZED VIEW
  `sheriff-o-matic.materialized.buildbucket_completed_builds_prod`
PARTITION BY
  DATE(create_time)
CLUSTER BY
  `project`,
  bucket,
  builder,
  id AS
SELECT
  id,
  builder.`project` AS `project`,
  builder.bucket AS bucket,
  builder.builder AS builder,
  number,
  status,
  create_time,
  end_time,
  critical,
  input.gitiles_commit AS input_commit,
  output.gitiles_commit AS output_commit,
  JSON_EXTRACT_SCALAR(input.properties,
    "$.builder_group") AS buildergroup,
  IFNULL(JSON_EXTRACT_STRING_ARRAY(input.properties, "$.gardener_rotations"),
    JSON_EXTRACT_STRING_ARRAY(input.properties, "$.sheriff_rotations")) AS sheriff_rotations,
  steps
FROM
  `cr-buildbucket.raw.completed_builds`
WHERE
  NOT input.experimental
  AND ((REGEXP_CONTAINS(builder.`project`, "^((chrome|chromium)(-m[0-9]+(-.*)?)?)$")
      AND JSON_EXTRACT_STRING_ARRAY(input.properties,
        "$.gardener_rotations") IS NOT NULL
      AND ARRAY_LENGTH(JSON_EXTRACT_STRING_ARRAY(input.properties,
          "$.gardener_rotations")) > 0 )
    OR NOT REGEXP_CONTAINS(builder.`project`, "^((chrome|chromium)(-m[0-9]+(-.*)?)?)$"))