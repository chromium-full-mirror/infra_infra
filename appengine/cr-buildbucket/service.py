# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import datetime
import logging
import urlparse

from google.appengine.ext import ndb

from components import utils

import model
import user

MAX_RETURN_BUILDS = 100
DEFAULT_LEASE_DURATION = datetime.timedelta(minutes=1)


@ndb.tasklet
def get_async(build_id):
  """Gets a build by |build_id|.

  Requires the current user to have permissions to view the build.
  """
  build = yield model.Build.get_by_id_async(build_id)
  if not build:
    raise ndb.Return(None)
  if not (yield user.has_perm_async(user.PERM_BUILDS_GET, build.bucket_id)):
    raise user.current_identity_cannot('view build %s', build.key.id())
  raise ndb.Return(build)
