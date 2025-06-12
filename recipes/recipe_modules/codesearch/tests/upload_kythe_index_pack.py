# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from recipe_engine.post_process import (DropExpectation, StatusException,
                                        StatusSuccess, StepCommandContains,
                                        StepSuccess, SummaryMarkdown)

DEPS = [
    'codesearch',
    'depot_tools/bot_update',
    'depot_tools/gclient',
    'recipe_engine/path',
    'recipe_engine/properties',
]


def RunSteps(api):
  api.gclient.set_config('infra')
  update_step = api.bot_update.ensure_checkout()
  properties = update_step.properties
  kythe_commit_hash = 'a' * 40
  if api.properties.get('set_kythe_commit_hash_to_none'):
    kythe_commit_hash = None
  if api.properties.get('set_got_revision_cp_to_none'):
    properties.pop('got_revision_cp', 0)
  project = api.properties.get('project', 'chromium')
  platform = api.properties.get('platform', 'linux')
  api.codesearch.set_config(
      api.properties.get('codesearch_config', 'chromium'),
      PROJECT=project,
      PLATFORM=platform,
      SYNC_GENERATED_FILES=api.properties.get('sync_generated_files', True),
      GEN_REPO_BRANCH=api.properties.get('gen_repo_branch', 'main'),
      CORPUS=api.properties.get('corpus', 'chromium'),
  )
  basename = '%s_%s.kzip' % (project, platform)
  index_pack_kythe_path = api.codesearch.c.out_path / basename
  api.codesearch.upload_kythe_index_pack(
      index_pack_kythe_path=index_pack_kythe_path,
      commit_hash=kythe_commit_hash,
      commit_timestamp=1337000000,
      commit_position=123,
  )


def GenTests(api):

  def GetBasicStepChecks(project, revision):
    return (
        api.post_process(StepSuccess, 'gsutil upload kythe index pack'),
        api.post_process(
            StepCommandContains, 'gsutil upload kythe index pack', [
                'gs://chrome-codesearch/prod/%s_linux_%s+1337000000.kzip' %
                (project, revision),
            ]),
        api.post_process(StepSuccess, 'gsutil upload compile_commands.json'),
    )

  yield api.test(
      'basic',
      *GetBasicStepChecks('chromium', '123_' + ('a' * 40)),
      api.post_process(StatusSuccess),
      api.post_process(DropExpectation),
  )

  yield api.test(
      'basic_chromiumos',
      api.properties(codesearch_config='chromiumos', project='chromiumos'),
      *GetBasicStepChecks('chromiumos', 'a' * 40),
      api.post_process(StatusSuccess),
      api.post_process(DropExpectation),
  )

  yield api.test(
      'without_kythe_revision',
      api.properties(set_kythe_commit_hash_to_none=True),
      *GetBasicStepChecks('chromium', '123_None'),
      api.post_process(StatusSuccess),
      api.post_process(DropExpectation),
  )

  yield api.test(
      'bucket_name_not_set_failed',
      api.properties(codesearch_config='base'),
      api.expect_exception('AssertionError'),
      api.post_process(
          SummaryMarkdown,
          "Uncaught Exception: AssertionError('Trying to upload Kythe index "
          "pack but no google storage bucket name')"),
      api.post_process(StatusException),
      api.post_process(DropExpectation),
  )

  yield api.test(
      'basic_without_got_revision_cp',
      api.properties(set_got_revision_cp_to_none=True),
      *GetBasicStepChecks('chromium', '123_' + ('a' * 40)),
      api.post_process(StatusSuccess),
      api.post_process(DropExpectation),
  )
