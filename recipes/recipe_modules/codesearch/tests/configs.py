# Copyright 2017 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from recipe_engine import post_process

DEPS = [
    'codesearch',
    'recipe_engine/properties',
]


def RunSteps(api):
  api.codesearch.set_config(
      api.properties.get('codesearch_config', 'base'),
      PROJECT=api.properties.get('project', 'chromium'),
      PLATFORM=api.properties.get('platform', 'linux'),
      SYNC_GENERATED_FILES=api.properties.get('sync_generated_files', True),
      GEN_REPO_BRANCH=api.properties.get('gen_repo_branch', 'main'),
      CORPUS=api.properties.get('corpus', 'chromium'),
      ROOT=api.properties.get('root', 'linux'),
  )
  for config in api.properties.get('codesearch_apply_config', []):
    api.codesearch.apply_config(config)


def GenTests(api):
  yield api.test(
      'base',
      api.post_process(post_process.DropExpectation),
  )

  yield api.test(
      'chromium',
      api.properties(codesearch_apply_config=['chromium']),
      api.post_process(post_process.DropExpectation),
  )

  yield api.test(
      'chromiumos',
      api.properties(codesearch_apply_config=['chromiumos']),
      api.post_process(post_process.DropExpectation),
  )

  yield api.test(
      'chrome',
      api.properties(codesearch_apply_config=['chrome']),
      api.post_process(post_process.DropExpectation),
  )

  yield api.test(
      'chromeos',
      api.properties(codesearch_apply_config=['chromeos']),
      api.post_process(post_process.DropExpectation),
  )
