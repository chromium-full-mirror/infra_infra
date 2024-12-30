#!/bin/bash
# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x
set -o pipefail

PREFIX="$1"

./raw_source_0.exe -quiet -layout "${PREFIX}" || (cat Setup_*_Failed.txt && exit 1)