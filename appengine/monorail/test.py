# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Sets up Monorail's test environment and runs tests."""

import os
import sys

import pytest

if __name__ == '__main__':
  # TODO(dtu): Re-enable -Werror once jinja2 is updated.
  args = []
  sys.exit(pytest.main(args + sys.argv[1:]))
