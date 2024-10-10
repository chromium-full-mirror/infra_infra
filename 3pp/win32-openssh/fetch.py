#!/usr/bin/env python3
# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import json
import os
import re
import urllib.request


def do_latest():
  print(
      json.load(
          urllib.request.urlopen(
              'https://api.github.com/repos/PowerShell/Win32-OpenSSH/releases/latest'
          ))['tag_name'])


_PLATFORMS = {
    'windows-386': 'Win32',
    'windows-amd64': 'Win64',
    'windows-arm64': 'ARM64',
}

_EXTENSION = {
    'windows': '.msi',
}


def get_download_url(version, platform):
  if platform not in _PLATFORMS:
    raise ValueError('unsupported platform {}'.format(platform))

  extension = _EXTENSION[platform.split('-')[0]]

  version_short = re.sub('p1-(?:Preview|Beta)$', '', version)

  url = (
      'https://github.com/PowerShell/Win32-OpenSSH/releases/download/{version}/'
      'OpenSSH-{platform}-{version_short}{extension}'.format(
          version=version,
          version_short=version_short,
          platform=_PLATFORMS[platform],
          extension=extension,
      ))

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
