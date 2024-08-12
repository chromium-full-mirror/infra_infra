# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from recipe_engine import post_process

DEPS = [
    'secret_manager',
]


def RunSteps(api):
  with api.secret_manager.fetch('foo-proj', 'foo-secret') as secret:
    assert (secret == 'abcd1234')


def GenTests(api):
  yield api.test(
      'basic',
      api.post_process(post_process.StepCommandRE,
                       'fetch secret.get content from secret manager', [
                           'vpython3',
                           '.*cloud_secret_manager.py',
                           '--project',
                           'foo-proj',
                           '--secret',
                           'foo-secret',
                           '--output',
                           '.*',
                       ]),
      api.post_process(post_process.DropExpectation),
  )
