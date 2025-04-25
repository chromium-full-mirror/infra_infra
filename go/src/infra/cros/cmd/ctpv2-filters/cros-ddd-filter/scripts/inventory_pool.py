# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from collections import Counter
import os
import typing

from ddd_cli import SolvedResults
from google.cloud import bigquery

BQ_PROJECT = "chromeos-swarming"
SQL_FILE_PATH = "sql/hwids_for_pool.sql"


class InventoryPool:

  def __init__(self, pool_name: str):
    sql_path = os.path.join(os.path.dirname(__file__), SQL_FILE_PATH)
    with open(file=sql_path, mode="r", encoding="utf-8") as f:
      query = f.read().format(pool_name)
    self._pool_name = pool_name
    self._client = bigquery.Client(project=BQ_PROJECT)
    self._results = self._client.query(query=query)

  @property
  def pool_name(self):
    return self._pool_name

  @property
  def hwids(self) -> typing.List[str]:
    """Gets the ufs swarming hwids for a pool from BQ table.

        Args:
          pool: name of the swarming pool

        Returns:
          List of hwids in the pool
        """
    hwids = sorted(
        [result.values()[0] for result in self._results if result.values()[0]])
    return hwids

  def get_hwid_counts(self) -> Counter:
    """Gets the ufs swarming hwids for a pool from BQ table.

        Args:
          pool: name of the swarming pool

        Returns:
          List of hwids in the pool
        """
    return Counter(self.hwids)

  def get_diff_eqc_inventory(self,
                             solved_results: SolvedResults) -> typing.Set[str]:
    """Returns a diff in the sets of hwids from a solved results and the
        inventory pool

        Args:
          solved_results: solved results

        Returns:
          Diff in the sets of hwids from solved results and inventory pool
        """
    all_eqc_hwids = [
        hwid for hwids in solved_results.get_hwids_by_eqcs().values()
        for hwid in hwids
    ]
    return set(self.hwids) - set(all_eqc_hwids)
