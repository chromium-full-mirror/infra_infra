#!/bin/bash -e
# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Searches cipd for the latest prebuild shellcheck subtool and pins it as the
# new version in shellcheck_ensure.
#
# After running this, a person with suitable permissions to the cipd
# infra/tricium/function/shellcheck package should be able to update like so:
#   1. make testrun (note: runs `cipd export -ensure-file shellcheck_ensure`)
#   2. Check test results.
#   3. Run 'cipd create -pkg-def cipd.yaml' to upload the new shellcheck_tricium
#      package to cipd.

SUBTOOL_PREFIX="chromiumos/infra/tools/shellcheck"
ENSURE_FILE="shellcheck_ensure"

# Takes a ref such as "latest" and resolves it to a cipd instance ID.
ref_to_instance() {
  cipd resolve "${SUBTOOL_PREFIX}" -version "$1" \
      | sed -n -E '2 {s/.*:(.*)/\1/; p}'
}

# Takes an instance ID and extracts the ebuild metadata in cipd.
ebuild_version() {
  cipd describe "${SUBTOOL_PREFIX}" -version "$1" \
      | sed -n -e '/ebuild_source/ { s/^ *//; p }'
}

new_instance="$(ref_to_instance "${1-latest}")"
new_version="$(ebuild_version "${new_instance}")"

echo "${SUBTOOL_PREFIX} ${new_instance}" > "${ENSURE_FILE}"

git add "${ENSURE_FILE}"
git commit -F- <<EOM
[tricium] Update shellcheck to ${new_version}

$(cipd describe "${SUBTOOL_PREFIX}" -version "${new_instance}")

Test: <<REPLACE-ME>>
Bug: <<REPLACE-ME>>
EOM

cat <<EOM
A git commit was created to update ${ENSURE_FILE}.  You need to amend this
commit with proper bug and test information, then upload it for review.
EOM
