#!/usr/bin/env python3
# Copyright 2021 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import json
import os


def do_latest():
  print('14.1.0-3')


def get_download_url():
  # These were found following the dependency chain in
  # https://packages.msys2.org/package/mingw-w64-x86_64-gcc?repo=mingw64.
  # Similarly, these could be found by running pacman -S mingw-w64-x86_64-gcc
  # and reading the list of packages being installed.
  # NOTE: these links will only be available until upstream package being
  # updated to newer version.
  packages = [
      "binutils-2.42-2-any.pkg.tar.zst",
      "crt-git-12.0.0.r32.gf977e1c38-1-any.pkg.tar.zst",
      "gcc-14.1.0-3-any.pkg.tar.zst",
      "gcc-libs-14.1.0-3-any.pkg.tar.zst",
      "gmp-6.3.0-2-any.pkg.tar.zst",
      "headers-git-12.0.0.r32.gf977e1c38-1-any.pkg.tar.zst",
      "isl-0.26-1-any.pkg.tar.zst",
      "libwinpthread-git-12.0.0.r32.gf977e1c38-1-any.pkg.tar.zst",
      "libiconv-1.17-4-any.pkg.tar.zst",
      "mpc-1.3.1-2-any.pkg.tar.zst",
      "mpfr-4.2.1-2-any.pkg.tar.zst",
      "zlib-1.3.1-1-any.pkg.tar.zst",
      "zstd-1.5.6-2-any.pkg.tar.zst",
      "windows-default-manifest-6.4-4-any.pkg.tar.zst",
      "winpthreads-git-12.0.0.r32.gf977e1c38-1-any.pkg.tar.zst",
  ]
  # Note: windows-arm64 will package the windows-amd64 build.
  url_prefix = "https://repo.msys2.org/mingw/mingw64/mingw-w64-x86_64-"
  urls = [url_prefix + p for p in packages]

  partial_manifest = {
      'url': urls,
      'name': packages,
      'ext': '.tar.zst',
  }
  print(json.dumps(partial_manifest))


def main():
  ap = argparse.ArgumentParser()
  sub = ap.add_subparsers(dest='action', required=True)

  latest = sub.add_parser("latest")
  latest.set_defaults(func=lambda _opts: do_latest())

  download = sub.add_parser("get_url")
  download.set_defaults(func=lambda _opts: get_download_url())

  opts = ap.parse_args()
  opts.func(opts)


if __name__ == '__main__':
  main()
