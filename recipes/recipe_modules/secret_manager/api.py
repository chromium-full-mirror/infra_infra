# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from contextlib import contextmanager
from recipe_engine import recipe_api


class SecretManagerApi(recipe_api.RecipeApi):

  @contextmanager
  def fetch(self, project, secret, step_name='fetch secret'):
    """Yield the latest version of the secret from Cloud Secret Manager.

    Args:
      project: GCP project name in string.
      secret: Secret name in the Secret Manager.
    """
    assert project, 'Project name is required.'
    assert secret, 'Must specify the secret name.'
    with self.m.step.nest(step_name):
      s = self.m.step(
          'get content from secret manager', [
              'vpython3',
              self.resource('cloud_secret_manager.py'),
              '--project',
              project,
              '--secret',
              secret,
              '--output',
              self.m.raw_io.output(),
          ],
          infra_step=True,
          step_test_data=lambda: self.m.raw_io.test_api.output('abcd1234'))
      yield s.raw_io.output.decode('UTF-8')
