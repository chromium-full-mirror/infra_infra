# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

#!/usr/bin/env python3
import argparse
import json

# Update this when upgrading NDK.
_VERSION = "r27c"
_URL = f"https://dl.google.com/android/repository/android-ndk-{_VERSION}-linux.zip"


def do_latest():
  print(_VERSION)


def do_get_url():
  partial_manifest = {
      'url': [_URL],
      'ext': '.zip',
  }
  print(json.dumps(partial_manifest))


def main():
  ap = argparse.ArgumentParser()
  sub = ap.add_subparsers(required=True)

  latest = sub.add_parser("latest")
  latest.set_defaults(func=do_latest)

  download = sub.add_parser("get_url")
  download.set_defaults(func=do_get_url)

  opts = ap.parse_args()
  opts.func()


if __name__ == '__main__':
  main()
