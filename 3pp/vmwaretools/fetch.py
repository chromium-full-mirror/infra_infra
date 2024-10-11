#!/usr/bin/env python3
# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import json
import os
import re
import sys
import urllib.request

_PLATFORMS = {
    'windows-amd64': 'x64',
}

_EXTENSION = {
    'windows': '.exe',
}

_HTTP_HEADERS = {
    'User-Agent': 'curl/8.5.0',
}

def do_latest():
  req = urllib.request.Request(
      'https://packages.vmware.com/tools/releases/latest/windows/',
      headers=_HTTP_HEADERS,
  )
  resp = urllib.request.urlopen(req)
  latest = re.findall(r'href=".*-(\d+.\d+.\d+-\d+).iso"',
                      resp.read().decode('utf-8'))[0]
  print(latest)


def get_download_url(version, platform):
  if platform not in _PLATFORMS:
    raise ValueError(f'unsupported platform {platform}')

  extension = _EXTENSION[platform.split('-')[0]]
  platform = _PLATFORMS[platform]

  base_url = f'https://packages.vmware.com/tools/releases/latest/windows/{platform}/'
  req = urllib.request.Request(base_url, headers=_HTTP_HEADERS)
  resp = urllib.request.urlopen(req)
  file_name = re.findall(rf'href="(VMware-tools-{version}-{platform}{extension})"',
                         resp.read().decode('utf-8'))[0]
  url = (f'{base_url}{file_name}')

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
