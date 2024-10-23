# Copyright 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import json

from recipe_engine import post_process

PYTHON_VERSION_COMPATIBILITY = 'PY2+3'

DEPS = [
    'recipe_engine/assertions',
    'recipe_engine/buildbucket',
    'recipe_engine/context',
    'recipe_engine/json',
    'recipe_engine/path',
    'recipe_engine/platform',
    'recipe_engine/properties',
    'recipe_engine/step',
    'infra_cipd',
]


def RunSteps(api):
  url = 'https://chromium.googlesource.com/infra/infra'
  rev = 'deadbeef' * 5
  # Assume path is where infra/infra is repo is checked out.
  path = api.path.cache_dir.joinpath('builder', 'assume', 'infra')
  with api.infra_cipd.context(path, api.properties.get('platform')):
    if api.platform.is_mac:
      api.infra_cipd.build(api.properties.get('signing_identity'))
    else:
      api.infra_cipd.build()
    api.infra_cipd.test()
    if not api.properties.get('no_buildnumbers'):
      api.infra_cipd.upload(api.infra_cipd.tags(url, rev))
    else:
      with api.assertions.assertRaises(ValueError):
        api.infra_cipd.upload(api.infra_cipd.tags(url, rev))


def GenTests(api):
  yield (api.test('luci-native') +
         api.properties(platform='linux-amd64') +
         api.buildbucket.ci_build(
             'infra-internal', 'ci', 'cross', build_number=5))
  yield (api.test('luci-native_codesign') + api.platform.name('mac') +
         api.properties(
             platform='mac-amd64',
             signing_identity='AAAAAAAAAAAAABBBBBBBBBBBBBXXXXXXXXXXXXXX') +
         api.buildbucket.ci_build(
             'infra-internal', 'ci', 'native', build_number=5))
  yield (
    api.test('no-buildnumbers') +
    api.properties(platform='linux-amd64', no_buildnumbers=True) +
    api.buildbucket.ci_build('infra-internal', 'ci', 'just-build-and-test') +
    api.post_process(post_process.DropExpectation))
