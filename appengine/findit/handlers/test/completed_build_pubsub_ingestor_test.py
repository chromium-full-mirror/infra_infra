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
  def testPushPendingBuild(self, mock_process_coverage, *_):
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
                                    'status': 'STARTED',
                                }})))),
        },
    })
    response = self.test_app.post(
        '/index-isolated-builds?format=json', params=request_body)
    self.assertFalse(mock_process_coverage.called)
    self.assertEqual(200, response.status_int)

  @mock.patch.object(completed_build_pubsub_ingestor,
                     '_HandlePossibleCodeCoverageBuild')
  def testSucessfulPushBadFormat(self, mock_process_coverage, *_):
    request_body = json.dumps({
        'message': {},
    })
    response = self.test_app.post(
        '/index-isolated-builds?format=json', params=request_body)
    self.assertFalse(mock_process_coverage.called)
    self.assertEqual(200, response.status_int)

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
