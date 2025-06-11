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
    'recipe_engine/path',
    'recipe_engine/properties',
    'recipe_engine/step',
]


def RunSteps(api):
  api.codesearch.set_config('chromium', PROJECT='chromium')
  kzip1 = api.path.start_dir / 'path' / 'to' / 'foo.kzip'
  kzip2 = api.path.start_dir / 'path' / 'to' / 'bar.kzip'
  kzip3 = api.path.start_dir / 'path' / 'to' / 'baz.kzip'
  combined_kzip_path = api.codesearch.run_kzip_merge(kzip1, kzip2, kzip3)
  assert combined_kzip_path


def GenTests(api):
  yield api.test(
      'do merge',
      api.post_process(
          StepCommandContains,
          'merge kzips',
          [
              '[START_DIR]/packages/kythe/tools/kzip',
              'merge',
              '--encoding',
              'PROTO',
              '--output',
              '[CLEANUP]/tmp_tmp_1',
              '[START_DIR]/path/to/foo.kzip',
              '[START_DIR]/path/to/bar.kzip',
              '[START_DIR]/path/to/baz.kzip',
          ],
      ),
      api.post_process(StatusSuccess),
      api.post_process(DropExpectation),
  )
