# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from google.cloud import datastore


def Get(project: str, issue_local_id: int) -> int:
  client = datastore.Client()
  key = project + ':' + str(issue_local_id)
  redirect_issue_entity = client.get(client.key('RedirectIssue', key))
  if not redirect_issue_entity:
    return None
  return int(redirect_issue_entity.get('RedirectID'))
