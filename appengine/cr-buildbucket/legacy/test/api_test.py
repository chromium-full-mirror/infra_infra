# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import datetime
import json
import os
import sys

from parameterized import parameterized

from google.appengine.ext import ndb

from go.chromium.org.luci.buildbucket.proto import project_config_pb2
import bbutil

REPO_ROOT_DIR = os.path.abspath(
    os.path.join(os.path.realpath(__file__), '..', '..', '..', '..')
)
sys.path.insert(
    0, os.path.join(REPO_ROOT_DIR, 'luci', 'appengine', 'third_party_local')
)

from google.protobuf import json_format
from google.protobuf import text_format

from components import auth
from components import utils
from testing_utils import testing
import mock
import gae_ts_mon

from legacy import api
from legacy import api_common
from go.chromium.org.luci.buildbucket.proto import builds_service_pb2 as rpc_pb2
from go.chromium.org.luci.buildbucket.proto import common_pb2
from go.chromium.org.luci.buildbucket.proto import project_config_pb2
from test import test_util
from test.test_util import future, future_exception
import bbutil
import config
import creation
import errors
import model
import search
import service
import user


class V1ApiTest(testing.EndpointsTestCase):
  api_service_cls = api.BuildBucketApi

  test_bucket = None
  future_ts = None
  future_date = None

  def setUp(self):
    super(V1ApiTest, self).setUp()
    gae_ts_mon.reset_for_unittest(disable=True)
    auth.disable_process_cache()

    self.perms = test_util.mock_permissions(self)

    self.patch(
        'components.utils.utcnow', return_value=datetime.datetime(2017, 1, 1)
    )
    self.future_date = utils.utcnow() + datetime.timedelta(days=1)
    # future_ts is str because INT64 values are formatted as strings.
    self.future_ts = str(utils.datetime_to_timestamp(self.future_date))

    self.make_bucket('chromium', 'try', user.ALL_PERMISSIONS)

    self.build_infra = test_util.build_bundle(id=1).infra
    self.build_infra.put()

  def make_bucket(self, project, bucket, perms=None):
    test_util.put_empty_bucket(project, bucket)
    self.perms['%s/%s' % (project, bucket)] = list(perms or [])

  def expect_error(self, method_name, req, error_reason):
    res = self.call_api(method_name, req).json_body
    self.assertIsNotNone(res.get('error'))
    self.assertEqual(res['error']['reason'], error_reason)

  @mock.patch('service.get_async', autospec=True)
  def test_get(self, get_async):
    get_async.return_value = future(test_util.build(id=1))
    resp = self.call_api('get', {'id': '1'}).json_body
    get_async.assert_called_once_with(1)
    self.assertEqual(resp['build']['id'], '1')

  @mock.patch('service.get_async', autospec=True)
  def test_get_auth_error(self, get_async):
    get_async.return_value = future_exception(auth.AuthorizationError())
    self.expect_error('get', {'id': 1}, 'BUILD_NOT_FOUND')

  @mock.patch('service.get_async', autospec=True)
  def test_get_nonexistent_build(self, get_async):
    get_async.return_value = future(None)
    self.expect_error('get', {'id': 1}, 'BUILD_NOT_FOUND')


  ####### SEARCH ###############################################################

  @mock.patch('search.search_async', autospec=True)
  def test_search(self, search_async):
    build = test_util.build(id=1)
    search_async.return_value = future(([build], 'the cursor'))

    time_low = model.BEGINING_OF_THE_WORLD
    time_high = datetime.datetime(2120, 5, 4)
    req = {
        'bucket': ['luci.chromium.try'],
        'cancelation_reason': 'CANCELED_EXPLICITLY',
        'created_by': 'user:x@chromium.org',
        'result': 'CANCELED',
        'status': 'COMPLETED',
        'tag': ['important'],
        'retry_of': '42',
        'canary': True,
        'creation_ts_low': utils.datetime_to_timestamp(time_low),
        'creation_ts_high': utils.datetime_to_timestamp(time_high),
    }

    res = self.call_api('search', req).json_body

    search_async.assert_called_once_with(
        search.Query(
            bucket_ids=['chromium/try'],
            tags=req['tag'],
            status=search.StatusFilter.COMPLETED,
            result=model.BuildResult.CANCELED,
            failure_reason=None,
            cancelation_reason=model.CancelationReason.CANCELED_EXPLICITLY,
            created_by='user:x@chromium.org',
            max_builds=None,
            start_cursor=None,
            retry_of=42,
            canary=True,
            create_time_low=time_low,
            create_time_high=time_high,
        )
    )
    self.assertEqual(len(res['builds']), 1)
    self.assertEqual(res['builds'][0]['id'], '1')
    self.assertEqual(res['next_cursor'], 'the cursor')


  ####### ERRORS ###############################################################

  @mock.patch('service.get_async', autospec=True)
  def error_test(self, error_class, reason, get_async):
    get_async.return_value = future_exception(error_class(reason))
    self.expect_error('get', {'id': 123}, reason)

  def test_build_not_found_error(self):
    # pylint: disable=no-value-for-parameter
    self.error_test(errors.BuildNotFoundError, 'BUILD_NOT_FOUND')

  def test_invalid_input_error(self):
    # pylint: disable=no-value-for-parameter
    self.error_test(errors.InvalidInputError, 'INVALID_INPUT')

  def test_lease_expired_error(self):
    # pylint: disable=no-value-for-parameter
    self.error_test(errors.LeaseExpiredError, 'LEASE_EXPIRED')


class ConvertBucketTest(testing.AppengineTestCase):

  def setUp(self):
    super(ConvertBucketTest, self).setUp()
    test_util.put_empty_bucket('chromium', 'try')
    self.perms = test_util.mock_permissions(self)
    self.perms['chromium/try'] = [user.PERM_BUCKETS_GET]

  def test_convert_bucket_native(self):
    self.assertEqual(api.convert_bucket('chromium/try'), 'chromium/try')

  def test_convert_bucket_luci(self):
    self.assertEqual(api.convert_bucket('luci.chromium.try'), 'chromium/try')

  def test_convert_bucket_resolution_fails(self):
    with self.assertRaises(auth.AuthorizationError):
      api.convert_bucket('master.x')

  def test_convert_bucket_access_check(self):
    test_util.put_empty_bucket('chromium', 'secret')
    with self.assertRaises(auth.AuthorizationError):
      api.convert_bucket('secret')


class SwarmingTestCases(testing.AppengineTestCase):

  @parameterized.expand([
      ({'changes': 0},),
      ({'changes': [0]},),
      ({'changes': [{'author': 0}]},),
      ({'changes': [{'author': {}}]},),
      ({'changes': [{'author': {'email': 0}}]},),
      ({'changes': [{'author': {'email': ''}}]},),
      ({'changes': [{'author': {'email': 'a@example.com'}, 'repo_url': 0}]},),
      ({'swarming': []},),
      ({'swarming': {'junk': 1}},),
      ({'swarming': {'recipe': []}},),
  ])
  def test_validate_known_build_parameters(self, parameters):
    with self.assertRaises(errors.InvalidInputError):
      api.validate_known_build_parameters(parameters)

  def test_changes(self):
    changes = [
        dict(
            repo_url='https://chromium.googlsource.com/chromium/src',
            author=dict(email='a@example.com'),
        ),
        dict(
            repo_url='https://chromium.googlsource.com/chromium/src',
            author=dict(email='b@example.com'),
        ),
    ]
    put_req = api.PutRequestMessage(
        bucket='chromium/try',
        parameters_json=json.dumps(dict(changes=changes))
    )
    build_req = api.put_request_message_to_build_request(put_req, set())
    props = bbutil.struct_to_dict(build_req.schedule_build_request.properties)
    self.assertEqual(
        props['repository'], 'https://chromium.googlsource.com/chromium/src'
    )
    self.assertEqual(props['blamelist'], ['a@example.com', 'b@example.com'])
