# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from .advisor import Advisor


class InventoryProperties(Advisor):
  CRITICAL_ERRORS = ()

  def __init__(
      self,
      command_path: str = None,
      inventory_swarming: bool = True,
      inventory_swarming_pool: str = None,
      inventory_file: str = None,
      outpath: str = None,
  ):
    options_map = {
        "inventoryswarming": inventory_swarming,
        "inventoryswarmingpool": inventory_swarming_pool,
        "inventoryfile": inventory_file,
        "outpathcsv": outpath,
    }
    super(InventoryProperties, self).__init__(
        command_path=command_path,
        sub_command="inventory_properties",
        options_map=options_map,
    )
