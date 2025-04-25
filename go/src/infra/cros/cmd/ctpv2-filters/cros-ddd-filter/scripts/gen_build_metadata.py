# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Generates Build Metadata dataset"""

import json
import logging
import os
import re
import typing
import sys

from chromiumos_root import CHROMIUMOS_ROOT

sys.path.append(CHROMIUMOS_ROOT)
from chromite.lib import build_query
from chromite.lib import cros_build_lib

KERNEL_VERSIONS_SQL = "sql/kernel_versions.sql"
GENERATED_DATASET = "generated/build_metadata.jsonproto"


def get_kernel_by_board() -> dict[str, str]:
  """Runs F1 query to generated kernel version to board mapping

    Returns:
        board to kernel_version dictionary
    """
  sql_path = os.path.join(os.path.dirname(__file__), KERNEL_VERSIONS_SQL)
  with open(file=sql_path, mode="r", encoding="utf-8") as f:
    query = f.read()
  results = cros_build_lib.run(
      ["f1-sql", "--csv_output"],
      input=query,
      capture_output=True,
      encoding="utf-8",
  )
  board_kernel_ver_map = {}
  if results.returncode == 0:
    for row_index, build_ker in enumerate(results.stdout.split()):
      # Ignore the first header row of the output
      if row_index != 0:
        board, kernel_version = tuple(
            val.strip('"') for val in build_ker.split(","))
        board_kernel_ver_map[board] = kernel_version
  return board_kernel_ver_map


def parse_board_chipset(
    board_results: build_query.QueryTarget,) -> typing.Optional[str]:
  """Parses board chipset from query target results

    Args:
        board_results: board query results

    Returns:
        chipset
    """
  chipset_filter = [
      re.search("^chipset-(.+)", overlay.name).group(1)
      for overlay in board_results.overlays
      if (overlay.name.startswith("chipset-") and "private" not in overlay.name)
  ]
  if len(chipset_filter) == 0:
    logging.warning(
        f"WARN: Chipset data not found for board: {board_results.name}")
  return chipset_filter[0] if len(chipset_filter) != 0 else None


def parse_board_kernel_version(
    board_results: build_query.QueryTarget,) -> typing.Optional[str]:
  """Parses board kernel version from query target results

    Args:
        board_results: board query results

    Returns:
        kernel version or None if no kernel data is found
    """
  kernel_filter = [
      re.search("^kernel-(.+)", use_flag).group(1)
      for use_flag in board_results.use_flags
      if use_flag.startswith("kernel-")
  ]
  if len(kernel_filter) == 0:
    logging.warning(
        f"WARN: Kernel data not found for board: {board_results.name}")
  return (kernel_filter[0].replace("_", ".")
          if len(kernel_filter) != 0 else None)


def transform_jsonproto(build_metadata_list: list[dict[str, str]]) -> dict:
  """Transforms build metadata list to a jsonproto format

    Args:
        build_metadata_list: build metadata list

    Returns:
        jsonproto
    """
  jsonproto = {"values": []}
  for build_metadata in build_metadata_list:
    image_item = {
        "buildTarget": {
            "portageBuildTarget": {
                "overlayName": build_metadata["build_target"]
            }
        },
        "package_summary": {
            "chipset": {
                "overlay": build_metadata["chipset"]
            },
            "kernel": {
                "version": build_metadata["kernel_version"]
            },
        },
    }
    jsonproto["values"].append(image_item)
  return jsonproto


if __name__ == "__main__":
  build_metadata_list = []
  board_kernel_versions = get_kernel_by_board()
  all_boards = build_query.Query(build_query.Board).all()
  for board in all_boards:
    try:
      version = parse_board_kernel_version(board_results=board)
      build_metadata_item = {
          "build_target":
              board.name,
          "chipset":
              parse_board_chipset(board_results=board),
          "kernel_version":
              (board_kernel_versions[board.name]
               if version == "upstream" or version is None else version),
      }
      build_metadata_list.append(build_metadata_item)
    except Exception as err:
      logging.warning(f"Error in parsing board {board.name} chipset: {err}")
  build_metadata_list.sort(key=lambda image: image["build_target"])
  output_path = os.path.join(os.path.dirname(__file__), GENERATED_DATASET)
  with open(output_path, "w", encoding="utf-8") as f:
    json.dump(
        obj=transform_jsonproto(build_metadata_list=build_metadata_list),
        fp=f,
        ensure_ascii=False,
        indent=2,
    )
