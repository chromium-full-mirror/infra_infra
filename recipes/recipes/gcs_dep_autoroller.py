# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from recipe_engine import post_process

from PB.recipes.infra import gcs_dep_autoroller as gcs_dep_autoroller_pb

DEPS = [
    'recipe_engine/file',
    'recipe_engine/path',
    'recipe_engine/properties',
    'depot_tools/git',
]

PROPERTIES = gcs_dep_autoroller_pb.Inputs


def RunSteps(api, inputs):
  source_dir = api.path.cleanup_dir / 'source_repo'
  api.git.checkout(
      inputs.source_url,
      submodules=False,
      ref='main',
      file_name='DEPS',
      dir_path=source_dir)
  destination_dir = api.path.cleanup_dir / 'destination_repo'
  api.git.checkout(
      inputs.destination_url,
      submodules=False,
      ref='main',
      file_name='DEPS',
      dir_path=destination_dir)

  api.file.listdir('list source repo', source_dir)
  api.file.listdir('list dest repo', destination_dir)


def GenTests(api):
  yield api.test(
      'basic',
      api.properties(
          source_url='https://chromium.googlesource.com/chromium/src.git',
          destination_url='https://pdfium.googlesource.com/pdfium.git',
      ),
      api.post_process(post_process.DropExpectation),
  )
