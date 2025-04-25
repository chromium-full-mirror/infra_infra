# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from ddd_cli.command import Command


class SolveExpression(Command):
  COMMAND_NAME = "solve_expression"
  CRITICAL_ERRORS = ("no inventory was provided",)

  def __init__(
      self,
      command_path: str = None,
      variants: dict = None,
      outpath: str = None,
      class_filter: str = None,
      variants_file: str = None,
      inventory_swarming: bool = True,
      inventory_swarming_pool: str = None,
      inventory_file: str = None,
  ):
    options_map = {
        "variants": variants,
        "classfilter": class_filter,
        "outpath": outpath,
        "variantsfile": variants_file,
        "inventoryswarming": inventory_swarming,
        "inventoryswarmingpool": inventory_swarming_pool,
        "inventoryfile": inventory_file,
    }
    options_map = {
        option_name: option_value
        for option_name, option_value in options_map.items()
        if option_value is not None
    }
    super(SolveExpression, self).__init__(
        command=self.COMMAND_NAME,
        command_path=command_path,
        options_map=options_map,
        critical_errors=self.CRITICAL_ERRORS,
    )
