# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from google.cloud import datastore


def Get(project: str, template_name: str) -> tuple[str, str]:
  client = datastore.Client()
  key = project + ':' + template_name
  entity = client.get(client.key('RedirectProjectTemplate', key))
  if not entity:
    return None, None
  return entity.get('RedirectComponentID'), entity.get('RedirectTemplateID')
