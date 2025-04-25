# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Common helper module to be utilized by cli_examples scripts."""

import os
import sys

sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from ddd_cli import DDDCli


def get_cli() -> DDDCli:
  """Helper method for instantiating a default ddd cli object for examples

    Returns:
      ddd cli object
    """
  ddd_dir = os.path.dirname(
      os.path.dirname(os.path.dirname(os.path.realpath(__file__))))
  return DDDCli(ddd_dir=ddd_dir)
