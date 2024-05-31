# Copyright 2022 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import fnmatch
import os
import posixpath
import shutil
import tarfile
import zipfile


def unc_path(path):
  if os.name != 'nt':
    return path

  prefix = '\\\\?\\'
  if path.startswith(prefix):
    # Already in UNC format.
    return path
  return prefix + os.path.abspath(path)


def untar(archive_file, output, stats, safe, include_filter):
  """Untars an archive using 'tarfile' python module.

  Works everywhere where Python works (Windows and POSIX).

  Args:
    archive_file: absolute path to an archive to untar.
    output: existing directory to untar to.
    stats: the stats dict (see unpack_cmd() for its form)
    safe: If True, skips extracting files which would escape `output`.
    include_filter: A function which is given the archive
      path and should return True if we should extract it.
  """
  # Open regular files in random-access mode, which allows seeking backwards
  # (needed to extract archives containing symlinks on some platforms).
  # Otherwise, we open the file in stream mode, though this may fail later
  # for the aforementioned case.
  unc_output = unc_path(output)
  open_mode = 'r:*' if os.path.isfile(archive_file) else 'r|*'

  # pylint: disable=protected-access
  with tarfile.open(archive_file, open_mode) as tf:
    # monkeypatch the TarFile object to allow printing messages for each
    # extracted file. extractall makes a single linear pass over the tarfile;
    # other naive implementations (such as `getmembers`) end up doing lots of
    # random access over the file. Also patch it to support Unicode filenames.
    em = tf._extract_member

    def _extract_member(tarinfo, targetpath, **kwargs):
      unc_targetpath = unc_path(targetpath)
      if safe and not unc_targetpath.startswith(unc_output):
        print('Skipping %s (would escape root)' % (tarinfo.name,))
        stats['skipped']['filecount'] += 1
        stats['skipped']['bytes'] += tarinfo.size
        stats['skipped']['names'].append(tarinfo.name)
        return

      if not include_filter(tarinfo.name):
        print('Skipping %s (does not match include_files)' % (tarinfo.name,))
        return

      print('Extracting %s' % (tarinfo.name,))
      stats['extracted']['filecount'] += 1
      stats['extracted']['bytes'] += tarinfo.size
      em(tarinfo, unc_targetpath, **kwargs)

    tf._extract_member = _extract_member
    tf.extractall(output)


def unzip(zip_file, output, stats, include_filter):
  """Unzips an archive using 'zipfile' python module.

  Works everywhere where Python works (Windows and POSIX).

  Args:
    zip_file: absolute path to an archive to unzip.
    output: existing directory to unzip to.
    stats: the stats dict (see unpack_cmd() for its form)
    include_filter: A function which is given the archive
      path and should return True if we should extract it.
  """
  with zipfile.ZipFile(zip_file) as zf:
    for zipinfo in zf.infolist():
      if not include_filter(zipinfo.filename):
        print('Skipping %s (does not match include_files)' %
              (zipinfo.filename,))
        continue

      print('Extracting %s' % zipinfo.filename)
      stats['extracted']['filecount'] += 1
      stats['extracted']['bytes'] += zipinfo.file_size
      zf.extract(zipinfo, unc_path(output))

      if os.name != 'nt':
        # POSIX may store permissions in the 16 most significant bits of the
        # file's external attributes.
        perms = (zipinfo.external_attr >> 16) & 0o777
        if perms:
          fullpath = os.path.join(output, zipinfo.filename)
          # Don't update permissions to be more restrictive.
          old = os.stat(fullpath).st_mode
          old_short = old & 0o777
          new = old | perms
          new_short = new & 0o777
          if old_short < new_short:
            print('Updating %s permissions (0%o -> 0%o)' %
                  (zipinfo.filename, old_short, new_short))
            os.chmod(fullpath, new)


def unpack_cmd(exe, include_files: str = '*') -> bool:
  ctx = exe.current_context
  out = os.path.join(os.getcwd(), '')

  src = os.path.abspath(ctx.src)
  _, ext = os.path.splitext(src)

  stats = {
      'extracted': {'filecount': 0, 'bytes': 0},
      'skipped': {'filecount': 0, 'bytes': 0, 'names': []},
  }

  def include_filter(path):
    path = posixpath.normpath(path)
    if path.startswith('./'):
      path = path[2:]
    for pattern in include_files:
      if fnmatch.fnmatch(path, pattern):
        return True
    return False

  print('Extracting %s -> %s ...' % (src, out))
  if ext in {
      '.tar',
      '.gz', '.tgz',
      '.xz', '.txz',
      '.bz2', '.tbz', '.tbz2', '.tb2',
  }:
    untar(src, out, stats, True, include_filter)
  elif ext in {'.zip'}:
    unzip(src, out, stats, include_filter)
  else:
    return False

  return True


def copy_cmd(exe) -> bool:
  ctx = exe.current_context
  src = os.path.abspath(ctx.src)
  dst = os.path.join(os.getcwd(), os.path.basename(src))

  if os.path.isdir(src):
    shutil.copytree(src, dst, symlinks=True)
  else:
    shutil.copyfile(src, dst)

  return True
