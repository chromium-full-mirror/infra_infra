#!/usr/bin/env python3
# Copyright 2022 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Resolve the latest git version based on GitSource."""
import dataclasses
import enum
import functools
import json
import operator
import re
import subprocess
import sys

from pkg_resources.extern import packaging

from typing import Callable
from typing import List
from typing import Optional
from typing import Tuple


class Operator(enum.Enum):
  UNKNOWN = 0
  LT = 1  # less-than
  LE = 2  # less-than-or-equal-to
  GT = 3  # greater-than
  GE = 4  # greater-than-or-equal-to
  EQ = 5  # equal-to
  NE = 6  # not-equal-to

# Maps the operator OP(a, b) to the reverse function. For example:
#
#    A < B   maps to   B > A
#
# This will allow us to use functools.partial to pre-fill the value of B.
FILTER_TO_REVERSE_OP = {
    Operator.LT: operator.gt,
    Operator.LE: operator.ge,
    Operator.GT: operator.lt,
    Operator.GE: operator.le,

    # EQ and NE are commutative comparisons, so they map directly to their
    # equivalent function in `operator`.
    Operator.EQ: operator.eq,
    Operator.NE: operator.ne,
}


@dataclasses.dataclass
class SemverRestriction:
  op: Operator
  val: str

  def __post_init__(self):
    if not isinstance(self.op, Operator):
      self.op = Operator(self.op)


@dataclasses.dataclass
class GitSource:
  """Same as the GitSource in 3pp proto spec.

  Reimplement the schema in python makes bundling it in Go binary easier.
  """
  repo: str
  tag_pattern: Optional[str] = None
  version_restriction: List[SemverRestriction] = (
      dataclasses.field(default_factory=list))
  version_join: Optional[str] = None
  tag_filter_re: Optional[str] = None
  fixed_commit: Optional[str] = None

  def __post_init__(self):
    if self.fixed_commit:
      if any((
          self.tag_pattern,
          self.version_restriction,
          self.version_join,
          self.tag_filter_re,
      )):
        raise AssertionError("fixed_commit is mutually exclusive with tags.")
      return

    self.tag_pattern = self.tag_pattern or '%s'
    self.version_join = self.version_join or '.'

    if self.version_restriction:
      self.version_restriction = [
          SemverRestriction(**kv) for kv in self.version_restriction
      ]


def resolve_latest(versions: List[str]) -> (str, str):
  """Return the latest version from the list based on Semantic Versioning."""
  highest_cmp = _parse_version('0')
  highest_str = ''
  git_tree_hash = ''
  for vers, v_str, git_hash in versions:
    if vers > highest_cmp:
      highest_cmp = vers
      highest_str = v_str
      git_tree_hash = git_hash

  assert highest_str
  version = highest_str

  source_hash = git_tree_hash

  return version, source_hash


def _parse_version(v: str) -> packaging.version.Version:
  try:
    return packaging.version.Version(v)
  except packaging.version.InvalidVersion:
    # Return 0.0.0 for "invalid" versions because they are not comparable.
    return  packaging.version.Version('0')


def _to_versions(
    raw_ls_remote_lines: str,
    version_join: str,
    tag_re: re.Pattern,
    tag_filter_re: re.Pattern,
) -> List[Tuple[str, str]]:
  """Converts raw ls-remote output lines to versions.

  Converts raw ls-remote output lines to a sorted (descending) list of
  (Version, v_str, git_hash) objects.

  This is used for source:git method to find latest version and git hash.

  Args:
    raw_ls_remote_lines: raw result of ls-remote output lines.
    version_join: the conjunction between version numbers.
    tag_re: Regular expression for valid tags.
    tag_filter_re: Regular expression to exclude tags.

  Returns:
    List[str, str]: list of versions and hashes.
  """
  ret = []
  for line in raw_ls_remote_lines:
    git_hash, ref = line.split('\t')
    if ref.startswith('refs/tags/'):
      tag = ref[len('refs/tags/'):]
      if tag_filter_re and not tag_filter_re.match(tag):
        continue
      m = tag_re.match(tag)
      if not m:
        continue

      v_str = m.group(1)
      if version_join:
        v_str = '.'.join(v_str.split(version_join))

      ret.append((_parse_version(v_str), v_str, git_hash))
  return sorted(ret, reverse=True)


def _filters_to_func(
    filters: List[SemverRestriction]) -> Callable[[str], bool]:
  """Convert SemverRestrictions to a filter function."""
  restrictions = [
      functools.partial(FILTER_TO_REVERSE_OP[f.op], _parse_version(f.val))
      for f in filters
  ]
  def _apply_filter(candidate_version):
    for restriction in restrictions:
      if not restriction(candidate_version):
        return False
    return True
  return _apply_filter


def _filter_versions(
    version_strs: List[str],
    filters: List[SemverRestriction],
) -> List[Tuple[str, str]]:
  if not filters:
    return version_strs
  filt_fn = _filters_to_func(filters)
  return [
      (vers, vers_s, git_hash)
      for vers, vers_s, git_hash in version_strs
      if filt_fn(vers)
  ]


def get_versions(src: GitSource) -> List[str]:
  """Get all valid versions base on the GitSource definition.

  List all versions in the repo which matched the format and are filtered
  by the regular expressions.

  Args:
    src: GitSource definition.

  Returns:
    List[str]: list of versions.
  """
  raw = subprocess.check_output(['git', 'ls-remote', '-t', src.repo]).decode()

  # We need to transform the tag_pattern (which is a python format-string
  # lookalike with `%s` in it) into a regex which we can use to scan over the
  # repo's tags.
  tag_re = re.escape(src.tag_pattern)
  tag_re = '^%s$' % (tag_re.replace('%s', '(.*)'),)

  tag_filter_re = None
  if src.tag_filter_re:
    tag_filter_re = re.compile(src.tag_filter_re)

  versions = _to_versions(
      raw.splitlines(),
      src.version_join,
      re.compile(tag_re),
      tag_filter_re)

  versions = _filter_versions(
      versions, src.version_restriction)

  return versions


def main() -> int:
  raw = json.loads(sys.argv[1])
  src = GitSource(**raw)

  if src.fixed_commit:
    version, commit = src.fixed_commit, src.fixed_commit
  else:
    versions = get_versions(src)
    version, commit = resolve_latest(versions)

  json.dump({
      'version': version,
      'git': {'url': raw['repo'], 'commit': commit},
  }, sys.stdout)
  return 0


if __name__ == '__main__':
  sys.exit(main())
