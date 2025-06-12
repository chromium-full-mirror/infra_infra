# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from recipe_engine.post_process import (DropExpectation, StatusSuccess,
                                        StepCommandContains,
                                        StepCommandDoesNotContain, StepSuccess)

DEPS = [
    'codesearch',
    'depot_tools/bot_update',
    'depot_tools/gclient',
    'recipe_engine/properties',
]


def RunSteps(api):
  api.gclient.set_config('infra')
  update_step = api.bot_update.ensure_checkout()
  properties = update_step.properties
  if api.properties.get('set_got_revision_cp_to_none'):
    properties.pop('got_revision_cp', 0)
  api.codesearch.set_config(
      api.properties.get('codesearch_config', 'chromium'),
      PROJECT=api.properties.get('project', 'chromium'),
      PLATFORM=api.properties.get('platform', 'linux'),
      SYNC_GENERATED_FILES=api.properties.get('sync_generated_files', True),
      GEN_REPO_BRANCH=api.properties.get('gen_repo_branch', 'main'),
      CORPUS=api.properties.get('corpus', 'chromium'),
      ROOT=api.properties.get('root', 'linux'),
  )
  index_pack_path = api.codesearch.create_kythe_index_pack(
      clang_target_arch=api.properties.get('target_architecture', None),
  )
  assert index_pack_path


def GenTests(api):

  def GetBasicStepChecks(project):
    return (
        api.post_process(StepSuccess, 'create kythe index pack'),
        api.post_process(StepCommandContains, 'create kythe index pack', [
            '--project',
            project,
        ]),
    )

  yield api.test(
      'basic',
      *GetBasicStepChecks('chromium'),
      api.post_process(StatusSuccess),
      api.post_process(DropExpectation),
  )

  yield api.test(
      'basic_chromiumos',
      api.properties(codesearch_config='chromiumos', project='chromiumos'),
      *GetBasicStepChecks('chromiumos'),
      api.post_process(StepCommandDoesNotContain, 'create kythe index pack',
                       ['--clang_target_arch']),
      api.post_process(StatusSuccess),
      api.post_process(DropExpectation),
  )

  yield api.test(
      'chromiumos_with_target_architecture',
      api.properties(
          codesearch_config='chromiumos',
          project='chromiumos',
          target_architecture='arm64'),
      *GetBasicStepChecks('chromiumos'),
      api.post_process(StepCommandContains, 'create kythe index pack',
                       ['--clang_target_arch', 'arm64']),
      api.post_process(StatusSuccess),
      api.post_process(DropExpectation),
  )

  yield api.test(
      'basic_without_got_revision_cp',
      api.properties(set_got_revision_cp_to_none=True),
      *GetBasicStepChecks('chromium'),
      api.post_process(StatusSuccess),
      api.post_process(DropExpectation),
  )

  yield api.test(
      'basic_without_kythe_root',
      api.properties(root=''),
      *GetBasicStepChecks('chromium'),
      api.post_process(StatusSuccess),
      api.post_process(DropExpectation),
  )
