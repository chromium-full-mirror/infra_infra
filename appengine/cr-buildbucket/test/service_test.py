# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import contextlib
import datetime

from components import auth
from components import utils
from google.appengine.ext import ndb
from google.appengine.ext.ndb import msgprop
from testing_utils import testing
import mock

from go.chromium.org.luci.buildbucket.proto import build_pb2
from go.chromium.org.luci.buildbucket.proto import common_pb2
from go.chromium.org.luci.buildbucket.proto import service_config_pb2
from test import test_util
from test.test_util import future
import config
import errors
import model
import notifications
import service
import swarming
import user


class BuildBucketServiceTest(testing.AppengineTestCase):

  TEST_BUCKETS = [
      'chromium/try',
      'chromium/luci',
      'chromium/master.foo',
      'chromium/master.bar',
  ]

  def setUp(self):
    super(BuildBucketServiceTest, self).setUp()

    self.current_identity = auth.Identity('service', 'unittest')
    self.patch(
        'components.auth.get_current_identity',
        side_effect=lambda: self.current_identity
    )
    self.now = datetime.datetime(2015, 1, 1)
    self.patch('components.utils.utcnow', side_effect=lambda: self.now)

    self.perms = test_util.mock_permissions(self)
    for b in self.TEST_BUCKETS:
      self.perms[b] = list(user.ALL_PERMISSIONS)

    test_util.put_empty_bucket('chromium', 'try')

    config.put_bucket(
        'chromium',
        'a' * 40,
        test_util.parse_bucket_cfg(
            '''
            name: "luci"
            swarming {
              builders {
                name: "linux"
                swarming_host: "chromium-swarm.appspot.com"
                build_numbers: YES
                recipe {
                  cipd_package: "infra/recipe_bundle"
                  cipd_version: "refs/heads/master"
                  name: "recipe"
                }
              }
            }
            '''
        ),
    )

    self.patch(
        'google.appengine.api.app_identity.get_default_version_hostname',
        autospec=True,
        return_value='buildbucket.example.com'
    )

    self.patch('tq.enqueue_async', autospec=True, return_value=future(None))
    self.patch(
        'config.get_settings_async',
        autospec=True,
        return_value=future(service_config_pb2.SettingsCfg())
    )
    self.patch(
        'swarming.cancel_task_transactionally_async',
        autospec=True,
        return_value=future(None)
    )

    self.patch('search.TagIndex.random_shard_index', return_value=0)

    test_util.build_bundle(id=1).infra.put()

  def mock_no_perm(self, perm):
    for perms in self.perms.values():
      perms.remove(perm)

  @staticmethod
  def classic_build(**build_proto_fields):
    build = test_util.build(**build_proto_fields)
    build.is_luci = False
    return build

  #################################### GET #####################################

  def test_get(self):
    self.classic_build(id=1).put()
    build = service.get_async(1).get_result()
    self.assertEqual(build, build)

  def test_get_nonexistent_build(self):
    self.assertIsNone(service.get_async(42).get_result())

  def test_get_with_auth_error(self):
    self.mock_no_perm(user.PERM_BUILDS_GET)
    self.classic_build(id=1).put()
    with self.assertRaises(auth.AuthorizationError):
      service.get_async(1).get_result()

  #################################### LEASE ###################################

  def lease(self, build_id, lease_expiration_date=None, expect_success=True):
    success, build = service.lease(
        build_id,
        lease_expiration_date=lease_expiration_date,
    )
    self.assertEqual(success, expect_success)
    return build

  def new_leased_build(self, **build_proto_fields):
    build = self.classic_build(**build_proto_fields)
    build.put()
    return self.lease(build.key.id())

  def test_lease(self):
    expiration_date = utils.utcnow() + datetime.timedelta(minutes=1)
    self.classic_build(id=1).put()
    build = self.lease(1, lease_expiration_date=expiration_date)
    self.assertTrue(build.is_leased)
    self.assertGreater(build.lease_expiration_date, utils.utcnow())
    self.assertEqual(build.leasee, self.current_identity)

  def test_lease_build_with_auth_error(self):
    self.mock_no_perm(user.PERM_BUILDS_LEASE)
    self.classic_build(id=1).put()
    with self.assertRaises(auth.AuthorizationError):
      self.lease(1)

  def test_cannot_lease_a_leased_build(self):
    self.new_leased_build(id=1)
    build = ndb.Key('Build', 1).get()
    self.lease(1, expect_success=False)
    after_build = ndb.Key('Build', 1).get()
    # make sure the NACK lease didn't change the build.
    self.assertEqual(build, after_build)

  def test_cannot_lease_a_nonexistent_build(self):
    with self.assertRaises(errors.BuildNotFoundError):
      service.lease(build_id=42)

  def test_cannot_lease_completed_build(self):
    build = self.classic_build(id=1, status=common_pb2.SUCCESS)
    build.put()
    self.lease(1, expect_success=False)

  def test_cannot_lease_luci_build(self):
    build = test_util.build(id=1)
    build.put()
    with self.assertRaises(errors.InvalidInputError):
      self.lease(1)


  ############################ UNREGISTER BUILDERS #############################

  def test_unregister_builders(self):
    model.Builder(
        id='chromium:try:linux_rel',
        last_scheduled=self.now - datetime.timedelta(weeks=8),
    ).put()
    service.unregister_builders()
    builders = model.Builder.query().fetch()
    self.assertFalse(builders)
