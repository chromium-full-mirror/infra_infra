# Copyright 2025 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from __future__ import annotations

import argparse
import os
import re
import subprocess
import textwrap

from pathlib import Path
from dataclasses import dataclass, field
from collections.abc import Callable


_LUCI_DEPS = [
  "github.com/op/go-logging", # <= go.chromium.org/luci/logging
  "github.com/envoyproxy/",
  "github.com/go-git/",
  "github.com/julienschmidt/httprouter",
  "github.com/klauspost/compress",
]


_WELL_KNOWN: dict[str, list[str]] = {
  # LUCI libraries
  '$luci': [
    'go.chromium.org/luci',
  ] + _LUCI_DEPS,

  # All infra modules (including LUCI)
  '$infra': [
    'go.chromium.org/',
  ] + _LUCI_DEPS,

  # First-party (Google-owned) modules
  '$google': [
    "cel.dev/expr",
    "cloud.google.com/",
    "github.com/bazelbuild/",
    "github.com/golang/",
    "github.com/google/",
    "github.com/googleapis/",
    "github.com/GoogleCloudPlatform/",
    "golang.org/",
    "google.golang.org/",
  ],

  # Common Googler/Xoogler modules
  '$google_ext': [
    "github.com/danjacques/gofslock",
    "github.com/maruel/subcommands",
  ]+[ # deps
    "github.com/texttheater/golang-levenshtein", # <= github.com/maruel/subcommands
  ],

  '$opencensus': [
    "go.opencensus.io",
    "github.com/census-instrumentation/opencensus-proto",
  ] + [
    "github.com/felixge/httpsnoop", # <= github.com/census-instrumentation/opencensus-proto
    "contrib.go.opencensus.io/exporter/stackdriver",
  ],

  '$otel': [
    "go.opentelemetry.io/",
  ] + [
    "github.com/go-logr/",  # <= go.opentelemetry.io/
  ],

  '$k8s': [
    'k8s.io/',
    'sigs.k8s.io/',
  ],

  '$prometheus': [
    'github.com/prometheus/',
  ],

  '$docker': [
    'github.com/containerd/',
    'github.com/docker/',
    'github.com/moby/',
    'github.com/opencontainers/',
  ],
}

def _parseSimple(pattern: str) -> str:
  if pattern.endswith('/'):
    return f'{re.escape(pattern)}.*'
  return re.escape(pattern)


# at this point _WELL_KNOWN will contain escaped and non-group patterns only.
_WELL_KNOWN = {
  key: list(
    _parseSimple(pattern) for pattern in value
  ) for key, value in _WELL_KNOWN.items()
}


def _parsePattern(pattern: str) -> list[str]:
  if pattern.startswith('$'):
    try:
      return _WELL_KNOWN[pattern]
    except KeyError as ex:
      raise ValueError(f'Unknown group {pattern!r}.') from ex

  return [_parseSimple(pattern)]


def _patternsToRe(*parsedPatterns: str) -> re.Pattern:
  pattern = ")|(".join(set(parsedPatterns))
  if len(parsedPatterns) > 1:
    pattern = f'({pattern})'
  pattern = f'^({pattern})$'
  return re.compile(pattern)


_WELL_KNOWN_RE = {key: _patternsToRe(*value) for key, value in _WELL_KNOWN.items()}

_EPILOG_DOC = textwrap.dedent('''
The mod.allow format is quite simple:
  * Blank lines and lines starting with # are skipped
  * All other lines are either:
    * A group (starting with $). See below for known groups.
    * A module (a Go module name like 'cel.dev/expr')
    * A module prefix (a partial Go module name ending with '/',
      like 'cloud.google.com')

The following groups are available in mod.allow files:

  $luci - go.chromium.org/luci and common dependencies of this.
  $infra - go.chromium.org/ and common dependencies of this.
  $google - Google official modules.
  $google_ext - Common Googler/Xoogler unofficial modules.
  $opencensus - OpenCensus and dependencies.
  $otel - OpenTelemetry and dependencies.
''')

_PREAMBLE_LINE = "# Restricts allowed modules. See infra.git/go/check_{imports,gomod}.py"


@dataclass
class _Failure:
  disallowed_modules: list[str]
  patterns_to_add: list[str]
  needs_preamble: bool


@dataclass
class _Processor:
  _loaded_pattern: re.Pattern
  _needs_preamble: bool

  def process(self, modules: list[str]) -> _Failure|None:
    disallowed = []
    to_add = set()

    for mod in modules:
      mod = mod.strip()
      if self._loaded_pattern.match(mod):
        continue
      else:
        for group, groupRE in _WELL_KNOWN_RE.items():
          if groupRE.match(mod):
            to_add.add(group)
            break
        else:
          to_add.add(mod)

        disallowed.append(mod)

    if disallowed:
      return _Failure(sorted(disallowed), sorted(to_add), self._needs_preamble)
    return None


ModuleLoaderFunc = Callable[[Path], list[str]]


@dataclass(order=True)
class Processable:
  path: Path
  include_own_module: bool
  load_modules: ModuleLoaderFunc

  extra_patterns: list[Path] = field(default_factory=list)

  @staticmethod
  def _parse_file(path: Path) -> tuple[bool, set[str]]:
    ret: set[str] = set()

    needs_preamble = True
    for i, line in enumerate(path.read_text(encoding='utf-8').splitlines()):
      cleanLine = line.strip()
      if i == 0 and cleanLine == _PREAMBLE_LINE:
        needs_preamble = False
        continue
      if not cleanLine or cleanLine.startswith('#'):
        continue
      ret.update(_parsePattern(cleanLine))

    return needs_preamble, ret


  def parse(self) -> _Processor:
    try:
      # Parse mod.allow, ignoring blank lines and lines starting with #.
      # If the line is $group,
      patterns: set[str] = set()
      # We start with needs_preamble = True to account for completely empty files
      # with no lines at all.
      needs_preamble, patterns = self._parse_file(self.path)
      for extra_path in self.extra_patterns:
        _, extra_patterns = self._parse_file(extra_path)
        patterns.update(extra_patterns)

      if self.include_own_module:
        # Find out the module that `path` is in - this will always be allowed.
        cur_module = subprocess.check_output(
            ['go', 'list', '-f', '{{.Module.Path}}'],
            cwd=self.path.parent,
            encoding='utf-8',
        ).strip()
        patterns.add(_parseSimple(cur_module))
      pattern = _patternsToRe(*patterns)
      return _Processor(pattern, needs_preamble)
    except Exception as ex:
        raise ValueError(f'while parsing {self.path}') from ex


def Process(
    description: str,
    epilog: str,
    scan: Callable[[Path], list[Processable]]) -> int:
  """Process takes help text (description + epilog) and implements
  a main() function which calls scan with the root path to find work in.

  Scan should return a list of Processables.

  For each Processable, it will load the allowed modules from the file, load the
  actual modules by calling the `Processable.load_modules` function, compare
  them, including to _WELL_KNOWN groups, and compute success or a _Failure
  object.

  If a _Failure happens, it will be reported to stdout, and if the user elected,
  a fix will be applied to the original module file.

  Finally, this returns the overall exit code for the process.
  """
  parser = argparse.ArgumentParser(
      description=description,
      epilog=f'{_EPILOG_DOC}\n{epilog}',
  )
  parser.add_argument(
      '--fix', action='store_true',
      help='If provided, check_imports will append missing rules.')
  parser.add_argument(
      'root_path', default='.', nargs='?',
      help=(
        'The root directory to explore for mod.allow files. '
        'Uses cwd by default.'))

  args = parser.parse_args()

  failed = False
  for to_process in scan(args.root_path):
    print(f'processing: {to_process.path}')
    processor = to_process.parse()
    if failure := processor.process(to_process.load_modules(to_process.path)):
      failed = True
      print(f'> disallowed modules:')
      for mod in failure.disallowed_modules:
        print(f'   {mod}')

      message = "adding" if args.fix else "consider adding (or pass --fix)"
      print(f'> {message}:\n   ', '\n   '.join(failure.patterns_to_add), sep='')

      if args.fix:
        with open(to_process.path, mode='r+', encoding='utf-8') as f:
          if failure.needs_preamble:
            contents = f.read()
            f.write(_PREAMBLE_LINE)
            if not contents.startswith('\n'):
              f.write('\n')
            f.write(contents)
            f.seek(0, os.SEEK_SET)
          # We can't seek to the end of the file minus one byte in text mode in
          # python (because we're using a variable width encoding). However,
          # these files are so tiny, just read the whole file to see if ends
          # with a newline - which will also seek us to the end of the file in
          # the process.
          if not f.read().endswith('\n'):
            print(file=f)
          print('\n'.join(failure.patterns_to_add), file=f)

  return 1 if not args.fix and failed else 0
