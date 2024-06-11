# Copyright 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Rolls recipes.cfg dependencies for public projects."""

PYTHON_VERSION_COMPATIBILITY = 'PY2+3'

DEPS = [
    'recipe_autoroller',
    'recipe_engine/buildbucket',
    'recipe_engine/json',
    'recipe_engine/properties',
    'recipe_engine/proto',
    'recipe_engine/time',
]

from recipe_engine import recipe_api

PROPERTIES = {
    'projects':
        recipe_api.Property(),
}


def RunSteps(api, projects):
  return api.recipe_autoroller.roll_projects(projects)


def GenTests(api):
  yield api.test(
      'basic',
      api.buildbucket.generic_build(
          project='infra', bucket='cron', builder='recipe-autoroller'),
      api.properties(projects=[
          ('build', 'https://example.com/build.git'),
      ]),
      api.recipe_autoroller.roll_data('build'),
  )
