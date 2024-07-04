#!/bin/bash
# Copyright 2018 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x
set -o pipefail

PREFIX="$1"
DEPS_PREFIX="$2"

mkdir cmake-build
cd cmake-build

# On Windows, paths should use forward slash.
if [[ $_3PP_PLATFORM =~ windows-* ]]; then
  CMAKE_INSTALL_PREFIX="${PREFIX//\\/\/}"
  CMAKE_DEPS_PREFIX="${DEPS_PREFIX//\\/\/}"
else
  CMAKE_INSTALL_PREFIX="${PREFIX}"
  CMAKE_DEPS_PREFIX="${DEPS_PREFIX}"
fi

# On RISC-V, some runtime functions are in libatomic instead of libc.
if [[ $_3PP_PLATFORM == "linux-riscv64" ]]; then
  export LDFLAGS+=" -latomic"
fi

# Use the cmake in path to bootstrap cmak'ing cmake!
# Force CMAKE_CXX_STANDARD to 14 because C++17 is buggy on gcc10.
# See also: https://github.com/scikit-build/cmake-python-distributions/issues/221
# Disable framework tests on Mac because the builders only have the macOS SDK.
cmake .. \
  -GNinja \
  -DCMAKE_BUILD_TYPE:STRING=Release \
  -DCMAKE_INSTALL_PREFIX:STRING="${CMAKE_INSTALL_PREFIX}" \
  -DCMAKE_USE_OPENSSL:BOOL=ON \
  -DOPENSSL_ROOT_DIR:STRING="${CMAKE_DEPS_PREFIX}" \
  -DBUILD_TESTING:BOOL=ON \
  -DCMAKE_CXX_STANDARD:STRING=14 \
  -DCMake_TEST_XcFramework:BOOL=OFF

# Our dockcross environment should automatically set the CMAKE toolchain to
# enable cross-compilation.

# Build all the stuffs.
cmake --build . -j "$(nproc)"

# Run the test suite, if not cross-compiling.
# TODO(fancl): Fix tests on windows.
if [[ "$_3PP_PLATFORM" == "$_3PP_TOOL_PLATFORM" && "$_3PP_PLATFORM" != windows-* ]]; then
  # CMake.CheckSourceTree checks if the source tree is clean, which won't be
  # because we patch the code.
  #
  # RunCMake.CPack_STGZ (Self extracting Tar GZip compression) will generate a
  # shell script use pax instead of tar if possible to unpack itself. This is
  # fine in most cases since STGZ use restricted-pax format which is supported
  # by pax, but may break if libarchive considers restricted-pax can't store the
  # metadata (e.g. UID/GID too big) and uses unrestricted pax format instead.
  # Unfortunately our UID do exceed the limit, which will result in unpacking
  # extra PaxHeader in the unittest.
  #
  # curl test hits the internet and is flaky.
  #
  # SwiftOnly test doesn't seem to work with our mac version after
  # https://gitlab.kitware.com/cmake/cmake/-/merge_requests/9379/.

  # Tests which are flaky when run in parallel.
  SERIAL_TESTS="CTestLimitDashJ|FileDownload|CTestTimeoutAfterMatch|kwsys.testProcess-1|TryCompile|RunCMake.ctest_test"

  if [[ "$_3PP_PLATFORM" != mac-* ]]; then
      CTEST_PARALLEL="--parallel $(nproc)"
  else
      CTEST_PARALLEL=""  # parallel testing is too flaky on Mac
  fi

  # Unset CMAKE_TOOLCHAIN_FILE to avoid using host cmake libraries in tests
  env -u CMAKE_TOOLCHAIN_FILE \
    ./bin/ctest ${CTEST_PARALLEL} \
    --force-new-ctest-process \
    --stop-on-failure \
    --output-on-failure \
    --exclude-regex "(CMake.CheckSourceTree|RunCMake.CPack_STGZ|curl|BootstrapTest|SwiftOnly|${SERIAL_TESTS})"

  env -u CMAKE_TOOLCHAIN_FILE \
    ./bin/ctest \
    --force-new-ctest-process \
    --stop-on-failure \
    --output-on-failure \
    --tests-regex "(${SERIAL_TESTS})"
fi

# Use the system cmake to actually do the install. Otherwise it will use the
# cmake binary we just built and fail when we're cross compiling.
cmake .. -P cmake_install.cmake
