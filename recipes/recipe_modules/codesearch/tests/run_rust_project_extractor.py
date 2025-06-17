# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from recipe_engine.post_process import (
    DropExpectation,
    StatusException,
    StatusSuccess,
    StepCommandContains,
    SummaryMarkdownRE,
)
from recipe_engine.recipe_api import Property

DEPS = [
    'codesearch',
    'recipe_engine/json',
    'recipe_engine/path',
    'recipe_engine/properties',
]


PROPERTIES = {
    'out_path': Property(),
}


def RunSteps(api, out_path):
  api.path.checkout_dir = api.path.cache_dir.joinpath('builder')
  api.codesearch.set_config(
      'chromium', PROJECT='chromium', CORPUS='test-corpus')
  api.codesearch.c.out_path = out_path
  api.codesearch.run_rust_project_extractor(
      source_dir=api.path.checkout_dir.joinpath('src'))


def GenTests(api):
  yield api.test(
      'basic',
      api.properties(
          out_path=api.path.checkout_dir.joinpath('src', 'out', 'linux-Debug')),
      api.post_process(
          StepCommandContains,
          'extract Rust kzips',
          [
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
                  'pattern': '([^.].*)',
                  'vname': {
                      'path': 'linux-Debug/@1@',
                      'root': 'out'
                  },
              }]),
              '--root',
              '[CACHE]/builder/src/out/linux-Debug',
              '--project_json',
              '[CACHE]/builder/src/out/linux-Debug/rust-project.json',
          ]),
      api.post_process(StatusSuccess),
      api.post_process(DropExpectation),
  )

  yield api.test(
      'bad_out_path_wrong_parent',
      api.properties(
          out_path=api.path.checkout_dir.joinpath('src', 'not_out', 'Debug')),
      api.expect_exception('AssertionError'),
      api.post_process(
          SummaryMarkdownRE,
          'Expected parent of output root to be out/ but got "not_out"'),
      api.post_process(StatusException),
      api.post_process(DropExpectation),
  )

  yield api.test(
      'bad_out_path_missing_parent',
      api.properties(out_path=api.path.checkout_dir.joinpath('src', 'Debug')),
      api.expect_exception('AssertionError'),
      api.post_process(SummaryMarkdownRE,
                       'Expected parent of output root to be out/ but got ""'),
      api.post_process(StatusException),
      api.post_process(DropExpectation),
  )

  yield api.test(
      'bad_out_path_outside_checkout',
      api.properties(out_path=api.path.cache_dir),
      api.expect_exception('AssertionError'),
      api.post_process(SummaryMarkdownRE,
                       'Expected output root to be child of source dir'),
      api.post_process(StatusException),
      api.post_process(DropExpectation),
  )
