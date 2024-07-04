#!/bin/bash
# Copyright 2021 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x
set -o pipefail

PREFIX="$1"

export CXXFLAGS+=" -fPIC"

# On RISC-V, some runtime functions are in libatomic instead of libc.
if [[ $_3PP_PLATFORM == "linux-riscv64" ]]; then
  export LDFLAGS+=" -latomic"
fi

PROTOC_OPT=
if [[ $_3PP_PLATFORM != $_3PP_TOOL_PLATFORM ]]; then  # cross compiling
  PROTOC_OPT="-Dprotobuf_BUILD_TESTS=OFF -DABSL_BUILD_TESTING=OFF"
fi

# Ugly Workaround for https://github.com/abseil/abseil-cpp/issues/1528
# Not sure what changed in cmake but after v3.30.0 targets' dependencies are
# always expanded, causing absl_cc_library for test targets failed.
sed -i 's/absl::random_internal_mock_overload_set$//' \
       'third_party/abseil-cpp/absl/random/CMakeLists.txt'
sed -i 's/GTest::gmock$//' \
       'third_party/abseil-cpp/absl/container/CMakeLists.txt'

mkdir cmake-build
cd cmake-build
cmake .. \
  -DCMAKE_BUILD_TYPE:STRING=Release \
  -DCMAKE_INSTALL_PREFIX:STRING="${PREFIX}" \
  -DCMAKE_CXX_STANDARD=14 \
  ${PROTOC_OPT}

make -j $(nproc)
if [[ $_3PP_PLATFORM == $_3PP_TOOL_PLATFORM ]]; then
  make test -j $(nproc)
fi
make install -j $(nproc)
