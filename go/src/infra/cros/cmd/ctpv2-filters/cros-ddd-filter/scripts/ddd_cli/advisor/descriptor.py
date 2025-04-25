# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from .advisor import Advisor


class Descriptor(Advisor):
  CRITICAL_ERRORS = ("model .+ has no descriptor",)

  def __init__(
      self,
      model: str = None,
      command_path: str = None,
      outpath: str = None,
  ):
    options_map = {
        "model": model,
        "outpath": outpath,
    }
    super(Descriptor, self).__init__(
        command_path=command_path,
        sub_command="descriptor",
        options_map=options_map,
    )
