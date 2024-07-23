#!/usr/bin/env vpython3
# Copyright 2016 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Script to sync generated kzip files to a remote git repository."""

import argparse
import errno
import os.path
import re
import shutil
import subprocess
import sys
from typing import List, Optional, Sequence, Set, Tuple, Union
import zipfile

sys.path.insert(0, os.path.dirname(__file__))
from kythe.proto import analysis_pb2


def has_allowed_extension(filename: str) -> bool:
  """Check whether this file has one of the approved extensions.

  Exclude everything except generated source code.

  Note that we use a allowlist here instead of a blocklist, because:
  1. If we allowlist, the problem is that some legit files might be excluded.
     The solution to this is simple; we just allowlist the filetype and then
     they show up in CS a few hours later.
  2. If we blocklist, the problem is that some large binary files of a new
     filetype may show up. This could go undetected for a long time, causing
     the Git repo to start expanding until it gets too big for the builders to
     fetch. The fix in this case is essentially to blow away the generated Git
     repo and start again.

  Since the problems caused by allowlisting are more easily managed than those
  caused by blocklisting, we use an allowlist.
  """
  allowed_extensions = {
      'build_metadata',
      'c',
      'cc',
      'cpp',
      'css',
      'cxx'
      'desugardeps',
      'gn',
      'h',
      'hpp',
      'htm',
      'html',
      'hxx',
      'inc',
      'java',
      'js',
      'json',
      'ninja',
      'proto',
      'py',
      'rs',
      'rsp',
      'runtime_deps',
      'strings',
      'txt',
      'typemap_config',
      'xml',
  }
  dot_index = filename.rfind('.')
  return dot_index != -1 and filename[dot_index + 1:] in allowed_extensions


def translate_root(source_root: str, target_root: str, filename: str) -> str:
  """Translate a filepath from one relative root to another.

  For example:
    translate_root('/foo', '/bar', '/foo/baz') => '/bar/baz'

  Args:
    source_root: The root that the input filepath is already relative to.
    target_root: The root that the output filepath will be relative to.
    filename: The file to translate.

  Returns:
    An updated filename, relative to target_root instead of source_root.
  """
  relative_to_root = os.path.join(filename[len(source_root) + 1:])
  return os.path.join(target_root, relative_to_root)


def kzip_input_paths(kzip_path: str) -> Set[str]:
  """Return the set of all required_inputs in the kzip."""
  required_inputs: Set[str] = set()
  try:
    with zipfile.ZipFile(
        kzip_path, mode='r', compression=zipfile.ZIP_DEFLATED,
        allowZip64=True) as kzip:

      for zip_info in kzip.infolist():
        # kzip should contain following structure:
        # foo/
        # foo/files
        # foo/files/bar
        # foo/pbunits
        # foo/pbunits/bar
        # We only care for the compilation units in foo/pbunits/*. See
        # https://kythe.io/docs/kythe-kzip.html for more on kzips.
        if not re.match(r'.*/pbunits/\w*', zip_info.filename):
          continue

        compilation = analysis_pb2.IndexedCompilation()
        with kzip.open(zip_info, 'r') as file:
          compilation.ParseFromString(file.read())

        for required_input in compilation.unit.required_input:
          path = required_input.v_name.path

          # Absolute paths refer to libraries. Ignore these.
          if os.path.isabs(path):
            continue
          if not has_allowed_extension(path):
            continue

          # package_index may adjust vname paths. Add possible adjustments to
          # the required_inputs set.
          parts = path.split(os.sep)

          # Don't sync any temporary files. These aren't actually referenced.
          if 'tmp' in parts:
            continue

          for i in range(len(parts)):
            # Kzips use forward slashes.
            required_inputs.add('/'.join(parts[i:]))
  except zipfile.BadZipfile as e:
    print(f'Error reading kzip file {kzip_path}: {e}')

  return required_inputs


def has_secrets(filepath: str) -> bool:
  """Check whether a file contains any secrets that shouldn't be indexed."""
  patterns: Tuple[str] = (
      # Patterns adapted from
      # https://www.ndss-symposium.org/wp-content/uploads/2019/02/ndss2019_04B-3_Meli_paper.pdf

      # OAuth
      rb'\b4/[0-9A-Za-z-_]+\b',  # Auth Code
      rb'\b1/[0-9A-Za-z-_]{43}\b|\b1/[0-9A-Za-z-_]{64}\b',  # Refresh Token
      rb'\bya29\.[0-9A-Za-z-_]+\b',  # Access Token
      rb'\bAIza[0-9A-Za-z-_]{35}\b',  # API Key

      # Private Key
      rb'\bPRIVATE KEY( BLOCK)?-----',
  )
  r = re.compile(b'|'.join(patterns))
  with open(filepath, 'rb') as f:
    text = f.read()
    return re.search(r, text) is not None


def copy_generated_files(source_dir: str,
                         dest_dir: str,
                         kzip_input_suffixes: Optional[Set[str]] = None,
                         ignore: Optional[Set[str]] = None) -> None:
  """Copy files from source_dir to dest_dir.

  Args:
    source_dir: The directory to copy from.
    dest_dir: The directory to copy into. Will be created if it doesn't already
      exist.
    kzip_input_suffixes: A set of file endings referenced by the kzip.
    ignore: A list of source paths to ignore when copying.
  """
  os.makedirs(dest_dir, exist_ok=True)
  if ignore is None:
    ignore = set()

  def is_referenced(path: str) -> bool:
    """Check whether the given source path is referenced by kzip_input_suffixes.

    Args:
      path: The source path to check.

    Returns:
      True if any suffix of this path is in kzip_input_suffixes.

    Raises:
      AssertionError: Called when kzip_input_paths is None.
    """
    assert kzip_input_paths is not None
    # Since kzip_input_suffixes is a set of path endings, check each ending
    # of dest_file for membership in the set. Checking this way is faster than
    # linear time search with endswith.
    dest_parts = path.split(os.sep)
    for i in range(len(dest_parts)):
      # Kzips use forward slashes.
      check = '/'.join(dest_parts[i:])
      if check in kzip_input_suffixes:
        return True
    return False

  def is_ignored(path: str) -> bool:
    """Check whether the given source path should be ignored.

    Args:
      path: The source path to check.

    Returns:
      True if the source path, or any of its parent dirs, is in the ignore set.
    """
    parts = os.path.normpath(path).split(os.sep)
    for i in range(len(parts)):
      if os.sep.join(parts[:i + 1]) in ignore:
        return True
    return False

  # First, delete everything in dest that:
  #   * isn't in source or
  #   * is ignored in the source directory or
  #   * doesn't match the allowed extensions or
  #   * may contain secrets or
  #   * (if kzip is provided,) isn't referenced in the kzip
  for dirpath, _, filenames in os.walk(dest_dir):
    for filename in filenames:
      dest_file = os.path.join(dirpath, filename)
      source_file = translate_root(dest_dir, source_dir, dest_file)

      delete = False
      if not os.path.exists(source_file):
        reason = f'source_file {source_file} does not exist.'
        delete = True
      elif is_ignored(source_file):
        reason = f'source_file {source_file} is ignored.'
        delete = True
      elif not has_allowed_extension(source_file):
        reason = \
            f'source_file {source_file} does not have an allowed extension.'
        delete = True
      elif has_secrets(dest_file):
        reason = f'dest_file {dest_file} may contain secrets.'
        delete = True
      elif kzip_input_suffixes and not is_referenced(dest_file):
        reason = f'dest_file {dest_file} not referenced by kzip.'
        delete = True

      if delete:
        print(f'Deleting dest_file {dest_file}: {reason}')
        os.remove(dest_file)

  # Second, copy everything that matches the allowlist from source to dest. If
  # kzip is provided, don't copy files that aren't referenced. Don't sync
  # ignored paths.
  for dirpath, _, filenames in os.walk(source_dir):
    if dirpath != source_dir:
      os.makedirs(translate_root(source_dir, dest_dir, dirpath), exist_ok=True)

    # Don't sync any temporary files. These aren't actually referenced.
    # Only check for tmp in the path relative to the source root, otherwise all
    # source files will be ignored if the source root contains a tmp component
    # (as is the case when running tests locally).
    if 'tmp' in os.path.relpath(dirpath, source_dir).split(os.sep):
      continue

    for filename in filenames:
      source_file = os.path.join(dirpath, filename)

      # arm-generic builder runs into an issue where source_file disappears by
      # the time it's copied. Check source_file so the builder doesn't fail.
      if not os.path.exists(source_file):
        print('File does not exist:', source_file)
        continue

      if not has_allowed_extension(filename) or has_secrets(source_file) \
          or ignore and is_ignored(source_file) \
          or kzip_input_suffixes and not is_referenced(filename):
        continue

      dest_file = translate_root(source_dir, dest_dir, source_file)

      if not os.path.exists(dest_file):
        print('Adding file:', dest_file)
      shutil.copyfile(source_file, dest_file)

  # Finally, delete any empty directories. We keep going to a fixed point, to
  # remove directories that contain only other empty directories.
  dirs_to_examine = [
      dirpath for (dirpath, _, _) in os.walk(dest_dir) if dirpath != dest_dir
  ]
  while dirs_to_examine:
    d = dirs_to_examine.pop()

    # We make no effort to deduplicate paths in dirs_to_examine, so we might
    # have already removed this path.
    if os.path.exists(d) and not os.listdir(d):
      print('Deleting empty directory:', d)
      os.rmdir(d)

      # The parent dir might be empty now, so add it back into the list.
      parent_dir = os.path.dirname(d.rstrip(os.sep))
      if parent_dir != dest_dir:
        dirs_to_examine.append(parent_dir)


def _parse_args(argv: List[str]) -> argparse.Namespace:
  """Parse command-line args.

  Args:
    argv: List of command-line arguments, excluding the script name.
  """
  parser = argparse.ArgumentParser()
  parser.add_argument('--message', help='commit message', required=True)
  parser.add_argument(
      '--dest-branch',
      help='git branch in the destination repo to sync to',
      default='main')
  parser.add_argument(
      '--kzip-prune',
      help='kzip to reference when selecting which source files to copy',
      default='')
  parser.add_argument(
      '--nokeycheck',
      action='store_true',
      help='if set, skips keycheck on git push.')
  parser.add_argument(
      '--dry-run',
      action='store_true',
      help='if set, skips pushing to the remote repo.')
  parser.add_argument(
      '--ignore',
      action='append',
      help='source paths to ignore when copying',
      default=[])
  parser.add_argument(
      '--copy',
      action='append',
      help=('a copy configuration that maps a source dir to a target dir in '
            'dest_repo. takes the format /path/to/src;dest_dir'),
      required=True)
  parser.add_argument('dest_repo', help='git checkout to copy files to')
  return parser.parse_args(argv)


def main(argv: List[str]) -> None:
  """Main script entrypoint.

  Args:
    argv: List of command-line arguments, excluding the script name.
  """
  opts = _parse_args(argv)

  kzip_input_suffixes: Optional[Set[str]] = None
  if opts.kzip_prune:
    kzip_input_suffixes = kzip_input_paths(opts.kzip_prune)

  ignore_paths: Set[str] = {os.path.normpath(i) for i in opts.ignore}

  for c in opts.copy:
    source, dest = c.split(';')
    copy_generated_files(source, os.path.join(opts.dest_repo, dest),
                         kzip_input_suffixes=kzip_input_suffixes,
                         ignore=ignore_paths)

  send_to_git(
      opts.dest_repo,
      opts.dest_branch,
      commit_message=opts.message,
      dry_run=opts.dry_run)


def send_to_git(dest_repo: str,
                dest_branch: str,
                commit_message: str,
                nokeycheck: bool = False,
                dry_run: bool = False) -> None:
  """Upload the generated files to Git.

  Args:
    dest_repo: The local checkout to which files were copied.
    dest_branch: The Git branch in the destination repo to sync to.
    commit_message: What to write for the Git commit message.
    nokeycheck: If True, skip keycheck during git push.
    dry_run: If True, don't actually push the files to the Git remote.
  """
  check_call(['git', 'add', '--', '.'], cwd=dest_repo)
  check_call(['git', 'status'], cwd=dest_repo)
  check_call(['git', 'diff'], cwd=dest_repo)
  status = subprocess.check_output(['git', 'status', '--porcelain'],
                                   cwd=dest_repo)
  if not status:
    print('No changes, exiting')
    return

  check_call(['git', 'commit', '-m', commit_message], cwd=dest_repo)

  cmd = ['git', 'push']
  if nokeycheck:
    cmd.extend(['-o', 'nokeycheck'])
  cmd.extend(['origin', f'HEAD:{dest_branch}'])
  check_call(cmd, cwd=dest_repo, dry_run=dry_run)


def check_call(cmd: Union[str, Sequence[str]],
               cwd: Optional[str] = None,
               dry_run: bool = False) -> int:
  """Wrapper for subprocess.check_call().

  Args:
    cmd: The command to run.
    cwd: The directory in which to run the command.
    dry_run: If True, don't actually run the command.

  Returns:
    The command's return code.
  """
  message = f'Pretending to run {cmd}' if dry_run else f'Running {cmd}'
  if cwd is not None:
    message = f'{message} in {cwd}'
  print(message)
  if dry_run:
    return 0
  return subprocess.check_call(cmd, cwd=cwd)


if '__main__' == __name__:
  sys.exit(main(sys.argv[1:]))
