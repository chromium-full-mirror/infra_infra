# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from google.appengine.ext import ndb


class RedirectIssue(ndb.Model):
  """Represents a issue redirect information."""
  ProjectName = ndb.StringProperty()
  MonorailLocalID = ndb.StringProperty()
  RedirectID = ndb.StringProperty()

  @classmethod
  def Get(cls, project: str, issue_local_id: int) -> str:
    key = project + ':' + str(issue_local_id)
    redirect_issue_entity = ndb.Key('RedirectIssue', key).get()
    if not redirect_issue_entity:
      return None
    return redirect_issue_entity.RedirectID
