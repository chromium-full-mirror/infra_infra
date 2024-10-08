#!/usr/bin/env vpython3
# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Convenience script for expect_tests"""

from __future__ import absolute_import
from __future__ import print_function
assert __name__ == '__main__'

import os
import subprocess
import sys


INFRA_ROOT = os.path.dirname(os.path.abspath(__file__))


# Allowlist of packages to test on Windows.
WIN_ENABLED_PACKAGES = [
  'packages/infra_libs/infra_libs/infra_types',
  'packages/infra_libs/infra_libs/logs',
  'packages/infra_libs/infra_libs/ts_mon',
]

WIN_ENABLED_PACKAGES_PY3_ONLY = [
    'infra/libs/decorators',
    'infra/libs/service_utils',
]

# Test shared GAE code and individual GAE apps only on 64-bit Posix. This
# matches GAE environment.
TEST_GAE = sys.platform != 'win32' and sys.maxsize == (2**63) - 1

# Tests to run under vpython3. These should not be part of any
# of the py2 modules listed below, and must contain their own
# __main__ implementation.
VPYTHON3_TESTS = [
    'infra/tools/dockerbuild/test/smoke_test.py',
]

if TEST_GAE and os.path.isdir(os.path.join(INFRA_ROOT, 'appengine_module')):
  VPYTHON3_TESTS += [
      'appengine_module/gae_ts_mon/test/test.py',
  ]


def usage():
  print("""\nUsage: %s <action> [--py3] [<test names>] [<expect_tests options>]

  where <action> is one of: list, test, train, debug.

  Examples:
  Run all tests:
    ./test.py test
  Run all tests in the given package:
    ./test.py test infra
    ./test.py test appengine/cr-buildbucket
  Run all tests and generate an HTML report:
    ./test.py test infra --html-report /path/to/report/folder
  Run one given test in the infra package:
    ./test.py test infra/libs/git2/test:*testCommitBogus

  --py3 runs python 3 tests; otherwise python 2 tests are run.

  See expect_tests documentation for more details
  """ % sys.argv[0])


def get_modules_with_coveragerc(root_module):
  """Returns submodules that have .coveragerc file present."""
  root_dir = os.path.join(INFRA_ROOT, root_module.replace('/', os.sep))
  if not os.path.isdir(root_dir):
    return []
  return [
    '%s/%s' % (root_module, d)
    for d in os.listdir(root_dir)
    if os.path.isfile(os.path.join(root_dir, d, '.coveragerc'))
  ]


# Parse command-line arguments
if len(sys.argv) == 1:
  usage()
  sys.exit(1)
else:
  if not sys.argv[1] in ('list', 'train', 'test', 'debug'):
    usage()
    sys.exit(1)

command = sys.argv[1]
args = sys.argv[2:]

modules = []
flags = []
py3 = False
# BUG: this will append everything after the first flag to `flags`. Thus,
# it fails to catch when (a) someone doesn't pass a directory after
# "--html-report", nor (b) if they pass multiple directories after that
# flag.
for arg in args:
  if arg.startswith('-'):
    if arg == '--py3':
      py3 = True
    else:
      flags.append(arg)
    continue
  if flags:
    flags.append(arg)
  else:
    modules.append(arg)

if py3:
  python_bin = sys.executable
  expect_tests_path = os.path.join(
      os.path.dirname(sys.executable), 'expect_tests')
else:
  vpyout = subprocess.run([
    os.path.join(INFRA_ROOT, 'cipd', 'legacy', 'vpython'),
    '-c', 'import sys;sys.stdout.write(sys.executable)',
  ], capture_output=True, encoding='utf-8')
  if vpyout.returncode != 0:
    sys.stderr.write(vpyout.stderr)
    sys.stdout.write(vpyout.stdout)
    vpyout.check_returncode()
  python_bin = vpyout.stdout
  expect_tests_path = os.path.join(os.path.dirname(python_bin), 'expect_tests')

# Set up default list of packages/directories if none have been provided.
if not modules:
  # On Windows, test only allowlisted subset of 'infra' and 'packages' modules.
  if sys.platform == 'win32':
    modules.extend([
      p for p in WIN_ENABLED_PACKAGES
      if os.path.isdir(os.path.join(INFRA_ROOT, p))
    ])
    if py3:
      modules.extend([
          p for p in WIN_ENABLED_PACKAGES_PY3_ONLY
          if os.path.isdir(os.path.join(INFRA_ROOT, p))
      ])
  else:
    if py3:
      modules.extend(['infra'])
    modules.extend(get_modules_with_coveragerc('packages'))

  # Skip py2.7 GAE tests for any builders that don't pull the appengine
  # directory. For py3, these tests run via their own test.py rather than
  # expect_tests.
  if not py3 and os.path.isdir(os.path.join(INFRA_ROOT, 'appengine')):
    if TEST_GAE:
      modules.append('appengine_module')
      modules.extend(get_modules_with_coveragerc('appengine'))

os.environ['PYTHONPATH'] = ''
os.chdir(INFRA_ROOT)
if '--help' not in flags and '-h' not in flags:
  subprocess.check_call(
      ['python3', '-u',
       os.path.join('bootstrap', 'remove_orphaned_pycs.py'),
       'infra', 'appengine', 'packages', 'luci'])
else:
  usage()
  sys.exit(subprocess.call([python_bin, expect_tests_path, command, '--help']))

if sys.platform == 'win32' and '--force-coverage' not in flags:
  flags.append('--no-coverage')

exit_code = 0
failed_modules = []
for module in modules:
  print('Running %s...%s' % (module, ' (py3)' if py3 else ''))
  module_flags = flags[:]
  # Remove any test glob, which comes after semicolon (:) and convert to a path.
  module_path = module.split(':')[0].replace('/', os.sep)
  if not any(flag.startswith('--coveragerc') for flag in module_flags):
    module_coveragerc = os.path.join(INFRA_ROOT, module_path, '.coveragerc')
    module_flags.append('--coveragerc=%s' % module_coveragerc)
  if not any(flag.startswith('--html-report-subdir') for flag in module_flags):
    module_flags.append('--html-report-subdir=%s' % module_path)
  cmd = [python_bin, expect_tests_path, command, module] + module_flags
  module_exit_code = subprocess.call(cmd)
  exit_code = module_exit_code or exit_code
  if module_exit_code:
    failed_modules.append(module)

# Tests to run with vpython3
if py3:
  VPYTHON = 'vpython3' + ('.bat' if sys.platform == 'win32' else '')
  for test in VPYTHON3_TESTS:
    print('Running %s... (py3)' % test)
    cmd = [VPYTHON, test]
    test_exit_code = subprocess.call(cmd)
    exit_code = test_exit_code or exit_code
    if test_exit_code:
      failed_modules.append(test)

if exit_code:
  print()
  print('Tests failed in modules:\n  %s' % '\n  '.join(failed_modules))
  if '--html-report' not in flags:
    print('\nFor detailed coverage report and per-line branch coverage,')
    print('rerun with --html-report <dir>')
else:
  print('All tests passed.')

sys.exit(exit_code)
