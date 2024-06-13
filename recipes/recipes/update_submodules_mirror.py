# Copyright 2018 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import json
import re

from recipe_engine.recipe_api import Property

PYTHON_VERSION_COMPATIBILITY = "PY3"

DEPS = [
    'recipe_engine/buildbucket',
    'recipe_engine/context',
    'recipe_engine/file',
    'recipe_engine/json',
    'recipe_engine/path',
    'recipe_engine/properties',
    'recipe_engine/raw_io',
    'recipe_engine/step',
    'depot_tools/gclient',
    'depot_tools/git',
    'depot_tools/gitiles',
]

PROPERTIES = {
    'source_repo':
        Property(help='The URL of the repo to be mirrored with submodules'),
    'target_repo':
        Property(help='The URL of the mirror repo to be built/maintained'),
    'extra_submodules':
        Property(
            default=[],
            help='A list of <path>=<url> strings, indicating extra submodules '
            'to add to the mirror repo.'),
    'cache_name':
        Property(
            default='codesearch_update_submodules_mirror',
            help='Name of LUCI cache path. This name must match the cache '
            'segment in cr-buildbucket.cfg.'),
    'overlays':
        Property(
            default=[],
            help='A list of DEPS prefixes to be included as submodules to the '
            'mirror repo. Prefix is stripped from DEPS path. Example: '
            'if DEPS contains foo/bar=baz.git and overlay is foo, then the '
            'mirror repo will have submodule baz.git in path bar.'),
    'internal':
        Property(default="", help="If should fetch internal source"),
    'with_tags':
        Property(default=True, help='Whether to clone, fetch and push tags.'),
    # Note: ref_patterns require a `git ls-remote` call. If performance becomes
    # a concern, we may add a `refs` for complete names of refs we care about.
    'ref_patterns':
        Property(
            default=['refs/heads/main'],
            help='A list of patterns for which matching refs should be added. '
            'Git supported pattern syntax: '
            'https://man7.org/linux/man-pages/man7/glob.7.html'),
    'refs_to_skip':
        Property(
            default=[],
            help='A list of refs which match a ref_patterns but should be '
            'excluded from the mirroring. This can be used to remove branches '
            'with invalid git hashes.'),
    'push_to_refs_cs':
        Property(
            default=False,
            help='A boolean if each created synthetic commit should be pushed '
            'to refs/cs/ namespace. There is no refs/ cleanup, so this should '
            'be used only on codesearch projects with relatively infrequent '
            'updates.'),
}

COMMIT_USERNAME = 'Submodules bot'
COMMIT_EMAIL_ADDRESS = \
    'infra-codesearch@chops-service-accounts.iam.gserviceaccount.com'

MAIN_REF = 'refs/heads/main'

SHA1_RE = re.compile(r'[0-9a-fA-F]{40}')


def RunSteps(api, source_repo, target_repo, extra_submodules, cache_name,
             overlays, internal, with_tags, ref_patterns, refs_to_skip,
             push_to_refs_cs):
  _, source_project = api.gitiles.parse_repo_url(source_repo)

  # NOTE: This name must match the definition in cr-buildbucket.cfg. Do not
  # change without adjusting that config to match.
  checkout_dir = api.m.path.cache_dir / cache_name
  api.m.file.ensure_directory('Create checkout parent dir', checkout_dir)

  # We assume here that we won't have a mirror for two repos with the same name.
  # If we do, the directories will have the same name. This shouldn't be an
  # issue, but if it is we should add an intermediate directory with an
  # unambiguous name.
  #
  # We want to keep the final component equal to the below, as gclient/DEPS can
  # be sensitive to the name of the directory a repo is checked out to.
  #
  # The slash on the end doesn't make a difference for source_checkout_dir. But
  # it's necessary for the other uses for source_checkout_name, below.
  source_checkout_name = source_project[source_project.rfind('/') + 1:] + '/'
  source_checkout_dir = checkout_dir / source_checkout_name

  if not api.m.path.exists(source_checkout_dir):
    # We don't depend on any particular cwd, as source_checkout_dir is absolute.
    # But we must supply *some* valid path, or it will fail to spawn the
    # process.
    with api.context(cwd=checkout_dir):
      # Don't use --no-tags even if with_tags is False here since clones may
      # time out.
      api.git('clone', source_repo, source_checkout_dir)

  # This is implicitly used as the cwd by all the git steps below.
  api.m.path.checkout_dir = source_checkout_dir

  refs_to_mirror_set = set()

  for ref_pattern in ref_patterns:
    resp = api.git(
        'ls-remote', source_repo, ref_pattern,
        stdout=api.raw_io.output_text()).stdout.splitlines()
    for line in resp:
      ref = line.split()[1]
      if ref not in refs_to_skip:
        refs_to_mirror_set.add(ref)

  refs_to_mirror = sorted(refs_to_mirror_set)

  # Processing O(100) refs takes hours and is prone to failure. Process main
  # branch first because other builders rely on this being up-to-date.
  if MAIN_REF in refs_to_mirror:
    refs_to_mirror.remove(MAIN_REF)
    refs_to_mirror.insert(0, MAIN_REF)

  try:
    # When .gitmodules is updated, the cached repository has more gitlinks than
    # gitmodules entries. This causes fetch to fail. Resetting the repository
    # before fetching fixes this. See crbug.com/1499932.
    api.git('reset', '--hard', 'origin/main')
    api.git('fetch', '-t' if with_tags else '-n')
    for ref in refs_to_mirror:
      if not ref.startswith('refs/heads'):
        api.git('fetch', 'origin', ref + ':' + RefToRemoteRef(ref))
  except api.step.StepFailure:
    # Remove broken source checkout so that subsequent runs don't fail because
    # of it.
    with api.context(cwd=checkout_dir):
      api.m.file.rmtree('Remove broken source checkout clone',
                        api.path.abspath(source_checkout_dir))
    raise

  for ref in refs_to_mirror:
    with api.step.nest('Process ' + ref):
      api.git('checkout', '-f', RefToRemoteRef(ref))

      gclient_spec = [{
          'managed': False,
          'name': source_checkout_name,
          'url': source_repo,
          'deps_file': 'DEPS'
      }]
      if internal:
        gclient_spec[0]['custom_vars'] = {'checkout_src_internal': True}
      gclient_spec_repr = ("solutions=" + repr(gclient_spec))
      with api.context(cwd=checkout_dir):
        if internal:
          # run sync to fetch additional repositories (may have additional
          # DEPS). We may want to consider doing this even for non-internal.
          api.gclient('sync', [
              'sync',
              '-n',
              '-p',
              '--no-history',
              '--shallow',
              '--no-bootstrap',
              '--spec',
              gclient_spec_repr,
          ])

        deps = json.loads(
            api.gclient(
                'evaluate DEPS', [
                    'revinfo', '--deps', 'all', '--ignore-dep-type=cipd',
                    '--ignore-dep-type=gcs', '--spec', gclient_spec_repr,
                    '--output-json=-'
                ],
                stdout=api.raw_io.output_text()).stdout)

      for item in extra_submodules:
        path, url = item.split('=')
        deps[path] = {'url': url, 'rev': 'main'}

      update_index_entries, gitmodules_entries = GetSubmodules(
          api, deps, source_checkout_name, overlays)

      # This adds submodule entries to the index without cloning the underlying
      # repository.
      api.git(
          'update-index', '--add', *update_index_entries, name='Add gitlinks')

      api.file.write_text('Write .gitmodules file',
                          source_checkout_dir / '.gitmodules',
                          '\n'.join(gitmodules_entries))
      api.git('add', '.gitmodules')

      api.git(
          '-c', 'user.name=%s' % COMMIT_USERNAME,
          '-c', 'user.email=%s' % COMMIT_EMAIL_ADDRESS,
          'commit', '-m', 'Synthetic commit for submodules',
          name='git commit')
      api.git(
          'push',
          '-o',
          'nokeycheck',
          target_repo,
          'HEAD:' + ref,
          # skip-validation is necessary as without it we cannot push >=10k
          # commits at once.
          '--push-option=skip-validation',
          # We've effectively deleted the commit that was at HEAD before. This
          # means that we've diverged from the remote repo, and hence must do a
          # force push.
          '--force',
          name='git push ' + ref)
      if push_to_refs_cs:
        commit_hash = api.git(
            'rev-parse',
            '--short',
            'HEAD',
            name='last commit hash',
            stdout=api.raw_io.output_text()).stdout.strip()
        date = api.git(
            'log',
            '-1',
            '--format=%cs',
            'HEAD',
            name='last commit date',
            stdout=api.raw_io.output_text()).stdout.strip()
        api.git(
            'push',
            '-o',
            'nokeycheck',
            target_repo,
            f'HEAD:refs/cs/{date}-{commit_hash}',
            # skip-validation is necessary as without it we cannot push >=10k
            # commits at once.
            '--push-option=skip-validation',
            name='git push ' + ref)

  api.git('push', '-o', 'nokeycheck', target_repo,
          'refs/remotes/origin/main:refs/heads/main-original')

  if with_tags:
    # You can't use --all and --tags at the same time for some reason.
    # --mirror pushes both, but it also pushes remotes, which we don't want.
    api.git(
        'push',
        '-o',
        'nokeycheck',
        '--tags',
        target_repo,
        name='git push --tags')


def RefToRemoteRef(ref):
  ref = ref.replace('refs/heads', 'refs/remotes/origin')
  ref = ref.replace('refs/branch-heads', 'refs/remotes/branch-heads')
  return ref


def GetSubmodules(api, deps, source_checkout_name, overlays):
  gitmodules_entries = []
  update_index_entries = []
  for path, entry in deps.items():
    url = entry['url']
    rev = entry['rev']
    if rev is None:
      rev = 'HEAD'

    # Filter out the root repo itself, which shows up for some reason.
    if path == source_checkout_name:
      continue
    # Filter out deps that are nested within other deps. Submodules can't
    # represent this.
    if any(path != other_path and path.startswith(other_path + '/')
           for other_path in deps.keys()):
      continue

    # Filter out any DEPS that point outside of the repo, as there's no way to
    # represent this with submodules, unless they are in overlays.
    #
    # Note that source_checkout_name has a slash on the end, so this will
    # correctly filter out any path which has the checkout name as a prefix.
    # For example, src-internal in the src DEPS file.
    if not path.startswith(source_checkout_name):
      should_include = False
      for overlay in overlays:
        if path.startswith(overlay) and path != overlay:
          should_include = True
          path = str(path[len(overlay.rstrip('/')) + 1:])
          break
      if not should_include:
        continue
    else:
      # json.loads returns unicode but the recipe framework can only handle str.
      path = str(path[len(source_checkout_name):])

    path = path.rstrip('/')

    if not SHA1_RE.match(rev):
      if rev.startswith('origin/'):
        rev = rev[len('origin/'):]
      rev = api.git(
          'ls-remote', url, rev,
          stdout=api.raw_io.output_text()).stdout.split()[0]

    update_index_entries.extend(['--cacheinfo', '160000,%s,%s' % (rev, path)])

    if url.endswith('.git'):
      url = url[:-4]
    gitmodules_entries.append('[submodule "%s"]\n\tpath = %s\n\turl = %s' %
                              (path, path, url))

  return update_index_entries, gitmodules_entries


fake_src_deps = """
{
  "src/v8": {
    "url": "https://chromium.googlesource.com/v8/v8.git",
    "rev": "4ad2459561d76217c9b7aff412c5c086b491078a"
  },
  "src/buildtools": {
    "url": "https://chromium.googlesource.com/chromium/buildtools",
    "rev": "13a00f110ef910a25763346d6538b60f12845656"
  },
  "src-internal": {
    "url": "https://chrome-internal.googlesource.com/chrome/src-internal.git",
    "rev": "34b7d6a218430e7ff716b81854743a30cfbd3967"
  },
  "tooling/some_tooling_repo": {
    "url": "https://chrome-internal.googlesource.com/some_tooling_repo.git",
    "rev": "0000000000000000000000000000000000000000"
  },
  "src/": {
    "url": "https://chromium.googlesource.com/chromium/src.git",
    "rev": null
  }
}
"""

fake_deps_with_symbolic_ref = """
{
  "src/v8": {
    "url": "https://chromium.googlesource.com/v8/v8.git",
    "rev": "origin/main"
  }
}
"""

fake_deps_with_nested_dep = """
{
  "src/third_party/gsutil": {
    "url": "https://chromium.googlesource.com/external/gsutil/src.git",
    "rev": "5cba434b828da428a906c8197a23c9ae120d2636"
  },
  "src/third_party/gsutil/boto": {
    "url": "https://chromium.googlesource.com/external/boto.git",
    "rev": "98fc59a5896f4ea990a4d527548204fed8f06c64"
  }
}
"""

fake_deps_with_trailing_slash = """
{
  "src/v8/": {
    "url": "https://chromium.googlesource.com/v8/v8.git",
    "rev": "4ad2459561d76217c9b7aff412c5c086b491078a"
  }
}
"""

def GenTests(api):
  yield (api.test('first_time_running') + api.properties(
      source_repo='https://chromium.googlesource.com/chromium/src',
      target_repo='https://chromium.googlesource.com/codesearch/src_mirror') +
         api.step_data(
             'git ls-remote',
             api.raw_io.stream_output_text(
                 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
                 stream='stdout')) +
         api.step_data(
             'Process refs/heads/main.gclient evaluate DEPS',
             api.raw_io.stream_output_text(fake_src_deps, stream='stdout')))

  yield (api.test('existing_checkout_git_dir') + api.properties(
      source_repo='https://chromium.googlesource.com/chromium/src',
      target_repo='https://chromium.googlesource.com/codesearch/src_mirror') +
         api.path.dirs_exist(api.m.path.cache_dir /
                             'codesearch_update_submodules_mirror' / 'src') +
         api.step_data(
             'git ls-remote',
             api.raw_io.stream_output_text(
                 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
                 stream='stdout')) + api.step_data('git fetch', retcode=128) +
         api.expect_status('INFRA_FAILURE'))

  yield (
      api.test('existing_checkout_new_commits') + api.properties(
          source_repo='https://chromium.googlesource.com/chromium/src',
          target_repo='https://chromium.googlesource.com/codesearch/src_mirror',
          push_to_refs_cs=True) +
      api.path.dirs_exist(api.m.path.cache_dir /
                          'codesearch_update_submodules_mirror' / 'src') +
      api.step_data(
          'git ls-remote',
          api.raw_io.stream_output_text(
              'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
              stream='stdout')) +
      api.step_data(
          'Process refs/heads/main.'
          'gclient evaluate DEPS',
          api.raw_io.stream_output_text(fake_src_deps, stream='stdout')) +
      api.step_data('Process refs/heads/main.'
                    'last commit hash',
                    api.raw_io.stream_output_text('012345', stream='stdout')) +
      api.step_data(
          'Process refs/heads/main.'
          'last commit date',
          api.raw_io.stream_output_text('2023-02-15', stream='stdout')))

  yield (
      api.test('existing_checkout_latest_commit_not_by_bot') + api.properties(
          source_repo='https://chromium.googlesource.com/chromium/src',
          target_repo='https://chromium.googlesource.com/codesearch/src_mirror')
      + api.path.dirs_exist(api.m.path.cache_dir /
                            'codesearch_update_submodules_mirror' / 'src') +
      api.step_data(
          'git ls-remote',
          api.raw_io.stream_output_text(
              'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
              stream='stdout')) +
      api.step_data(
          'Process refs/heads/main.gclient evaluate DEPS',
          api.raw_io.stream_output_text(fake_src_deps, stream='stdout')))

  yield (api.test('ref_that_needs_resolving') + api.properties(
      source_repo='https://chromium.googlesource.com/chromium/src',
      target_repo='https://chromium.googlesource.com/codesearch/src_mirror') +
         api.path.dirs_exist(api.m.path.cache_dir /
                             'codesearch_update_submodules_mirror' / 'src') +
         api.step_data(
             'git ls-remote',
             api.raw_io.stream_output_text(
                 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
                 stream='stdout')) + api.step_data(
                     'Process refs/heads/main.gclient evaluate DEPS',
                     api.raw_io.stream_output_text(
                         fake_deps_with_symbolic_ref, stream='stdout')) +
         api.step_data(
             'Process refs/heads/main.git ls-remote',
             api.raw_io.stream_output_text(
                 '91c13923c1d136dc688527fa39583ef61a3277f7\trefs/heads/main',
                 stream='stdout')))

  yield (api.test('nested_deps') + api.properties(
      source_repo='https://chromium.googlesource.com/chromium/src',
      target_repo='https://chromium.googlesource.com/codesearch/src_mirror') +
         api.path.dirs_exist(api.m.path.cache_dir /
                             'codesearch_update_submodules_mirror' / 'src') +
         api.step_data(
             'git ls-remote',
             api.raw_io.stream_output_text(
                 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
                 stream='stdout')) + api.step_data(
                     'Process refs/heads/main.gclient evaluate DEPS',
                     api.raw_io.stream_output_text(
                         fake_deps_with_nested_dep, stream='stdout')))

  yield (api.test('trailing_slash') + api.properties(
      source_repo='https://chromium.googlesource.com/chromium/src',
      target_repo='https://chromium.googlesource.com/codesearch/src_mirror') +
         api.path.dirs_exist(api.m.path.cache_dir /
                             'codesearch_update_submodules_mirror' / 'src') +
         api.step_data(
             'git ls-remote',
             api.raw_io.stream_output_text(
                 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
                 stream='stdout')) + api.step_data(
                     'Process refs/heads/main.gclient evaluate DEPS',
                     api.raw_io.stream_output_text(
                         fake_deps_with_trailing_slash, stream='stdout')))

  yield (api.test('extra_submodule') + api.properties(
      source_repo='https://chromium.googlesource.com/chromium/src',
      target_repo='https://chromium.googlesource.com/codesearch/src_mirror',
      extra_submodules=['src/extra=https://extra.googlesource.com/extra']) +
         api.path.dirs_exist(api.m.path.cache_dir /
                             'codesearch_update_submodules_mirror' / 'src') +
         api.step_data(
             'git ls-remote',
             api.raw_io.stream_output_text(
                 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
                 stream='stdout')) +
         api.step_data(
             'Process refs/heads/main.gclient evaluate DEPS',
             api.raw_io.stream_output_text(fake_src_deps, stream='stdout')) +
         api.step_data(
             'Process refs/heads/main.git ls-remote',
             api.raw_io.stream_output_text(
                 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
                 stream='stdout')))

  yield (
      api.test('extra_branches') + api.properties(
          source_repo='https://chromium.googlesource.com/chromium/src',
          target_repo='https://chromium.googlesource.com/codesearch/src_mirror',
          ref_patterns=['refs/heads/main', 'refs/branch-heads/4044'],
      ) + api.step_data(
          'git ls-remote',
          api.raw_io.stream_output_text(
              'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
              stream='stdout')) +
      api.step_data(
          'git ls-remote (2)',
          api.raw_io.stream_output_text(
              'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/branch-heads/4044',
              stream='stdout')) +
      api.step_data(
          'Process refs/heads/main.gclient evaluate DEPS',
          api.raw_io.stream_output_text(fake_src_deps, stream='stdout')) +
      api.step_data(
          'Process refs/branch-heads/4044.gclient evaluate DEPS',
          api.raw_io.stream_output_text(fake_src_deps, stream='stdout')))

  yield (api.test('overlays') + api.properties(
      source_repo='https://chromium.googlesource.com/chromium/src',
      target_repo='https://chromium.googlesource.com/codesearch/src_mirror',
      overlays=['tooling']) +
         api.path.dirs_exist(api.m.path.cache_dir /
                             'codesearch_update_submodules_mirror' / 'src') +
         api.step_data(
             'git ls-remote',
             api.raw_io.stream_output_text(
                 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
                 stream='stdout')) +
         api.step_data(
             'Process refs/heads/main.gclient evaluate DEPS',
             api.raw_io.stream_output_text(fake_src_deps, stream='stdout')))

  yield (api.test('fetch_internal') + api.properties(
      source_repo='https://chromium.googlesource.com/chromium/src',
      target_repo='https://chromium.googlesource.com/codesearch/src_mirror',
      internal="true") + api.step_data(
          'git ls-remote',
          api.raw_io.stream_output_text(
              'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
              stream='stdout')) +
         api.step_data(
             'Process refs/heads/main.gclient evaluate DEPS',
             api.raw_io.stream_output_text(fake_src_deps, stream='stdout')))

  yield (api.test('with_tags_false') + api.properties(
      source_repo='https://chromium.googlesource.com/chromium/src',
      target_repo='https://chromium.googlesource.com/codesearch/src_mirror',
      internal='true',
      with_tags=False) + api.step_data(
          'git ls-remote',
          api.raw_io.stream_output_text(
              'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
              stream='stdout')) +
         api.step_data(
             'Process refs/heads/main.gclient evaluate DEPS',
             api.raw_io.stream_output_text(fake_src_deps, stream='stdout')))

  yield (api.test('with_ref_patterns') + api.properties(
      source_repo='https://chromium.googlesource.com/chromium/src',
      target_repo='https://chromium.googlesource.com/codesearch/src_mirror',
      ref_patterns=['refs/heads/main', 'refs/branch-heads/517*']) +
         api.step_data(
             'git ls-remote',
             api.raw_io.stream_output_text(
                 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main',
                 stream='stdout')) +
         api.step_data(
             'git ls-remote (2)',
             api.raw_io.stream_output_text(
                 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/branch-heads/5172\n' +
                 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/branch-heads/5173\n' +
                 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/branch-heads/5174\n',
                 stream='stdout')) +
         api.step_data(
             'Process refs/branch-heads/5172.gclient evaluate DEPS',
             api.raw_io.stream_output_text(fake_src_deps, stream='stdout')) +
         api.step_data(
             'Process refs/branch-heads/5174.gclient evaluate DEPS',
             api.raw_io.stream_output_text(fake_src_deps, stream='stdout')) +
         api.step_data(
             'Process refs/branch-heads/5173.gclient evaluate DEPS',
             api.raw_io.stream_output_text(fake_src_deps, stream='stdout')) +
         api.step_data(
             'Process refs/heads/main.gclient evaluate DEPS',
             api.raw_io.stream_output_text(fake_src_deps, stream='stdout')))
