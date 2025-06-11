# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from recipe_engine.post_process import (
    DropExpectation,
    StatusSuccess,
    StepCommandContains,
)

DEPS = [
    'codesearch',
    'recipe_engine/json',
    'recipe_engine/path',
    'recipe_engine/properties',
]


def RunSteps(api):
  api.path.checkout_dir = api.path.cache_dir.joinpath('builder', 'src')
  api.codesearch.set_config(
      'chromium', PROJECT='chromium', CORPUS='test-corpus')
  api.codesearch.run_rust_project_extractor(checkout_dir=api.path.checkout_dir)


def GenTests(api):
  yield api.test(
      'basic',
      api.post_process(StepCommandContains, 'extract Rust kzips', [
          '[START_DIR]/packages/kythe/extractors/rustproject_extractor',
          '--corpus',
          'test-corpus',
          '--output',
          '[CLEANUP]/tmp_tmp_1',
          '--vnames_json_path',
          # api.raw_io.input_text outputs a placeholder that is resolved to a
          # filename when running the recipe, however for testing purposes will
          # resolve to the actual file contents.
          api.json.dumps([{
              'pattern': '../../(.*)',
              'vname': {
                  'path': '@1@'
              },
          }, {
              'pattern': '(.*)',
              'vname': {
                  'path': '@1@',
                  'root': 'out'
              },
          }]),
          '--root',
          '[CACHE]/builder/src/out/Debug',
          '--project_json',
          '[CACHE]/builder/src/out/Debug/rust-project.json',
      ]),
      api.post_process(StatusSuccess),
      api.post_process(DropExpectation),
  )
