#!/bin/bash
# Copyright 2022 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x
set -o pipefail

PREFIX="$1"
BASE_DIR=$(pwd)
BIN_EXT=""

if [[ $_3PP_PLATFORM =~ windows-.*  ]]; then
  # Move /usr/bin to the end of PATH because otherwise nmake will use
  # /usr/bin/link, which doesn't work, instead of the MSVC linker.
  PATH=$(echo $PATH | sed 's/:\/usr\/bin//g'):/usr/bin
  BIN_EXT=".exe"
  if [[ $_3PP_PLATFORM = "windows-386" ]]; then
    BUILD_DIR=x86
  elif [[ $_3PP_PLATFORM = "windows-arm64" ]]; then
    BUILD_DIR=arm64
  else
    BUILD_DIR=x64
  fi
  BUILD_CMD="nmake PLATFORM=$BUILD_DIR NO_ASM_GNU=1"
elif [[ $_3PP_PLATFORM =~ mac-.* ]]; then
  BUILD_CMD="make -j -f ../../cmpl_clang.mak"
  BUILD_DIR="b/c"
elif [[ $_3PP_PLATFORM = "linux-amd64" ]]; then
  BUILD_CMD="make -j -f ../../cmpl_gcc.mak"
  BUILD_DIR="b/g"
else
  echo "Unsupported architecture: $_3PP_PLATFORM"
  exit 1
fi

cd $BASE_DIR/CPP/7zip/Bundles/Alone
$BUILD_CMD
cp $BUILD_DIR/7za$BIN_EXT $PREFIX

cd $BASE_DIR/CPP/7zip/Bundles/Alone2
$BUILD_CMD
cp $BUILD_DIR/7zz$BIN_EXT $PREFIX

cd $BASE_DIR/CPP/7zip/Bundles/Alone7z
$BUILD_CMD
cp $BUILD_DIR/7zr$BIN_EXT $PREFIX
