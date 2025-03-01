#!/usr/bin/env python3
# Copyright 2025 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from __future__ import annotations

import os
import subprocess
import sys
import textwrap

from pathlib import Path

import modallow


def findParsableFiles(root: Path) -> list[modallow.Processable]:
  found: set[Path] = set()
  for dirpath, _, filenames in os.walk(root):
    if 'mod.allow' in filenames:
      found.add(Path(dirpath)/'mod.allow')
  return sorted(
      modallow.Processable(path, True, getUsedModules)
      for path in found)


GOOS = ['windows', 'darwin', 'linux']

def getUsedModules(modAllow: Path) -> list[str]:
  ret = set()
  for osname in GOOS:
    env = os.environ.copy()
    env['GOOS'] = osname
    ret.update(
        line.strip() for line in
        subprocess.check_output(
            ['go', 'list', '-deps', '-f', '{{ if .Module }}{{.Module.Path}}{{end}}'],
            cwd=modAllow.parent,
            encoding='utf-8',
            env=env,
        ).splitlines()
    )
  return sorted(ret)


def main():
  return modallow.Process(
      textwrap.dedent('''
      Checks that Go packages with `mod.allow` files only rely on the
      modules listed.
      '''),
      textwrap.dedent('''
      Recommended procedure to add mod.allow to packages:

        1. `touch mod.allow` in the package you want to check.
           Typically this will be a 'main' package for some actual
           built executable, but this will work on intermediate
           library packages, too.
        2. Run `check_imports.py --fix`. This will populate the
           'mod.allow' file, prefering well-known groups first,
           and single modules for everything else.
        3. Review 'mod.allow' contents and edit (e.g. replace
           individual modules with appropriate prefixes).
        4. Commit these.

      Why add a mod.allow file at all?

      The rationale here is that infra.git (and luci-go) are both quite
      large Go modules with lots of different functionality, libraries
      and programs. This script allows us to restrict particular programs
      to a limited set of allowed modules to prune this down over time,
      or at least to ensure that the list does not grow via changes in
      transitive dependencies without additional review.
      '''),
      findParsableFiles,
  )


if __name__ == '__main__':
  sys.exit(main())
