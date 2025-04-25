# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Generates DLM Devices Metadata dataset"""

import json
import os
import sys

sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from google.cloud import bigquery

BQ_PROJECT = "chromeos-swarming"


def generate_data(file_path: str, out_path: str):
  sql_path = os.path.join(os.path.dirname(__file__), file_path)
  output = os.path.join(os.path.dirname(__file__), out_path)
  with open(file=sql_path, mode="r", encoding="utf-8") as f:
    query = f.read()
  client = bigquery.Client(project=BQ_PROJECT)
  results = client.query(query=query)
  results_json = {"values": [dict(list(result.items())) for result in results]}
  with open(file=output, mode="w", encoding="utf-8") as f:
    json.dump(obj=results_json, fp=f, ensure_ascii=False, indent=2)


if __name__ == "__main__":
  data_sets = [("sql/dlm_devices.sql", "generated/dlm_devices.json")]
  for data_set in data_sets:
    generate_data(*data_set)
