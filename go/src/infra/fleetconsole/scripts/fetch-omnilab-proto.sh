#!/usr/bin/env bash

# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
set -e


NC='\033[0m'        # No Color (resets all attributes to default)
GREY='\033[2;90m'
GREEN='\033[1;32m'


root=$(pwd)
if [[ $(basename ${root}) != "fleetconsole" ]]; then
  echo "This script must be run from the fleetconsole directory"
  exit 1
fi

TEMP_DIR=$root/.tmp/omnilab-github
DESTINATION_DIR=$root/omnilab

echo -ne "${GREY}" # Make all subsequent logs grey

echo $root
echo $TEMP_DIR
echo $DESTINATION_DIR

rm -rf $TEMP_DIR || true
mkdir -p $TEMP_DIR

echo "Cloning omnilab repo..."
git clone https://github.com/google/device-infra.git $TEMP_DIR

echo "Moving file..."
mv ${TEMP_DIR}/src/devtools/mobileharness/infra/monitoring/proto/monitored_record.proto ${DESTINATION_DIR}/omnilab-pubsub.proto

echo "Adding go_package option..."
cat << EOF >> ${DESTINATION_DIR}/omnilab-pubsub.proto
// FROM HERE ONWARDS IS SPECIFIED BY \`fetch-omnilab-proto.sh\`

option go_package = "go.chromium.org/infra/fleetconsole/omnilab/omnilab-pubsub";
EOF

rm -rf $TEMP_DIR

echo -ne "${NC}" # Make all subsequent logs the default color

echo "Successfully fetched omnilab proto"
echo -e "Run ${GREEN}go generate ./omnilab${NC} to generate the go bindings"

