# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

#!/usr/bin/env python3
import argparse
import sys
import json

# Update these when upgrading QEMU static.
_VERSION = "9.0.2"
_DEB_PACKAGE = f"qemu-user-static_{_VERSION}+ds-1_amd64.deb"
_URL = f"http://http.us.debian.org/debian/pool/main/q/qemu/{_DEB_PACKAGE}"


def do_latest():
  print(_VERSION)


def do_get_url():
  partial_manifest = {
      'url': [_URL],
      'ext': '.deb',
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
  sys.exit(main())
