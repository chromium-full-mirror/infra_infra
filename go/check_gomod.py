#!/usr/bin/env python3
# Copyright 2025 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from __future__ import annotations

import functools
import json
import os
import subprocess
import sys
import textwrap

from pathlib import Path

import modallow


def findParsableFiles(root: Path) -> list[modallow.Processable]:
  found: list[modallow.Processable] = []

  for dirpath, dirnames, filenames in os.walk(root):
    direct_path = None
    if 'go.mod.allow_direct' in filenames:
      direct_path = Path(dirpath)/'go.mod.allow_direct'
      found.append(modallow.Processable(
          direct_path,
          False,
          functools.partial(getUsedModules, False),
      ))

    if 'go.mod.allow_indirect' in filenames:
      found.append(modallow.Processable(
          Path(dirpath)/'go.mod.allow_indirect',
          False,
          functools.partial(getUsedModules, True),
          extra_patterns = [direct_path],
      ))

    if 'go.mod' in filenames:
      dirnames.clear()
  return found


def getUsedModules(indirect: bool, modAllow: Path) -> list[str]:
  raw = subprocess.check_output(
      ["go", "list", "-m", "-json", "all"],
      cwd=modAllow.parent,
      encoding='utf-8',
  )
  dec = json.JSONDecoder()
  offset = 0
  ret: list[str] = []
  while offset < len(raw):
    obj, offset = dec.raw_decode(raw, offset)
    offset += 1
    if "Dir" not in obj:
      # This indicates that, while the module is reachable from the go.mod
      # graph, it is not actually USED by any imports. This is important to
      # prune out e.g. transitive test-only dependencies which never make it
      # into anything that we really need.
      continue

    if indirect != obj.get("Indirect", False):
      continue

    ret.append(obj["Path"])
  return ret


def main():
  return modallow.Process(
      textwrap.dedent('''
      Checks that Go modules with `go.mod.allow` files only rely on the
      modules listed.
      '''),
      textwrap.dedent('''
      Recommended procedure to add go.mod.allow:

        1. `touch go.mod.allow` right next to `go.mod`.
        2. Run `check_gomod.py --fix`. This will populate the
           'go.mod.allow' file, prefering well-known groups first,
           and single modules for everything else.
        3. Review 'go.mod.allow' contents and edit (e.g. replace
           individual modules with appropriate prefixes).
        4. Commit it.

      Why add a go.mod.allow file at all?

      The rationale here is that infra.git (and luci-go) are both quite
      large Go modules with lots of different functionality, libraries
      and programs. This script allows us to ensure that the ADDITION of
      new module dependencies recieves adequate review, while allowing
      updates to already-accepted modules (or module groups) to be
      reviewable by a much broader set of reviewers.
      '''),
      findParsableFiles,
  )


if __name__ == '__main__':
  sys.exit(main())
