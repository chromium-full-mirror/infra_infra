#!/bin/bash
# Copyright 2018 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x
set -o pipefail

PREFIX=$1
DEPS_PREFIX=$2

# generate configure, using libtool from DEPS_PREFIX
PATH=$DEPS_PREFIX/bin:$PATH ./autogen.sh

./configure --enable-static --disable-shared \
  --disable-docs \
  --host "$CROSS_TRIPLE" \
  --prefix "$PREFIX"
make install -j $(nproc)

# Some programs (like python) expect to be able to `#include <ffi.h>`, so
# create those symlinks. The newer libffi used by riscv64 does this during
# `make install`.
if [[ $_3PP_PLATFORM != "linux-riscv64" ]]; then
  mkdir $PREFIX/include
  (cd $PREFIX/include && ln -s ../lib/libffi*/include/*.h ./)
  (cd $PREFIX/lib && [ -z "$(ls ../lib64/* 2>/dev/null)" ] || ln -s ../lib64/* ./)
fi
