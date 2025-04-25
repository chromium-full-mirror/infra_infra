# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from .advisor import Advisor


class Properties(Advisor):
  CRITICAL_ERRORS = ()

  def __init__(
      self,
      hwid: str = None,
      command_path: str = None,
      outpath: str = None,
  ):
    options_map = {
        "hwid": f'"{hwid}"',
        "outpath": outpath,
    }
    super(Properties, self).__init__(
        command_path=command_path,
        sub_command="properties",
        options_map=options_map,
    )
