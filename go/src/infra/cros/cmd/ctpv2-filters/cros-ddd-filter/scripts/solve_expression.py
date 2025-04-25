# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import csv
import typing
import os
import sys

sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from ddd_cli import DDDCli
from ddd_cli import SolvedResults
from enums import Categories
from enums import Pools
# from expression_generator import ExpressionGenerator
from inventory_pool import InventoryPool
from protos.ttcp.syntax import syntax_pb2

BQ_PROJECT = "chromeos-swarming"
SQL_FILE_PATH = "sql/hwids_for_pool.sql"


def get_eqc_device_counts(
    solved_results: SolvedResults,
) -> typing.Dict[str, typing.Dict[str, typing.Any]]:
  """

    Args:
      solved_results: Solved category results.
      pool: pool

    Returns:
      Dictionary where key=<eqc_name> value=<dict[total_count, hwid_counts]>
    """
  eqc_hwids = solved_results.get_hwids_by_eqcs()
  pool = InventoryPool(pool_name=solved_results.pool)
  hwids_in_pool = pool.get_hwid_counts()
  eqc_counts = {}

  for eqc, hwids in eqc_hwids.items():
    eqc_counts[eqc] = {"total_count": 0, "hwids": {}}
    for hwid in hwids:
      if hwid in hwids_in_pool:
        eqc_counts[eqc]["total_count"] += hwids_in_pool[hwid]
        eqc_counts[eqc]["hwids"].update({hwid: hwids_in_pool[hwid]})
  return eqc_counts


def get_eqc_counts(solved_results: SolvedResults,
                   pool: InventoryPool) -> typing.Tuple:
  """

    Args:
      solved_results: Solved category results.
      pool: pool

    Returns:

    """
  eqc_device_counts = get_eqc_device_counts(solved_results=solved_results)
  eqc_counts = {
      eqc_name: count_data["total_count"]
      for eqc_name, count_data in eqc_device_counts.items()
  }
  eqc_count_tup = sorted(eqc_counts.items(), key=lambda item: item[1])[::-1]
  return eqc_count_tup


def get_eqc_name_counts(solved_results: SolvedResults, eqc_name: str):
  eqc_device_counts = get_eqc_device_counts(solved_results=solved_results)
  return eqc_device_counts.get(eqc_name)


def generate_eqc_device_counts_csv(solved_results: SolvedResults,
                                   pool: InventoryPool, eqc_expression: str):
  eqc_counts_sorted = get_eqc_counts(solved_results=solved_results, pool=pool)
  eqc_data = [(eqc_expression, eqc_count_data[0], eqc_count_data[1], pool)
              for eqc_count_data in eqc_counts_sorted]
  with open("equiv_device_counts.csv", "w", encoding="UTF8") as f:
    writer = csv.writer(f)
    header = ["eqc_expression", "eqc_name", "device_count", "pool"]
    writer.writerow(header)
    for data in eqc_data:
      writer.writerow(data)


if __name__ == "__main__":
  pool = Pools._WIFICELL.value
  ddd_dir = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
  cli = DDDCli(ddd_dir=ddd_dir)
  # exp_gen = ExpressionGenerator()

  variant_expression = syntax_pb2.CategoryExpression(
      name=Categories._WIFIBTCHIPSET_SOC_KERNEL.value)

  # # Example 1
  # variant_expression = syntax_pb2.CategoryExpression(
  #     name="HWID:wireless_field:distinct_values"
  # )

  # # Example 2
  # wifichips_expression = syntax_pb2.CategoryExpression(
  #     name="HWID:wireless_field:distinct_values"
  # )
  # models_exp = syntax_pb2.CategoryExpression(
  #     name="dlm:model"
  # )
  # combo_list = [wifichips_expression, models_exp]
  # variant_expression = exp_gen.generate_combo_expression(sub_categories=combo_list)

  # # Example 3
  # exp_gen = ExpressionGenerator()
  # variant_expression = exp_gen.generate_enum_expression(name="dlm:build_targets:nissa")

  # # Example 4
  # wifichips_expression = syntax_pb2.CategoryExpression(
  #     name="HWID:wireless_field:distinct_values"
  # )
  # models_exp = syntax_pb2.CategoryExpression(
  #     name="dlm:model"
  # )
  # union_list = [wifichips_expression, models_exp]
  # variant_expression = exp_gen.generate_union_expression(sub_categories=union_list)

  solved_results = cli.execute_solve_expression(
      variant_expression=variant_expression,
      # inventory_swarming_pool=pool,
      # inventory_file_path="/usr/local/google/home/thandakas/chromiumos/src/config-internal/ttcp/scripts/mau_inventory.txt"
  )
  solved_results.print_eqc_targets()
