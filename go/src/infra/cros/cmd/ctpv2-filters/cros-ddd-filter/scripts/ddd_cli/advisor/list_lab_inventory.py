# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from .advisor import Advisor


class ListLabInventory(Advisor):
  CRITICAL_ERRORS = ()

  def __init__(
      self,
      command_path: str = None,
      outpath: str = None,
  ):
    options_map = {
        "outpath": outpath,
    }
    super(ListLabInventory, self).__init__(
        command_path=command_path,
        sub_command="list_lab_inventory",
        options_map=options_map,
    )
