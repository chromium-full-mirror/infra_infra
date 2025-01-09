#!/usr/bin/env python3
# Copyright 2022 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import json
import os
import urllib.request


def do_latest():
  # All releases of Kythe are "pre-release" so just getting "/latest" fails.
  # Leaving the code here for when Kythe starts making non-"pre-release"
  # releases.
  if False:
    print(
        json.load(
            urllib.request.urlopen(
                'https://api.github.com/repos/kythe/kythe/releases/latest'))
        ['tag_name'])

  # Get all releases (pre-release or not), look at the most recent one, and
  # grab the tag.
  print(
      json.load(
          urllib.request.urlopen(
              'https://api.github.com/repos/kythe/kythe/releases'
          )
      )[0]['tag_name']
  )


def get_download_url(version, platform):
  _ = platform  # Unused.

  url = (
      'https://github.com/kythe/kythe/releases/download/{0}/kythe-{0}.tar.gz'
  ).format(version)

  manifest = {
      'url': [url],
      'ext': '.tar.gz',
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
