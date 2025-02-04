#!/usr/bin/env python3
# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import json
import os
import packaging.version
import re
import ssl
import sys
import urllib.request

import certifi

# Make sure up-to-date root certificates are used.
urllib.request.install_opener(
    urllib.request.build_opener(
        urllib.request.HTTPSHandler(
            context=ssl.create_default_context(cafile=certifi.where()))))

MAJOR_VERSION = '8'

_EXTENSION = {
    'windows': '.msi',
}

_PLATFORMS = {
    'windows-amd64': 'x64',
}


def do_latest():
  request = urllib.request.urlopen(
      f'https://downloads.puppetlabs.com/windows/puppet{MAJOR_VERSION}/')
  highest = None
  href_re = re.compile(rf'href="puppet-agent-({MAJOR_VERSION}\.\d+\.\d+).*"')
  for m in href_re.finditer(request.read().decode('utf-8')):
    v = packaging.version.parse(m.group(1))
    if not highest or v > highest:
      highest = v
  print(highest)


def get_download_url(version, platform):
  if platform not in _PLATFORMS:
    raise ValueError(f'unsupported platform {platform}')
  extension = _EXTENSION[platform.split('-')[0]]
  platform = _PLATFORMS[platform]
  url = (f'https://downloads.puppetlabs.com/windows/puppet{MAJOR_VERSION}/'
         f'puppet-agent-{version}-{platform}{extension}')
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
  return opts.func(opts)


if __name__ == '__main__':
  sys.exit(main())
