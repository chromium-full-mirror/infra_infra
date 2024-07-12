# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import contextlib
import copy
import functools
import json
import logging

from google.appengine.ext import ndb
from google.protobuf import json_format
from protorpc import messages
from protorpc import message_types
from protorpc import remote
import endpoints

from components import auth
from components.config import validation as config_validation
from components import utils
import gae_ts_mon

from legacy import api_common
from go.chromium.org.luci.buildbucket.proto import builder_common_pb2
from go.chromium.org.luci.buildbucket.proto import builds_service_pb2 as rpc_pb2
from go.chromium.org.luci.buildbucket.proto import common_pb2
from go.chromium.org.luci.buildbucket.proto import project_config_pb2
import bbutil
import buildtags
import config
import creation
import errors
import model
import search
import service
import swarmingcfg
import user
import validation

_PARAM_SWARMING = 'swarming'
_PARAM_CHANGES = 'changes'


class ErrorMessage(messages.Message):
  reason = messages.EnumField(errors.LegacyReason, 1, required=True)
  message = messages.StringField(2, required=True)


def exception_to_error_message(ex):
  assert isinstance(ex, errors.Error)
  assert ex.legacy_reason is not None
  return ErrorMessage(reason=ex.legacy_reason, message=ex.message)


class BuildResponseMessage(messages.Message):
  build = messages.MessageField(api_common.BuildMessage, 1)
  error = messages.MessageField(ErrorMessage, 2)


def builds_to_messages(builds, include_lease_key=False):
  """Converts model.Build objects to BuildMessage objects.

  Fetches children of the model.Build entities.
  """
  bundle_futs = [
      model.BuildBundle.get_async(
          b, infra=True, input_properties=True, output_properties=True
      ) for b in builds
  ]
  return [
      api_common.build_to_message(
          f.get_result(), include_lease_key=include_lease_key
      ) for f in bundle_futs
  ]


def build_to_message(build, include_lease_key=False):
  return builds_to_messages([build], include_lease_key=include_lease_key)[0]


def build_to_response_message(build, include_lease_key=False):
  msg = build_to_message(build, include_lease_key=include_lease_key)
  return BuildResponseMessage(build=msg)


def id_resource_container(body_message_class=message_types.VoidMessage):
  return endpoints.ResourceContainer(
      body_message_class,
      id=messages.IntegerField(1, required=True),
  )


def catch_errors(fn, response_message_class):

  @functools.wraps(fn)
  def decorated(svc, *args, **kwargs):
    try:
      return fn(svc, *args, **kwargs)
    except errors.Error as ex:
      assert hasattr(response_message_class, 'error')
      return response_message_class(error=exception_to_error_message(ex))
    except auth.AuthorizationError as ex:  # pragma: no cover
      logging.warning(
          'Authorization error.\n%s\nPeer: %s\nIP: %s', ex.message,
          auth.get_peer_identity().to_bytes(), svc.request_state.remote_address
      )
      raise endpoints.ForbiddenException(ex.message)

  return decorated


def convert_bucket(bucket):
  """Converts a bucket string to a bucket id and checks access.

  A synchronous wrapper for api_common.to_bucket_id_async that also checks
  access.

  Raises:
    auth.AuthorizationError if the requester doesn't have access to the bucket.
    errors.InvalidInputError if bucket is invalid or ambiguous.
  """
  bucket_id = api_common.to_bucket_id_async(bucket).get_result()

  # Check access here to return user-supplied bucket name,
  # as opposed to computed bucket id to prevent sniffing bucket names.
  if not bucket_id or not user.has_perm(user.PERM_BUCKETS_GET, bucket_id):
    raise user.current_identity_cannot('access bucket %r', bucket)

  return bucket_id


def convert_bucket_list(buckets):
  # This could be made concurrent, but in practice we search by at most one
  # bucket.
  return map(convert_bucket, buckets)


def buildbucket_api_method(
    request_message_class, response_message_class, **kwargs
):
  """Defines a buildbucket API method."""

  init_auth = auth.endpoints_method(
      request_message_class, response_message_class, **kwargs
  )

  def decorator(fn):
    fn = catch_errors(fn, response_message_class)
    fn = init_auth(fn)

    ts_mon_time = lambda: utils.datetime_to_timestamp(utils.utcnow()) / 1e6
    fn = gae_ts_mon.instrument_endpoint(time_fn=ts_mon_time)(fn)

    # ndb.toplevel must be the last one.
    # We use it because codebase uses the following pattern:
    #   results = [f.get_result() for f in futures]
    # without ndb.Future.wait_all.
    # If a future has an exception, get_result won't be called successive
    # futures, and thus may be left running.
    return ndb.toplevel(fn)

  return decorator


def parse_datetime(timestamp):  #pragma: no cover
  if timestamp is None:
    return None
  try:
    return utils.timestamp_to_datetime(timestamp)
  except OverflowError:  # pragma: no cover
    raise errors.InvalidInputError('Could not parse timestamp: %s' % timestamp)


@auth.endpoints_api(
    name='buildbucket', version='v1', title='Build Bucket Service'
)
class BuildBucketApi(remote.Service):
  """API for scheduling builds."""

  ####### GET ##################################################################

  @buildbucket_api_method(
      id_resource_container(),
      BuildResponseMessage,
      path='builds/{id}',
      http_method='GET'
  )
  @auth.public
  def get(self, request):
    """Returns a build by id."""
    try:
      build = service.get_async(request.id).get_result()
    except auth.AuthorizationError:
      build = None
    if build is None:
      raise errors.BuildNotFoundError()
    return build_to_response_message(build)

  ####### SEARCH ###############################################################

  SEARCH_REQUEST_RESOURCE_CONTAINER = endpoints.ResourceContainer(
      message_types.VoidMessage,
      start_cursor=messages.StringField(1),
      bucket=messages.StringField(2, repeated=True),
      # All specified tags must be present in a build.
      tag=messages.StringField(3, repeated=True),
      status=messages.EnumField(search.StatusFilter, 4),
      result=messages.EnumField(model.BuildResult, 5),
      cancelation_reason=messages.EnumField(model.CancelationReason, 6),
      failure_reason=messages.EnumField(model.FailureReason, 7),
      created_by=messages.StringField(8),
      max_builds=messages.IntegerField(9, variant=messages.Variant.INT32),
      retry_of=messages.IntegerField(10),
      canary=messages.BooleanField(11),
      # search by canary_preference is not supported
      creation_ts_low=messages.IntegerField(12),  # inclusive
      creation_ts_high=messages.IntegerField(13),  # exclusive
      include_experimental=messages.BooleanField(14),
  )

  class SearchResponseMessage(messages.Message):
    builds = messages.MessageField(api_common.BuildMessage, 1, repeated=True)
    next_cursor = messages.StringField(2)
    error = messages.MessageField(ErrorMessage, 3)

  @buildbucket_api_method(
      SEARCH_REQUEST_RESOURCE_CONTAINER,
      SearchResponseMessage,
      path='search',
      http_method='GET'
  )
  @auth.public
  def search(self, request):
    """Searches for builds."""
    assert isinstance(request.tag, list)
    if (request.status is not None or request.result is not None or
        request.failure_reason is not None or
        request.cancelation_reason is not None):  # pragma: no cover
      logging.warning(
          '%s is searching by legacy property: %s',
          auth.get_peer_identity().to_bytes(), request
      )
    builds, next_cursor = search.search_async(
        search.Query(
            bucket_ids=convert_bucket_list(request.bucket),
            tags=request.tag,
            status=request.status,
            result=request.result,
            failure_reason=request.failure_reason,
            cancelation_reason=request.cancelation_reason,
            max_builds=request.max_builds,
            created_by=request.created_by,
            start_cursor=request.start_cursor,
            retry_of=request.retry_of,
            canary=request.canary,
            create_time_low=parse_datetime(request.creation_ts_low),
            create_time_high=parse_datetime(request.creation_ts_high),
            include_experimental=request.include_experimental,
        )
    ).get_result()
    return self.SearchResponseMessage(
        builds=builds_to_messages(builds),
        next_cursor=next_cursor,
    )
