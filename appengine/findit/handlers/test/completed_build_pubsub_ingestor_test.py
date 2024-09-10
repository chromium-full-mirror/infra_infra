# Copyright 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import base64
import json
import mock
import six

from flask import Flask

from google.appengine.api import taskqueue
from go.chromium.org.luci.buildbucket.proto.build_pb2 import Build
from testing_utils.testing import AppengineTestCase

from common.findit_http_client import FinditHttpClient
from common.waterfall import buildbucket_client
from handlers import completed_build_pubsub_ingestor
from model.isolated_target import IsolatedTarget


class CompletedBuildPubsubIngestorTest(AppengineTestCase):
  app_module = Flask(__name__)
  app_module.add_url_rule(
      '/index-isolated-builds',
      view_func=completed_build_pubsub_ingestor.CompletedBuildPubsubIngestor()
      .Handle,
      methods=['POST'])

  @mock.patch.object(completed_build_pubsub_ingestor,
                     '_HandlePossibleCodeCoverageBuild')
  @mock.patch.object(FinditHttpClient, 'Post')
  def testPushNoBuild(self, mock_post, *_):
    mock_headers = {'X-Prpc-Grpc-Code': '5'}
    mock_post.return_value = (404, 'Build not found', mock_headers)

    request_body = json.dumps({
        'message': {
            'attributes': {
                'build_id': '123456',
            },
            'data':
                six.ensure_str(
                    base64.b64encode(
                        six.ensure_binary(
                            json.dumps({
                                'build': {
                                    'project':
                                        'chromium',
                                    'bucket':
                                        'luci.chromium.ci',
                                    'status':
                                        'COMPLETED',
                                    'result':
                                        'SUCCESS',
                                    'parameters_json':
                                        '{"builder_name": "builder"}',
                                }
                            })))),
        },
    })
    response = self.test_app.post(
        '/index-isolated-builds?format=json', params=request_body, status=200)
    self.assertEqual(200, response.status_int)

  @mock.patch.object(completed_build_pubsub_ingestor,
                     '_HandlePossibleCodeCoverageBuild')
  @mock.patch.object(FinditHttpClient, 'Post')
  def testPushPendingBuild(self, mock_post, *_):
    request_body = json.dumps({
        'message': {
            'attributes': {
                'build_id': '123456',
            },
            'data':
                six.ensure_str(
                    base64.b64encode(
                        six.ensure_binary(
                            json.dumps({
                                'build': {
                                    'project':
                                        'chromium',
                                    'bucket':
                                        'luci.chromium.ci',
                                    'status':
                                        'PENDING',
                                    'parameters_json':
                                        '{"builder_name": "builder"}',
                                }
                            })))),
        },
    })
    response = self.test_app.post(
        '/index-isolated-builds?format=json', params=request_body)
    self.assertFalse(mock_post.called)
    self.assertEqual(200, response.status_int)

  @mock.patch.object(completed_build_pubsub_ingestor,
                     '_HandlePossibleCodeCoverageBuild')
  @mock.patch.object(FinditHttpClient, 'Post')
  def testSucessfulPushBadFormat(self, mock_post, *_):
    request_body = json.dumps({
        'message': {},
    })
    response = self.test_app.post(
        '/index-isolated-builds?format=json', params=request_body)
    self.assertFalse(mock_post.called)
    self.assertEqual(200, response.status_int)

  @mock.patch.object(completed_build_pubsub_ingestor,
                     '_HandlePossibleCodeCoverageBuild')
  @mock.patch.object(buildbucket_client, 'GetV2Build')
  @mock.patch.object(FinditHttpClient, 'Post')
  def testNonIsolateBuild(self, mock_post, mock_get_build, *_):
    # This build does not isolate any targets.
    mock_build = Build()
    mock_build.id = 8945610992972640896
    mock_build.status = 12
    mock_build.input.properties['builder_group'] = 'chromium.linux'
    mock_build.output.properties['buildername'] = 'Linux Tester'
    gitiles_commit = mock_build.input.gitiles_commit
    gitiles_commit.host = 'gitiles.host'
    gitiles_commit.project = 'gitiles/project'
    gitiles_commit.ref = 'refs/heads/mockmaster'
    mock_build.builder.project = 'mock_luci_project'
    mock_build.builder.bucket = 'mock_bucket'
    mock_build.builder.builder = 'Linux Tester'
    mock_headers = {'X-Prpc-Grpc-Code': '0'}
    binary_data = mock_build.SerializeToString()
    mock_post.return_value = (200, binary_data, mock_headers)
    mock_get_build.return_value = mock_build

    request_body = json.dumps({
        'message': {
            'attributes': {
                'build_id': str(mock_build.id),
            },
            'data':
                six.ensure_str(
                    base64.b64encode(
                        six.ensure_binary(
                            json.dumps({
                                'build': {
                                    'project':
                                        'chromium',
                                    'bucket':
                                        'luci.chromium.ci',
                                    'status':
                                        'COMPLETED',
                                    'parameters_json':
                                        '{"builder_name": "builder"}',
                                }
                            })))),
        },
    })
    response = self.test_app.post(
        '/index-isolated-builds?format=json', params=request_body)
    self.assertEqual(200, response.status_int)
    self.assertNotIn(six.ensure_binary('created_rows'), response.body)

  @mock.patch.object(completed_build_pubsub_ingestor,
                     '_HandlePossibleCodeCoverageBuild')
  @mock.patch.object(buildbucket_client, 'GetV2Build')
  @mock.patch.object(FinditHttpClient, 'Post')
  def testNoMasternameBuild(self, mock_post, mock_get_build, *_):
    mock_build = Build()
    mock_build.id = 8945610992972640896
    mock_build.status = 12
    mock_build.output.properties['buildername'] = 'Linux Builder'
    mock_build.output.properties.get_or_create_struct(
        'swarm_hashes_ref/heads/mockmaster(at){#123}'
    )['mock_target'] = 'mock_hash'
    gitiles_commit = mock_build.input.gitiles_commit
    gitiles_commit.host = 'gitiles.host'
    gitiles_commit.project = 'gitiles/project'
    gitiles_commit.ref = 'refs/heads/mockmaster'
    mock_build.builder.project = 'mock_luci_project'
    mock_build.builder.bucket = 'mock_bucket'
    mock_build.builder.builder = 'Linux Builder'
    mock_headers = {'X-Prpc-Grpc-Code': '0'}
    binary_data = mock_build.SerializeToString()
    mock_post.return_value = (200, binary_data, mock_headers)
    mock_get_build.return_value = mock_build

    request_body = json.dumps({
        'message': {
            'attributes': {
                'build_id': str(mock_build.id),
            },
            'data':
                six.ensure_str(
                    base64.b64encode(
                        six.ensure_binary(
                            json.dumps({
                                'build': {
                                    'project':
                                        'chromium',
                                    'bucket':
                                        'luci.chromium.ci',
                                    'status':
                                        'COMPLETED',
                                    'parameters_json':
                                        '{"builder_name": "builder"}',
                                }
                            })))),
        },
    })
    response = self.test_app.post(
        '/index-isolated-builds?format=json', params=request_body)
    self.assertEqual(200, response.status_int)
    self.assertNotIn(six.ensure_binary('created_rows'), response.body)

  @mock.patch.object(completed_build_pubsub_ingestor,
                     '_HandlePossibleCodeCoverageBuild')
  def testPushV2_InvokeCodeCoverage(self, mock_code_coverage, *_):
    request_body = json.dumps({
        'message': {
            'attributes': {
                'version': 'v2',
            },
            'data':
                six.ensure_str(
                    base64.b64encode(
                        six.ensure_binary(
                            json.dumps(
                                {'build': {
                                    'id': 123456,
                                    'status': 'SUCCESS',
                                }})))),
        },
    })
    response = self.test_app.post(
        '/index-isolated-builds?format=json', params=request_body)
    self.assertTrue(mock_code_coverage.called)
    self.assertEqual(200, response.status_int)
