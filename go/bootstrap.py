#!/usr/bin/env python3
# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Prepares a local Go installation."""

from __future__ import absolute_import
from __future__ import print_function
import argparse
import collections
import contextlib
import json
import logging
import os
import re
import shutil
import stat
import subprocess
import sys
import tempfile


LOGGER = logging.getLogger(__name__)


# If this env var is set to '1', bootstrap.py will not "go install ..." tools.
INFRA_GO_SKIP_TOOLS_INSTALL = 'INFRA_GO_SKIP_TOOLS_INSTALL'

# This env var defines what version variant of the toolset to install.
INFRA_GO_VERSION_VARIANT = 'INFRA_GO_VERSION_VARIANT'

# /path/to/infra
INFRA_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# The current overarching Infra version. If this changes, everything will be
# updated regardless of its version.
INFRA_VERSION = 1

# Default workspace with infra go code.
WORKSPACE = os.path.join(INFRA_ROOT, 'go')

# Platform depended suffix for executable files.
EXE_SFX = '.exe' if sys.platform == 'win32' else ''

# On Windows we use git from depot_tools.
GIT_EXE = 'git.bat' if sys.platform == 'win32' else 'git'

# A file with info about tools installed into GOBIN.
TOOLS_SPEC_FILE = '.tools_spec.json'

# Version of Go CIPD package (infra/3pp/tools/go/${platform}) to install per
# value of INFRA_GO_VERSION_VARIANT env var.
#
# Some builders use "legacy" and "bleeding_edge" variants. This mechanism is
# sometimes used to test new toolsets. Most of the time all variants are set
# to the same version.
#
# A value of `go.mod` means to read the version from `toolchain` directive in
# the main module's go.mod file.
TOOLSET_VERSIONS = {
    'default': 'go.mod',
    'legacy': 'go.mod',
    'bleeding_edge': 'go.mod',
}

# Layout is the layout of the bootstrap installation.
Layout = collections.namedtuple(
    'Layout',
    (
        # The path where the Go toolset is checked out at.
        'toolset_root',

        # The workspace path.
        'workspace',

        # Path to the main module's go.mod file.
        'go_mod',

        # The list of paths to tools.go files which are parsed to figure out
        # what binaries to "go install ..." into the GOBIN.
        'go_tools_specs',

        # A list of directories to delete. Useful when changing the layout
        # structure and cleaning up .gitignore.
        'cleanup_dirs',
    ))


# A base empty Layout.
_EMPTY_LAYOUT = Layout(
    toolset_root=None,
    workspace=None,
    go_mod=None,
    go_tools_specs=None,
    cleanup_dirs=None)


# Infra standard layout.
LAYOUT = Layout(
    toolset_root=os.path.join(WORKSPACE, 'golang'),
    workspace=WORKSPACE,
    go_mod=os.path.join(WORKSPACE, 'src', 'infra', 'go.mod'),
    # Note: order is important, a tool is installed only the first time it is
    # mentioned, using go.mod matching the corresponding tools.go for
    # dependencies.
    go_tools_specs=[
        os.path.join(WORKSPACE, 'src', 'go.chromium.org', 'luci', 'tools.go'),
        os.path.join(WORKSPACE, 'src', 'infra', 'tools.go'),
    ],
    cleanup_dirs=[
        # Clean up former locations of GOCACHE and GOMODCACHE directories after
        # they were moved under layout.toolset_root. These directories used to
        # be in .gitignore. If we don't clean them, they'll show up in git
        # diffs.
        os.path.join(WORKSPACE, '.gocache'),
        os.path.join(WORKSPACE, '.modcache'),
        # Same for old directories that are completely unused now.
        os.path.join(WORKSPACE, '.cache'),
        os.path.join(WORKSPACE, '.glide'),
        os.path.join(WORKSPACE, '.vendor'),
    ],
)


# Describes a modification of os.environ, see get_go_environ_diff(...).
EnvironDiff = collections.namedtuple('EnvironDiff', [
    'env',          # {k:v} with vars to set or delete (if v == None)
    'env_prefixes', # {k: [path]} with entries to prepend
    'env_suffixes', # {k: [path]} with entries to append
])


class Failure(Exception):
  """Bootstrap failed."""


def read_file(path):
  """Returns contents of a given file or None if not readable."""
  assert isinstance(path, (list, tuple))
  try:
    with open(os.path.join(*path), 'r') as f:
      return f.read()
  except IOError:
    return None


def write_file(path, data):
  """Writes |data| to a file."""
  assert isinstance(path, (list, tuple))
  with open(os.path.join(*path), 'w') as f:
    f.write(data)


def remove_file(path):
  """Removes the given file if it exists."""
  assert isinstance(path, (list, tuple))
  try:
    os.remove(os.path.join(*path))
  except FileNotFoundError:
    pass


def read_json(path):
  """Reads |path| and parses it as JSON, returning None if it is missing."""
  blob = read_file(path)
  return json.loads(blob) if blob is not None else None


def write_json(path, data):
  """Serializes |data| to JSON and writes it at |path|."""
  write_file(path, json.dumps(data, indent=2, sort_keys=True))


def remove_directory(p):
  """Recursively removes a directory."""
  if not os.path.exists(p):
    return
  # Crutch to remove read-only files (.git/* and .modcache/* in particular).
  touched = set()

  def onerror(func, path, _exc_info):
    # Need to make both the file and its parent directory writable.
    for p in (path, os.path.dirname(path)):
      if p not in touched:
        os.chmod(p, os.stat(p).st_mode | stat.S_IWUSR)
        touched.add(p)
    func(path)

  shutil.rmtree(p, onerror=onerror)


def read_toolchain(p):
  """Reads `toolchain ...` directive from go.mod."""
  with open(p, 'r') as f:
    for line in f:
      if line.startswith('toolchain '):
        val = line.split(' ')[1].strip()
        if not val.startswith('go'):
          raise ValueError('Bad toolchain directive: %s' % line.strip())
        return val[2:]
  raise ValueError('%s doesn\'t have toolchain directive' % p)


def install_toolset(toolset_root, version):
  """Downloads and installs Go toolset from CIPD.

  GOROOT would be <toolset_root>/go/.
  """
  cmd = subprocess.Popen([
      'cipd.bat' if sys.platform == 'win32' else 'cipd',
      'ensure',
      '-ensure-file',
      '-',
      '-root',
      toolset_root,
  ],
                         stdin=subprocess.PIPE,
                         universal_newlines=True)
  cmd.communicate('$ParanoidMode CheckIntegrity\n'
                  '@Subdir go\n'
                  'infra/3pp/tools/go/${platform} version:3@%s\n' % version)
  if cmd.returncode:
    raise Failure('CIPD call failed, exit code %d' % cmd.returncode)
  LOGGER.info('Validating...')
  check_hello_world(toolset_root)


def cleanup_toolset(toolset_root):
  """Deletes files installed by install_toolset(...)."""
  remove_directory(os.path.join(toolset_root, 'go'))


@contextlib.contextmanager
def temp_dir(path):
  """Creates a temporary directory, then deletes it."""
  tmp = tempfile.mkdtemp(dir=path)
  try:
    yield tmp
  finally:
    remove_directory(tmp)


def check_hello_world(toolset_root):
  """Compiles and runs 'hello world' program to verify that toolset works."""
  with temp_dir(toolset_root) as tmp:
    path = os.path.join(tmp, 'hello.go')
    write_file([path], r"""
        package main
        import "fmt"
        func main() { fmt.Println("hello, world") }
    """)
    out = call_bare_go(toolset_root, tmp, ['run', path])
    if out != 'hello, world':
      raise Failure('Unexpected output from the sample program:\n%s' % out)


def call_bare_go(toolset_root, workspace, args):
  """Calls 'go <args>' in the given workspace scrubbing all other Go env vars.

  Args:
    toolset_root: where Go is installed at.
    workspace: the working directory.
    args: command line arguments for 'go' tool.

  Returns:
    Captured stripped stdout+stderr.

  Raises:
    Failure if the call failed. All details are logged in this case.
  """
  cmd = [get_go_exe(toolset_root)] + args
  env = get_go_environ(_EMPTY_LAYOUT._replace(
      toolset_root=toolset_root,
      workspace=workspace))
  proc = subprocess.Popen(
      cmd,
      env=env,
      cwd=workspace,
      stdout=subprocess.PIPE,
      stderr=subprocess.STDOUT,
      universal_newlines=True)
  out, _ = proc.communicate()
  if proc.returncode:
    LOGGER.error('Failed to run %s: exit code %d', cmd, proc.returncode)
    LOGGER.error('Environment:')
    for k, v in sorted(env.items()):
      LOGGER.error('  %s = %s', k, v)
    LOGGER.error('Output:\n\n%s', out)
    raise Failure('Go invocation failed, see the log')
  return out.strip()


def infra_version_outdated(root):
  infra = read_file([root, 'INFRA_VERSION'])
  if not infra:
    return True
  return int(infra.strip()) < INFRA_VERSION


def write_infra_version(root):
  write_file([root, 'INFRA_VERSION'], str(INFRA_VERSION))


def ensure_toolset_installed(toolset_root, version):
  """Installs or updates Go toolset if necessary.

  Returns True if new toolset was installed.
  """
  installed = read_file([toolset_root, 'INSTALLED_TOOLSET'])
  if infra_version_outdated(toolset_root):
    LOGGER.info('Infra version is out of date.')
  elif installed == version:
    LOGGER.debug('Go toolset is up-to-date: %s', installed)
    return False

  LOGGER.info('Installing Go toolset.')
  LOGGER.info('  Old toolset is %s', installed)
  LOGGER.info('  New toolset is %s', version)
  remove_file([toolset_root, 'INSTALLED_TOOLSET'])
  cleanup_toolset(toolset_root)
  install_toolset(toolset_root, version)
  LOGGER.info('Go toolset installed: %s', version)
  write_file([toolset_root, 'INSTALLED_TOOLSET'], version)
  write_infra_version(toolset_root)
  return True


def get_git_repository_head(path):
  head = subprocess.check_output([GIT_EXE, '-C', path, 'rev-parse', 'HEAD'],
                                 universal_newlines=True)
  return head.strip()


def install_go_tools(layout, force):
  """Calls "go install ..." for each tool mentioned in tools.go.

  Always succeeds, even if some installation fails. This happens quite often
  if go.mod/go.sum is in a bad shape. We still need a working Go environment
  to fix them (e.g. to run "go mod tidy" or "go get"), so a failing installation
  should not block the bootstrap.
  """
  if not layout.go_tools_specs:
    return

  # Figure out what needs to be installed and from where.
  spec = []
  seen = set()
  for path in layout.go_tools_specs:
    with open(path, 'r') as f:
      tools = re.findall(r'^\s+_ "(.*)" *(//.*)?$', f.read(), re.MULTILINE)
    spec.append({
        'spec':
            os.path.relpath(path, layout.workspace),
        'rev':
            get_git_repository_head(os.path.dirname(path)),
        'tools': [
            tool[0]
            for tool in tools
            if tool[0] not in seen and 'noinstall' not in tool[1]
        ],
    })
    seen.update(spec[-1]['tools'])

  # Use HEAD revisions to detect if anything has changed. This is sloppy but
  # significantly faster than relying on Go to check its build caches.
  tools_spec_path = [layout.workspace, TOOLS_SPEC_FILE]
  if not force and read_json(tools_spec_path) == spec:
    return

  ok = True
  for entry in spec:
    if not entry['tools']:
      continue
    LOGGER.info('Installing tools from %s', entry['spec'])
    code = subprocess.call(
        [get_go_exe(layout.toolset_root), 'install'] + entry['tools'],
        stdout=sys.stderr,
        stderr=sys.stderr,
        cwd=os.path.join(layout.workspace, os.path.dirname(entry['spec'])),
        env=get_go_environ(layout))
    if code:
      LOGGER.warning(
          'Failed to install Go tools, your gclient checkout is likely in '
          'inconsistent state.')
      ok = False

  if ok:
    write_json(tools_spec_path, spec)


def get_go_environ_diff(layout):
  """Returns what modifications must be applied to the environ to enable Go.

  Pure function of 'layout', doesn't depend on current os.environ or state on
  disk.

  Args:
    layout: The Layout to derive the environment from.

  Returns:
    EnvironDiff.
  """
  # Need to make sure we pick up our `go` and tools before the system ones.
  path_prefixes = [
      os.path.join(layout.toolset_root, 'go', 'bin'),
      os.path.join(INFRA_ROOT, 'cipd'),
      os.path.join(INFRA_ROOT, 'cipd', 'bin'),
      os.path.join(INFRA_ROOT, 'luci', 'appengine', 'components', 'tools'),
      os.path.join(INFRA_ROOT, 'cipd', 'gcloud', 'bin'),
  ]

  # GOBIN often contain "WIP" variant of system binaries, pick them up last.
  path_suffixes = [os.path.join(layout.workspace, 'bin')]

  env = {
      'GOROOT': os.path.join(layout.toolset_root, 'go'),
      'GOBIN': os.path.join(layout.workspace, 'bin'),

      # Run in modules mode.
      'GO111MODULE': 'on',
      'GOPROXY': None,
      'GOPATH': None,
      'GOPRIVATE': '*.googlesource.com,*.git.corp.google.com,google.com',

      # Don't use default cache in '~' to avoid leaking it on builders.
      'GOCACHE': os.path.join(layout.toolset_root, 'gocache'),
      'GOMODCACHE': os.path.join(layout.toolset_root, 'modcache'),

      # Use only the toolchain version we fetched from CIPD. We prefer updating
      # the code to be compatible with the latest toolchain rather than using
      # multiple different toolchains at the same time.
      'GOTOOLCHAIN': 'local',

      # Infra Go workspace doesn't use advanced build systems,
      # which inject custom `gopackagesdriver` binary. See also
      # https://github.com/golang/tools/blob/54c614fe050cac95ace393a63f164149942ecbde/go/packages/external.go#L49
      'GOPACKAGESDRIVER': 'off',

      # Instruct `gae.py deploy` to use modules-aware cloudbuildhelper to
      # stage *.go files before deployment.
      'GAE_PY_USE_CLOUDBUILDHELPER': '1',
  }

  if sys.platform == 'win32':
    # Windows doesn't have gcc.
    env['CGO_ENABLED'] = '0'

  return EnvironDiff(
      env=env,
      env_prefixes={'PATH': path_prefixes},
      env_suffixes={'PATH': path_suffixes},
  )


def get_go_environ(layout):
  """Returns a copy of os.environ with mutated GO* environment variables.

  This function primarily targets environ on workstations. It assumes
  the developer may be constantly switching between infra and infra_internal
  go environments and it has some protection against related edge cases.

  Args:
    layout: The Layout to derive the environment from.
  """
  diff = get_go_environ_diff(layout)

  env = os.environ.copy()
  for k, v in diff.env.items():
    if v is not None:
      env[k] = v
    else:
      env.pop(k, None)

  path = env['PATH'].split(os.pathsep)
  path_prefixes = diff.env_prefixes['PATH']
  path_suffixes = diff.env_suffixes['PATH']

  def should_keep(p):
    # Skip entries we'll insert in the correct position below.
    p = os.path.normpath(p)
    if p in path_prefixes or p in path_suffixes:
      return False
    # Skip GOROOT/bin of anything resembling an infra Go environment. That way
    # switching between different environments works correctly. It is important
    # when switching between infra and infra_internal environments. Use
    # INSTALLED_TOOLSET marker file to detect an infra environment.
    if (p.endswith(os.path.join('go', 'bin')) and
        os.path.exists(os.path.join(p, '..', '..', 'INSTALLED_TOOLSET'))):
      return False
    # Skip GOBIN with tools from potentially different Go environment. Use
    # TOOLS_SPEC_FILE marker file to detect this. It is present if anything was
    # installed into GOBIN.
    if (p.endswith(os.sep + 'bin') and
        os.path.exists(os.path.join(p, '..', TOOLS_SPEC_FILE))):
      return False
    return True
  path = list(filter(should_keep, path))

  # Insert new entries to PATH.
  env['PATH'] = os.pathsep.join(path_prefixes + path + path_suffixes)

  # Add a tag to the prompt
  infra_prompt_tag = env.get('INFRA_PROMPT_TAG')
  if infra_prompt_tag is None:
    infra_prompt_tag = '[cr go] '
  if infra_prompt_tag:
    prompt = env.get('PS1')
    if prompt and infra_prompt_tag not in prompt:
      env['PS1'] = infra_prompt_tag + prompt

  return env


def get_go_exe(toolset_root):
  """Returns path to go executable."""
  return os.path.join(toolset_root, 'go', 'bin', 'go' + EXE_SFX)


def bootstrap(layout, logging_level, args=None):
  """Installs all dependencies in default locations.

  Supposed to be called at the beginning of some script (it modifies logger).

  Args:
    layout: instance of Layout describing what to install and where.
    logging_level: logging level of bootstrap process.
    args: positional arguments of bootstrap.py (if any).

  Raises:
    Failure if bootstrap fails.
  """
  logging.basicConfig()
  LOGGER.setLevel(logging_level)

  # One optional positional argument is a path to write JSON with env diff to.
  # This is used by recipes which use it in `with api.context(env=...): ...`.
  json_output = None
  if args is not None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        'json_output',
        nargs='?',
        metavar='PATH',
        help='Where to write JSON with necessary environ adjustments')
    json_output = parser.parse_args(args=args).json_output

  # Figure out what Go version to install based on INFRA_GO_VERSION_VARIANT.
  variant = os.environ.get(INFRA_GO_VERSION_VARIANT) or 'default'
  toolset_version = TOOLSET_VERSIONS.get(variant)
  if not toolset_version:
    raise Failure('Unrecognized INFRA_GO_VERSION_VARIANT %r' % variant)
  if toolset_version == 'go.mod':
    toolset_version = read_toolchain(layout.go_mod)

  # We may need to build Go binaries during bootstrap, so make sure
  # cross-compilation mode is disabled . Restore it back once done.
  prev_environ = {}
  for k in ('GOOS', 'GOARCH', 'GOARM'):
    prev_environ[k] = os.environ.pop(k, None)

  try:
    toolset_updated = ensure_toolset_installed(
        layout.toolset_root, toolset_version)
    if os.environ.get(INFRA_GO_SKIP_TOOLS_INSTALL) != '1':
      install_go_tools(layout, toolset_updated)
  finally:
    # Restore os.environ back. Have to do it key-by-key to actually modify the
    # process environment (replacing os.environ object as a whole does nothing).
    for k, v in prev_environ.items():
      if v is not None:
        os.environ[k] = v

  # Clean up no longer used directory that are or were in .gitignore. This is
  # useful when changing the structure of the layout.
  for p in (layout.cleanup_dirs or []):
    if os.path.exists(p):
      LOGGER.info('Cleaning up no longer necessary directory: %s', p)
      remove_directory(p)

  output = get_go_environ_diff(layout)._asdict()
  output['go_version'] = toolset_version

  json_blob = json.dumps(
      output,
      sort_keys=True,
      indent=2,
      separators=(',', ': '))

  if json_output == '-':
    print(json_blob)
  elif json_output:
    with open(json_output, 'w') as f:
      f.write(json_blob)


def prepare_go_environ():
  """Returns dict with environment variables to set to use Go toolset.

  Installs or updates the toolset if necessary.
  """
  bootstrap(LAYOUT, logging.INFO)
  return get_go_environ(LAYOUT)


def find_executable(name, workspaces):
  """Returns full path to an executable in some bin/ (in GOROOT or GOBIN)."""
  basename = name
  if EXE_SFX and basename.endswith(EXE_SFX):
    basename = basename[:-len(EXE_SFX)]
  roots = [os.path.join(LAYOUT.toolset_root, 'go', 'bin')]
  for path in workspaces:
    roots.append(os.path.join(path, 'bin'))
  for root in roots:
    full_path = os.path.join(root, basename + EXE_SFX)
    if os.path.exists(full_path):
      return full_path
  return name


def main(args):
  bootstrap(LAYOUT, logging.DEBUG, args)
  return 0


if __name__ == '__main__':
  sys.exit(main(sys.argv[1:]))
