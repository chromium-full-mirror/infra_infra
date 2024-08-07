#!/usr/bin/env python3
# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import json
import os
import urllib.request


def do_latest():
  print(
      json.load(
          urllib.request.urlopen(
              'https://api.github.com/repos/google/googet/releases/latest'))
      ['tag_name'])


def get_download_url(version, ext=".zip"):
  url = ('https://github.com/google/googet/archive/refs/tags/{tag}{ext}'.format(
      tag=version, ext=ext))

  manifest = {
      'url': [url],
      'ext': ext,
  }

  print(json.dumps(manifest))


def main():
  ap = argparse.ArgumentParser()
  sub = ap.add_subparsers(dest='action', required=True)

  latest = sub.add_parser("latest")
  latest.set_defaults(func=lambda _opts: do_latest())

  download = sub.add_parser("get_url")
  download.set_defaults(
      func=lambda opts: get_download_url(os.environ['_3PP_VERSION']))

  opts = ap.parse_args()
  opts.func(opts)


if __name__ == '__main__':
  main()
