# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

#!/usr/bin/env python3
import argparse
import os
import re
import sys
import json
import urllib.request

import packaging.version



def do_latest():
  page_data = urllib.request.urlopen("http://http.us.debian.org/debian/pool/main/q/qemu/")
  highest = None
  href_re = re.compile(r'href="qemu-user-static_([\d.]+)(\+.*)_amd64.deb"')
  for m in href_re.finditer(page_data.read().decode('utf-8')):
    try:
      v = packaging.version.parse(m.group(1))
    except packaging.version.InvalidVersion:
      print(m)
      continue
    if not highest or v > highest[0]:
      highest = (v, m.group(1) + m.group(2))
  print(highest[1])


def do_get_url():
  pkg = f"qemu-user-static_{os.environ['_3PP_VERSION']}_amd64.deb"
  url = f"http://http.us.debian.org/debian/pool/main/q/qemu/{pkg}"
  partial_manifest = {
      'url': [url],
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
