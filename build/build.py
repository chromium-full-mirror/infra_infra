#!/usr/bin/env vpython3
# Copyright 2015 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This script builds Go binaries and invokes CIPD client to package and upload
them to the CIPD repository.

See build/packages/*.yaml for definition of packages and README.md for more
details.
"""

import argparse
import collections
import contextlib
import copy
import errno
import functools
import glob
import hashlib
import json
import os
import platform
import re
import shutil
import socket
import subprocess
import sys
import tempfile

import yaml

# Root of infra.git repository.
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# Where to upload packages to by default.
PACKAGE_REPO_SERVICE = 'https://chrome-infra-packages.appspot.com'

# Mapping of CIPD arch strings to Go build env vars.
#
# See https://pkg.go.dev/go.chromium.org/luci/cipd/client/cipd/ensure.
CIPD_ARCHS = {
    '386': {
        'GOARCH': '386',
        'GO386': 'sse2'
    },
    'amd64': {
        'GOARCH': 'amd64',
        'GOAMD64': 'v1'
    },
    'arm64': {
        'GOARCH': 'arm64'
    },
    'armv6l': {
        'GOARCH': 'arm',
        'GOARM': '6'
    },
    'armv7l': {
        'GOARCH': 'arm',
        'GOARM': '7'
    },
    'loong64': {
        'GOARCH': 'loong64'
    },
    'mips': {
        'GOARCH': 'mips',
        'GOMIPS': 'hardfloat'
    },
    'mips64': {
        'GOARCH': 'mips64',
        'GOMIPS64': 'hardfloat'
    },
    'mips64le': {
        'GOARCH': 'mips64le',
        'GOMIPS64': 'hardfloat'
    },
    'mipsle': {
        'GOARCH': 'mipsle',
        'GOMIPS': 'hardfloat'
    },
    'ppc64': {
        'GOARCH': 'ppc64',
        'GOPPC64': 'power8'
    },
    'ppc64le': {
        'GOARCH': 'ppc64le',
        'GOPPC64': 'power8'
    },
    'riscv64': {
        'GOARCH': 'riscv64'
    },
    's390x': {
        'GOARCH': 's390x'
    },
}

# Default values for some existing GO{ARCH} vars taken from
# https://github.com/golang/go/blob/master/src/cmd/dist/build.go
#
# This simplifies some code below. This also indirectly used to know what env
# vars affect the build.
#
# Note that GOARM has no predefined default value (it depends on the host
# environment) and we always require it to be set explicitly since we don't want
# our arm32 builds to depend on specifics of a particular builder machine.
GO_ARCH_DEFAULTS = {
    'GO386': 'sse2',
    'GOAMD64': 'v1',
    'GOARM': None,
    'GOMIPS': 'hardfloat',
    'GOMIPS64': 'hardfloat',
    'GOPPC64': 'power8',
}

# Go env vars potentially affecting the build (at least the ones we care about).
GO_BUILD_ENV_VARS = [
    'GOOS', 'GOARCH', 'CGO_ENABLED', 'CGO_CFLAGS', 'CGO_LDFLAGS'
] + list(GO_ARCH_DEFAULTS)

# All CIPD platforms that support 'go build -race'.
RACE_SUPPORTED_PLATFORMS = frozenset([
    'freebsd-amd64',
    'linux-amd64',
    'linux-arm64',
    'linux-ppc64le',
    'mac-amd64',
    'windows-amd64',
])

# A package prefix => cwd to use when building this package.
#
# Can be extended via `--map-go-module` command line flag.
DEFAULT_MODULE_MAP = {
    # The luci-go module is checked out separately, use its go.mod.
    'go.chromium.org/luci/':
        os.path.join(ROOT, 'go', 'src', 'go.chromium.org', 'luci'),
    # All infra packages should use go.mod in infra.git.
    'infra/':
        os.path.join(ROOT, 'go', 'src', 'infra'),
    'go.chromium.org/infra/':
        os.path.join(ROOT, 'go', 'src', 'infra'),
    # Use infra's go.mod when building goldctl.
    'go.skia.org/infra/gold-client/cmd/goldctl':
        os.path.join(ROOT, 'go', 'src', 'infra')
}


def check_output(*args, **kwargs):
  return subprocess.check_output(*args, text=True, **kwargs)


# This can be replaced with functools.cache once we are on Py 3.9
def cache(f):
  value = []

  @functools.wraps(f)
  def wrapper():
    if not value:
      value.append(f())
    return value[0]

  return wrapper


class PackageDefException(Exception):
  """Raised if a package definition is invalid."""
  def __init__(self, path, msg):
    super(PackageDefException, self).__init__('%s: %s' % (path, msg))


class UnsupportedException(Exception):
  """Raised if some combination of parameters is not supported."""


class BuildException(Exception):
  """Raised on errors during package build step."""


class SearchException(Exception):
  """Raised on errors during package search step."""


class TagException(Exception):
  """Raised on errors during package tag step."""


class UploadException(Exception):
  """Raised on errors during package upload step."""


class PackageDef(collections.namedtuple(
    '_PackageDef', ('path', 'pkg_def'))):
  """Represents parsed package *.yaml file."""

  @property
  def name(self):
    """Returns name of YAML file (without the directory path and extension)."""
    return os.path.splitext(os.path.basename(self.path))[0]

  @property
  def platforms(self):
    """Returns a list of CIPD platforms to build the package for."""
    return self.pkg_def.get('platforms') or []

  @property
  def disabled(self):
    """Returns True if the package should be excluded from the build."""
    return self.pkg_def.get('disabled', False)

  @property
  def update_latest_ref(self):
    """Returns True if 'update_latest_ref' in the YAML file is set.

    Defaults to True."""
    return bool(self.pkg_def.get('update_latest_ref', True))

  @property
  def go_packages(self):
    """Returns a list of Go packages that must be installed for this package."""
    return self.pkg_def.get('go_packages') or []

  def go_build_environ_key(self, key, cipd_platform):
    """Looks up a key in "go_build_environ" recognizing per-platform values."""
    val = self.pkg_def.get('go_build_environ', {}).get(key)

    # Legacy key name for "cgo".
    if val is None and key == 'cgo':
      val = self.pkg_def.get('go_build_environ', {}).get('CGO_ENABLED')
      if val:
        print('DEPRECATED: replace "CGO_ENABLED" with "cgo" in %s' % self.path)

    if not isinstance(val, dict):
      return val

    # Per-platform setting.
    if cipd_platform in val:
      return val[cipd_platform]

    # Per-OS setting.
    for k, v in val.items():
      if cipd_platform.startswith('%s-' % k):
        return v
      # Support 'darwin' GOOS as a legacy value for 'mac' CIPD OS.
      if k == 'darwin':
        print('DEPRECATED: replace "darwin" with "mac" in %s' % self.path)
      if k == 'darwin' and cipd_platform.startswith('mac-'):
        return v

    # No setting found for the platform.
    return None

  def cgo_enabled(self, cipd_platform):
    """True if the package needs cgo, False to disable it.

    By default cgo is disabled.
    """
    return bool(self.go_build_environ_key('cgo', cipd_platform))

  @property
  def pkg_root(self):
    """Absolute path to a package root directory."""
    root = self.pkg_def['root'].replace('/', os.sep)
    if os.path.isabs(root):
      return root
    return os.path.abspath(os.path.join(os.path.dirname(self.path), root))

  def with_race(self, cipd_platform):
    """Returns True if should build with `-race` flag.

    To build with race:
      - race should be enabled on the cipd_platform or in general;
      - cgo should be enabled on cipd_platform;
      - cipd_platform should be one of the supported platforms.
    """
    val = self.go_build_environ_key('race', cipd_platform)
    if not val:
      return False

    cgo_enabled = self.cgo_enabled(cipd_platform)
    if not cgo_enabled:
      print(
          'go build -race cannot be enabled because CGO is not enabled on %s' %
          cipd_platform)
      return False

    if cipd_platform in RACE_SUPPORTED_PLATFORMS:
      return True
    print('go build -race is not supported on %s' % cipd_platform)
    return False

  def validate(self):
    """Raises PackageDefException if the package definition looks invalid."""
    if not self.platforms:
      raise PackageDefException(
          self.path,
          'At least one platform should be specified in "platforms" section.')
    for var_name in self.pkg_def.get('go_build_environ', {}):
      if var_name not in ['CGO_ENABLED', 'cgo', 'race']:
        raise PackageDefException(
            self.path, 'Unsupported go_build_environ key %s' % var_name)

  def preprocess(self, build_root, pkg_vars, cipd_exe, sign_id=None):
    """Parses the definition and filters/extends it before passing to CIPD.

    This process may generate additional files that are put into the package.

    Args:
      build_root: root directory for building cipd package.
      pkg_vars: dict with variables passed to cipd as -pkg-var.
      cipd_exe: path to cipd executable.
      sign_id: identity used for Mac codesign.

    Returns:
      Path to filtered package definition YAML.

    Raises:
      BuildException on error.
    """
    pkg_def = copy.deepcopy(self.pkg_def)

    pkg_def['root'] = build_root

    bat_files = [
        d['file'] for d in pkg_def['data'] if d.get('generate_bat_shim')
    ]

    def process_cipd_export(ensure_contents,
                            dest,
                            pkg_vars=pkg_vars,
                            cipd_exe=cipd_exe):
      # Render target_platform in the ensure file.
      ensure_contents = ensure_contents.replace('${target_platform}',
                                                pkg_vars['platform'])
      cipd_export(ensure_contents, dest, cipd_exe)

    if 'mac_bundle' in pkg_def:
      bundle_def = pkg_def['mac_bundle']
      bundle = create_mac_bundle(build_root, bundle_def)
      pkg_def['data'].append({
          'dir':
              os.path.relpath(bundle['root'], build_root).replace(os.sep, '/')
      })

      for d in bundle_def['data']:
        if 'file' in d:
          file_path = render_path(d['file'], pkg_vars)
          src = os.path.join(self.pkg_root, file_path)
          dst = os.path.join(bundle['files_root'], d['path'],
                             os.path.basename(file_path))
          shutil.copy(src, dst)
        elif 'cipd_export' in d:
          process_cipd_export(d['cipd_export'], bundle['root'])

      if 'codesign' in bundle_def:
        cmd = ['/usr/bin/codesign', '--deep', '--force']
        if sign_id:
          for k, v in bundle_def['codesign'].items():
            cmd.extend(['--' + k, v])
          cmd.extend(['--sign', sign_id])
        else:
          # Ignoring all codesign args and use ad-hoc signing for testing.
          cmd.extend(['--sign', '-'])
        cmd.append(bundle['root'])

        print('Running %s' % ' '.join(cmd))
        subprocess.check_call(cmd)

    for cp in pkg_def.get('copies', ()):
      plat = cp.get('platforms')
      if plat and pkg_vars['platform'] not in plat:
        continue
      dst = os.path.join(build_root, render_path(cp['dst'], pkg_vars))
      shutil.copy(os.path.join(self.pkg_root, render_path(cp['src'], pkg_vars)),
                  dst)
      pkg_def['data'].append(
          {'file': os.path.relpath(dst, build_root).replace(os.sep, '/')})
      if cp.get('generate_bat_shim'):
        bat_files.append(cp['dst'])

    if 'cipd_export' in pkg_def:
      process_cipd_export(pkg_def['cipd_export'], build_root)

    # Copy all included files into build root if not existed. This must be after
    # steps generating files and before any steps referring a symbolic link.
    for d in self.pkg_def['data']:
      path = d.get('file') or d.get('dir')
      if path:
        copy_if_not_exist(self.pkg_root, build_root, path, pkg_vars)

    if not is_targeting_windows(pkg_vars):
      for sym in pkg_def.get('posix_symlinks', ()):
        dst = os.path.join(build_root, render_path(sym['dst'], pkg_vars))
        try:
          os.remove(dst)
        except OSError:
          pass
        os.symlink(
            os.path.join(build_root, render_path(sym['src'], pkg_vars)), dst)
        pkg_def['data'].append(
            {'file': os.path.relpath(dst, build_root).replace(os.sep, '/')})

    # Generate *.bat shims when targeting Windows.
    if is_targeting_windows(pkg_vars):
      for f in bat_files:
        # Generate actual *.bat.
        bat_abs = generate_bat_shim(build_root, render_path(f, pkg_vars))
        # Make it part of the package definition (use slash paths there).
        pkg_def['data'].append(
            {'file': os.path.relpath(bat_abs, build_root).replace(os.sep, '/')})
        # Stage it for cleanup.

    # Keep generated yaml in the same directory to avoid rewriting paths.
    out_path = os.path.join(build_root, self.name + '.processed_yaml')
    with open(out_path, 'w') as f:
      json.dump(pkg_def, f)
    return out_path

  def on_change_info(self, pkg_vars):
    """Returns tags and path to check package changed."""
    on_change_tags = [
        get_on_change_tag(self.pkg_root, d, pkg_vars)
        for d in self.pkg_def.get('upload_on_change', [])
    ]

    # If any on_change tags are in use, also create one for the spec itself.
    # This will capture changes such as `copies` or `cipd_export`.
    if on_change_tags:
      on_change_tags.append(
          get_on_change_tag(
              os.path.dirname(self.path), {'file': os.path.basename(self.path)},
              pkg_vars))

    pkg_path = render_path(
        self.pkg_def.get('package'), pkg_vars, replace_sep=False)
    return on_change_tags, pkg_path


class GoToolset(
    collections.namedtuple(
        'GoToolset',
        [
            'env',  # env vars to assign as {str => str}
            'env_prefixes',  # paths to prepend to existing vars {str => [str]}
            'env_suffixes',  # paths to append to existing vars {str => [str]}
            'version',  # go version
            'go_env',  # full 'go env' dict captured when installing the toolset
        ])):
  """Represents a Go build environment.

  Carries os.environ modifications necessary to activate a Go toolset.
  """

  def _apply_toolset_env(self):
    """Modifies current os.environ to activate the Go toolset."""
    for k, v in self.env.items():
      if v is not None:
        os.environ[k] = v
      else:
        os.environ.pop(k, None)
    def split_path(env_val, filter_out):
      if not env_val:
        return []
      return [p for p in env_val.split(os.pathsep) if p not in filter_out]
    # env_prefixes['PATH'] is e.g. `["/.../a/bin", ".../b/bin"]. Need to prepend
    # these paths to `PATH` to get "/.../a/bin:/.../b/bin:$PATH". Cleanup dups
    # while at it. Same for suffixes.
    for k, v in self.env_prefixes.items():
      cur = split_path(os.environ.get(k, ''), v)
      os.environ[k] = os.pathsep.join(v + cur)
    for k, v in self.env_suffixes.items():
      cur = split_path(os.environ.get(k, ''), v)
      os.environ[k] = os.pathsep.join(cur + v)

  @contextlib.contextmanager
  def build_env(self, go_environ):
    """Prepares os.environ to build Go code.

    Args:
      go_environ: instance of GoEnviron object with go related env vars.
    """
    orig_cwd = os.getcwd()
    orig_environ = os.environ.copy()

    # Note: the order is important, we want to allow go_environ to override
    # env vars present in the default Go environ (in particular CGO_ENABLED).
    self._apply_toolset_env()
    go_environ.apply()

    try:
      yield
    finally:
      os.chdir(orig_cwd)
      # Apparently 'os.environ = orig_environ' doesn't actually modify process
      # environment, only modifications of os.environ object itself do.
      for k, v in orig_environ.items():
        os.environ[k] = v
      for k in os.environ.keys():
        if k not in orig_environ:
          os.environ.pop(k)

  def clean(self, go_environ, packages):
    """Removes object files and executables left from building given packages.

    Transitively cleans all dependencies (including stdlib!) and removes
    executables from GOBIN. In Go modules mode this also appears to be
    downloading modules.

    Args:
      go_environ: instance of GoEnviron object with go related env vars.
      packages: list of go packages to clean (can include '...' patterns).
    """
    with self.build_env(go_environ):
      print_go_step_title('Preparing:\n  %s' % '\n  '.join(packages))
      subprocess.check_call(
          args=['go', 'clean', '-i', '-r'] + list(packages),
          stderr=subprocess.STDOUT)
      # Above command is either silent (without '-x') or too verbose
      # (with '-x'). Prefer the silent version, but add a note that it's
      # alright.
      print('Done.')

  def install(self, go_environ, packages):
    """Builds (and installs) Go packages into GOBIN via 'go install ...'.

    Compiles and installs packages into default GOBIN, which is
    <go_workspace>/bin (it is setup by the bootstrap script).

    Args:
      go_environ: instance of GoEnviron object with go related env vars.
      packages: list of go packages to build (can include '...' patterns).
      rebuild: if True, will forcefully rebuild all dependences.
    """
    args = [
        'go', 'install', '-trimpath', '-ldflags=-buildid=', '-buildvcs=false',
        '-v'
    ]
    if go_environ.with_race:
      args.append('-race')

    args += list(packages)
    with self.build_env(go_environ):
      print_go_step_title('Building:\n  %s' % '\n  '.join(packages))
      subprocess.check_call(args=args, stderr=subprocess.STDOUT)

  def build(self, go_environ, package, output):
    """Builds a single Go package.

    Args:
      go_environ: instance of GoEnviron object with go related env vars.
      package: go package to build.
      output: where to put the resulting binary.
    """
    args = [
        'go', 'build', '-trimpath', '-ldflags=-buildid=', '-buildvcs=false',
        '-v', '-o', output
    ]
    if go_environ.with_race:
      args.append('-race')

    args.append(package)
    with self.build_env(go_environ):
      print_go_step_title('Building %s' % (package,))
      subprocess.check_call(args=args, stderr=subprocess.STDOUT)


class GoEnviron(
    collections.namedtuple('GoEnviron',
                           ['cipd_platform', 'cgo_enabled', 'with_race', 'cwd'])
):
  """Defines a per-package Go build environment modification.

  It is applied on top of the default Go toolset environment from GoToolset
  before building the package.
  """

  @staticmethod
  def new(cipd_platform):
    """Prepares to compile for the given target CIPD platform."""
    return GoEnviron(
        cipd_platform=cipd_platform,
        cgo_enabled=False,
        with_race=False,
        cwd=os.getcwd(),
    )

  def apply(self):
    """Applies GoEnviron to the current os.environ and cwd."""
    for k in GO_BUILD_ENV_VARS:
      os.environ.pop(k, None)
    os.environ.update(cipd_platform_to_go_env(self.cipd_platform))
    os.environ['CGO_ENABLED'] = '1' if self.cgo_enabled else '0'

    # Make sure we target our minimum supported macOS version.
    if self.cgo_enabled and self.cipd_platform.startswith('mac-'):
      if self.cipd_platform.endswith('amd64'):
        min_os_version = '10.13'
      else:
        min_os_version = '11.0'
      # Preserve default `-O2 -g` values for these flags, and add
      # `-mmacosx-version-min`.
      flags = '-O2 -g -mmacosx-version-min=%s' % min_os_version
      os.environ['CGO_CFLAGS'] = flags
      os.environ['CGO_LDFLAGS'] = flags

    if self.cwd is not None:
      os.chdir(self.cwd)


def render_path(p, pkg_vars, replace_sep=True):
  """Renders ${...} substitutions in paths, converts them to native slash."""
  for k, v in pkg_vars.items():
    assert '${' not in v  # just in case, to avoid recursive expansion
    p = p.replace('${%s}' % k, v)
  if replace_sep:
    return os.path.normpath(p.replace('/', os.sep))
  return p


def copy_if_not_exist(src_root, dst_root, path, pkg_vars):
  """Copies a file from src_root to dst_root if it doesn't exist there."""
  file_path = render_path(path, pkg_vars)
  src = os.path.join(src_root, file_path)
  dst = os.path.join(dst_root, file_path)
  if os.path.exists(dst):
    return

  try:
    os.makedirs(os.path.dirname(dst))
  except OSError as e:
    if e.errno != errno.EEXIST:
      raise

  copy_tree(src_root, src, dst)


def cipd_export(ensure_contents, dst_root, cipd_exe):
  """Installs cipd_pkg with the given version tag to dst_root."""
  args = [cipd_exe, 'export', '-ensure-file', '-', '-root', dst_root]
  cmd = subprocess.Popen(
      args,
      stdin=subprocess.PIPE,
      stderr=subprocess.STDOUT,
      executable=cipd_exe)
  out, _ = cmd.communicate(ensure_contents.encode())
  if cmd.returncode:
    raise subprocess.CalledProcessError(cmd.returncode, args, output=out)


def copy_tree(src_root, src, dst):
  """Copies a directory from src to dst. If it's a symlink, convert it pointing
  to relative path."""
  if os.path.islink(src):
    linkto = os.readlink(src)
    if os.path.commonprefix([src_root, linkto]) == src_root:
      linkto = os.path.relpath(linkto, os.path.dirname(src))
    os.symlink(linkto, dst)
  elif os.path.isdir(src):
    os.mkdir(dst)
    for name in os.listdir(src):
      copy_tree(src_root, os.path.join(src, name), os.path.join(dst, name))
  else:
    shutil.copy(src, dst)


def generate_bat_shim(pkg_root, target_rel):
  """Writes a shim file side-by-side with target and returns abs path to it."""
  target_name = os.path.basename(target_rel)
  bat_name = os.path.splitext(target_name)[0] + '.bat'
  base_dir = os.path.dirname(os.path.join(pkg_root, target_rel))
  bat_path = os.path.join(base_dir, bat_name)
  with open(bat_path, 'w') as fd:
    fd.write('\n'.join([  # python turns \n into CRLF
        '@set CIPD_EXE_SHIM="%%~dp0%s"' % (target_name,),
        '@shift',
        '@%CIPD_EXE_SHIM% %*',
        '',
    ]))
  return bat_path


def create_mac_bundle(pkg_root, bundle_def):
  """
  Generate the Mac Bundle structure.

  something.app
    Contents
      Info.plist
      MacOS
        something
      _CodeSignature # Generated by codesign
        CodeResources
  """
  bundle_root = os.path.join(pkg_root, bundle_def['name'])
  shutil.rmtree(bundle_root, ignore_errors=True)

  contents_path = os.path.join(bundle_root, 'Contents')
  os.makedirs(contents_path)

  with open(os.path.join(contents_path, 'Info.plist'), 'w') as info_plist:
    info_plist.write(bundle_def['info'])

  files_root = os.path.join(contents_path, 'MacOS')
  os.mkdir(files_root)
  return {
      'root': bundle_root,
      'files_root': files_root,
  }


def find_cipd():
  """Finds a CIPD client in PATH."""
  exts = ('.exe', '.bat') if sys.platform == 'win32' else ('',)
  for p in os.environ.get('PATH', '').split(os.pathsep):
    base = os.path.join(p, 'cipd')
    for ext in exts:
      candidate = base + ext
      if os.path.isfile(candidate):
        return candidate
  return 'cipd' + ('.exe' if sys.platform == 'win32' else '')


def run_cipd(cipd_exe, cmd, args):
  """Invokes CIPD, parsing -json-output result.

  Args:
    cipd_exe: path to cipd client binary to run.
    cmd: cipd subcommand to run.
    args: list of command line arguments to pass to the subcommand.

  Returns:
    (Process exit code, parsed JSON output or None).
  """
  temp_file = None
  try:
    fd, temp_file = tempfile.mkstemp(suffix='.json', prefix='cipd_%s' % cmd)
    os.close(fd)

    cmd_line = [cipd_exe, cmd, '-json-output', temp_file] + list(args)

    print('Running %s' % ' '.join(cmd_line))
    exit_code = subprocess.call(args=cmd_line, executable=cmd_line[0])
    try:
      with open(temp_file, 'r') as f:
        json_output = json.load(f)
    except (IOError, ValueError):
      json_output = None

    return exit_code, json_output
  finally:
    try:
      if temp_file:
        os.remove(temp_file)
    except OSError:
      pass


def print_title(title):
  """Pretty prints a banner to stdout."""
  sys.stdout.flush()
  sys.stderr.flush()
  print()
  print('-' * 80)
  print(title)
  print('-' * 80)


def print_go_step_title(title):
  """Same as 'print_title', but also appends values of GOOS, GOARCH, etc."""
  go_mod = None
  if os.environ.get('GO111MODULE') != 'off' and os.path.exists('go.mod'):
    go_mod = os.path.abspath('go.mod')
  go_vars = [(k, os.environ[k]) for k in GO_BUILD_ENV_VARS if k in os.environ]
  if go_vars or go_mod:
    title += '\n' + '-' * 80
  if go_mod:
    title += '\n  go.mod: %s' % go_mod
  for k, v in go_vars:
    title += '\n  %s=%s' % (k, v)
  print_title(title)


def bootstrap_go_toolset(go_bootstrap_script):
  """Makes sure the go toolset is installed and returns it as GoToolset."""
  print_title('Making sure Go toolset is installed')

  # Do not install tools from tools.go, they are used only during development.
  # This saves a bit of time.
  bootstrap_env = os.environ.copy()
  bootstrap_env['INFRA_GO_SKIP_TOOLS_INSTALL'] = '1'

  # bootstrap.py installs Go toolset if it is missing. It returns what changes
  # to os.environ are necessary to use the installed toolset.
  output = json.loads(
      check_output(
          args=[
              sys.executable,
              '-u',
              go_bootstrap_script,
              '-',  # emit JSON with environ modification into stdout
          ],
          env=bootstrap_env))
  go_toolset = GoToolset(
      output['env'],
      output['env_prefixes'],
      output['env_suffixes'],
      None,  # don't know the version yet
      None,  # don't know the full env yet
  )

  with go_toolset.build_env(GoEnviron.new(get_host_cipd_platform())):
    # This would be something like "go version go1.15.8 darwin/amd64".
    output = check_output(['go', 'version'])
    print(output.strip())
    print()

    # We want only "go1.15.8" part.
    version = re.match(r'go version (go[\d\.]+)', output).group(1)

    # See https://github.com/golang/go/blob/master/src/cmd/go/env.go for format
    # of the output.
    output = check_output(['go', 'env'])
    print(output.strip())
    go_env = {}
    for line in output.splitlines():
      k, _, v = line.lstrip('set ').partition('=')
      if v.startswith('"') and v.endswith('"'):
        v = v.strip('"')
      elif v.startswith("'") and v.endswith("'"):
        v = v.strip("'")
      go_env[k] = v
    assert go_env['GOBIN']

    return GoToolset(go_toolset.env, go_toolset.env_prefixes,
                     go_toolset.env_suffixes, version, go_env)


def find_main_module(module_map, pkg):
  """Returns a path to the main module to use when building `pkg`.

  Args:
    module_map: a dict "go package prefix => directory with main module".
    pkg: a Go package name to look up.
  """
  matches = set()
  for pfx, main_dir in module_map.items():
    if pkg.startswith(pfx):
      matches.add(main_dir)
  if len(matches) == 0:
    raise BuildException(
        'Package %r is not in the module map %s' %
        (pkg, module_map))
  if len(matches) > 1:
    raise BuildException(
        'Package %r matches multiple modules in the module map %s' %
        (pkg, module_map))
  return list(matches)[0]


def build_go_code(go_toolset, cipd_platform, module_map, pkg_defs):
  """Builds and installs all Go packages used by the given PackageDefs.

  In the end $GOBIN will have all built binaries, and only them (regardless of
  whether we are cross-compiling or not).

  Args:
    go_toolset: instance of GoToolset object to use in the build.
    cipd_platform: target CIPD platform to build for.
    module_map: a dict "go package prefix => directory with main module".
    pkg_defs: list of PackageDef objects that define what to build.
  """
  # Exclude all disabled packages.
  pkg_defs = [p for p in pkg_defs if not p.disabled]

  # Values for GOOS, GOARCH, etc. based on the target platform.
  base_environ = GoEnviron.new(cipd_platform)

  # Grab a set of all go packages we need to build and install into GOBIN,
  # figuring out a go environment (and cwd) they want.
  go_packages = {}  # go package name => GoEnviron

  # Validate that all binary names are unique, since we rely on this fact.
  bin_name_to_pkg = {}

  # The name of the binary we will produce from this go package.
  exe_suffix = get_package_vars(cipd_platform)['exe_suffix']

  def binary_name(go_pkg):
    return go_pkg[go_pkg.rfind('/') + 1:] + exe_suffix

  for pkg_def in pkg_defs:
    pkg_env = base_environ
    pkg_env = pkg_env._replace(
        cgo_enabled=pkg_def.cgo_enabled(cipd_platform),
        with_race=pkg_def.with_race(cipd_platform))
    for name in pkg_def.go_packages:
      pkg_env = pkg_env._replace(cwd=find_main_module(module_map, name))
      if name in go_packages and go_packages[name] != pkg_env:
        raise BuildException(
            'Go package %s is being built in two different go environments '
            '(%s and %s), this is not supported' %
            (name, pkg_env, go_packages[name]))
      go_packages[name] = pkg_env
      bin_name = binary_name(name)
      if bin_name in bin_name_to_pkg and bin_name_to_pkg[bin_name] != name:
        raise BuildException(
            'Go package %s produces binary name %s, which collides with '
            'package %s' % (name, bin_name, bin_name_to_pkg[bin_name]))
      bin_name_to_pkg[bin_name] = name

  # Group packages by the environment they want.
  packages_per_env = {}  # GoEnviron => [str]
  for name, pkg_env in go_packages.items():
    packages_per_env.setdefault(pkg_env, []).append(name)

  # Execute build command for each individual environment.
  for pkg_env, to_install in sorted(packages_per_env.items()):
    to_install = sorted(to_install)
    if not to_install:
      continue

    # Make sure there are no stale files in the workspace.
    go_toolset.clean(pkg_env, to_install)

    if cipd_platform == get_host_cipd_platform():
      # If not cross-compiling, build all Go code in a single "go install" step,
      # it's faster that way. We can't do that when cross-compiling, since
      # 'go install' isn't supposed to be used for cross-compilation and the
      # toolset actively complains with "go install: cannot install
      # cross-compiled binaries when GOBIN is set".
      go_toolset.install(pkg_env, to_install)
    else:
      # Prebuild stdlib once. 'go build' calls below are discarding build
      # results, so it's better to install as much shared stuff as possible
      # beforehand.
      go_toolset.install(pkg_env, ['std'])

      # Build packages one by one and put the resulting binaries into GOBIN, as
      # if they were installed there. It's where the rest of the build.py code
      # expects them to be (see also 'root' property in package definition
      # YAMLs).
      go_bin = go_toolset.go_env['GOBIN']
      for pkg in to_install:
        go_toolset.build(pkg_env, pkg, os.path.join(go_bin, binary_name(pkg)))


def enumerate_packages(package_def_dir, package_def_files):
  """Returns a list PackageDef instances for files in build/packages/*.yaml.

  Args:
    package_def_dir: path to build/packages dir to search for *.yaml.
    package_def_files: optional list of filenames to limit results to.

  Returns:
    List of PackageDef instances parsed from *.yaml files under packages_dir.
  """
  paths = []
  if not package_def_files:
    # All existing packages by default.
    paths = glob.glob(os.path.join(package_def_dir, '*.yaml'))
  else:
    # Otherwise pick only the ones in 'package_def_files' list.
    for name in package_def_files:
      abs_path = os.path.abspath(os.path.join(package_def_dir, name))
      if not os.path.isfile(abs_path):
        raise PackageDefException(name, 'No such package definition file')
      paths.append(abs_path)
  # Load and validate YAMLs.
  pkgs = []
  for p in sorted(paths):
    pkg = PackageDef(p, read_yaml(p))
    pkg.validate()
    pkgs.append(pkg)
  return pkgs


def read_yaml(path):
  """Returns content of YAML file as python dict."""
  with open(path, 'rb') as f:
    return yaml.safe_load(f)


def cipd_platform_to_go_env(platform):
  """Given e.g. linux-armv7l returns {GOOS: linux, GOARCH: arm, GOARM: 7}."""
  parts = platform.split('-')
  if len(parts) != 2:
    raise UnsupportedException('Bad CIPD platform: %s' % platform)
  os, arch = parts
  env = {'GOOS': 'darwin' if os == 'mac' else os}
  if arch not in CIPD_ARCHS:
    raise UnsupportedException('Unrecognized CIPD architecture: %s' % arch)
  env.update(CIPD_ARCHS[arch])
  return env


def go_env_to_cipd_platform(env):
  """Given GOOS, GOARCH, etc. env vars returns the corresponding CIPD platform.

  At least GOOS, GOARCH must be present in the environment. Keys not related to
  Go are simply ignored.

  Raises UnsupportedException if this combination doesn't match any supported
  CIPD architecture.
  """
  assert 'GOOS' in env and 'GOARCH' in env, env
  os = 'mac' if env['GOOS'] == 'darwin' else env['GOOS']

  # GOARM is a special case, since it has no default value in "go build".
  if env['GOARCH'] == 'arm' and 'GOARM' not in env:
    raise UnsupportedException('GOARM is required when GOARCH=arm')

  # Fill in `env` with default values for various go arch env vars. Do the same
  # for possible CIPD_ARCH values. Then find a direct match between given go env
  # and one of CIPD_ARCH envs. This takes care of handling cases when the caller
  # explicitly sets an already default value of some env var. Or if they set it
  # to something we don't yet support.
  def normalized(env):
    keys = ['GOARCH'] + list(GO_ARCH_DEFAULTS)
    return sorted(
        '%s=%s' % (k, env.get(k, GO_ARCH_DEFAULTS.get(k, ''))) for k in keys)

  got = normalized(env)
  for arch, varz in CIPD_ARCHS.items():
    if got == normalized(varz):
      return '%s-%s' % (os, arch)
  raise UnsupportedException('Unsupported Go build configuration: %s' % got)


@cache
def get_host_cipd_platform():
  """Derives CIPD platform of the host running this script.

  This completely ignores GOOS, GOARCH, GOARM, etc. It just looks at the host
  OS and platform using Python.
  """
  platform_variant = {
      'darwin': 'mac',
      'linux': 'linux',
      'linux2': 'linux',
      'win32': 'windows',
  }.get(sys.platform)
  if not platform_variant:
    raise ValueError('Unknown OS: %s' % sys.platform)

  sys_arch = None
  if sys.platform in ('linux', 'linux2'):
    sys_arch = get_linux_host_arch()

  # If we didn't override our system architecture, identify it using "platform".
  sys_arch = sys_arch or platform.machine()
  sys_arch_lower = sys_arch.lower()

  # A set of architectures that we expect can be building packages. This doesn't
  # need to cover all CIPD architectures.
  platform_arch = {
      'amd64': 'amd64',
      'i386': '386',
      'i686': '386',
      'x86': '386',
      'x86_64': 'amd64',
      'arm64': 'arm64',
      'armv6l': 'armv6l',
      'armv7l': 'armv6l',  # we prefer to use older instruction set for builds
  }.get(sys_arch_lower, sys_arch_lower)

  # Most 32-bit Linux Chrome Infra bots are in fact running 64-bit kernel with
  # 32-bit userland. Detect this case (based on bitness of the python
  # interpreter) and report the bot as '386'.
  if (platform_variant == 'linux' and
      platform_arch == 'amd64' and
      sys.maxsize == (2 ** 31) - 1):
    platform_arch = '386'

  # E.g. 'linux-amd64'.
  return '%s-%s' % (platform_variant, platform_arch)


def get_linux_host_arch():
  """The Linux host architecture, or None if it could not be resolved."""
  try:
    # Query "dpkg" to identify the userspace architecture.
    return check_output(['dpkg', '--print-architecture']).strip()
  except OSError:
    # This Linux distribution doesn't use "dpkg".
    return None


def get_cipd_platform_from_env():
  """Returns CIPD platform to use by default in this build invocation.

  This is used if `--cipd-platform` is unset (usually when running the build
  script locally to test stuff). It looks at `GOOS`, `GOARCH` etc and also at
  the host CIPD platform.

  Note this runs before `go` is in PATH, so it can't just ask `go env`.
  """
  # Merge explicitly set GO* env vars with ones that are based on the host.
  # This allows e.g. setting only GOARCH=... to do cross compilation into this
  # arch for the host OS.
  merged = os.environ.copy()
  for k, v in cipd_platform_to_go_env(get_host_cipd_platform()).items():
    if not merged.get(k):
      merged[k] = v
  return go_env_to_cipd_platform(merged)


def get_package_vars(cipd_platform):
  """Returns a dict with variables that describe the package target environment.

  Variables can be referenced in the package definition YAML as
  ${variable_name}. It allows to reuse exact same definition file for similar
  packages (e.g. packages with same cross platform binary, but for different
  platforms).
  """
  return {
      'exe_suffix': '.exe' if cipd_platform.startswith('windows-') else '',
      'platform': cipd_platform,
  }


def is_targeting_windows(pkg_vars):
  """Returns true if 'platform' in pkg_vars indicates Windows."""
  return pkg_vars['platform'].startswith('windows-')


def get_on_change_tag(root, pkg_data, pkg_vars):
  """Get the tag for detecting package on change"""
  h = hashlib.sha256()
  data_file = render_path(pkg_data['file'], pkg_vars)
  with open(os.path.join(root, data_file), 'rb') as f:
    for chunk in iter(lambda: f.read(h.block_size * 256), b""):
      h.update(chunk)
  return ':'.join(['on_change', data_file, h.name, h.hexdigest()])


def build_pkg(cipd_exe, pkg_def, out_file, package_vars, sign_id=None):
  """Invokes CIPD client to build a package.

  Args:
    cipd_exe: path to cipd client binary to use.
    pkg_def: instance of PackageDef representing this package.
    out_file: where to store the built package.
    package_vars: dict with variables to pass as -pkg-var to cipd.
    sign_id: identity used for Mac codesign.

  Returns:
    {'package': <name>, 'instance_id': <hash>}

  Raises:
    BuildException on error.
  """
  print_title('Building: %s' % os.path.basename(out_file))

  # Make sure not stale output remains.
  if os.path.isfile(out_file):
    os.remove(out_file)

  try:
    build_root = tempfile.mkdtemp(prefix="build_py")

    # Parse the definition and filter/extend it before passing to CIPD. This
    # process may generate additional files that are put into the package.
    processed_yaml = pkg_def.preprocess(
        build_root, package_vars, cipd_exe, sign_id=sign_id)

    # Build the package.
    args = ['-pkg-def', processed_yaml]
    for k, v in sorted(package_vars.items()):
      args.extend(['-pkg-var', '%s:%s' % (k, v)])
    args.extend(['-out', out_file])
    exit_code, json_output = run_cipd(cipd_exe, 'pkg-build', args)
    if exit_code:
      print()
      print('FAILED! ' * 10, file=sys.stderr)
      raise BuildException('Failed to build the CIPD package, see logs')

    # Expected result is {'package': 'name', 'instance_id': 'hash'}
    info = json_output['result']
    print('%s %s' % (info['package'], info['instance_id']))
    return info
  finally:
    shutil.rmtree(build_root, ignore_errors=True)


def upload_pkg(cipd_exe, pkg_file, service_url, tags, update_latest_ref,
               service_account):
  """Uploads existing *.cipd file to the storage and tags it.

  Args:
    cipd_exe: path to cipd client binary to use.
    pkg_file: path to *.cipd file to upload.
    service_url: URL of a package repository service.
    tags: a list of tags to attach to uploaded package instance.
    update_latest_ref: a bool of whether or not to update the 'latest' CIPD ref
    service_account: path to *.json file with service account to use.

  Returns:
    {'package': <name>, 'instance_id': <hash>}

  Raises:
    UploadException on error.
  """
  print_title('Uploading: %s' % os.path.basename(pkg_file))

  args = ['-service-url', service_url]
  for tag in sorted(tags):
    args.extend(['-tag', tag])
  if update_latest_ref:
    args.extend(['-ref', 'latest'])
  if service_account:
    args.extend(['-service-account-json', service_account])
  args.append(pkg_file)
  exit_code, json_output = run_cipd(cipd_exe, 'pkg-register', args)
  if exit_code:
    print()
    print('FAILED! ' * 10, file=sys.stderr)
    raise UploadException('Failed to upload the CIPD package, see logs')
  info = json_output['result']
  info['url'] = '%s/p/%s/+/%s' % (
      service_url, info['package'], info['instance_id'])
  print('%s %s' % (info['package'], info['instance_id']))
  return info


def search_pkg(cipd_exe, pkg_name, service_url, tags, service_account):
  """Search existing cipd packages with given tags.

  Args:
    cipd_exe: path to cipd client binary to use.
    pkg_name: name of the cipd package.
    service_url: URL of a package repository service.
    tags: tags to search in the package repository.
    service_account: path to *.json file with service account to use.

  Returns:
    {'package': <name>, 'instance_id': <hash>}

  Raises:
    SearchException on error.
  """
  print_title('Searching: %s by on_change tags %s' % (pkg_name, tags))

  args = ['-service-url', service_url]
  for tag in tags:
    args.extend(['-tag', tag])
  if service_account:
    args.extend(['-service-account-json', service_account])
  args.append(pkg_name)
  exit_code, json_output = run_cipd(cipd_exe, 'search', args)
  if exit_code:
    if json_output['error_code'] == 'auth_error':
      # Maybe the package doesn't exist
      return None
    print()
    print('FAILED! ' * 10, file=sys.stderr)
    raise SearchException('Failed to search the CIPD package, see logs')
  result = json_output['result']
  if result and len(result) > 1:
    print()
    print('FAILED! ' * 10, file=sys.stderr)
    raise SearchException('Multiple CIPD package matched, %s', result)
  return result[0] if result else None


def tag_pkg(cipd_exe, pkg_name, pkg_version, service_url, tags,
            update_latest_ref, service_account):
  """Tag existing cipd package with given tags.

  Args:
    cipd_exe: path to cipd client binary to use.
    pkg_name: name of the cipd package.
    pkg_version: version of the cipd package.
    service_url: URL of a package repository service.
    tags: tags to set to the cipd package.
    update_latest_ref: a bool of whether or not to update the 'latest' CIPD ref.
    service_account: path to *.json file with service account to use.

  Raises:
    TagException on error.
  """
  print_title('Tagging: %s, %s' % (pkg_name, tags))

  args = ['-service-url', service_url]
  for tag in tags:
    args.extend(['-tag', tag])
  if update_latest_ref:
    args.extend(['-ref', 'latest'])
  if service_account:
    args.extend(['-service-account-json', service_account])
  args.extend(['-version', pkg_version])
  args.append(pkg_name)
  exit_code, _ = run_cipd(cipd_exe, 'attach', args)
  if exit_code:
    print()
    print('FAILED! ' * 10, file=sys.stderr)
    raise TagException('Failed to tag the CIPD package, see logs')


def get_build_out_file(package_out_dir, pkg_def, out_files_sfx):
  """Returns a path where to put built *.cipd package file.

  Args:
    package_out_dir: root directory where to put *.cipd files.
    pkg_def: instance of PackageDef being built.
    out_files_sfx: a suffix to append to the filename.
  """
  return os.path.join(package_out_dir, pkg_def.name + out_files_sfx + '.cipd')


def run(
    cipd_platform,
    go_bootstrap_script,
    module_map,
    package_def_dir,
    package_out_dir,
    package_def_files,
    build,
    upload,
    sign_id,
    service_url,
    tags,
    service_account_json,
    json_output,
):
  """Rebuilds python and Go universes and CIPD packages.

  Args:
    cipd_platform: a CIPD platform to build for or "" to auto-detect.
    go_bootstrap_script: a script to use to bootstrap Go environment.
    module_map: a dict "go package prefix => directory with main module".
    package_def_dir: path to build/packages dir to search for *.yaml.
    package_out_dir: where to put built packages.
    package_def_files: names of *.yaml files in package_def_dir or [] for all.
    build: False to skip building packages (valid only when upload==True).
    upload: True to also upload built packages, False just to build them.
    sign_id: identity used for Mac codesign.
    service_url: URL of a package repository service.
    tags: a list of tags to attach to uploaded package instances.
    service_account_json: path to *.json service account credential.
    json_output: path to *.json file to write info about built packages to.

  Returns:
    0 on success, 1 or error.
  """
  assert build or upload, 'Both build and upload are False, nothing to do'

  # If --cipd-platform is unset, derived it based on the host and environ. Check
  # all involved platforms are actually supported.
  try:
    cipd_platform = cipd_platform or get_cipd_platform_from_env()
    _ = cipd_platform_to_go_env(cipd_platform)
    _ = cipd_platform_to_go_env(get_host_cipd_platform())
  except UnsupportedException as exc:
    print(exc, file=sys.stderr)
    return 1

  # If cross-compiling, append a suffix to the *.cipd files stored in the output
  # directory. That way we can store many different variants of the same package
  # there.
  out_files_sfx = ''
  if cipd_platform != get_host_cipd_platform():
    out_files_sfx = '+' + cipd_platform

  # Load all package definitions.
  try:
    defs = enumerate_packages(package_def_dir, package_def_files)
  except PackageDefException as exc:
    print(exc, file=sys.stderr)
    return 1

  # Pick ones we want to build based on the CIPD platform. Log when skipping
  # some explicitly requested packages.
  packages_to_visit = []
  for p in defs:
    if cipd_platform in p.platforms:
      packages_to_visit.append(p)
    elif package_def_files:
      print('Skipping %s since it doesn\'t list %s in its "platforms" '
            'section in the YAML.' % (p.name, cipd_platform))

  # Fail if was given an explicit list of packages to build (usually just one),
  # but they all were filtered out.
  if package_def_files and not packages_to_visit:
    print(
        'None of the requested packages match CIPD platform %s, adjust their '
        '"platforms" section in the YAML if necessary.' % cipd_platform,
        file=sys.stderr)
    return 1

  # Make sure we have a Go toolset and it matches the host platform we detected
  # in get_host_cipd_platform() as an extra check that the environment looks
  # good. In theory we can use any toolset, since we are going to be setting all
  # GOOS, GOARCH etc env vars explicitly, enabling cross-compilation, but an
  # extra check won't hurt.
  go_toolset = bootstrap_go_toolset(go_bootstrap_script)
  expected_host_env = cipd_platform_to_go_env(get_host_cipd_platform())
  if go_toolset.go_env['GOHOSTARCH'] != expected_host_env['GOARCH']:
    print(
        'Go toolset GOHOSTARCH (%s) doesn\'t match expected architecture (%s)' %
        (go_toolset.go_env['GOHOSTARCH'], expected_host_env['GOARCH']),
        file=sys.stderr)
    return 1

  # Append tags related to the build host. They are especially important when
  # cross-compiling: cross-compiled packages can be identified by comparing the
  # platform in the package name with value of 'build_host_platform' tag.
  tags = list(tags)
  tags.append('build_host_hostname:' + socket.gethostname().split('.')[0])
  tags.append('build_host_platform:' + get_host_cipd_platform())
  tags.append('go_version:' + go_toolset.version)

  print_title('Overview')
  print('CIPD platform:')
  print('  target = %s' % cipd_platform)
  print('  host = %s' % get_host_cipd_platform())
  print()
  if upload:
    print('Service URL: %s' % service_url)
    print()
  print('Package definition files to process:')
  for pkg_def in packages_to_visit:
    print('  %s' % pkg_def.name)
  if not packages_to_visit:
    print('  <none>')
  print()
  print('Variables to pass to CIPD:')
  package_vars = get_package_vars(cipd_platform)
  for k, v in sorted(package_vars.items()):
    print('  %s = %s' % (k, v))
  if upload:
    print()
    print('Tags to attach to uploaded packages:')
    for tag in sorted(tags):
      print('  %s' % tag)
  if not packages_to_visit:
    print()
    print('Nothing to do.')
    return 0

  # Find a CIPD client in PATH to use for building and uploading packages.
  print_title('CIPD client')
  cipd_exe = find_cipd()
  print('Binary: %s' % cipd_exe)
  subprocess.check_call(['cipd', 'version'], executable=cipd_exe)

  # Remove old build artifacts to avoid stale files in case the script crashes
  # for some reason.
  if build:
    print_title('Cleaning %s' % package_out_dir)
    if not os.path.exists(package_out_dir):
      os.makedirs(package_out_dir)
    cleaned = False
    for pkg_def in packages_to_visit:
      out_file = get_build_out_file(package_out_dir, pkg_def, out_files_sfx)
      if os.path.exists(out_file):
        print('Removing stale %s' % os.path.basename(out_file))
        os.remove(out_file)
        cleaned = True
    if not cleaned:
      print('Nothing to clean')

  # Build the world.
  if build:
    build_go_code(go_toolset, cipd_platform, module_map, packages_to_visit)

  # Package it.
  failed = []
  succeeded = []
  for pkg_def in packages_to_visit:
    if pkg_def.disabled:
      print_title('Skipping building disabled %s' % pkg_def.name)
      continue
    out_file = get_build_out_file(package_out_dir, pkg_def, out_files_sfx)
    try:
      info = None
      if build:
        info = build_pkg(
            cipd_exe, pkg_def, out_file, package_vars, sign_id=sign_id)
      if upload:
        on_change_tags, pkg_path = pkg_def.on_change_info(package_vars)
        if on_change_tags:
          existed_pkg = search_pkg(cipd_exe, pkg_path, service_url,
                                   on_change_tags, service_account_json)
          if existed_pkg:
            print('Not uploading %s, since all change tags are present.'
                  ' result: %s' % (pkg_def.name, existed_pkg))
            tag_pkg(
                cipd_exe,
                existed_pkg['package'],
                existed_pkg['instance_id'],
                service_url,
                tags,
                pkg_def.update_latest_ref,
                service_account_json,
            )
            succeeded.append({
                'pkg_def_name': pkg_def.name,
                'info': existed_pkg
            })
            continue
          tags.extend(on_change_tags)

        info = upload_pkg(
            cipd_exe,
            out_file,
            service_url,
            tags,
            pkg_def.update_latest_ref,
            service_account_json,
        )
      assert info is not None
      succeeded.append({'pkg_def_name': pkg_def.name, 'info': info})
    except (BuildException, UploadException) as e:
      failed.append({'pkg_def_name': pkg_def.name, 'error': str(e)})

  print_title('Summary')
  for d in failed:
    print('FAILED %s, see log above' % d['pkg_def_name'])
  for d in succeeded:
    print('%s %s' % (d['info']['package'], d['info']['instance_id']))

  if json_output:
    with open(json_output, 'w') as f:
      summary = {
          'failed': failed,
          'succeeded': succeeded,
          'tags': sorted(tags),
          'vars': package_vars,
      }
      json.dump(summary, f, sort_keys=True, indent=2, separators=(',', ': '))

  return 1 if failed else 0


def main(args):
  parser = argparse.ArgumentParser(description='Builds infra CIPD packages')
  parser.add_argument(
      'yamls', metavar='YAML', type=str, nargs='*',
      help='name of a file in build/packages/* with the package definition')
  parser.add_argument(
      '--cipd-platform',
      metavar='CIPD_PLATFORM',
      help=('CIPD platform to build packages for (if unset, derived from '
            'the environment)'),
  )
  parser.add_argument(
      '--signing-identity',
      metavar='IDENTITY',
      dest='sign_id',
      default=None,
      help='Signing identity used for mac codesign. '
      'Use adhoc sign if not provided.')
  parser.add_argument(
      '--upload',
      action='store_true',
      dest='upload',
      default=False,
      help='upload packages into the repository')
  parser.add_argument(
      '--no-rebuild',
      action='store_false',
      dest='build',
      default=True,
      help='when used with --upload means upload existing *.cipd files')
  parser.add_argument(
      '--go-bootstrap-script',
      metavar='PATH',
      default=os.path.join(ROOT, 'go', 'bootstrap.py'),
      help='a script to use to bootstrap Go environment',
  )
  parser.add_argument(
      '--package-definition-dir',
      metavar='PATH',
      default=os.path.join(ROOT, 'build', 'packages'),
      help=('points at either infra.git/build/packages or '
            'infra_internal.git/build/packages.'),
  )
  parser.add_argument(
      '--package-out-dir',
      metavar='PATH',
      default=os.path.join(ROOT, 'build', 'out'),
      help=('points at either infra.git/build/out or '
            'infra_internal.git/build/out.'),
  )
  parser.add_argument(
      '--map-go-module', metavar='MOD=PATH', action='append',
      help='go package prefix = directory containing go.mod.',
  )
  parser.add_argument(
      '--service-url', metavar='URL', dest='service_url',
      default=PACKAGE_REPO_SERVICE,
      help='URL of the package repository service to use')
  parser.add_argument(
      '--service-account-json', metavar='PATH', dest='service_account_json',
      help='path to credentials for service account to use')
  parser.add_argument(
      '--json-output', metavar='PATH', dest='json_output',
      help='where to dump info about built package instances')
  parser.add_argument(
      '--tags', metavar='KEY:VALUE', type=str, dest='tags', nargs='*',
      help='tags to attach to uploaded package instances')
  args = parser.parse_args(args)
  if not args.build and not args.upload:
    parser.error('--no-rebuild doesn\'t make sense without --upload')

  module_map = DEFAULT_MODULE_MAP.copy()
  for mapping in (args.map_go_module or ()):
    mod, path = mapping.split('=')
    module_map[mod] = path

  return run(
      args.cipd_platform,
      args.go_bootstrap_script,
      module_map,
      args.package_definition_dir,
      args.package_out_dir,
      [n + '.yaml' if not n.endswith('.yaml') else n for n in args.yamls],
      args.build,
      args.upload,
      args.sign_id,
      args.service_url,
      args.tags or [],
      args.service_account_json,
      args.json_output,
  )


if __name__ == '__main__':
  sys.exit(main(sys.argv[1:]))
