#!/usr/bin/env python3
# Copyright 2024 The Chromium Authors
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
    'windows': '.iso',
}

_HTTP_HEADERS = {
    'User-Agent': 'curl/8.5.0',
}

_BASE_URL = 'https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/'

def do_latest():
  req = urllib.request.Request(_BASE_URL, headers=_HTTP_HEADERS)
  resp = urllib.request.urlopen(req)
  latest = re.findall(r'href=".*virtio-win-(\d+.\d+.\d+).iso"',
                      resp.read().decode('utf-8'))[0]
  print(latest)


def get_download_url(platform):
  if platform not in _PLATFORMS:
    raise ValueError(f'unsupported platform {platform}')

  extension = _EXTENSION[platform.split('-')[0]]
  file_name = f'virtio-win{extension}'
  url = (f'{_BASE_URL}{file_name}')

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
      os.environ['_3PP_PLATFORM']))

  opts = ap.parse_args()
  return opts.func(opts)


if __name__ == '__main__':
  sys.exit(main())
