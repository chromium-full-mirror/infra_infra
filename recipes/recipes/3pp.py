# Copyright 2018 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This recipe builds and packages third party software, such as Git."""

import hashlib

from recipe_engine.recipe_api import Property
from recipe_engine.config import ConfigList, ConfigGroup, Single, List

from PB.go.chromium.org.luci.buildbucket.proto import build as build_pb2
from PB.go.chromium.org.luci.buildbucket.proto import common as common_pb2


PYTHON_VERSION_COMPATIBILITY = 'PY3'

DEPS = [
    'recipe_engine/buildbucket',
    'recipe_engine/bcid_reporter',
    'recipe_engine/cipd',
    'recipe_engine/file',
    'recipe_engine/path',
    'recipe_engine/properties',
    'recipe_engine/raw_io',
    'recipe_engine/runtime',
    'recipe_engine/step',
    'depot_tools/git',
    'depot_tools/tryserver',
    'support_3pp',
]


PROPERTIES = {
    'package_locations':
        Property(
            help=('URL of repo containing package definitions.'
                  'Cross-compiling requires docker on $PATH.'),
            kind=ConfigList(
                lambda: ConfigGroup(
                    repo=Single(str),
                    ref=Single(str, required=False),
                    subdir=Single(str, required=False),
                ),)),
    'to_build':
        Property(
            help=(
                'The names (and optionally versions) of the packages to build '
                'and upload. Leave empty to build and upload all known '
                'packages. If you want to specify a version other than '
                '"latest", pass the package name like "some_package@1.3.4".'),
            kind=List(str),
            default=(),
        ),
    'platform':
        Property(
            kind=str,
            default=None,
            help=('Target platform. Must be a valid CIPD ${platform}. '
                  'Cross-compiling requires docker on $PATH.')),
    'force_build':
        Property(
            kind=bool,
            default=False,
            help=(
                'Forces building the packages given in to_build, even if they '
                'are available on the CIPD server already. Doing this disables '
                'uploads.')),
    'package_prefix':
        Property(
            kind=str,
            help=(
                'Joins this CIPD namespace before all downloaded/uploaded '
                'packages. Allows comprehensive testing of the entire '
                'packaging process without uploading into the prod namespace. '
                'If this recipe is run in experimental mode (according to the '
                '`runtime` module), then this will default to '
                '"experimental/support_3pp/".')),
    'source_cache_prefix':
        Property(
            kind=str,
            default='sources',
            help=(
                'Joins this CIPD namespace after the package_prefix to store '
                'the source of all downloaded/uploaded packages. This gives '
                'the flexibility to use different prefixes for different repos '
                '(Default to "sources").')),
    'use_pkgbuild':
        Property(
            kind=bool,
            default=False,
            help=(
                'Uses pkgbuild for building the packages given in to_build, '
                'This should only be used for experimenting.')),
}


def RunSteps(api, package_locations, to_build, platform, force_build,
             package_prefix, source_cache_prefix, use_pkgbuild):
  if api.tryserver.is_tryserver:
    revision = api.tryserver.gerrit_change_fetch_ref
    force_build = True  # Disallow uploading packages from tryjobs
  else:
    revision = 'refs/heads/main'
    # Report task stage to snoopy.
    if not api.runtime.is_experimental:
      try:
        api.bcid_reporter.report_stage("start")
      except Exception:  # pragma: no cover
        api.step.active_result.presentation.status = api.step.FAILURE

  # NOTE: We essentially ignore the on-machine CIPD cache here. We do this in
  # order to make sure this builder always operates with the current set of tags
  # on the server... Practically speaking, when messing with these builders it's
  # easier to delete packages (especially packages which haven't been rolled out
  # to any other machines).
  #
  # Without dumping the cache, the persisted tag cache can lead to weird
  # behaviors where things like 'describe' permanently tries to load data about
  # a deleted instance, leading to continual re-uploads of packages.
  with api.cipd.cache_dir(api.path.mkdtemp()):
    package_repos = api.path.cache_dir / 'builder'
    current_repos = set()
    try:
      current_repos = set(p.pieces[-1] for p in api.file.glob_paths(
        'read cached checkouts', package_repos, '*',
        test_data=[
          'deadbeef',
          'badc0ffe',
        ]
      ))
    except api.file.Error as err:  # pragma: no cover
      if err.errno_name != 'ENOENT':
        raise

    api.support_3pp.set_package_prefix(package_prefix)
    api.support_3pp.set_source_cache_prefix(source_cache_prefix)

    actual_repos = set()
    tryserver_affected_files = []
    with api.step.nest('load packages from desired repos'):
      for pl in package_locations:
        repo = pl['repo']
        ref = pl.get('ref', revision)
        subdir = pl.get('subdir', '')

        hash_name = hashlib.sha1(str("%s:%s" %
                                     (repo, ref)).encode('utf-8')).hexdigest()
        actual_repos.add(hash_name)

        checkout_path = package_repos / hash_name
        api.git.checkout(repo, ref, checkout_path, submodules=False)

        package_path = checkout_path
        if subdir:
          package_path = package_path / subdir
        api.support_3pp.load_packages_from_path(package_path)

        if api.tryserver.is_tryserver:
          repo_tryserver_affected_files = api.git(
              '-c',
              'core.quotePath=false',
              'diff',
              '--name-only',
              'HEAD~',
              name='git diff to find changed files',
              stdout=api.raw_io.output_text()).stdout.split()
          tryserver_affected_files += [
              checkout_path / f for f in repo_tryserver_affected_files
          ]
    if api.tryserver.is_tryserver:
      assert (tryserver_affected_files != [])

    with api.step.nest('remove unused repos'):
      leftovers = current_repos - actual_repos
      for hash_name in sorted(leftovers):
        api.file.rmtree('rm %s' % (hash_name,),
                        package_repos / hash_name)

    if use_pkgbuild:
      api.support_3pp.pkgbuild(
          to_build,
          platform,
          upload=not api.tryserver.is_tryserver,
          cipd_service='https://chrome-infra-packages-dev.appspot.com/',
      )
    else:
      _, unsupported = api.support_3pp.ensure_uploaded(
          to_build,
          platform,
          force_build=force_build,
          tryserver_affected_files=tryserver_affected_files,
      )

      if unsupported:
        api.step.empty(
            '%d packages unsupported for %r' % (len(unsupported), platform),
            step_text='<br/>' + '<br/>'.join(sorted(unsupported)))

    # Report task stage to snoopy.
    if not api.tryserver.is_tryserver and not api.runtime.is_experimental:
      try:
        api.bcid_reporter.report_stage("upload-complete")
      except Exception:  # pragma: no cover
        api.step.active_result.presentation.status = api.step.FAILURE



def GenTests(api):
  def defaults():
    return (api.properties(
        package_locations=[{
            'repo': 'https://example.repo',
            'subdir': 'support_3pp',
        }],
        package_prefix='hello_world',
    ) + api.runtime())

  yield (api.test('basic') + defaults() +
         api.buildbucket.ci_build(experiments=['security.snoopy']))

  yield (api.test('pkgbuild') + defaults() + api.properties(use_pkgbuild=True) +
         api.step_data(
             'build packages (pkgbuild)',
             api.step.sub_build(build_pb2.Build(status=common_pb2.SUCCESS))) +
         api.buildbucket.ci_build(experiments=['security.snoopy']))

  pkgs = sorted(dict(
    pkg_a='''
    create { unsupported: true }
    upload { pkg_prefix: "prefix/deps" }
    ''',

    pkg_b='''
    create { unsupported: true }
    upload { pkg_prefix: "prefix/deps" }
    ''',
  ).items())

  test = (
    api.test('unsupported') + defaults() +
    api.step_data('load packages from desired repos.find package specs',
                  api.file.glob_paths([n+'/3pp.pb' for n, _ in pkgs]))
  )
  for pkg, spec in pkgs:
    test += api.step_data(
      "load packages from desired repos."
      "load package specs.read '%s/3pp.pb'" % pkg,
      api.file.read_text(spec))
  yield test

  test = (
      api.test('basic-tryjob') + defaults() +
      api.buildbucket.try_build('infra') +
      api.tryserver.gerrit_change_target_ref('refs/branch-heads/foo') +
      api.override_step_data(
          'load packages from desired repos.git diff to find changed files',
          stdout=api.raw_io.output_text('support_3pp/a/3pp.pb')))
  yield test
