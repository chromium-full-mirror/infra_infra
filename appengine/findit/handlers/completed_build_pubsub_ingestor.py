# Copyright 2018 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""This serves as a handler for PubSub push for builds."""

import base64
import json
import logging
import six

from google.appengine.api import taskqueue
from google.protobuf.json_format import Parse

from go.chromium.org.luci.buildbucket.proto import notification_pb2
from go.chromium.org.luci.buildbucket.proto import common_pb2

from common.base_handler import BaseHandler, Permission
from common.waterfall.buildbucket_client import GetV2Build


class CompletedBuildPubsubIngestor(BaseHandler):
  """Adds isolate targets to the index when pubsub notifies of completed build.

  This handler is invoked at every status change of a build
  """

  PERMISSION_LEVEL = Permission.ANYONE  # Protected with login:admin.

  def HandlePost(self, **kwargs):
    build_id = None
    status = None
    builder_name = None
    try:
      envelope = self.request.get_json(force=True)
      # See the list of available 'attributes' at https://bit.ly/47fCmXC
      version = envelope['message']['attributes'].get('version')
      if version and (version != 'v1' and version != 'v2'):
        logging.info('Ignoring versions other than v1 & v2')
        return
      if not version or version == 'v1':
        build_id = envelope['message']['attributes']['build_id']
        build = json.loads(base64.b64decode(
            envelope['message']['data']))['build']
        status = build['status']
        parameters_json = json.loads(build['parameters_json'])
        builder_name = parameters_json['builder_name']

        # Legacy Buildbucket Status
        # Visit https://bit.ly/3nnra8P to understand the mapping to new status
        # enum
        if status == 'COMPLETED':
          # Filter the builds Findit doesn't have permission to read
          bb_build = GetV2Build(build_id)
          if not bb_build:
            logging.error('Failed to download build for %s/%r.', builder_name,
                          build_id)
            return
          _HandlePossibleCodeCoverageBuild(int(build_id))
        # We don't care about pending or non-supported builds, so we accept the
        # notification by returning 200, and prevent pubsub from retrying it.

      else:
        result = Parse(
            base64.b64decode(envelope['message']['data']),
            notification_pb2.BuildsV2PubSub(),
            ignore_unknown_fields=True)
        build_id = result.build.id
        status = result.build.status
        if (status
            & common_pb2.Status.ENDED_MASK == common_pb2.Status.ENDED_MASK):
          # We don't need to check if the build is accessible, as in the v2 we
          # add configuration so that we only receive the builds we care about,
          # instead of everything.
          _HandlePossibleCodeCoverageBuild(int(build_id))

    except (ValueError, KeyError) as e:
      # Ignore requests with invalid message.
      logging.debug('build_id: %r', build_id)
      logging.error('Unexpected PubSub message format: %s', six.text_type(e))
      logging.debug('Post body: %s', self.request.get_json(force=True))
      return


def _HandlePossibleCodeCoverageBuild(build_id):  # pragma: no cover
  """Schedules a taskqueue task to process the code coverage data."""
  # https://cloud.google.com/appengine/docs/standard/python/taskqueue/push/creating-tasks#target
  try:
    taskqueue.add(
        name='coveragedata-%s' % build_id,  # Avoid duplicate tasks.
        url='/coverage/task/process-data/build/%s' % build_id,
        target='code-coverage-backend',  # Always use the default version.
        queue_name='code-coverage-process-data')
  except (taskqueue.TombstonedTaskError, taskqueue.TaskAlreadyExistsError):
    logging.warning('Build %s was already scheduled to be processed', build_id)
