#!/usr/bin/env python3
# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import json
import os
import urllib.request


_PLATFORMS = {
    'windows-386': '.i386',
    'windows-amd64': '',
    'windows-arm64': '.arm64',
}


def do_latest():
  print(
      json.load(
          urllib.request.urlopen(
              'https://api.github.com/repos/ipxe/wimboot/releases/latest'))
      ['tag_name'])


def get_download_url(version, platform):
  if platform not in _PLATFORMS:
    raise ValueError(f'unsupported platform {platform}')

  extension = _PLATFORMS[platform]
  file = f'wimboot{extension}'
  url = (f'https://github.com/ipxe/wimboot/releases/download/{version}/{file}')

  manifest = {
      'url': [url],
      'ext': extension,
  }

  print(json.dumps(manifest))


def main():
  ap = argparse.ArgumentParser()
  sub = ap.add_subparsers(dest='action', required=True)

  latest = sub.add_parser("latest")
  latest.set_defaults(func=lambda _opts: do_latest())

  download = sub.add_parser("get_url")
  download.set_defaults(func=lambda opts: get_download_url(
      os.environ['_3PP_VERSION'], os.environ['_3PP_PLATFORM']))

  opts = ap.parse_args()
  opts.func(opts)


if __name__ == '__main__':
  main()
