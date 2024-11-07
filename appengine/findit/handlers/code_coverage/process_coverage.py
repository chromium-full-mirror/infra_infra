# Copyright 2021 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import json
import logging
import re
import time
from six.moves.urllib.parse import urlparse
import zlib

from google.appengine.api import taskqueue
from google.appengine.ext import ndb
from google.protobuf import json_format
from google.protobuf.field_mask_pb2 import FieldMask

from go.chromium.org.luci.buildbucket.proto import builds_service_pb2
from go.chromium.org.luci.buildbucket.proto import common_pb2

from common import constants
from common import monitoring
from common.base_handler import BaseHandler, Permission
from common.findit_http_client import FinditHttpClient
from common.waterfall.buildbucket_client import GetV2Build
from gae_libs.appengine_util import IsInternalInstance
from gae_libs.gitiles.cached_gitiles_repository import CachedGitilesRepository
from handlers.code_coverage import utils
from libs.deps import chrome_dependency_fetcher
from model.code_coverage import BlockingStatus
from model.code_coverage import DependencyRepository
from model.code_coverage import FileCoverageData
from model.code_coverage import LowCoverageBlocking
from model.code_coverage import PostsubmitReport
from model.code_coverage import PresubmitCoverageData
from model.code_coverage import SummaryCoverageData
from model.proto.gen.code_coverage_pb2 import CoverageReport
from services.code_coverage import code_coverage_util
from waterfall import waterfall_config

# The regex to extract the build id from the url path.
_BUILD_ID_REGEX = re.compile(r'.*/build/(\d+)$')
_CHROMIUM_TO_GOOGLER_MAPPING_PATH = '/cr2goog/cr2goog.txt'
_DEFAULT_TRIGGER_INC_COV_THRESHOLD_FOR_BLOCKING = 70
_DEFAULT_RELAX_ABS_COV_THRESHOLD_FOR_BLOCKING = 80
_DEFAULT_MINIMUM_LINES_OF_CHANGE_FOR_BLOCKING = 5
_BUILDBUCKET_HOST = 'cr-buildbucket.appspot.com'
_BUILDBUCKET_V2_SEARCH_BUILDS_ENDPOINT = (
    'https://{hostname}/prpc/buildbucket.v2.Builds/SearchBuilds'.format(
        hostname=_BUILDBUCKET_HOST))

# https://github.com/grpc/grpc-go/blob/master/codes/codes.go
GRPC_OK = '0'

def _AddDependencyToManifest(path, url, revision,
                             manifest):  # pragma: no cover.
  """Adds a dependency to the given manifest.

  Args:
    path (str): Path to the dependency repo.
    url (str): The url to the Gitiles project of the root repository.
    revision (str): The revision of the root repository.
    manifest: A list of DependencyRepository.
  """
  assert path.startswith('//')
  if not path.endswith('/'):
    path = path + '/'

  # Parse the url to extract the hostname and project name.
  # For "https://chromium.google.com/chromium/src.git", we get
  # ParseResult(netloc='chromium.google.com', path='/chromium/src.git', ...)
  result = urlparse(url)
  assert result.path, 'No project extracted from %s' % url

  manifest.append(
      DependencyRepository(
          path=path,
          server_host=result.netloc,
          project=result.path[1:],  # Strip the leading '/'.
          revision=revision))


def _GetDisallowedDeps():  # pragma: no cover.
  """Returns a map of disallowed dependencies to skip adding to manifest.

  Main use case is to skip dependency repos that have malformed structures, and
  the mapping is from root repo url to list of dependency paths (relative to
  the root of the checkout).
  """
  return waterfall_config.GetCodeCoverageSettings().get('blacklisted_deps', {})


def _GetAllowedChromiumTryBuilders():
  prefix = 'chromium/try/'
  return [
      x[len(prefix):] for x in waterfall_config.GetCodeCoverageSettings().get(
          'allowed_builders', []) if x.startswith(prefix)
  ]


def _IsBlockingChangesAllowed(project):
  return project in waterfall_config.GetCodeCoverageSettings().get(
      'block_low_coverage_changes_projects', [])


def _IsBlockingOperational(config):
  return config.get('is_operational', False)


def _IsAuthorInAllowlistForBlocking(config, author_email):
  """Returns True if an author is in allowlist for blocking changes.

  Returns False if the author doesn't belong to google. If there's no
  such allowlist, returns True for all googlers.
  """
  if not author_email.endswith("@google.com"):
    return False
  author = author_email[:author_email.find("@")]
  blocked_authors = config.get('monitored_authors', [])
  if not blocked_authors:
    return True
  return author in blocked_authors


def _IsFileInAllowlistForBlocking(config, file_path):
  assert file_path.startswith('//')
  for allowed_dir in config.get('included_directories', []):
    if file_path.startswith(allowed_dir):
      for disallowed_dir in config.get('excluded_directories', []):
        if file_path.startswith(disallowed_dir):
          return False
      return True
  return False


def _IsFileTypeAllowedForBlocking(config, file_path):
  blocking_file_types = config.get('monitored_file_types', [])
  # A CL may be blocked for any file type if no blocking
  # file types are specified.
  if not blocking_file_types:
    return True
  for file_type in blocking_file_types:
    if file_path.endswith(file_type):
      return True
  return False


def _HaveEnoughLinesChangedForBlocking(config, inc_coverage):
  return inc_coverage.total_lines >= config.get(
      'minimum_loc', _DEFAULT_MINIMUM_LINES_OF_CHANGE_FOR_BLOCKING)


def _CanBeExemptFromBlocking(config, abs_coverage):
  coverage = (abs_coverage.covered_lines * 100.0) / abs_coverage.total_lines
  return coverage >= config.get('relax_threshold',
                                _DEFAULT_RELAX_ABS_COV_THRESHOLD_FOR_BLOCKING)


def _HasLowCoverageForBlocking(config, inc_coverage):
  coverage = (inc_coverage.covered_lines * 100.0) / inc_coverage.total_lines
  return coverage < config.get('trigger_threshold',
                               _DEFAULT_TRIGGER_INC_COV_THRESHOLD_FOR_BLOCKING)


def _RetrieveChromeManifest(repo_url, revision,
                            os_platform):  # pragma: no cover.
  """Returns the manifest of all the dependencies for the given revision.

  Args:
    repo_url (str): The url to the Gitiles project of the root repository.
    revision (str): The revision of the root repository.
    os_platform (str): The platform of the code checkout.

  Returns:
    A list of DependencyRepository instances ordered reversely by the relative
    path of each dependency checkout in the checkout of the root repository.
    The longer the relative path, the smaller index in the returned list.

    The reverse order is to make it easy to reliably determine which dependency
    a file is from, when given a file path relative to the root repository.
  """
  manifest = []

  # Add the root repository.
  _AddDependencyToManifest('//', repo_url, revision, manifest)

  # Add all the dependent repositories.
  # DEPS fetcher now assumes chromium/src and main branch.
  dep_fetcher = chrome_dependency_fetcher.ChromeDependencyFetcher(
      CachedGitilesRepository.Factory(FinditHttpClient()))
  deps = dep_fetcher.GetDependency(revision, os_platform)
  for path, dep in deps.items():
    # Remove clause when crbug.com/929315 gets fixed.
    if path in _GetDisallowedDeps().get(repo_url, []):
      continue

    # Public DEPS paths have the src/ prefix, and they need to be striped to be
    # converted to source absolute path format.
    path = '//' + path[len('src/'):]
    _AddDependencyToManifest(path, dep.repo_url, dep.revision, manifest)

  manifest.sort(key=lambda x: len(x.path), reverse=True)
  return manifest



def _GetValidatedData(gs_path):  # pragma: no cover.
  """Returns the json data from the given GS path after validation.

  Args:
    gs_path (str): Path to the file, in the format /bucket/object.

  Returns:
    json_data (dict): the json data of the file pointed by the given GS url, or
        None if the data can't be retrieved.
  """
  logging.info('Fetching data from %s', gs_path)
  content = utils.GetFileContentFromGs(gs_path)
  assert content, 'Failed to fetch coverage json data from %s' % gs_path

  logging.info('Decompressing and loading coverage data...')
  decompressed_data = zlib.decompress(content)

  del content  # Explicitly release memory.
  data = json.loads(decompressed_data)
  del decompressed_data  # Explicitly release memory.
  logging.info('Finished decompressing and loading coverage data.')

  # According to https://developers.google.com/discovery/v1/type-format, certain
  # serialization APIs will automatically convert int64 to string when
  # serializing to JSON, and to facilitate later computations, the following for
  # loops convert them back to int64 (int in Python).
  # The following workaround should be removed when the service migrates away
  # from JSON.
  for file_data in data.get('files', []):
    for line_data in file_data.get('lines', []):
      line_data['count'] = int(line_data['count'])

  # Validate that the data is in good format.
  logging.info('Validating coverage data...')
  report = CoverageReport()
  json_format.ParseDict(data, report, ignore_unknown_fields=False)
  del report  # Explicitly delete the proto message to release memory.
  logging.info('Finished validating coverage data.')

  return data


def _GetAllowedBuilders():
  """Returns a set of allowed builders that the service should process.

  builders are specified in canonical string representations, and following is
  an example config:
  {
    'allowed_builders': [
      'chromium/try/linux-rel',
      'chromium/try/linux-chromeos-rel',
    ]
  }
  """
  return set(waterfall_config.GetCodeCoverageSettings().get(
      'allowed_builders', []))


def _GetFluctuationCheckOptOutProjects():
  """Returns a set of projects which want all reports to be marked visible."""
  return set(waterfall_config.GetCodeCoverageSettings().get(
      'fluctuation_check_opt_out', []))


def _IsReportSuspicious(report):
  """Returns True if the newly generated report is suspicious to be incorrect.

  A report is determined to be suspicious if and only if the absolute difference
  between its line coverage percentage and the most recent visible report is
  greater than 1.50%.

  Args:
    report (PostsubmitReport): The report to be evaluated.

  Returns:
    True if the report is suspicious, otherwise False.
  """

  def _GetLineCoveragePercentage(report):  # pragma: no cover
    line_coverage_percentage = None
    summary = report.summary_metrics
    for feature_summary in summary:
      if feature_summary['name'] == 'line':
        line_coverage_percentage = float(
            feature_summary['covered']) / feature_summary['total']
        break

    assert line_coverage_percentage is not None, (
        'Given report has invalid summary')
    return line_coverage_percentage

  target_server_host = report.gitiles_commit.server_host
  target_project = report.gitiles_commit.project
  target_bucket = report.bucket
  target_builder = report.builder
  most_recent_visible_reports = PostsubmitReport.query(
      PostsubmitReport.gitiles_commit.project == target_project,
      PostsubmitReport.gitiles_commit.server_host == target_server_host,
      PostsubmitReport.bucket == target_bucket,
      PostsubmitReport.builder == target_builder,
      PostsubmitReport.visible == True, PostsubmitReport.modifier_id ==
      0).order(-PostsubmitReport.commit_timestamp).fetch(1)
  if not most_recent_visible_reports:
    logging.warn('No existing visible reports to use for reference, the new '
                 'report is determined as not suspicious by default')
    return False

  most_recent_visible_report = most_recent_visible_reports[0]
  if abs(
      _GetLineCoveragePercentage(report) -
      _GetLineCoveragePercentage(most_recent_visible_report)) > 0.015:
    return True

  return False


def _FetchCoverageBuildsStatus(host, change, patchset):
  predicate = {
      'gerrit_changes': [{
          'host': host,
          'change': change,
          'patchset': patchset
      }]
  }
  request = builds_service_pb2.SearchBuildsRequest(predicate=predicate)
  try_builders = _GetAllowedChromiumTryBuilders()
  status_code, content, response_headers = FinditHttpClient().Post(
      _BUILDBUCKET_V2_SEARCH_BUILDS_ENDPOINT,
      request.SerializeToString(),
      headers={'Content-Type': 'application/prpc; encoding=binary'})
  if status_code == 200 and response_headers.get('X-Prpc-Grpc-Code') == GRPC_OK:
    result = builds_service_pb2.SearchBuildsResponse()
    result.ParseFromString(content)
    builds_status = {}
    for build in result.builds:
      if build.builder.builder not in try_builders:
        continue
      builds_status[build.builder.builder] = build.status
    logging.info("build_status for host=%s, change=%d, patch=%d = %r", host,
                 change, patchset, builds_status)
    return builds_status
  logging.warning('Unexpected prpc code: %s',
                  response_headers.get('X-Prpc-Grpc-Code'))
  return None


class ProcessCodeCoverageData(BaseHandler):
  PERMISSION_LEVEL = Permission.ADMIN_OR_APP_SELF

  def _ProcessFullRepositoryData(self, commit, data, full_gs_metadata_dir,
                                 builder, build_id, mimic_builder_name):
    # Load the commit log first so that we could fail fast before redo all.
    repo_url = 'https://%s/%s.git' % (commit.host, commit.project)
    change_log = CachedGitilesRepository(FinditHttpClient(),
                                         repo_url).GetChangeLog(commit.id)
    assert change_log is not None, 'Failed to retrieve the commit log'

    # TODO(crbug.com/921714): output the manifest as a build output property,
    # and make it project agnostic.
    if (commit.host == 'chromium.googlesource.com' and
        commit.project == 'chromium/src'):
      manifest = _RetrieveChromeManifest(repo_url, commit.id, 'unix')
    else:
      # For projects other than chromium/src, dependency repos are ignored for
      # simplicity.
      manifest = []
      _AddDependencyToManifest('//', repo_url, commit.id, manifest)

    manifest = [x for x in manifest if x.revision]
    report = PostsubmitReport.Create(
        server_host=commit.host,
        project=commit.project,
        ref=commit.ref,
        revision=commit.id,
        bucket=builder.bucket,
        builder=mimic_builder_name,
        commit_timestamp=change_log.committer.time,
        manifest=manifest,
        summary_metrics=data.get('summaries'),
        build_id=build_id,
        visible=False)
    report.put()

    # Save the file-level, directory-level and line-level coverage data.
    for data_type in ('dirs', 'components', 'files', 'file_shards'):
      sub_data = data.get(data_type)
      if not sub_data:
        continue

      logging.info('Processing %d entries for %s', len(sub_data), data_type)

      actual_data_type = data_type
      if data_type == 'file_shards':
        actual_data_type = 'files'

      def FlushEntries(entries, total, last=False):
        # Flush the data in a batch and release memory.
        if len(entries) < 100 and not (last and entries):
          return entries, total

        ndb.put_multi(entries)
        total += len(entries)
        logging.info('Dumped %d coverage data entries of type %s', total,
                     actual_data_type)

        return [], total

      def IterateOverFileShards(file_shards):
        for file_path in file_shards:
          url = '%s/%s' % (full_gs_metadata_dir, file_path)
          # Download data one by one.
          yield _GetValidatedData(url).get('files', [])

      if data_type == 'file_shards':
        data_iterator = IterateOverFileShards(sub_data)
      else:
        data_iterator = [sub_data]

      entities = []
      total = 0

      component_summaries = []
      for dataset in data_iterator:
        for group_data in dataset:
          if actual_data_type == 'components':
            component_summaries.append({
                'name': group_data['path'],
                'path': group_data['path'],
                'summaries': group_data['summaries'],
            })

          if actual_data_type == 'files' and group_data.get('revision', ''):
            self._FetchAndSaveFileIfNecessary(report, group_data['path'],
                                              group_data['revision'])

          if actual_data_type == 'files':
            coverage_data = FileCoverageData.Create(
                server_host=commit.host,
                project=commit.project,
                ref=commit.ref,
                revision=commit.id,
                path=group_data['path'],
                bucket=builder.bucket,
                builder=mimic_builder_name,
                data=group_data)
          else:
            coverage_data = SummaryCoverageData.Create(
                server_host=commit.host,
                project=commit.project,
                ref=commit.ref,
                revision=commit.id,
                data_type=actual_data_type,
                path=group_data['path'],
                bucket=builder.bucket,
                builder=mimic_builder_name,
                data=group_data)
          entities.append(coverage_data)
          entities, total = FlushEntries(entities, total, last=False)
        del dataset  # Explicitly release memory.
      FlushEntries(entities, total, last=True)

      if component_summaries:
        component_summaries.sort(key=lambda x: x['path'])
        SummaryCoverageData.Create(
            server_host=commit.host,
            project=commit.project,
            ref=commit.ref,
            revision=commit.id,
            data_type='components',
            path='>>',
            bucket=builder.bucket,
            builder=mimic_builder_name,
            data={
                'dirs': component_summaries,
                'path': '>>'
            }).put()
        component_summaries = []
        logging.info('Summary of all components are saved to datastore.')

    if not _IsReportSuspicious(
        report) or commit.project in _GetFluctuationCheckOptOutProjects():
      report.visible = True
      report.put()

      monitoring.code_coverage_full_reports.increment({
          'host':
              commit.host,
          'project':
              commit.project,
          'ref':
              commit.ref or 'refs/heads/main',
          'builder':
              '%s/%s/%s' %
              (builder.project, builder.bucket, mimic_builder_name),
      })

    monitoring.code_coverage_report_timestamp.set(
        int(time.time()),
        fields={
            'host':
                commit.host,
            'project':
                commit.project,
            'ref':
                commit.ref or 'refs/heads/main',
            'builder':
                '%s/%s/%s' %
                (builder.project, builder.bucket, mimic_builder_name),
            'is_success':
                report.visible,
        })

  def _FetchAndSaveFileIfNecessary(self, report, path, revision):
    """Fetches the file from gitiles and store to cloud storage if not exist.

    Args:
      report (PostsubmitReport): The report that the file is associated with.
      path (str): Source absolute path to the file.
      revision (str): The gitile revision of the file in its own repo.
    """
    # Due to security concerns, don't cache source files for internal projects.
    if IsInternalInstance():
      return

    assert path.startswith('//'), 'All file path should start with "//"'
    assert revision, 'A valid revision is required'

    gs_path = utils.ComposeSourceFileGsPath(report.manifest, path, revision)
    if utils.IsFileAvailableInGs(gs_path):
      return

    # Fetch the source files from gitile and save it in gs so that coverage
    # file view can be quickly rendered.
    url = ('/coverage/task/fetch-source-file')
    params = {
        'report_key': report.key.urlsafe(),
        'path': path,
        'revision': revision
    }
    taskqueue.add(
        method='POST',
        url=url,
        target='code-coverage-backend',
        queue_name='code-coverage-fetch-source-file',
        params=params)

  def _ProcessCLPatchData(self,
                          mimic_builder,
                          patch,
                          coverage_data,
                          is_rts=False):
    """Processes and updates coverage data for per-cl build.

    Part of the responsibility of this method is to calculate per-file coverage
    percentage for the following use cases:
    1. Surface them on Gerrit to provide an overview of the test coverage of
       the CL for authors and reviewers.
    2. For metrics tracking to understand the impact of the coverage data.

    Args:
      mimic_builder (string): Name of the builder that we are mimicking coverage
                              data belongs to. For example, if linux-rel is
                              producing unit tests coverage, mimic_builder name
                              would be 'linux-rel_unit'.
      patch (buildbucket.v2.GerritChange): A gerrit change with fields: host,
                                           project, change, patchset.
      coverage_data (list): A list of File in coverage proto.
    """

    @ndb.tasklet
    @ndb.transactional
    def _UpdateCoverageDataAsync():

      def _GetEntity(entity, merged_coverage_data, equivalent_patchset):
        if entity:
          entity.data = code_coverage_util.MergeFilesCoverageDataForPerCL(
              entity.data, merged_coverage_data)
          entity.data_orig = code_coverage_util.MergeFilesCoverageDataForPerCL(
              entity.data_orig, coverage_data)
          entity.times_updated = entity.times_updated + 1
        else:
          entity = PresubmitCoverageData.Create(
              server_host=patch.host,
              change=patch.change,
              patchset=patch.patchset,
              data=merged_coverage_data,
              data_orig=coverage_data)
        entity.merged_with = equivalent_patchset
        entity.absolute_percentages = (
            code_coverage_util.CalculateAbsolutePercentages(entity.data))
        entity.incremental_percentages = (
            code_coverage_util.CalculateIncrementalPercentages(
                patch.host, patch.project, patch.change, patch.patchset,
                entity.data))
        if is_rts:
          entity.data_rts = code_coverage_util.MergeFilesCoverageDataForPerCL(
              entity.data_rts, coverage_data)
          entity.absolute_percentages_rts = (
              code_coverage_util.CalculateAbsolutePercentages(entity.data_rts))
        return entity

      def _GetEntityForUnit(entity, merged_coverage_data, equivalent_patchset):
        if entity:
          entity.data_unit = code_coverage_util.MergeFilesCoverageDataForPerCL(
              entity.data_unit, merged_coverage_data)
          entity.data_unit_orig = (
              code_coverage_util.MergeFilesCoverageDataForPerCL(
                  entity.data_unit_orig, coverage_data))
          entity.times_updated_unit = entity.times_updated_unit + 1
        else:
          entity = PresubmitCoverageData.Create(
              server_host=patch.host,
              change=patch.change,
              patchset=patch.patchset,
              data_unit=merged_coverage_data,
              data_unit_orig=coverage_data)
        entity.merged_with = equivalent_patchset
        entity.absolute_percentages_unit = (
            code_coverage_util.CalculateAbsolutePercentages(entity.data_unit))
        entity.incremental_percentages_unit = (
            code_coverage_util.CalculateIncrementalPercentages(
                patch.host, patch.project, patch.change, patch.patchset,
                entity.data_unit))
        if is_rts:
          entity.data_unit_rts = (
              code_coverage_util.MergeFilesCoverageDataForPerCL(
                  entity.data_unit_rts, coverage_data))
          entity.absolute_percentages_unit_rts = (
              code_coverage_util.CalculateAbsolutePercentages(
                  entity.data_unit_rts))
        return entity

      # Chromium Recipe performs optimization where sometimes they don't run
      # certain tests, if they passes earlier on an equivalent patchset's
      # build. In such cases merely processing coverage data from current
      # build won't suffice because we would be missing coverage data from
      # bypassed tests. To guard against such optmizations, we merge current
      # coverage data with coverage data of a previous equivalent patchset
      equivalent_patchsets = code_coverage_util.GetEquivalentPatchsets(
          patch.host, patch.project, patch.change, patch.patchset)
      merged_coverage_data = coverage_data
      equivalent_patchset = None
      if equivalent_patchsets:
        equivalent_entity = None
        for ps in sorted(equivalent_patchsets, reverse=True):
          equivalent_entity = PresubmitCoverageData.Get(
              server_host=patch.host, change=patch.change, patchset=ps)
          if equivalent_entity:
            equivalent_patchset = equivalent_entity.cl_patchset.patchset
            break
        if equivalent_entity:
          # Merge current coverage data with data from equivalent_entity
          if mimic_builder.endswith('_unit'):
            merged_coverage_data = (
                code_coverage_util.MergeFilesCoverageDataForPerCL(
                    equivalent_entity.data_unit, coverage_data))
          else:
            merged_coverage_data = (
                code_coverage_util.MergeFilesCoverageDataForPerCL(
                    equivalent_entity.data, coverage_data))

      logging.info("mimic_builder = %s", mimic_builder)
      entity = yield PresubmitCoverageData.GetAsync(
          server_host=patch.host, change=patch.change, patchset=patch.patchset)
      # Update/Create entity with unit test coverage fields populated
      # if mimic_builder represents a unit tests only builder.
      if mimic_builder.endswith('_unit'):
        entity = _GetEntityForUnit(entity, merged_coverage_data,
                                   equivalent_patchset)
      else:
        entity = _GetEntity(entity, merged_coverage_data, equivalent_patchset)
      yield entity.put_async()
      raise ndb.Return(entity)

    update_future = _UpdateCoverageDataAsync()

    # Following code invalidates the dependent patchsets whenever the coverage
    # data of the current patchset changes, and it is based on the assumption
    # that the coverage data of the dependent patchsets is always a subset of
    # the current patchset.
    #
    # There is one scenario where the above mentioned assumption doesn't hold:
    # 1. User triggers builder1 on ps1, so ps1 has builder1's coverage data.
    # 2. Ps2 is a trivial-rebase of ps1, and once its coverage data is
    #    requested, it reuses ps1's, which is to say that ps2 now has builder1's
    #    coverage data.
    # 3. User triggers builder2 on ps2, so ps2 contains coverage data from both
    #    builder1 and builder2.
    # 4. User triggers builder3 on ps1, so now ps1 has builder1 and builder3's
    #    coverage data, and it also invalidates ps2, but it's NOT entirely
    #    correct because ps2 has something (builder2) that ps1 doesn't have.
    #
    # In practice, the described scenario is rather extreme corner case because:
    # 1. Most users triggers cq dry run instead of specific builders.
    # 2. When users upload a new trivial-rebase patchset, most likely they'll
    #    never go back to previous patchset to trigger builds.
    #
    # Therefore, it makes sense to do nothing about it for now.
    delete_futures = ndb.delete_multi_async(
        PresubmitCoverageData.query(
            PresubmitCoverageData.cl_patchset.server_host == patch.host,
            PresubmitCoverageData.cl_patchset.change == patch.change,
            PresubmitCoverageData.based_on == patch.patchset).fetch(
                keys_only=True))

    update_future.get_result()
    for f in delete_futures:
      f.get_result()

  def _GetLowCoverageFiles(self, cohort, config, entity):
    """Returns a list of low coverage files as per the configs.

    Also returns a boolean, indicating if any of the files matched the
    monitored directories and monitored file type scope of the cohort.
    If the low coverage file list is non-empty, this is guaranteed to be True.
    This is done to enable cohort based reporting."""
    low_coverage_files = []
    is_cohort_file_match = False
    for inc_metrics in entity.incremental_percentages:
      if not _IsFileInAllowlistForBlocking(config, inc_metrics.path):
        logging.info("%s is not in allowed dirs for cohort %s",
                     inc_metrics.path, cohort)
        continue
      if not _IsFileTypeAllowedForBlocking(config, inc_metrics.path):
        logging.info("%s is not of allowed file type for cohort %s",
                     inc_metrics.path, cohort)
        continue
      is_cohort_file_match = True
      # Do not block because of test/main files
      if re.match(utils.TEST_FILE_REGEX, inc_metrics.path) or re.match(
          utils.MAIN_FILE_REGEX, inc_metrics.path) or re.match(
              utils.EXAMPLE_FILE_REGEX, inc_metrics.path):
        logging.info("%s is a test/main/example file for cohort %s",
                     inc_metrics.path, cohort)
        continue
      if not _HaveEnoughLinesChangedForBlocking(config, inc_metrics):
        logging.info("%s doesn't have enough lines changed for cohort %s",
                     inc_metrics.path, cohort)
        continue
      if _HasLowCoverageForBlocking(config, inc_metrics):
        logging.info("%s has low incremental coverage for cohort %s",
                     inc_metrics.path, cohort)
        for abs_metrics in entity.absolute_percentages:
          if (abs_metrics.path == inc_metrics.path and
              not _CanBeExemptFromBlocking(config, abs_metrics)):
            logging.info("%s has low absolute coverate too for cohort %s",
                         inc_metrics.path, cohort)
            low_coverage_files.append(inc_metrics.path)
    return low_coverage_files, is_cohort_file_match

  # TODO(crbug/1412897): Cache this
  def _GetChromiumToGooglerMapping(self):
    content = utils.GetFileContentFromGs(_CHROMIUM_TO_GOOGLER_MAPPING_PATH)
    assert content, ('Failed to fetch account mappings data from %s' %
                     _CHROMIUM_TO_GOOGLER_MAPPING_PATH)
    return json.loads(content)

  def _MayBeBlockCLForLowCoverage(self, patch):
    assert LowCoverageBlocking.Get(
        server_host=patch.host, change=patch.change, patchset=patch.patchset
    ).blocking_status == BlockingStatus.READY_FOR_VERDICT, \
      "Change %d, patchset %d for host %s is not ready for blocking" % (
        patch.change, patch.project, patch.host)
    entity = PresubmitCoverageData.Get(
        server_host=patch.host, change=patch.change, patchset=patch.patchset)
    # We block some CLs based on overall coverage metrics.
    if _IsBlockingChangesAllowed(patch.project):
      change_details = code_coverage_util.FetchChangeDetails(
          patch.host, patch.project, patch.change, detailed_accounts=True)
      if 'revert_of' in change_details:
        logging.info("Bypassing the check as %d is a revert CL", patch.change)
        return
      low_coverage_threshold_with_violators = {}
      cohorts_matched = []
      cohorts_violated = []
      any_cohort_author_match = False
      for cohort, config in waterfall_config.GetCodeCoverageSettings().get(
          'block_low_coverage_changes', {}).items():
        if not _IsBlockingOperational(config):
          continue
        author_email = change_details['owner']['email']
        author_email = self._GetChromiumToGooglerMapping().get(
            author_email, author_email)
        if not _IsAuthorInAllowlistForBlocking(config, author_email):
          logging.info(
              "Bypassing the check for cohort %s" +
              " as %s is not in allowlist", cohort, author_email)
          continue
        any_cohort_author_match = True
        # Block CL only if some files have low coverage
        low_coverage_files, is_cohort_file_match = self._GetLowCoverageFiles(
            cohort, config, entity)
        if low_coverage_files:
          low_coverage_threshold_with_violators[config.get(
              'trigger_threshold',
              _DEFAULT_TRIGGER_INC_COV_THRESHOLD_FOR_BLOCKING
          )] = low_coverage_files
          cohorts_violated.append(cohort)
        if is_cohort_file_match:
          cohorts_matched.append(cohort)

      @ndb.transactional
      def _UpdateBlockingStatus(status):
        blocking_entity = LowCoverageBlocking.Get(
            server_host=patch.host,
            change=patch.change,
            patchset=patch.patchset)
        blocking_entity.blocking_status = status
        blocking_entity.put()

      def _PostReviewToGerrit(data, cohorts_matched, cohorts_violated):
        taskqueue.add(
            name='%s-%d-%d' %
            (patch.host.replace('.', '_'), patch.change, patch.patchset),
            queue_name=constants.POSTREVIEW_REQUEST_QUEUE,
            target=constants.CODE_COVERAGE_REFERENCED_COVERAGE_WORKER,
            payload=json.dumps({
                'host': patch.host,
                'change': patch.change,
                'patchset': patch.patchset,
                'data': data,
                'cohorts_matched': cohorts_matched,
                'cohorts_violated': cohorts_violated
            }),
            headers={'Content-Type': 'application/json'},
            url='/coverage/task/low-coverage-blocking')

      if low_coverage_threshold_with_violators:
        _UpdateBlockingStatus(BlockingStatus.VERDICT_BLOCK)
        msg_header = (
            'This change will be blocked from submission as there are files '
            'which do not meet the coverage criteria.')
        for threshold, low_coverage_files in \
          low_coverage_threshold_with_violators.items():
          logging.info("low_coverage_files = %r", low_coverage_files)
          msg_body = (
              'Following files have incremental coverage(all tests) < %d%%. ' %
              threshold)
          file_names_with_bullets = ["- %s" % x for x in low_coverage_files]
          msg_body += "\n" + "\n".join(file_names_with_bullets)
        msg_footer = ('Please add tests for uncovered lines, '
                      'or add Low-Coverage-Reason:<reason> in '
                      'the change description to bypass. '
                      'See https://bit.ly/46jhjS9 to understand '
                      'when it is okay to bypass. If you think coverage is '
                      'underreported, file a bug at https://bit.ly/3ENM7Pe')
        data = {
            'labels': {
                'Code-Coverage': -1
            },
            'message': "\n".join([msg_header, msg_body, "", msg_footer]),
            'notify': 'OWNER',
            'add_to_attention_set': {
                'notify': 'OWNER'
            }
        }
        logging.info(('Adding CodeCoverage-1 label for '
                      'project %s, change %d,  patchset %d'), patch.project,
                     patch.change, patch.patchset)
        _PostReviewToGerrit(data, cohorts_matched, cohorts_violated)
      # Add a positive Code Coverage label only for authors in the allowlist.
      # This is done to reduce noise.
      elif cohorts_matched and any_cohort_author_match:
        _UpdateBlockingStatus(BlockingStatus.VERDICT_NOT_BLOCK)
        data = {
            'labels': {
                'Code-Coverage': +1
            },
            'message': 'This change meets the code coverage requirements.',
            'notify': 'OWNER'
        }
        logging.info(('Adding CodeCoverage+1 label for '
                      'project %s, change %d,  patchset %d'), patch.project,
                     patch.change, patch.patchset)
        _PostReviewToGerrit(data, cohorts_matched, cohorts_violated)

  @ndb.transactional
  def _UpdateBlockingLowCoverageTracker(self,
                                        patch,
                                        expected_builders=None,
                                        successful_builders=None,
                                        processed_builders=None,
                                        has_builder_failure=False):
    # Get tracking entity. Create one if it doesn't exist
    tracking_entity = LowCoverageBlocking.Get(
        server_host=patch.host, change=patch.change, patchset=patch.patchset)
    logging.info(expected_builders)
    logging.info(successful_builders)
    logging.info(processed_builders)
    if not tracking_entity:
      logging.info("Creating blocking entity for host=%s, change=%d, patch=%d",
                   patch.host, patch.change, patch.patchset)
      tracking_entity = LowCoverageBlocking.Create(
          server_host=patch.host, change=patch.change, patchset=patch.patchset)
    # Update all builders' list
    tracking_entity.expected_builders = set(expected_builders or []).union(
        set(tracking_entity.expected_builders))
    tracking_entity.successful_builders = set(successful_builders or []).union(
        set(tracking_entity.successful_builders))
    tracking_entity.processed_builders = set(processed_builders or []).union(
        set(tracking_entity.processed_builders))

    # Update blocking status
    if has_builder_failure:
      # pylint: disable=line-too-long
      tracking_entity.blocking_status = BlockingStatus.DONT_BLOCK_BUILDER_FAILURE
    elif set(tracking_entity.processed_builders) == set(
        tracking_entity.expected_builders):
      tracking_entity.blocking_status = BlockingStatus.READY_FOR_VERDICT
    logging.info("Updating status for change %d, patch %d", patch.change,
                 patch.patchset)
    logging.info(tracking_entity.expected_builders)
    logging.info(tracking_entity.successful_builders)
    logging.info(tracking_entity.processed_builders)
    tracking_entity.put()

  def _ProcessCodeCoverageData(self, build_id):
    build = GetV2Build(
        build_id,
        fields=FieldMask(paths=[
            'id', 'output.properties', 'output.gitiles_commit', 'input',
            'builder', 'status'
        ]))

    if not build:
      return BaseHandler.CreateError(
          'Could not retrieve build #%d from buildbucket, retry' % build_id,
          404)

    builder_id = '%s/%s/%s' % (build.builder.project, build.builder.bucket,
                               build.builder.builder)
    if builder_id not in _GetAllowedBuilders():
      logging.info('%s is not allowed', builder_id)
      return

    # Convert the Struct to standard dict, to use .get, .items etc.
    properties = dict(build.output.properties.items())
    gs_bucket = properties.get('coverage_gs_bucket')
    gs_metadata_dirs = properties.get('coverage_metadata_gs_paths') or []
    # Get mimic builder names from builder output properties. Multiple test
    # types' coverage data will be uploaded to separated folders, mimicking
    # these come from different builders.
    mimic_builder_names = properties.get('mimic_builder_names') or []

    if properties.get('process_coverage_data_failure'):
      monitoring.code_coverage_cq_errors.increment({
          'project': build.builder.project,
          'bucket': build.builder.bucket,
          'builder': build.builder.builder,
      })

    def _IsTryBuild():
      return build.builder.bucket == 'try'

    if _IsTryBuild():
      patch = build.input.gerrit_changes[0]
      builders_status = _FetchCoverageBuildsStatus(patch.host, patch.change,
                                                   patch.patchset)
      expected_builders = [x for x in builders_status]
      successful_builders = []
      has_builder_failure = False
      for builder, status in builders_status.items():
        if status in [
            common_pb2.Status.SUCCESS, common_pb2.Status.FAILURE,
            common_pb2.Status.INFRA_FAILURE, common_pb2.Status.CANCELED
        ]:
          if status == common_pb2.Status.SUCCESS:
            successful_builders.append(builder)
          else:
            has_builder_failure = True
      self._UpdateBlockingLowCoverageTracker(
          patch,
          expected_builders=expected_builders,
          successful_builders=successful_builders,
          has_builder_failure=has_builder_failure)

    if _IsTryBuild() and build.status == common_pb2.Status.SUCCESS:
      # For presubmit coverage, save the whole data in json.
      # Assume there is only 1 patch which is true in CQ.
      assert len(build.input.gerrit_changes) == 1, 'Expect only one patchset'
      assert (len(mimic_builder_names) == len(gs_metadata_dirs)
             ), 'mimic builder names and gs paths should be of the same length'
      for gs_metadata_dir, mimic_builder_name in zip(gs_metadata_dirs,
                                                     mimic_builder_names):
        full_gs_metadata_dir = '/%s/%s' % (gs_bucket, gs_metadata_dir)
        all_json_gs_path = '%s/all.json.gz' % full_gs_metadata_dir
        data = _GetValidatedData(all_json_gs_path)
        patch = build.input.gerrit_changes[0]
        is_rts = properties.get('rts_was_used', False)
        self._ProcessCLPatchData(mimic_builder_name, patch, data['files'],
                                 is_rts)
      self._UpdateBlockingLowCoverageTracker(
          patch=build.input.gerrit_changes[0],
          processed_builders=[build.builder.builder])
      if PresubmitCoverageData.Get(
          server_host=patch.host, change=patch.change,
          patchset=patch.patchset) and LowCoverageBlocking.Get(
              server_host=patch.host,
              change=patch.change,
              patchset=patch.patchset
          ).blocking_status == BlockingStatus.READY_FOR_VERDICT:
        logging.info("checking for low coverage for change=%d, patch=%d",
                     patch.change, patch.patchset)
        self._MayBeBlockCLForLowCoverage(patch)
    elif not _IsTryBuild():  # CI builds are processed even with failures
      if not gs_bucket or not gs_metadata_dirs:
        logging.error('coverage GS bucket info not available in %r', build.id)
        return
      if not mimic_builder_names:
        logging.error('Couldn\'t find valid mimic_builder_names property from '
                      'builder output properties.')
        return
      assert (len(mimic_builder_names) == len(gs_metadata_dirs)
             ), 'mimic builder names and gs paths should be of the same length'
      if properties.get('coverage_override_gitiles_commit', False):
        self._SetGitilesCommitFromOutputProperty(
            build, properties['gitiles_commit_host'],
            properties['gitiles_commit_project'],
            properties['gitiles_commit_ref'], properties['gitiles_commit_id'])
      elif not self._IsGitilesCommitAvailable(build.input.gitiles_commit):
        self._SetGitilesCommitFromOutputProperty(
            build, build.output.gitiles_commit.host,
            build.output.gitiles_commit.project,
            build.output.gitiles_commit.ref, build.output.gitiles_commit.id)

      assert self._IsGitilesCommitAvailable(build.input.gitiles_commit), (
          'gitiles commit information is expected to be available either in '
          'input properties or output properties')

      for gs_metadata_dir, mimic_builder_name in zip(gs_metadata_dirs,
                                                     mimic_builder_names):
        full_gs_metadata_dir = '/%s/%s' % (gs_bucket, gs_metadata_dir)
        all_json_gs_path = '%s/all.json.gz' % full_gs_metadata_dir
        data = _GetValidatedData(all_json_gs_path)
        self._ProcessFullRepositoryData(build.input.gitiles_commit, data,
                                        full_gs_metadata_dir, build.builder,
                                        build_id, mimic_builder_name)

  def _IsGitilesCommitAvailable(self, gitiles_commit):
    """Returns True if gitiles_commit is available in the property."""
    return (gitiles_commit.host and gitiles_commit.project and
            gitiles_commit.ref and gitiles_commit.id)

  def _SetGitilesCommitFromOutputProperty(self, build, gitiles_commit_host,
                                          gitiles_commit_project,
                                          gitiles_commit_ref,
                                          gitiles_commit_id):
    """Set gitiles_commit of the build to a custom value."""
    logging.info('Modifying gitiles commit in the input properties.')
    build.input.gitiles_commit.host = gitiles_commit_host
    build.input.gitiles_commit.project = gitiles_commit_project
    build.input.gitiles_commit.ref = gitiles_commit_ref
    build.input.gitiles_commit.id = gitiles_commit_id

  def HandlePost(self, **kwargs):
    """Loads the data from GS bucket, and dumps them into ndb."""
    logging.info('Processing: %s', self.request.path)
    match = _BUILD_ID_REGEX.match(self.request.path)
    if not match:
      logging.info('Build id not found')
      return

    build_id = int(match.group(1))
    return self._ProcessCodeCoverageData(build_id)

  def HandleGet(self, **kwargs):
    return self.HandlePost(**kwargs)  # For local testing purpose.
