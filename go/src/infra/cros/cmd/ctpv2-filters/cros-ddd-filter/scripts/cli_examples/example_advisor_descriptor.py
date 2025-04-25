# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Example usage of cli command advisor->descriptor."""

import common

if __name__ == "__main__":
  cli = common.get_cli()
  result = cli.execute_descriptor(model="redrix")
  print(result)
