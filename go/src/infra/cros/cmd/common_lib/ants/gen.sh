#!/bin/bash
# Copyright 2024 The LUCI Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script generates the android build API client library from its discovery endpoint.

curl https://androidbuildinternal.googleapis.com/discovery/v1/apis/androidbuildinternal/v3/rest > temp.json
google-api-go-generator -gendir "./" -api_json_file "./temp.json"
rm temp.json

# It is important to use the exact same code as used by
# `google-api-go-generator` binary which is pulled in tools.go from
#`google.golang.org/api` module. Otherwise we may be generating incompatible
# code.
api_mod_dir=$(go mod download -json google.golang.org/api | \
  python3 -c "import sys, json; print(json.load(sys.stdin)['Dir'])")
# Copy internal/gensupport package from there into our tree.
rm -f ./gensupport/*.go
cp ${api_mod_dir}/internal/gensupport/* ./gensupport
chmod -R u+w ./gensupport
# Copy license
cp ${api_mod_dir}/LICENSE ./gensupport

# Update package version in README.chromium
version=$(echo ${api_mod_dir} | sed "s|^.*@||")
sed -i "s/Version:.*$/Version: ${version}/" ./README.chromium

# We don't really care about tests nor can really run them with a partial copy.
rm ./gensupport/*_test.go

# Avoid linking to internal.Version, this is impossible.
sed -i 's/internal.Version/"luci-go"/g' androidbuildinternal/v3/*.go gensupport/*.go

# Update import path of gensupport.
sed -i 's/google.golang.org\/api\/internal\/gensupport/infra\/cros\/cmd\/common_lib\/ants\/gensupport/g' androidbuildinternal/v3/*.go
goimports -w androidbuildinternal/v3/*.go gensupport/*.go

rm androidbuildinternal/v3/*.json

sed  -i '1i // common_typos_disable' androidbuildinternal/v3/*.go


# If the global environment var has been written to the generated file then do
# not duplicate the line
if ! grep -q "var Environment = \"v3\"" androidbuildinternal/v3/androidbuildinternal-gen.go; then
  sed  -i '$a var Environment = \"v3\"' androidbuildinternal/v3/androidbuildinternal-gen.go
fi

# Change all hardcoded "v3" endpoint uses to instead use the global env
# variable. This will allow us to swap between enivonments without duplicating
# the common types.
sed -i 's/\/v3\//\/\"\+ Environment \+ \"\//g' androidbuildinternal/v3/androidbuildinternal-gen.go
