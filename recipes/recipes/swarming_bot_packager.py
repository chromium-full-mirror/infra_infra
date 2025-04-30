# Copyright 2024 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Uploads Swarming bot code as a CIPD package."""

DEPS = [
    'depot_tools/git',
    'recipe_engine/buildbucket',
    'recipe_engine/cipd',
    'recipe_engine/path',
    'recipe_engine/step',
]

# Note the package is cross-platform and doesn't need a platform suffix.
SWARMING_BOT_CIPD_PACKAGE = 'infra/luci/swarming_bot'

LUCI_PY = 'https://chromium.googlesource.com/infra/luci/luci-py'


def RunSteps(api):
  # Checkout the luci-py repository.
  checkout = api.path.cache_dir.joinpath('builder', 'repo')
  revision = api.git.checkout(
      url=LUCI_PY,
      ref=api.buildbucket.gitiles_commit.id or 'refs/heads/main',
      dir_path=checkout,
      submodules=True)

  # Run the script that stages Swarming bot files into a directory.
  staged_bot_dir = api.path.mkdtemp('swarming_bot')
  api.step('bot_archive.py', [
      'python3',
      checkout.joinpath('appengine', 'swarming', 'tools', 'bot_archive.py'),
      '--output-dir',
      staged_bot_dir,
  ])

  # Upload staged files as a Swarming bot CIPD package.
  pkg = api.cipd.PackageDefinition(
      package_name=SWARMING_BOT_CIPD_PACKAGE,
      package_root=staged_bot_dir,
      install_mode='copy',
  )
  pkg.add_dir(staged_bot_dir)
  api.cipd.create_from_pkg(
      pkg,
      refs=['latest'],
      tags={
          'git_revision':
              revision,
          'git_repository':
              LUCI_PY,
          'luci_build':
              '%s/%s/%d' % (
                  api.buildbucket.build.builder.bucket,
                  api.buildbucket.build.builder.builder,
                  api.buildbucket.build.number,
              ),
      },
  )


def GenTests(api):
  yield (api.test('works') + api.buildbucket.ci_build(
      'project', 'bucket', 'builder', git_repo=LUCI_PY, build_number=123))
