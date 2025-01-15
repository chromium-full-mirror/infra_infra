#!/usr/bin/env python3
# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Runs all Go unit tests in a directory.

Expects Go toolset to be in PATH, GOPATH and GOROOT correctly set. Use ./env.py
to set them up.

Usage:
  test.py [root package path]

By default runs all tests for infra/*.
"""

# TODO(vadimsh): Get rid of this and call "go test ./..." directly from recipes.
# This file once had a much more complicated implementation that verified code
# coverage and allowed skipping tests per platform.

from __future__ import absolute_import
from __future__ import print_function
import errno
import json
import os
import subprocess
import sys

# /path/to/infra
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# Result adapter is deployed here by DEPS.
ADAPTER_DIR = os.path.join(ROOT, "cipd", "result_adapter")


def _check_go_available():
  """Returns True if go executable is in the PATH."""
  try:
    subprocess.check_output(['go', 'version'], stderr=subprocess.STDOUT)
    return True
  except subprocess.CalledProcessError:
    return False
  except OSError as err:
    if err.errno == errno.ENOENT:
      return False


def _clean_go_bin():
  """Removes all files in GOBIN.

  GOBIN is in PATH in our environment. There are some binaries there (like 'git'
  for gitwrapper), that get mistakenly picked up by the tests.
  """
  gobin = os.environ.get('GOBIN')
  if not gobin or not os.path.exists(gobin):
    return
  for p in os.listdir(gobin):
    os.remove(os.path.join(gobin, p))


def _use_resultdb():
  """Checks the luci context to determine if resultdb is configured."""
  ctx_filename = os.environ.get("LUCI_CONTEXT")
  if ctx_filename:
    try:
      with open(ctx_filename) as ctx_file:
        ctx = json.load(ctx_file)
        rdb = ctx.get('resultdb', {})
        return rdb.get('hostname') and rdb.get('current_invocation')
    except (OSError, ValueError):
      print(
          "Failed to open LUCI_CONTEXT; skip enabling resultdb integration",
          file=sys.stderr)
      return False
  return False


def _get_adapter_path():
  adapter_fname = "result_adapter"
  if sys.platform == "win32":
    adapter_fname += ".exe"
  return os.path.join(ADAPTER_DIR, adapter_fname)


def _print_and_run(command: list[str],
                   capture_stdout: bool = False) -> (int, bytes):
  print(f'$ {" ".join(command)}')
  p = subprocess.run(
      command, stdout=subprocess.PIPE if capture_stdout else None)
  if capture_stdout:
    sys.stdout.buffer.write(p.stdout)  # pass through stdout
  if p.returncode:
    print(f'## {" ".join(command)} had exit code {p.returncode}')
  return (p.returncode, p.stdout)


def _run_vet(package_root):
  """Runs 'go vet <package_root>/...'

  Returns:
   0 if and only if all tests pass.
  """
  if not _check_go_available():
    print('Can\'t find Go executable in PATH.')
    print('Go vet not supported')
    return 1

  # Turn off copylock analysis. Eventually, when we stop copying protobufs
  # in various places, we can turn it on.
  command = ['go', 'vet', '-copylocks=false', f'{package_root}/...']

  # TODO: adapt results of go vet to resultdb.

  return _print_and_run(command)[0]


def run_tests(package_root):
  """Runs 'go test <package_root>/...'.

  Exported symbol because this function is used outside this repo.

  Returns:
    0 if all tests pass..
  """
  command = ['go', 'test', '-v', f'{package_root}/...']

  prev_env = os.environ.copy()
  if _use_resultdb():
    # Silence goconvey reporter to avoid interference with result_adapter.
    # https://github.com/smartystreets/goconvey/blob/0fc5ef5371303f55e76d89a57286fb7076777e5b/convey/init.go#L37
    os.environ['GOCONVEY_REPORTER'] = 'silent'
    command = [_get_adapter_path(), 'go', '--'] + command
  try:
    # First run all tests with CGO disabled.
    os.environ['CGO_ENABLED'] = '0'
    (res, output) = _print_and_run(command, capture_stdout=True)
    # Look for tests which were marked skipped because they require CGO.
    rerun_tests = []
    rerun_next = False
    for line in output.splitlines():
      line = line.decode('utf-8')
      if 'Requires CGO_ENABLED=1' in line:
        rerun_next = True
      elif rerun_next and line.startswith(('ok', 'FAIL')):
        rerun_tests.append(line.split('\t')[1])
        rerun_next = False
    if rerun_tests:
      # Re-run skipped CGO tests with CGO enabled.
      os.environ['CGO_ENABLED'] = '1'
      print('Re-running tests requiring CGO: %r' % rerun_tests)
      del command[-1]
      command.extend(rerun_tests)
      res = _print_and_run(command)[0] or res
    return res
  finally:
    os.environ.clear()
    os.environ.update(prev_env)


def _run_build(package_root):
  """Runs 'go build <package_root>/...'.

  Returns:
    0 if everything builds.
  """
  return _print_and_run(['go', 'build', f'{package_root}/...'])[0]


def run_all(package_root):
  """Run go vet and then go tests

  Returns:
    0 if and only if all tests pass.
  """
  if not _check_go_available():
    print('Can\'t find Go executable in PATH.')
    print('Use python3 test.py')
    return 1
  _clean_go_bin()

  # Always run every applicable action so we give the user as much information
  # as possible.
  results = [
    _run_build(package_root),
    _run_vet(package_root),
    run_tests(package_root),
  ]

  for res in results:
    if res:
      return res
  return 0


def main(args):
  if not args:
    package_root = 'infra'
  elif len(args) == 1:
    package_root = args[0]
  else:
    print(sys.modules['__main__'].__doc__.strip(), file=sys.stderr)
    return 1
  return run_all(package_root)


if __name__ == '__main__':
  sys.exit(main(sys.argv[1:]))
