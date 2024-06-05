#!/bin/bash
# Copyright 2018 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x
set -o pipefail

PREFIX="$1"
export CFLAGS="${CFLAGS} -fPIC"

# TODO(iannucci): Remove this (and the patch to enable this) once the fleet
# is using GLIBC 2.25 or higher. Currently the bulk of the fleet runs on
# Ubuntu 16.04, which as of this comment, uses GLIBC 2.23.
#
# This ALSO affects OS X on 10.11 and under when compiling with a newer version
# of XCode, EVEN if MACOSX_DEPLOYMENT_TARGET is 10.10.
#
# OpenSSL links against getentropy as a weak symbol... but unfortunately
# when we compile executables such as `git` and `python` against this static
# OpenSSL lib, the 'weakness' of this symbol is destroyed, and the linker
# immediately resolves it. On linux-amd64 this is not a problem, because we
# use the 'manylinux1' based docker containers, which have very old libc.
#
# However there's no manylinux equivalent for arm, and the Dockcross
# containers currently use a linux version which has a modern enough version
# of glibc to resolve getentropy, causing problems at runtime for
# linux-arm64 bots.
#
# When getentropy is not available, OpenSSL falls back to getrandom.
ARGS="-DNO_GETENTROPY=1"

case $_3PP_PLATFORM in
  windows-*)
    PTHREAD=""
    PERL="perl.bat"
    # Move /usr/bin to the end of PATH because otherwise nmake will use
    # /usr/bin/link, which doesn't work, instead of the MSVC linker.
    PATH=$(echo $PATH | sed 's/:\/usr\/bin//g'):/usr/bin
    ;;
  *)
    PTHREAD="-lpthread"
    PERL="perl"
    ;;
esac

# Unset ${perl} from pkgbuild. This will confuse Configure.
unset perl

case $_3PP_PLATFORM in
  mac-amd64)
    TARGET=darwin64-x86_64-cc
    ;;
  mac-arm64)
    TARGET=darwin64-arm64-cc
    ;;
  linux-armv6l)
    TARGET=linux-armv4
    ;;
  linux-riscv64)
    TARGET=linux64-riscv64
    ;;
  linux-*)
    TARGET="linux-${CROSS_TRIPLE%%-*}"
    ;;
  windows-amd64)
    TARGET="VC-WIN64A"
    ;;
  windows-386)
    TARGET="VC-WIN32"
    ;;
  windows-arm64)
    TARGET="VC-WIN64-ARM"
    ;;
  *)
    echo IDKWTF
    exit 1
    ;;
esac

echo PATH=$PATH
${PERL} Configure $PTHREAD --prefix="$PREFIX" --cross-compile-prefix= \
  no-shared $ARGS "$TARGET"

case $_3PP_PLATFORM in
  windows-*)
    nmake
    if [[ $_3PP_PLATFORM == $_3PP_TOOL_PLATFORM ]]; then # not cross-compiling
      # openssl tests require windows tools.
      # TODO(fancl): add a "system" build dependency to replace ad-hoc fixes.
      PATH="$(cygpath "${SYSTEMROOT}\\System32"):${PATH}" nmake test
    fi
    nmake install_sw
    ;;
  *)
    make -j "$(nproc)"
    if [[ $_3PP_PLATFORM == $_3PP_TOOL_PLATFORM ]]; then # not cross-compiling
      make test
    fi
    make install_sw
    ;;
esac

# pkg-config will have the original build prefix, which is not useful
# for relocatable packages. Remove the configs completely.
rm -rf $PREFIX/lib/pkgconfig
