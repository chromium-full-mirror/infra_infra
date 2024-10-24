# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Test sysmon.cipd package by running it with tsmon disabled"""

import os
import subprocess
import sys

# .exe on Windows.
EXE_SUFFIX = '.exe' if sys.platform == 'win32' else ''


def main():
  sysmon = os.path.join(os.getcwd(), 'sysmon' + EXE_SUFFIX)
  return subprocess.call([
      sysmon,
      '-log-level', 'debug',
      '-sysmon-exit-after', '1s',
      '-ts-mon-config-file', '',
      '-ts-mon-endpoint', 'file://',
  ], executable=sysmon)


if __name__ == '__main__':
  sys.exit(main())
