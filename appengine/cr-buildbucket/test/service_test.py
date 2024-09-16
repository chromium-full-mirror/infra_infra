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
