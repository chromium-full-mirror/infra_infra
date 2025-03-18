# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import functools

from google.cloud import datastore


# In local testing, a cache entry was ~350 bytes, so 500k entries is < 200 MB.
@functools.lru_cache(maxsize=500000)
def Get(project: str, issue_local_id: int) -> int:
  client = datastore.Client()
  key = project + ':' + str(issue_local_id)
  redirect_issue_entity = client.get(client.key('RedirectIssue', key))
  if not redirect_issue_entity:
    return None
  return int(redirect_issue_entity.get('RedirectID'))
