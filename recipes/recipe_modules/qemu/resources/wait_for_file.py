#!/usr/bin/python3

# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import os
import pathlib
import sys
import time

# Waits for the given file to become available


def main():
  desc = ''' Wait for a file to be created. Exits as soon as it sees the
             file on the filesystem
            '''
  parser = argparse.ArgumentParser(description=desc)
  parser.add_argument(
      '-t', '--timeout', type=int, help='Timeout in seconds', default=10)
  parser.add_argument(
      'file',
      metavar='/file/path/to/watch',
      type=pathlib.Path,
      help='Path to the file to watch')

  args = parser.parse_args()
  print(args)

  start = time.monotonic()
  while time.monotonic() - start < args.timeout:
    if args.file.exists():
      sys.exit(os.EX_OK)
    else:
      time.sleep(0.01)
  sys.exit(os.EX_UNAVAILABLE)


main()
