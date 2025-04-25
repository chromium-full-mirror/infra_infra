# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Example usage of cli command advisor->properties."""

import common


def get_wireless_field_comp_id(components: dict) -> str:
  """Helper method to access the wireless_field component id value.

    Args:
      components(dict): raw dictionary containing the HWID components

    Returns:
      comp_id(str)

    """
  comp_id = ""
  wireless_field_comps = components.get("wireless_field")
  if wireless_field_comps:
    comp_ids = [
        comp["ID"]
        for comp_val in wireless_field_comps
        for comp in comp_val["ComponentsValues"]
    ]
  comp_id = comp_ids[0] if comp_ids else comp_id
  return comp_id


if __name__ == "__main__":
  hwid = "BOXY-GLMX B2C-B2D-A2A-A2E-A8L"
  cli = common.get_cli()
  result = cli.execute_properties(hwid=hwid)
  wireless_comp_id = get_wireless_field_comp_id(components=result["Components"])
  print(wireless_comp_id)
