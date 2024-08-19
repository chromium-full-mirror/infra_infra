#!/bin/sh
# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Reference: http://google3/third_party/luci_py/latest/appengine/swarming/setup_bigquery.sh

set -eu

cd "$(dirname $0)"

if ! (which bq) > /dev/null; then
  echo "Please install 'bq' from gcloud SDK"
  echo "  https://cloud.google.com/sdk/install"
  exit 1
fi

if ! (which bqschemaupdater) > /dev/null; then
  echo "Please install 'bqschemaupdater' from Chrome's infra.git"
  echo "  Checkout infra.git then run: eval \`./go/env.py\`"
  exit 1
fi

if [ $# != 1 ]; then
  echo "usage: setup_bigquery.sh <instanceid>"
  echo ""
  echo "Pass one argument which is the instance name"
  exit 1
fi

APPID=$1

echo "- Make sure the BigQuery API is enabled for the project:"
# It is enabled by default for new projects, but it wasn't for older projects.
gcloud services enable --project "${APPID}" bigquery-json.googleapis.com

# Permission is grantes via overground, skipping here

echo "- Create the dataset:"
echo ""
echo "  Warning: On first 'bq' invocation, it'll try to find out default"
echo "    credentials and will ask to select a default app; just press enter to"
echo "    not select a default."

if ! (bq --location=US mk --dataset \
  --description 'unified fleet system statistics' "${APPID}":ufs); then
  echo ""
  echo "Dataset creation failed. Assuming the dataset already exists. At worst"
  echo "the following command will fail."
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.ChromePlatformRow  \
    -table "${APPID}".ufs.chrome_platforms); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.chrome_platforms"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.ChromePlatformRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.chrome_platforms_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.chrome_platforms_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.VlanRow  \
    -table "${APPID}".ufs.vlans); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.vlans"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.VlanRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.vlans_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.vlans"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.MachineRow  \
    -table "${APPID}".ufs.machines); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.machines"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.MachineRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.machines_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.machines_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.RackRow  \
    -table "${APPID}".ufs.racks); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.racks"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.RackRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.racks_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.racks_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.RackLSEPrototypeRow  \
    -table "${APPID}".ufs.rack_lse_prototypes); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.rack_lse_prototypes"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.RackLSEPrototypeRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.rack_lse_prototypes_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.rack_lse_prototypes_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.MachineLSEPrototypeRow  \
    -table "${APPID}".ufs.machine_lse_prototypes); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.machine_lse_prototypes"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.MachineLSEPrototypeRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.machine_lse_prototypes_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.machine_lse_prototypes_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.MachineLSERow  \
    -table "${APPID}".ufs.machine_lses); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.machine_lses"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.MachineLSERow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.machine_lses_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.machine_lses_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.RackLSERow  \
    -table "${APPID}".ufs.rack_lses); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.rack_lses"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.RackLSERow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.rack_lses_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.rack_lses_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.KVMRow  \
    -table "${APPID}".ufs.kvms); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.kvms"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.KVMRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.kvms_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.kvms_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.RPMRow  \
    -table "${APPID}".ufs.rpms); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.rpms"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.RPMRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.rpms_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.rpms_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.SwitchRow  \
    -table "${APPID}".ufs.switches); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.switches"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.SwitchRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.switches_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.switches_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.DracRow  \
    -table "${APPID}".ufs.dracs); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.dracs"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.DracRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.dracs_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.dracs_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.NicRow  \
    -table "${APPID}".ufs.nics); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.nics"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.NicRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.nics_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.nics_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.DHCPConfigRow  \
    -table "${APPID}".ufs.dhcps); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.dhcps"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.DHCPConfigRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.dhcps_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.dhcps_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.IPRow  \
    -table "${APPID}".ufs.ips); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.ips"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.IPRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.ips_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.ips_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.StateRecordRow  \
    -table "${APPID}".ufs.state_records); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.state_records"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.StateRecordRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.state_records_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.state_records_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.ChangeEventRow  \
    -table "${APPID}".ufs.change_events); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.change_events"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.VMRow  \
    -table "${APPID}".ufs.vms); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.vms"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.VMRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.vms_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.vms_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.AssetRow  \
    -table "${APPID}".ufs.assets); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.assets"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.AssetRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.assets_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.assets_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.DUTStateRecordRow  \
    -table "${APPID}".ufs.dutstates); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.dutstates"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.DUTStateRecordRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.dutstates_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.dutstates_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.MachineLSEDeploymentRow  \
    -table "${APPID}".ufs.machine_lse_deployments); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.machine_lse_deployments"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.MachineLSEDeploymentRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.machine_lse_deployments_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.machine_lse_deployments_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.SchedulingUnitRow  \
    -table "${APPID}".ufs.scheduling_units); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.scheduling_units"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.SchedulingUnitRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.scheduling_units_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.scheduling_units_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.HwidDataRow  \
    -table "${APPID}".ufs.hwid_data); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.hwid_data"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.HwidDataRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.hwid_data_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.hwid_data_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi


echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.DeviceConfigRow  \
    -table "${APPID}".ufs.device_configs); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.device_configs"
  echo ""
  echo "and run this script again."
  exit 1
fi

echo "- Populate the BigQuery schema:"
echo ""
echo "  Warning: On first 'bqschemaupdater' invocation, it'll request default"
echo "    credentials which is stored independently than 'bq'."
if ! (bqschemaupdater -force \
    -I ../../../../../../ \
    -message unifiedfleet.api.v1.models.bigquery.DeviceConfigRow  \
    -partitioning-type HOUR \
    -partitioning-expiration 3999h \
    -table "${APPID}".ufs.device_configs_hourly); then
  echo ""
  echo ""
  echo "Oh no! You may need to restart from scratch. You can do so with:"
  echo ""
  echo "  bq rm ${APPID}:ufs.device_configs_hourly"
  echo ""
  echo "and run this script again."
  exit 1
fi