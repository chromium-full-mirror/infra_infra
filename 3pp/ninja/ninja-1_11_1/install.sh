#!/bin/bash
# Copyright 2018 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x
set -o pipefail

PREFIX="$1"

if [[ $_3PP_PLATFORM == mac* ]]; then
  XCODE_SDK_PATH=$(xcrun --show-sdk-path)
  # the min version is configured in mac_sdk.gni
  # https://crrev.com/e840c4b48a861be294f206bd694ebce986ddbb88/build/config/mac/mac_sdk.gni#28
  MACOSX_VERSION_MIN="10.13"
  MACOSX_FLAGS="-isysroot${XCODE_SDK_PATH} -mmacosx-version-min=${MACOSX_VERSION_MIN}"
  CFLAGS="${CFLAGS} ${MACOSX_FLAGS}"
  LDFLAGS="${LDFLAGS} ${MACOSX_FLAGS}"
fi

if [[ $_3PP_TOOL_PLATFORM != $_3PP_PLATFORM ]]; then
  # Cross compiling; rely on `ninja` in $PATH.
  python3 ./configure.py
  ninja -j $(nproc)
  # Can't run tests when cross-compiling.
else
  CFLAGS="${CFLAGS}" LDFLAGS="${LDFLAGS}" python3 ./configure.py --bootstrap
  ./ninja -j $(nproc) all

  if [[ $_3PP_PLATFORM == windows* ]]; then
    # Override the PATH to avoid using posix tools in ninja_test
    PATH="$(cygpath "${SYSTEMROOT}\\System32")" ./ninja_test
  else
    ./ninja_test
  fi
fi

if [[ $_3PP_PLATFORM == windows* ]]; then
  cp ninja.exe "$PREFIX"
else
  cp ninja "$PREFIX"
fi

# Add README.chromium file to the package.
THIS_DIR=$(dirname "$0")
cp "$THIS_DIR/README.chromium" "$PREFIX/README.chromium"
