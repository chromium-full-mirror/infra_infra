# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from google.cloud import datastore


def GetHotlist(project: str, label: str) -> str:
  client = datastore.Client()
  key = project + ':' + label
  entity = client.get(client.key('RedirectCustomLabelsToHotlists', key))
  if not entity:
    return None
  return entity.get('HotlistId')


def GetCustomFieldMap() -> dict[str, str]:
  custom_fields_map = {}
  client = datastore.Client()
  results = client.query(kind='RedirectToCustomFields').fetch()
  for result in results:
    custom_fields_map[result.key.name] = {
        'monorail_prefix': result.get('MonorailPrefix'),
        'custom_field_id': result.get('CustomFieldId'),
        'expected_value_type': result.get('ExpectedValueType'),
        'process_redirect_value': result.get('ProcessRedirectValue')
    }
  return custom_fields_map
