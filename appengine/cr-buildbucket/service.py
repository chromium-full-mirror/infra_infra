# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import datetime
import logging
import urlparse

from google.appengine.api import taskqueue
from google.appengine.api import modules
from google.appengine.ext import deferred
from google.appengine.ext import ndb
from google.protobuf import struct_pb2

from components import auth
from components import utils
import gae_ts_mon

from go.chromium.org.luci.buildbucket.proto import common_pb2
import buildtags
import config
import errors
import events
import model
import search
import swarming
import user

MAX_RETURN_BUILDS = 100
DEFAULT_LEASE_DURATION = datetime.timedelta(minutes=1)


def validate_lease_key(lease_key):
  if lease_key is None:
    raise errors.InvalidInputError('Lease key is not provided')


def validate_url(url):
  if url is None:
    return
  if not isinstance(url, basestring):
    raise errors.InvalidInputError('url must be string')
  parsed = urlparse.urlparse(url)
  if not parsed.netloc:
    raise errors.InvalidInputError('url must be absolute')
  if parsed.scheme.lower() not in ('http', 'https'):
    raise errors.InvalidInputError(
        'Unexpected url scheme: "%s"' % parsed.scheme
    )


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


def _get_leasable_build(build_id, perm):
  assert perm in (user.PERM_BUILDS_LEASE, user.PERM_BUILDS_RESET), perm
  build = model.Build.get_by_id(build_id)
  if build is None:
    raise errors.BuildNotFoundError()
  if not user.has_perm(perm, build.bucket_id):
    action = 'reset' if perm == user.PERM_BUILDS_RESET else 'lease'
    raise user.current_identity_cannot('%s build %s', action, build.key.id())
  if build.is_luci:
    raise errors.InvalidInputError('cannot lease a swarmbucket build')
  return build


def lease(build_id, lease_expiration_date=None):
  """Leases the build, makes it unavailable for the leasing.

  Changes lease_key to a different value.

  After the lease expires, a cron task will make the build leasable again.

  Args:
    build_id (int): build id.
    lease_expiration_date (datetime.datetime): lease expiration date.
      Defaults to 10 seconds from now.

  Returns:
    Tuple:
      success (bool): True if the build was leased
      build (ndb.Build)
  """
  errors.validate_lease_expiration_date(lease_expiration_date)
  if lease_expiration_date is None:
    lease_expiration_date = utils.utcnow() + DEFAULT_LEASE_DURATION

  @ndb.transactional
  def try_lease():
    build = _get_leasable_build(build_id, user.PERM_BUILDS_LEASE)

    ok = False
    if not (build.proto.status != common_pb2.SCHEDULED or build.is_leased):
      ok = True
      build.lease_expiration_date = lease_expiration_date
      build.regenerate_lease_key()
      build.leasee = auth.get_current_identity()
      build.never_leased = False

    # we do a put() unconditionally so that the legacy status fields can
    # be updated in the pre-put hooks.
    build.put()
    return ok, build

  updated, build = try_lease()
  if updated:
    events.on_build_leased(build)
  return updated, build


def _check_lease(build, lease_key):
  if lease_key != build.lease_key:
    raise errors.LeaseExpiredError(
        'lease_key for build %s is incorrect. Your lease might be expired.' %
        build.key.id()
    )


def start(build_id, lease_key, url):
  """Marks build as STARTED. Idempotent.

  Args:
    build_id: id of the started build.
    lease_key: current lease key.
    url (str): a URL to a build-system-specific build, viewable by a human.

  Returns:
    The updated Build.
  """
  validate_lease_key(lease_key)
  validate_url(url)

  @ndb.transactional
  def txn():
    build = _get_leasable_build(build_id, user.PERM_BUILDS_LEASE)

    if build.proto.status == common_pb2.STARTED:
      if build.url == url:
        return False, build
      build.url = url
      build.put()
      return True, build

    if build.is_ended:
      raise errors.BuildIsCompletedError('Cannot start a completed build')

    assert build.proto.status == common_pb2.SCHEDULED

    _check_lease(build, lease_key)

    now = utils.utcnow()
    build.proto.start_time.FromDatetime(now)
    build.proto.status = common_pb2.STARTED
    build.status_changed_time = now
    build.url = url
    _fut_results(build.put_async(), events.on_build_starting_async(build))
    return True, build

  updated, build = txn()
  if updated:
    events.on_build_started(build)
  return build


def _fut_results(*futures):
  return [f.get_result() for f in futures]
