# Copyright 2022 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""General purpose build workflow.

The module is supposed to be executed by stdenv derivation and will assume all
packages and environment variables listed in the stdenv available during the
runtime. Default behaviours can be customized or overridden by various of hooks.
"""

import dataclasses
import enum
import itertools
import os
import pathlib
import re
import shlex
import subprocess
import sys

from typing import Callable
from typing import Dict
from typing import List
from typing import Optional

from setup import extract

# Python converts all environment variables to uppercase if the platform's
# environment variables are case insensiteve. We want to access and preserve
# the original value to reduce cross-platform inconsistencies.
if os.name == 'nt':
  import nt
  environ = nt.environ
else:
  environ = os.environ


class PlatType(enum.Enum):
  BUILD = -1
  HOST = 0
  TARGET = 1


class HookReturnFalseError(Exception):
  """Return False from a hook script.

  We can only raise exception from exec. ErrorHookReturnFalse will be
  interpreted as returning False from hook function by _execute_implicit_hook.
  """
  pass


@dataclasses.dataclass
class Execution:
  """Execution stores all the states during a standard build workflow.

  Any hooks can access the value of Execution using variable 'exe' or function
  argument. Execution also includes all the default implementation of build
  steps.
  """

  ##############################################################################
  # Runtime hooks

  hooks: Dict[str, List[Callable[['Execution'], bool]]] = (
      dataclasses.field(default_factory=dict))

  class HookContext:
    """Context for hook execution.

    An optional HookContext may be set to exe.current_context when a hook is
    triggered.
    """

    @dataclasses.dataclass
    class Setup:
      host: PlatType
      target: PlatType

    @dataclasses.dataclass
    class ActivatePkg:
      pkg: pathlib.Path
      host: PlatType
      target: PlatType

    @dataclasses.dataclass
    class UnpackCmd:
      src: str

    @dataclasses.dataclass
    class ExecutionCmd:
      args: List[str]

  current_context: Optional[HookContext] = None

  ##############################################################################
  # Environment variables

  env: Dict[str, str] = dataclasses.field(default_factory=environ.copy)

  ENV_PATH = 'PATH'
  ENV_XDG_DATA_DIRS = 'XDG_DATA_DIRS'
  ENV_PKG_CONFIG_PATH = 'PKG_CONFIG_PATH'

  ENV_SOURCES = 'srcs'
  ENV_SOURCE_ROOT = 'sourceRoot'

  ENV_PATCHES = 'patches'

  ENV_CONFIGURE_SCRIPT = 'configureScript'
  ENV_CONFIGURE_FLAGS = 'configureFlags'

  ENV_MAKE_FILE = 'makefile'
  ENV_MAKE_FLAGS = 'makeFlags'

  ENV_BUILD_FLAGS = 'buildFlags'

  ENV_INSTALL_FLAGS = 'installFlags'
  ENV_INSTALL_TARGETS = 'installTargets'

  ##############################################################################
  # Utilities

  def add_hook(self, name: str, f: Callable[['Execution'], bool]) -> None:
    self.hooks.setdefault(name, []).append(f)

  def execute_one_hook(
      self, name: str, ctx: Optional[HookContext] = None) -> bool:
    """Execute hooks until the first one succeeded."""
    print(f'execute_one_hook: name={name} ctx={ctx} env={self.env}', flush=True)
    try:
      self.current_context = ctx
      return any(itertools.chain(
          (self._execute_implicit_hook(name, False),),
          (f(self) for f in reversed(self.hooks.get(name, []))),
      ))
    finally:
      self.current_context = None

  def execute_all_hooks(
      self, name: str, ctx: Optional[HookContext] = None) -> bool:
    """Execute all hooks unless any one failed."""
    print(
        f'execute_all_hooks: name={name} ctx={ctx} env={self.env}', flush=True)
    try:
      self.current_context = ctx
      return all(itertools.chain(
          (self._execute_implicit_hook(name, True),),
          (f(self) for f in reversed(self.hooks.get(name, []))),
      ))
    finally:
      self.current_context = None

  def _execute_implicit_hook(self, name: str, default: bool) -> bool:
    """Execute implicitly defined hook.

    A hook can be implicitly defined in an environment variable, as a python
    global function, or a hook file without calling add_hook.

    Args:
      name: name of the hook.
      default: return value if hook isn't implicitly defined.

    Returns:
      Hook successfully triggered or not.
    """
    py_name = _camel_to_snake(name)
    if name in self.env:
      try:
        # pylint: disable=exec-used
        exec(compile(
            self.env[name],
            f'env:{name}',
            mode='exec',
            dont_inherit=True,
        ), globals(), {'exe': self})
      except HookReturnFalseError:
        return False
      return True
    elif py_name in globals():
      # If the hook is defined as function
      ret = globals()[py_name](self)
      assert isinstance(ret, bool)  # Prevent returning None
      return ret
    elif os.path.isfile(name):
      with open(name) as f:
        try:
          # pylint: disable=exec-used
          exec(compile(
              f.read(),
              name,
              mode='exec',
              dont_inherit=True,
          ), globals(), {'exe': self})
        except HookReturnFalseError:
          return False
        return True
    return default

  def append_to_search_path(
      self, name: str, path: pathlib.Path,
      delimiter: str = os.path.pathsep) -> None:
    """Update the environment variable and append the path to it."""
    if not path.is_dir():
      return

    ori = self.env.get(name, '')
    self.env[name] = f'{ori}{delimiter if ori else ""}{path}'

  def prepend_to_search_path(
      self, name: str, path: pathlib.Path,
      delimiter: str = os.path.pathsep) -> None:
    """Update the environment variable and prepend the path to it."""
    if not path.is_dir():
      return

    ori = self.env.get(name, '')
    self.env[name] = f'{path}{delimiter if ori else ""}{ori}'

  def activate_pkg(
      self, pkg: pathlib.Path, host: PlatType, target: PlatType) -> None:
    """Activate the package and trigger hooks.

    PATH and XDG_DATA_DIRS will be updated if the build platform matched the
    package's host platform.

    Args:
      pkg: the path of the package to be activated
      host: the host platform of the package
      target: the target platform of the package
    """
    # Sanity check
    assert host.value <= target.value, 'invalid dependency type'

    self.execute_all_hooks('activatePkg', Execution.HookContext.ActivatePkg(
        pkg=pkg,
        host=host,
        target=target,
    ))

    # Only dependencies whose host platform matches the build platform are
    # guaranteed their binaries to be executable.
    if host == PlatType.BUILD:
      self.append_to_search_path(Execution.ENV_PATH, pkg.joinpath('bin'))
      self.append_to_search_path(
          Execution.ENV_XDG_DATA_DIRS, pkg.joinpath('share'))

    # Only dependencies whose host platform matches the host platform are
    # guaranteed their libraries can be linked.
    # TODO(fancl): Move this to pkg-config package.
    if host == PlatType.HOST:
      self.append_to_search_path(
          Execution.ENV_PKG_CONFIG_PATH, pkg.joinpath('lib', 'pkgconfig'))

  def activate_pkg_hook(
      self, pkg: pathlib.Path, host: PlatType, target: PlatType) -> None:
    if (hook := pkg.joinpath('build-support', 'setup-hook.py')).is_file():
      self.execute_one_hook(str(hook), Execution.HookContext.Setup(
          host=host,
          target=target,
      ))

  def activate_pkgs(self) -> None:
    """Activate all dependencies in deps** environment variables."""

    def pkgs(name: str) -> List[pathlib.Path]:
      return map(pathlib.Path, _split(self.env.get(name), os.path.pathsep))

    # Activate the setup-hook first so packages can add activatePkg hook.
    for activate in (self.activate_pkg_hook, self.activate_pkg):
      for pkg in pkgs('depsBuildBuild'):
        activate(pkg, PlatType.BUILD, PlatType.BUILD)
      for pkg in pkgs('depsBuildHost'):
        activate(pkg, PlatType.BUILD, PlatType.HOST)
      for pkg in pkgs('depsBuildTarget'):
        activate(pkg, PlatType.BUILD, PlatType.TARGET)
      for pkg in pkgs('depsHostHost'):
        activate(pkg, PlatType.HOST, PlatType.HOST)
      for pkg in pkgs('depsHostTarget'):
        activate(pkg, PlatType.HOST, PlatType.TARGET)
      for pkg in pkgs('depsTargetTarget'):
        activate(pkg, PlatType.TARGET, PlatType.TARGET)

  def execute_cmd(self, args) -> None:
    """Execute an external command."""
    # Flush python's output buffer before executing commands
    sys.stdout.flush()
    sys.stderr.flush()

    # Print execution detail
    print(f'execute_cmd: args={args} env={os.environ}', flush=True)

    if not self.execute_one_hook(
        'executeCmd',
        Execution.HookContext.ExecutionCmd(
            args=args,
        )
    ):
      subprocess.check_call(args, env=self.env)

  ##############################################################################
  # Default implementation for generic builder

  def execute_phase(self, name: str) -> None:
    """Execute the phase.

    Phases can be overridden by environment variables or global function. Do
    nothing if phase is not defined.

    Args:
      name: name of the phase.
    """
    print(f'execute_phase: name={name} env={self.env}', flush=True)
    py_name = _camel_to_snake(name)
    if name in self.env:
      # If the phase is overridden by environment variable
      # pylint: disable=exec-used
      exec(compile(
          self.env[name],
          f'env:{name}',
          mode='exec',
          dont_inherit=True,
      ), globals(), {'exe': self})
    elif py_name in globals():
      # If the phase is overridden by defining function
      globals()[py_name](self)
    else:
      # ...Otherwise call the default function if exist
      getattr(self, py_name, lambda: None)()

  def unpack_phase(self) -> None:
    """Upack the source code archives listed in ENV_SOURCES."""
    srcs = _split(self.env[Execution.ENV_SOURCES], os.path.pathsep)

    # To determine the source directory created by unpacking the
    # source archives, we record the contents of the current
    # directory, then look below which directory got added.
    dirs_before = set(f for f in os.listdir() if os.path.isdir(f))
    for src in srcs:
      if os.path.isdir(src):
        files = os.listdir(src)
        assert len(files) == 1
        src = os.path.join(src, files[0])
      if not self.execute_one_hook(
          'unpackCmd', Execution.HookContext.UnpackCmd(src=src)):
        raise RuntimeError('unpacker not available')

    if not self.execute_one_hook('setSourceRoot'):
      dirs_after = set(f for f in os.listdir() if os.path.isdir(f))
      added = dirs_after - dirs_before
      if len(added) > 1:
        raise RuntimeError('unpacker produced multiple directories')
      if added:
        self.env[Execution.ENV_SOURCE_ROOT] = os.path.abspath(added.pop())

    if Execution.ENV_SOURCE_ROOT not in self.env:
      raise RuntimeError('unpacker appears to have produced no directorie')

  def patch_phase(self) -> None:
    """Apply Patches in ENV_PATCHES to the source code."""
    if patches := _split(self.env.get(Execution.ENV_PATCHES), os.path.pathsep):
      # Ignore whitespace to workaround CRLF/LF differences.
      self.execute_cmd(['git', 'apply', '--ignore-whitespace', '-v'] + patches)
    pass

  def configure_phase(self) -> None:
    """Run configure command in the source directory."""
    if not (script := self.env.get(Execution.ENV_CONFIGURE_SCRIPT)):
      if os.path.isfile('configure'):
        script = os.path.abspath('configure')
      elif os.path.isabs('CMakeLists.txt'):
        # TODO(fancl): Add cmake support.
        pass

    if not script:
      print('no configure script, doing nothing')
      return

    args = [script]
    args.extend(shlex.split(self.env.get(Execution.ENV_CONFIGURE_FLAGS, '')))
    args.append(f'--prefix={self.env["prefix"]}')
    # flags.append('--disable-static')
    self.execute_cmd(args)

  def build_phase(self) -> None:
    """Run build command in the source directory."""
    flags = shlex.split(self.env.get(Execution.ENV_MAKE_FLAGS, ''))
    file = self.env.get('makefile', '')

    if not (flags or file) and not any(os.path.isfile(f) for f in [
        'Makefile', 'makefile', 'GNUmakefile']):
      print('no Makefile, doing nothing')
      return

    args = ['make', file] if file else ['make']
    args.extend(flags)
    args.extend(shlex.split(self.env[Execution.ENV_BUILD_FLAGS]))
    self.execute_cmd(args)

  def install_phase(self) -> None:
    """Install the package to the ${out} directory."""
    flags = shlex.split(self.env.get(Execution.ENV_MAKE_FLAGS, ''))
    file = self.env.get('makefile', '')

    if prefix := self.env['prefix']:
      os.makedirs(prefix, exist_ok=True)

    args = ['make', file] if file else ['make']
    args.extend(flags)
    args.extend(shlex.split(self.env[Execution.ENV_INSTALL_FLAGS]))
    args.extend(
        shlex.split(self.env.get(Execution.ENV_INSTALL_TARGETS, 'install')))
    self.execute_cmd(args)


def _camel_to_snake(s: str) -> str:
  return re.sub('([A-Z]+)', r'_\1', s).lower()


def _phase_hook(prefix: str, phase: str) -> str:
  return prefix + phase[0].upper() + phase.removesuffix('Phase')[1:]


def _split(s: str, sep: Optional[str] = None) -> List[str]:
  """Fix python's "broken" split behaviour.

  Args:
    s: String to be splitted
    sep: Separator

  Returns:
    List of splitted string

  Return an empty list when s is empty, whether there is a sep or not.
  See also: https://bugs.python.org/issue28937
  """
  if not s:
    return []
  return s.split(sep)


def main() -> None:
  """The entrypoint of the setup package.

  The main() function is supposed to be executed by stdenv derivation and will
  assume all packages and environment variables listed in the stdenv available
  during the runtime.

  Hooks can be added by environment variables, global functions, script files
  or Execution.add_hook(...). An Execution instance will always be available
  from argument or local variable 'exe'. See _execute_implicit_hook(...) for how
  it's implemented.

  Raises:
    RuntimeError: Failed to execute any phases.
  """
  exe = Execution()

  # Extra default hooks
  exe.add_hook('unpackCmd', extract.copy_cmd)
  exe.add_hook('unpackCmd', extract.unpack_cmd)

  ##############################################################################
  # Begin execution

  exe.execute_all_hooks('preHook')

  # Initialization
  exe.activate_pkgs()
  exe.env['TZ'] = 'UTC'
  exe.env['prefix'] = exe.env['out']

  # TODO(fancl): Move to go package
  go_env_root = pathlib.Path().joinpath('.go')
  def pre_build_go(exe) -> bool:
    def set_go_env(env: str) -> None:
      d = go_env_root.joinpath(env).absolute()
      exe.env[env] = str(d)
      os.makedirs(d)
    for env in ('GOPATH', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR'):
      set_go_env(env)
    return True
  exe.add_hook('preBuild', pre_build_go)

  # Generic Builder
  for phase in (
      'unpackPhase',
      'patchPhase',
      'configurePhase',
      'buildPhase',
      'verifyPhase',
      'installPhase',
      'installVerifyPhase',
  ):
    if exe.env.get(_phase_hook('dont', phase)):
      continue
    if not exe.execute_all_hooks(_phase_hook('pre', phase)):
      raise RuntimeError(f'failed to execute pre-hook for {phase}')
    if _phase_hook('skip', phase) not in exe.env:
      exe.execute_phase(phase)
    if not exe.execute_all_hooks(_phase_hook('post', phase)):
      raise RuntimeError(f'failed to execute post-hook for {phase}')

    if phase == 'unpackPhase' and Execution.ENV_SOURCE_ROOT in exe.env:
      os.chdir(exe.env[Execution.ENV_SOURCE_ROOT])

  exe.execute_all_hooks('postHook')

  # End execution
  ##############################################################################

if __name__ == '__main__':
  main()
