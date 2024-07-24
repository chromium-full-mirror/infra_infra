# Copyright 2021 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import logging
from datetime import datetime
from datetime import timedelta

from google.appengine.ext import ndb

from libs import time_util
from model.code_coverage import FileCoverageData
from model.code_coverage import PostsubmitReport
from services import bigquery_helper

_PAGE_SIZE = 100

# List of builders for which coverage metrics to be exported.
# These should be ci builders.
_CHROMIUM_SOURCE_BUILDERS = [
    'linux-code-coverage', 'linux-code-coverage_unit', 'win10-code-coverage',
    'win10-code-coverage_unit', 'android-code-coverage',
    'android-code-coverage_unit', 'ios-simulator-code-coverage',
    'ios-simulator-code-coverage_unit', 'linux-chromeos-code-coverage',
    'linux-chromeos-code-coverage_unit', 'linux-lacros-code-coverage',
    'linux-lacros-code-coverage_unit', 'mac-code-coverage',
    'mac-code-coverage_unit', 'android-code-coverage-native',
    'android-code-coverage-native_unit', 'android-cronet-code-coverage-java',
    'android-cronet-code-coverage-java_unit',
    'android-cronet-code-coverage-native',
    'android-cronet-code-coverage-native_unit'
]

_CHROMIUM_SOURCE_REVIVER_BUILDERS = [
    'android-code-coverage', 'android-code-coverage-native'
]


def ExportFilesAbsoluteCoverage():
  """Exports metrics for files to Bigquery.

  Reads FileCoverageData for latest revision and exports them to a Bigquery
  table.
  """

  for builder in _CHROMIUM_SOURCE_BUILDERS:
    _ExportAbsoluteCoverageForBuilder(
        builder=builder,
        project='chromium/src',
        server_host='chromium.googlesource.com')

  for reviver_builder in _CHROMIUM_SOURCE_REVIVER_BUILDERS:
    _ExportAbsoluteCoverageForBuilder(
        builder=reviver_builder,
        project='chromium/src',
        server_host='chromium.googlesource.com',
        bucket='reviver')

  _ExportAbsoluteCoverageForBuilder(
      builder='fuchsia-coverage',
      project='fuchsia',
      server_host='fuchsia.googlesource.com')


def _ExportAbsoluteCoverageForBuilder(builder,
                                      project,
                                      server_host,
                                      bucket='ci'):
  """Export Absolute coverage for files in a given ci builder

  Args:
    builder (String): Name of the CI builder.
  """
  total_rows = 0
  # Find latest revision
  query = PostsubmitReport.query(
      PostsubmitReport.gitiles_commit.server_host == server_host,
      PostsubmitReport.gitiles_commit.project == project,
      PostsubmitReport.bucket == bucket, PostsubmitReport.builder == builder,
      PostsubmitReport.visible == True, PostsubmitReport.modifier_id ==
      0).order(-PostsubmitReport.commit_timestamp)
  entities = query.fetch(limit=1)
  report = entities[0]
  latest_revision = report.gitiles_commit.revision
  logging.info("Latest Revision: %s", latest_revision)
  commit_timestamp = report.commit_timestamp
  # Process File Coverage reports for the latest revision
  query = FileCoverageData.query(
      FileCoverageData.gitiles_commit.server_host ==
      report.gitiles_commit.server_host,
      FileCoverageData.gitiles_commit.project == report.gitiles_commit.project,
      FileCoverageData.gitiles_commit.ref == report.gitiles_commit.ref,
      FileCoverageData.gitiles_commit.revision == latest_revision,
      FileCoverageData.bucket == report.bucket,
      FileCoverageData.builder == report.builder,
      FileCoverageData.modifier_id == 0)
  more = True
  cursor = None
  while more:
    # NDB caches each result in the in-context cache while accessing.
    # This is problematic as due to the size of the result set,
    # cache grows beyond the memory quota. `use_cache = False` turns this off.
    #
    # Read more at:
    # https://cloud.google.com/appengine/docs/standard/python/ndb/cache#incontext
    # https://github.com/googlecloudplatform/datastore-ndb-python/issues/156#issuecomment-110869490
    results, cursor, more = query.fetch_page(
        _PAGE_SIZE,
        start_cursor=cursor,
        config=ndb.ContextOptions(use_cache=False))
    bq_rows = _CreateBigqueryRows(results, commit_timestamp)
    if bq_rows:
      bigquery_helper.ReportRowsToBigquery(bq_rows, 'findit-for-me',
                                           'code_coverage_summaries',
                                           'files_absolute_coverage')
      total_rows += len(bq_rows)
    logging.info('Total rows added so far = %d', total_rows)

  logging.info('Total rows added for builder %s = %d', builder, total_rows)


def _CreateBigqueryRows(file_coverage_results, commit_timestamp):
  """Create bigquery rows containing absolute coverage info per file.

  Returns a list of dict objects whose keys are column names and
  values are column values corresponding to the schema of the bigquery table.

  Args:
    file_coverage_results (list): List of FileCoverageData for the latest
      full codebase report
    commit_timestamp (ndb.DateTimeProperty): Commit timestamp of the
      revision for which full codebase report was generated.
  """
  bq_rows = []
  for file_coverage_result in file_coverage_results:
    data = file_coverage_result.data
    for metric in data['summaries']:
      if metric['name'] == 'line':
        total_lines = metric['total']
        covered_lines = metric['covered']
        break
    bq_rows.append({
        'project': file_coverage_result.gitiles_commit.project,
        'revision': file_coverage_result.gitiles_commit.revision,
        'path': data['path'][2:],
        'total_lines': total_lines,
        'covered_lines': covered_lines,
        'commit_timestamp': commit_timestamp.isoformat(),
        'insert_timestamp': time_util.GetUTCNow().isoformat(),
        'builder': file_coverage_result.builder
    })
  return bq_rows
