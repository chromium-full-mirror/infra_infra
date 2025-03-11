# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Archive a git repository.

The script can be used to download the source code of a git repository at a
certain commit. It should generate binary reproducible result on any operating
system with same version of python.
"""

import os
import pathlib
import subprocess
import sys
import tarfile
import tempfile
from typing import Dict


def archive(
    output: tarfile.TarFile, workdir: pathlib.Path,
    prefix: pathlib.Path, repository: str, commit: str) -> None:
  """Archive the provided git repository with all its submodules.

  The archive(...) function uses git client to archive each of the repository
  and merge them into a single tar file. It only interacts with the bare git
  repository without checkout, which eliminates filesystem limitations.

  Args:
    output: merged tar file.
    workdir: temporary directory for cloning bare git repository.
    prefix: path prefix inside the tar file.
    repository: url of the git repository.
    commit: commit to be archived.
  """
  print(f'Archiving {repository} at {commit} for {prefix}')
  git_dir = workdir.joinpath(prefix, '.git')
  subprocess.check_call([
      'git',
      'clone', '--quiet', '--bare', repository, git_dir,
  ])

  with subprocess.Popen([
      'git', f'--git-dir={git_dir}',
      '-c', 'core.autocrlf=false', '-c', 'core.eol=lf',
      'archive', '--format=tar', f'--prefix={prefix.as_posix()}/', commit,
  ], stdout=subprocess.PIPE) as p:
    try:
      with tarfile.open(fileobj=p.stdout, mode='r|') as tar:
        for m in tar:
          # Members with unknown types are treated as regular files.
          if m.isreg() or m.type not in tarfile.SUPPORTED_TYPES:
            output.addfile(m, tar.extractfile(m))
          else:
            output.addfile(m)
      while p.stdout.read(4096):
        # discard rest of the stdout
        pass
      if retcode := p.wait():
        raise subprocess.CalledProcessError(retcode, p.args)
    except:
      p.kill()
      raise

  # Load .gitmodules
  mods_hash = subprocess.check_output([
      'git', f'--git-dir={git_dir}',
      'ls-tree', '--object-only', commit, '.gitmodules',
  ]).decode().strip()
  if not mods_hash:
    # No submodule available
    return
  mods_raw = subprocess.check_output([
      'git', f'--git-dir={git_dir}',
      'config', '-z', '--blob', mods_hash, '--list'
  ]).decode().strip('\0')

  mods: Dict[str, Dict[str, str]] = {'path': {}, 'url': {}}
  for m in mods_raw.split('\0'):
    if m := m.strip():
      k, v = m.split('\n', 1)
      _, mname, mkey = k.split('.')
      if mkey in mods:
        mods[mkey][mname] = v

  # List all nodes with type commit
  nodes_raw = subprocess.check_output([
      'git', f'--git-dir={git_dir}',
      'ls-tree', '-r', '-z', commit,
  ]).decode().strip('\0')

  nodes: Dict[str, str] = {}
  for n in nodes_raw.split('\0'):
    fmode, ftype, fhash, fpath = n.split(maxsplit=3)

    # For submodules, mode is 160000 and type is 'commit'.
    # Ref: S_IFGITLINK in https://github.com/git/git/blob/main/cache.h
    if fmode != '160000' or ftype != 'commit':
      continue
    nodes[fpath] = fhash

  # Iterate all submodule paths and recursively archive repositories.
  for k, v in mods['path'].items():
    archive(
        output=output,
        workdir=workdir,
        prefix=prefix.joinpath(v),
        repository=mods['url'][k],
        commit=nodes[v],
    )


def main() -> None:
  if len(sys.argv) != 3:
    print('usage: git_archive.py [REPOSITORY] [COMMIT]')
    sys.exit(1)

  output_dir = pathlib.Path(os.environ.get('out'))

  with tarfile.open(
      name=output_dir.joinpath('src.tar'), mode='w:',
      format=tarfile.PAX_FORMAT, encoding='utf-8') as output:
    with tempfile.TemporaryDirectory(dir='.') as tmpdir:
      archive(
          output=output,
          workdir=pathlib.Path(tmpdir),
          prefix=pathlib.Path('src'),
          repository=sys.argv[1],
          commit=sys.argv[2],
      )

if __name__ == '__main__':
  main()
