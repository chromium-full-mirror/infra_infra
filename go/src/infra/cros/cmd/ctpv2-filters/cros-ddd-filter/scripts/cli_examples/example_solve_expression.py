# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Example usage of cli command solve_expression."""

import common

if __name__ == "__main__":
  pool = "DUT_POOL_QUOTA"
  category_expression = "HWID:wireless_field:distinct_values"
  cli = common.get_cli()
  result = cli.execute_solve_expression(
      variant_expression_map={"name": category_expression},
      inventory_swarming_pool=pool,
  )
  print(result)
