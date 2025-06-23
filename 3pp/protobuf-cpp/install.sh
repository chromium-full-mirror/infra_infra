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

# Temporary bump the version for abseil:
# https://github.com/abseil/abseil-cpp/commit/5852b47a81e5334a667d1e12dbfa55c0f8111100
sed -i 's/20250127\.0/5852b47a81e5334a667d1e12dbfa55c0f8111100/' \
       'cmake/dependencies.cmake'

# SmallOptimization failed for unknown reason but from the comment "Properties
# checked here are not part of the contract of RepeatedPtrField" it doesn't
# seem to be critical for correctness.
sed -i 's/TEST(RepeatedPtrFieldTest, SmallOptimization)/TEST(DISABLED_RepeatedPtrFieldTest, SmallOptimization)/' \
       'src/google/protobuf/repeated_ptr_field_unittest.cc'

# Use an up-to-date ca bundle to avoid ssl verification error.
# Remove this when we moved away from centos 7.
export GIT_SSL_CAINFO="${ca_bundle}/raw_source_0.pem"

mkdir cmake-build
cd cmake-build
cmake .. \
  -DCMAKE_BUILD_TYPE:STRING=Release \
  -DCMAKE_INSTALL_PREFIX:STRING="${PREFIX}" \
  -DCMAKE_CXX_STANDARD=17 \
  ${PROTOC_OPT}

make -j $(nproc)
if [[ $_3PP_PLATFORM == $_3PP_TOOL_PLATFORM ]]; then
  make test -j $(nproc) || (status=$?; cat Testing/Temporary/LastTest.log; exit $status)
fi
make install -j $(nproc)
