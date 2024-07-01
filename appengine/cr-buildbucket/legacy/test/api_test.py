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

  ####### PUT ##################################################################

  @mock.patch('creation.add_async', autospec=True)
  def test_put(self, add_async):
    build = test_util.build(id=1, tags=[dict(key='a', value='b')])
    add_async.return_value = future(build)
    props = {'foo': 'bar'}
    parameters_json = json.dumps({
        api_common.BUILDER_PARAMETER: 'linux',
        api_common.PROPERTIES_PARAMETER: props,
    })
    req = {
        'client_operation_id': '42',
        'bucket': 'luci.chromium.try',
        'tags': ['a:b'],
        'parameters_json': parameters_json,
        'pubsub_callback': {
            'topic': 'projects/foo/topic/bar',
            'user_data': 'hello',
            'auth_token': 'secret',
        },
    }
    resp = self.call_api('put', req).json_body
    add_async.assert_called_once_with(
        creation.BuildRequest(
            schedule_build_request=rpc_pb2.ScheduleBuildRequest(
                builder=dict(
                    project='chromium',
                    bucket='try',
                    builder='linux',
                ),
                tags=[dict(key='a', value='b')],
                request_id='42',
                notify=dict(
                    pubsub_topic='projects/foo/topic/bar',
                    user_data='hello',
                ),
                properties=bbutil.dict_to_struct(props),
            ),
            parameters={},
            pubsub_callback_auth_token='secret',
        )
    )
    self.assertEqual(resp['build']['id'], '1')
    self.assertEqual(resp['build']['bucket'], req['bucket'])
    self.assertIn('a:b', resp['build']['tags'])

  @mock.patch('creation.add_async', autospec=True)
  def test_put_with_commit(self, add_async):
    buildset = (
        'commit/gitiles/gitiles.example.com/chromium/src/+/'
        'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
    )
    buildset_tag = 'buildset:' + buildset
    gitiles_ref_tag = 'gitiles_ref:refs/heads/master'

    gitiles_commit = common_pb2.GitilesCommit(
        host='gitiles.example.com',
        project='chromium/src',
        ref='refs/heads/master',
        id='aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    )
    build = test_util.build(
        id=1,
        input=dict(gitiles_commit=gitiles_commit),
        tags=[dict(key='t', value='0')],
    )
    build.tags.append(buildset_tag)
    build.tags.append(gitiles_ref_tag)
    build.tags.sort()
    add_async.return_value = future(build)

    req = {
        'client_operation_id': '42',
        'bucket': 'luci.chromium.try',
        'tags': [buildset_tag, gitiles_ref_tag, 't:0'],
        'parameters_json': json.dumps({api_common.BUILDER_PARAMETER: 'linux'}),
    }
    resp = self.call_api('put', req).json_body
    add_async.assert_called_once_with(
        creation.BuildRequest(
            schedule_build_request=rpc_pb2.ScheduleBuildRequest(
                builder=dict(
                    project='chromium',
                    bucket='try',
                    builder='linux',
                ),
                gitiles_commit=gitiles_commit,
                tags=[dict(key='t', value='0')],
                request_id='42',
                properties=dict(),
            ),
            parameters={},
        )
    )
    self.assertEqual(resp['build']['id'], '1')
    self.assertIn(buildset_tag, resp['build']['tags'])
    self.assertIn(gitiles_ref_tag, resp['build']['tags'])
    self.assertIn('t:0', resp['build']['tags'])

  @mock.patch('creation.add_async', autospec=True)
  def test_put_with_gerrit_change(self, add_async):
    buildset = 'patch/gerrit/gerrit.example.com/1234/5'
    buildset_tag = 'buildset:' + buildset

    props = {'patch_project': 'repo'}
    expected_sbr = rpc_pb2.ScheduleBuildRequest(
        builder=dict(
            project='chromium',
            bucket='try',
            builder='linux',
        ),
        gerrit_changes=[
            dict(
                host='gerrit.example.com',
                project='repo',
                change=1234,
                patchset=5,
            )
        ],
        tags=[dict(key='t', value='0')],
        request_id='42',
        properties=bbutil.dict_to_struct(props),
    )
    expected_request = creation.BuildRequest(
        schedule_build_request=expected_sbr,
        parameters={},
    )

    build = test_util.build(
        id=1,
        input=dict(
            gerrit_changes=expected_sbr.gerrit_changes,
            properties=expected_sbr.properties,
        ),
        tags=expected_sbr.tags,
    )
    build.tags.append(buildset_tag)
    build.tags.sort()
    add_async.return_value = future(build)

    req = {
        'client_operation_id':
            '42',
        'bucket':
            'luci.chromium.try',
        'tags': [buildset_tag, 't:0'],
        'parameters_json':
            json.dumps({
                api_common.BUILDER_PARAMETER: 'linux',
                api_common.PROPERTIES_PARAMETER: props,
            }),
    }
    resp = self.call_api('put', req).json_body
    add_async.assert_called_once_with(expected_request)
    self.assertEqual(resp['build']['id'], '1')
    self.assertIn(buildset_tag, resp['build']['tags'])
    self.assertIn('t:0', resp['build']['tags'])

  @mock.patch('creation.add_async', autospec=True)
  def test_put_with_v2_gerrit_changes(self, add_async):
    changes = [
        common_pb2.GerritChange(
            host='chromium.googlesource.com',
            project='project',
            change=1,
            patchset=1,
        ),
        common_pb2.GerritChange(
            host='chromium.googlesource.com',
            project='project',
            change=2,
            patchset=1,
        ),
    ]
    expected_sbr = rpc_pb2.ScheduleBuildRequest(
        builder=dict(
            project='chromium',
            bucket='try',
            builder='linux',
        ),
        properties=dict(),
        gerrit_changes=changes,
    )
    expected_request = creation.BuildRequest(
        schedule_build_request=expected_sbr,
        parameters={},
    )

    add_async.return_value = future(test_util.build(id=1))

    params = {
        api_common.BUILDER_PARAMETER: 'linux',
        'gerrit_changes': [json_format.MessageToDict(c) for c in changes],
    }
    req = {
        'bucket': 'luci.chromium.try',
        'parameters_json': json.dumps(params),
    }
    self.call_api('put', req)
    add_async.assert_called_once_with(expected_request)

  @mock.patch('creation.add_async', autospec=True)
  def test_put_with_generic_buildset(self, add_async):
    tags = [
        dict(key='buildset', value='x'),
        dict(key='t', value='0'),
    ]
    build = test_util.build(id=1, tags=tags)
    add_async.return_value = future(build)

    req = {
        'client_operation_id': '42',
        'bucket': 'luci.chromium.try',
        'tags': ['buildset:x', 't:0'],
        'parameters_json': json.dumps({api_common.BUILDER_PARAMETER: 'linux'}),
    }
    resp = self.call_api('put', req).json_body
    add_async.assert_called_once_with(
        creation.BuildRequest(
            schedule_build_request=rpc_pb2.ScheduleBuildRequest(
                builder=dict(
                    project='chromium',
                    bucket='try',
                    builder='linux',
                ),
                tags=tags,
                request_id='42',
                properties=dict(),
            ),
            parameters={},
        )
    )
    self.assertEqual(resp['build']['id'], '1')
    self.assertIn('buildset:x', resp['build']['tags'])
    self.assertIn('t:0', resp['build']['tags'])

  def test_put_with_invalid_request(self):
    req = {
        'bucket': 'luci.chromium.try',
        'client_operation_id': 'slash/is/forbidden',
    }
    self.expect_error('put', req, 'INVALID_INPUT')

  def test_put_with_non_dict_properties(self):
    parameters = {
        api_common.PROPERTIES_PARAMETER: [],
    }
    req = {
        'bucket': 'luci.chromium.try',
        'parameters_json': json.dumps(parameters),
    }
    self.expect_error('put', req, 'INVALID_INPUT')

  @mock.patch('creation.add_async', autospec=True)
  def test_put_with_leasing(self, add_async):
    expiration = utils.utcnow() + datetime.timedelta(hours=1)
    build = test_util.build(id=1)
    build.lease_expiration_date = expiration
    add_async.return_value = future(build)
    req = {
        'bucket': 'luci.chromium.try',
        'lease_expiration_ts': str(utils.datetime_to_timestamp(expiration)),
    }
    resp = self.call_api('put', req).json_body
    build_req = add_async.call_args[0][0]
    self.assertEqual(build_req.lease_expiration_date, expiration)
    self.assertEqual(
        resp['build']['lease_expiration_ts'], req['lease_expiration_ts']
    )

  def test_put_with_malformed_parameters_json(self):
    req = {
        'bucket': 'luci.chromium.try',
        'parameters_json': '}non-json',
    }
    self.expect_error('put', req, 'INVALID_INPUT')

  @mock.patch('creation.add_async', autospec=True)
  def test_put_empty_request(self, add_async):
    add_async.return_value = future_exception(errors.InvalidInputError())
    req = {'bucket': ''}
    self.expect_error('put', req, 'INVALID_INPUT')

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
