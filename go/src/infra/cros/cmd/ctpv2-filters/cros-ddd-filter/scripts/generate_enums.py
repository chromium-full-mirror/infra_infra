# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import json
import os
import re
import typing
import sys

sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from google.cloud import bigquery
import inflect

BQ_PROJECT = "chromeos-swarming"
SQL_FILE_PATH = "sql/swarming_pools.sql"
ENUM_PATH = "enums"


def get_static_categories() -> typing.List[str]:
  """Gets all static categories from datasets/static dir.

    Returns:
      List of static category names
    """
  all_categories = []
  ddd_dir = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
  ds_static_dir = os.path.join(ddd_dir, "datasets/static")

  static_filenames = [
      filename for filename in os.listdir(ds_static_dir)
      if filename.endswith(".json")
  ]

  for static_file in static_filenames:
    with open(file=os.path.join(ds_static_dir, static_file)) as f:
      data = json.load(fp=f)
      all_categories.extend(list(data.get("categories", {})))
  return all_categories


def get_ufs_swarming_pools() -> typing.List[str]:
  """Gets the ufs swarming pools from BQ table.

    Returns:
      List of swarming pools
    """
  sql_path = os.path.join(os.path.dirname(__file__), SQL_FILE_PATH)
  with open(file=sql_path, mode="r", encoding="utf-8") as f:
    query = f.read()
  client = bigquery.Client(project=BQ_PROJECT)
  results = client.query(query=query)
  pools = [result.values()[0] for result in results if result.values()[0]]
  return pools


def generate_enum_from_values(values: typing.List[str], enum_name: str) -> None:
  """Generates enum file from a list of values and specified enum class name

    Args:
      values: list of values of the enum attributes
      enum_name: Enum class name
    """
  enum_file = os.path.join(
      os.path.dirname(__file__), ENUM_PATH, f"{enum_name}.py")
  enum_map = get_value_map(values=values)

  with open(enum_file, "w") as f:
    f.write("# Copyright 2025 The Chromium Authors\n")
    f.write(
        "# Use of this source code is governed by a BSD-style license that can be\n"
    )
    f.write("# found in the LICENSE file.\n\n")
    f.write("from enum import Enum\n\n\n")
    f.write(f"class {enum_name.capitalize()}(Enum):\n")
    for enum_name, value in sorted(enum_map.items()):
      f.write(f"  _{enum_name.upper()} = '{value}'\n")


def get_value_map(values: typing.List[str]) -> typing.Dict[str, str]:
  """Genertates an value map from a list of values.

    Returns a dictionary where the key is a transformed into a valid python
    identifier of the value, and the dictionary value is the raw input value.

    Invalid characters for the transformed 'key' will be replaced with '_'.
    Number value will be replaced with their word representation (ex: '1' -> 'one')
    for the key.

    Example:
    ["input:1"] -> {"input_1": "input:1"}
    ["input@2", "input:2"] -> {"input_2": "input@2", "input__2": "input:2"}

    Args:
      values: list of input string values

    Returns:
      Dictionary with transformed key and raw input value
    """
  enum_map = {}
  infl = inflect.engine()

  for value in values:
    enum_name = (infl.number_to_words(int(value)) if value.isdigit() else value)
    enum_name = re.sub("\W+", "_", enum_name).lower()
    if enum_name in enum_map:
      max_underscore_len = len(max(re.findall("_+", enum_name)))
      enum_name = re.sub("_+", "_" * (max_underscore_len + 1), enum_name)
    enum_map[enum_name] = value
  return enum_map


if __name__ == "__main__":
  pools = get_ufs_swarming_pools()
  all_static_categories = get_static_categories()
  enum_list = [
      (get_static_categories, "categories"),
      (get_ufs_swarming_pools, "pools"),
  ]
  for func, enum_name in enum_list:
    generate_enum_from_values(values=sorted(func()), enum_name=enum_name)
