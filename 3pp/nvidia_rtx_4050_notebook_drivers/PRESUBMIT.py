# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

PRESUBMIT_VERSION = '2.0.0'


def _GetPCNDEnv(input_api):
  """Gets the common environment for running PCND tests."""
  PCND_env = dict(input_api.environ)
  current_path = input_api.PresubmitLocalPath()
  PCND_env.update({
      'PYTHONPATH': current_path,
      'PYTHONDONTWRITEBYTECODE': '1',
  })
  return PCND_env


def CheckScriptsUnittests(input_api, output_api):
  """Runs the unittests in the scripts/ directory."""
  return input_api.canned_checks.RunUnitTestsInDirectory(
      input_api,
      output_api,
      input_api.PresubmitLocalPath(),
      [r'^.+_unittest\.py$'],
      env=_GetPCNDEnv(input_api),
  )
