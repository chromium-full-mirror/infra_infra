# Copyright 2018 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import contextlib
import collections

from recipe_engine import recipe_api


class InfraCIPDApi(recipe_api.RecipeApi):
  """API for building packages defined in infra's public and intenral repos.

  Essentially a shim around scripts in
  https://chromium.googlesource.com/infra/infra.git/+/main/build/
  and its internal counterpart.
  """

  def __init__(self, **kwargs):
    super(InfraCIPDApi, self).__init__(**kwargs)
    self._path_to_repo = None
    self._cipd_platform = None

  @contextlib.contextmanager
  def context(self, path_to_repo, cipd_platform):
    """Sets context building CIPD packages.

    Arguments:
      path_to_repo (path): path infra or infra_internal repo root dir.
        Expects to find `build/build.py` inside provided dir.
      cipd_platform (str): the target CIPD platform to build packages for.

    Doesn't support nesting.
    """
    if self._path_to_repo is not None:  # pragma: no cover
      raise ValueError('Nesting contexts not allowed')
    self._path_to_repo = path_to_repo
    self._cipd_platform = cipd_platform
    try:
      yield
    finally:
      self._path_to_repo = None
      self._cipd_platform = None

  @property
  def _ctx_path_to_repo(self):
    if self._path_to_repo is None:  # pragma: no cover
      raise Exception('must be run under infra_cipd.context')
    return self._path_to_repo

  @property
  def _ctx_cipd_platform(self):
    if self._cipd_platform is None:  # pragma: no cover
      raise Exception('must be run under infra_cipd.context')
    return self._cipd_platform

  def build(self, sign_id=None):
    """Builds packages."""
    args = [
        'vpython3',
        self._ctx_path_to_repo / 'build' / 'build.py',
        '--cipd-platform', self._ctx_cipd_platform,
    ]
    if sign_id:
      args.extend(['--signing-identity', sign_id])

    return self.m.step(
        'cipd %s: build' % self._ctx_cipd_platform,
        args,
    )

  def test(self):
    """Tests previously built packages integrity."""
    return self.m.step(
        'cipd %s: test' % self._ctx_cipd_platform,
        ['vpython3', self._ctx_path_to_repo / 'build' / 'test_packages.py'],
    )

  def upload(self, tags, step_test_data=None):
    """Uploads previously built packages."""
    args = [
      'vpython3',
      self._ctx_path_to_repo / 'build' / 'build.py',
      '--cipd-platform', self._ctx_cipd_platform,
      '--no-rebuild',
      '--upload',
      '--json-output', self.m.json.output(),
      '--tags',
    ] + list(tags)
    try:
      return self.m.step(
          'cipd %s: upload' % self._ctx_cipd_platform,
          args,
          step_test_data=step_test_data or self.test_api.example_upload,
      )
    finally:
      step_result = self.m.step.active_result
      output = step_result.json.output or {}
      p = step_result.presentation
      for pkg in output.get('succeeded', []):
        info = pkg['info']
        title = '%s %s' % (info['package'], info['instance_id'])
        p.links[title] = info.get(
            'url', 'http://example.com/not-implemented-yet')

  def tags(self, git_repo_url, revision):
    """Returns tags to be attached to uploaded CIPD packages."""
    if self.m.buildbucket.build.number <= 0:
      raise ValueError('buildnumbers must be enabled')
    return [
      'luci_build:%s/%s/%s' % (
        self.m.buildbucket.build.builder.bucket,
        self.m.buildbucket.build.builder.builder,
        self.m.buildbucket.build.number),
      'git_repository:%s' % git_repo_url,
      'git_revision:%s' % revision,
    ]
