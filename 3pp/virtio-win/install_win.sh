#!/bin/bash
# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x
set -o pipefail

PREFIX="$1"

7zz x raw_source_0.iso "-o$PREFIX/tmp" -y
cp $PREFIX/tmp/virtio-win-gt-x64.msi $PREFIX/virtio-win-gt-x64.msi
cp $PREFIX/tmp/virtio-win-guest-tools.exe $PREFIX/virtio-win-guest-tools.exe
rm -rf $PREFIX/tmp
rm -f raw_source_0.iso