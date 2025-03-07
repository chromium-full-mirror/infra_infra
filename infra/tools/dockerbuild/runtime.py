# Copyright 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import collections
import contextlib
import logging
import multiprocessing
import os
import sys
import subprocess
import threading
import tempfile

from . import concurrency
from . import cipd
from . import dockcross
from . import source
from . import util
from . import build_platform

from .builder import PlatformNotSupported


class MissingToolsError(RuntimeError):
  """Raised if required system tools could not be identified."""
  pass


class System(object):
  """Represents the local system facilities."""

  _Tools = collections.namedtuple('_Tools', ('cipd',))

  _Dirs = collections.namedtuple('_Dirs', (
      'root', 'repo', 'bin', 'wheel', 'pkg', 'cipd_cache'))

  class SubcommandError(Exception):
    def __init__(self, returncode, output):
      super(System.SubcommandError, self).__init__(
          'subcommand returned %d' % (returncode,))
      self.returncode = returncode
      self.output = output

  def __init__(self, tools, dirs, leak, upload_sources, force_source_download):
    self._tools = tools
    self._dirs = dirs
    self._repo = source.Repository(self, dirs.repo, upload=upload_sources,
                                   force_download=force_source_download)
    self._leak = leak
    self._cipd = cipd.Cipd(self, dirs.cipd_cache)

    self._dockcross_images = {}

  @classmethod
  def initialize(cls,
                 root,
                 leak=False,
                 upload_sources=False,
                 force_source_download=False):
    tools = cls._Tools(
        cipd=cls._find_tool('cipd'),
    )
    missing_tools = [k for k, v in tools._asdict().items() if not v]
    if missing_tools:
      raise MissingToolsError('Missing required tools: %r' % (
          sorted(missing_tools),))

    dirs = cls._Dirs(
        root=root,
        repo=util.ensure_directory(root, 'source_repo'),
        bin=util.ensure_directory(root, 'bin'),
        wheel=util.ensure_directory(root, 'wheels'),
        pkg=util.ensure_directory(root, 'packages'),
        cipd_cache=util.ensure_directory(root, 'cipd_cache'),
    )
    return cls(tools, dirs, leak, upload_sources, force_source_download)

  @classmethod
  def _find_tool(cls, name):
    # This function doesn't support relative or absolute paths, only bare
    # executable names.
    assert os.sep not in name

    # We can't use distutils.spawn.find_executable, as on Windows it doesn't
    # check the full list of extensions, only '.exe'. So it fails for cipd,
    # which is a batch file on Windows.
    path = os.getenv('PATH').split(os.pathsep)
    if sys.platform == 'win32':
      extensions = os.getenv('PATHEXT').split(os.pathsep)
    else:
      extensions = ['']

    for directory in path:
      for extension in extensions:
        filename = os.path.join(directory, name + extension)
        if os.path.isfile(filename):
          return filename

    return None

  @property
  def tools(self):
    return self._tools

  @property
  def root(self):
    return self._dirs.root

  @property
  def cipd(self):
    return self._cipd

  @property
  def bin_dir(self):
    return self._dirs.bin

  @property
  def wheel_dir(self):
    return self._dirs.wheel

  @property
  def pkg_dir(self):
    return self._dirs.pkg

  @property
  def pip_cache_dir(self):
    # Make sure the pip cache directory exists within our root, so that
    # it is not reused across builds. Also, tie it to the thread ident,
    # which ensures that two threads are not sharing the cache (this has
    # been observed to cause failures).
    return util.ensure_directory(
        self.root, 'temp', 'pip_cache_%d' % concurrency.Pool.local.thread_index)

  @property
  def repo(self):
    return self._repo

  @property
  def numcpu(self):
    return multiprocessing.cpu_count()

  def dockcross_image(self, plat, rebuild=False):
    if not rebuild:
      dx = self._dockcross_images.get(plat.name)
      if dx is not None:
        return dx

    if plat.dockcross_base:
      if not sys.platform.startswith('linux'):
        raise PlatformNotSupported(
            ('Docker builds are only supported on Linux, skipping %r' %
             plat.name))
      builder = dockcross.Builder(self)
      dx = builder.build(plat, rebuild=rebuild)
    else:
      # No "dockcross" image associated with this platform. We can return a
      # native image if the target platform is the current platform.
      native_platforms = build_platform.NativePlatforms()
      if plat in native_platforms:
        util.LOGGER.info('Using native platform for [%s].', plat.name)
        dx = dockcross.NativeImage(self, plat)
      else:
        raise PlatformNotSupported(
            ('Platform %r unsupported on host platform %r '
             '(supported platforms are %r)') %
            (plat.name, sys.platform, native_platforms))

    self._dockcross_images[plat.name] = dx
    return dx

  @contextlib.contextmanager
  def temp_subdir(self, prefix):
    temp_root = util.ensure_directory(self.root, 'temp')
    tdir = tempfile.mkdtemp(dir=temp_root, prefix=prefix)
    try:
      yield tdir
    finally:
      if self._leak:
        util.LOGGER.info('(--leak): Leaking temporary subdirectory: %s', tdir)
      else:
        util.LOGGER.debug('Removing temporary subdirectory: %s', tdir)
        try:
          util.removeall(tdir)
        except Exception as e:
          util.LOGGER.info('Removing temporary subdirectory failed: %s: %s',
                           tdir, repr(e))

  _devnull = open(os.devnull, 'w')

  # A wrapper for subprocess.Popen() which adds required locking, as well as
  # logging and error handling. In addition to the Popen() parameters, two
  # other parameters are supported:
  #
  # env_prefix (List[(str, str)]): String to be prepended to an environment
  #   variable, in the form [('VARNAME', 'PREFIXSTR'), ...]. The prefix will be
  #   separated from any existing value using the OS path separator. The same
  #   variable may appear multiple times; all values will be prepended and will
  #   appear in the order given in env_prefix, followed by any existing value.
  # env_suffix (List[(str, str)]): String to be appended to an environment
  #   variable, in the form [('VARNAME', 'SUFFIXSTR'), ...]. The suffix will be
  #   separated from any existing value using the OS path separator. The same
  #   variable may appear multiple times; all values will be appended and will
  #   appear in the order given in env_prefix.
  def run(self,
          args,
          cwd=None,
          env=None,
          stdout=subprocess.PIPE,
          stderr=subprocess.STDOUT,
          stdin=_devnull,
          retcodes=None,
          env_prefix=None,
          env_suffix=None):
    # Fold environment augmentations into the default system environment.
    cwd = cwd or os.getcwd()
    util.LOGGER.debug(
        'Running command: %s (env=%s; cwd=%s; env_prefix=%s; env_suffix=%s)',
        args, env, cwd, env_prefix, env_suffix)

    kwargs = {
        'cwd': cwd,
        'env': os.environ.copy(),
        'stdin':  stdin,
        'stdout': stdout,
        'stderr': stderr,
    }
    if env is not None:
      kwargs['env'].update(env)

    # Process environment prefixes/suffixes.
    #
    # Note: for commands run in a Docker container, env_prefix/env_suffix have
    # already been handled by Image.run() before this function is called.
    env_prefix = env_prefix or []
    env_suffix = env_suffix or []

    # Iterate prefixes backwards, so the items appear in-order in the output.
    for k, v in reversed(env_prefix):
      old_val = kwargs['env'].get(k, '')
      new_val = v
      if old_val:
        new_val += os.pathsep + old_val
      kwargs['env'][k] = new_val
    for k, v in env_suffix:
      old_val = kwargs['env'].get(k, '')
      if old_val:
        old_val += os.pathsep
      kwargs['env'][k] = old_val + v

    stdout_lines = []

    # Acquire an exclusive lock before running the subprocess. The lock is
    # acquired in shared mode by threads writing files, to ensure the subprocess
    # doesn't inherit file handles and keep them open. See the comment on
    # PROCESS_SPAWN_LOCK for more detail.
    with concurrency.PROCESS_SPAWN_LOCK.write():
      # Flush any pending writes before executing. Not exactly sure, but this
      # seems to reduce problems with shell scripts failing to execute with the
      # error: "bash: bad interpreter: text file busy". Although it could just
      # be that the extra time taken here reduces the chance of a race.
      if sys.platform != 'win32':
        subprocess.call('sync')

      proc = subprocess.Popen(args, **kwargs)

    if kwargs['stdout'] is subprocess.PIPE:
      for stdout_line in iter(proc.stdout.readline, b""):
        # TODO: Once ported fully to Python 3 we can specify the encoding in the
        # Popen constructor instead.
        stdout_line = stdout_line.decode('utf-8', errors='surrogateescape').rstrip()
        util.LOGGER.debug('STDOUT: "%s"', stdout_line)
        stdout_lines.append(stdout_line)

    returncode = proc.wait()

    stdout = '\n'.join(stdout_lines)

    util.LOGGER.debug('Command finished with return code %d.', returncode)
    if retcodes is not None and proc.returncode not in retcodes:
      if not util.LOGGER.isEnabledFor(logging.DEBUG):
        util.LOGGER.error('Command failed: %s (rc=%d):\n%s', args, returncode,
                          stdout)
      else:
        # Already dumped STDOUT to debug.
        util.LOGGER.error('Command failed: %s (rc=%d)', args, returncode)
      raise self.SubcommandError(returncode, stdout)
    return returncode, stdout

  def check_run(self, args, **kwargs):
    kwargs.setdefault('retcodes', [0])
    return self.run(args, **kwargs)

  def docker(self, args, **kwargs):
    return self.run(['docker'] + args, **kwargs)
