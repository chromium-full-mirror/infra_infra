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


def unregister_builders():
  """Unregisters builders that didn't have builds for 4 weeks."""
  threshold = utils.utcnow() - model.BUILDER_EXPIRATION_DURATION
  q = model.Builder.query(model.Builder.last_scheduled < threshold)
  keys = q.fetch(keys_only=True)
  if keys:  # pragma: no branch
    logging.warning('unregistered builders: %s', [k.id() for k in keys])
    ndb.delete_multi(keys)


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
