# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from PB.go.chromium.org.luci.buildbucket.proto.common import GerritChange

DEPS = [
    'recipe_engine/archive',
    'recipe_engine/buildbucket',
    'recipe_engine/context',
    'recipe_engine/file',
    'recipe_engine/json',
    'recipe_engine/nodejs',
    'recipe_engine/path',
    'recipe_engine/platform',
    'recipe_engine/resultdb',
    'recipe_engine/step',
    'depot_tools/git',
    'depot_tools/gsutil',
    'cloudbuildhelper',
]

GAE_REPO_URL = 'https://chrome-internal.googlesource.com/infradata/gae'


def RunSteps(api):
  assert api.platform.is_linux, 'Unsupported platform, only Linux is supported.'

  # Checkout the repo with GAE deployment configs.
  api.cloudbuildhelper.do_roll(
      repo_url=GAE_REPO_URL,
      root=api.path.cache_dir / 'builder' / 'gae',
      callback=lambda gae_dir: _try_promote_staging_to_prod(api, gae_dir))


def _try_promote_staging_to_prod(api, gae_dir):
  # Check whether the staging version and stable version matches.
  channel_json_file = gae_dir / 'apps' / 'luci-milo' / 'channels.json'
  channel_json_data = api.file.read_json('read channels.json',
                                         channel_json_file)
  luci_ui_versions = channel_json_data.get('tarballs').get(
      'luci-go/luci-milo-ui')
  staging_tarball_version = luci_ui_versions.get('staging')
  stable_tarball_version = luci_ui_versions.get('stable')
  # No need to promote if the staging version is the same as the stable version.
  if staging_tarball_version == stable_tarball_version:
    return None

  # Find and load `<staging-tarball-version>.json`.
  staging_tarball_file = gae_dir / 'tarballs' / 'luci-go' / 'luci-milo-ui' / (
      staging_tarball_version + '.json')
  staging_tarball_data = api.file.read_json('read <tarball-version>.json',
                                            staging_tarball_file)

  # Checkout the luci-ui source code at the commit used to build the tarball.
  luci_go_source = staging_tarball_data.get('metadata').get('source')
  luci_go_dir = api.path.cache_dir / 'luci-go'
  api.git.checkout(
      luci_go_source.get('repo'),
      ref=luci_go_source.get('revision'),
      dir_path=luci_go_dir,
      submodules=False)

  # Download the tarball and extract the content.
  tarball_dir = api.path.cleanup_dir / 'tarball'
  tarball_file = tarball_dir / 'tarball.tar.gz'
  extract_dir = tarball_dir / 'milo'
  api.gsutil.download_url(staging_tarball_data.get('location'), tarball_file)
  api.archive.extract('extract tarball', tarball_file, extract_dir)

  # Read the desired nodejs version from <repo>/build/NODEJS_VERSION.
  version = api.file.read_text(
      'read NODEJS_VERSION',
      luci_go_dir / 'build' / 'NODEJS_VERSION',
      test_data='6.6.6\n',
  ).strip().lower()

  # Bootstrap nodejs at that version and run LUCI UI integration tests against
  # the staging tarball.
  luci_ui_dir = luci_go_dir / 'milo' / 'ui'
  tarball_dist_dir = extract_dir / 'service-ui' / 'ui' / 'dist'
  with api.nodejs(version), api.context(
      cwd=luci_ui_dir, env={'VITE_LOCAL_BASE_OUT_DIR': tarball_dist_dir}):
    api.step('npm ci', ['npm', 'ci'])
    api.step('e2e', api.resultdb.wrap(['make', 'e2e']))

  # Promote the staging version to production
  res = api.step(
      name='promote.py',
      cmd=[
          gae_dir / 'scripts' / 'promote.py',
          '--canary',
          '--stable',
          '--json',
          'luci-milo',
          # Promote the UI service only.
          '--tarballs',
          'luci-go/luci-milo-ui'
      ],
      stdout=api.json.output())

  return api.cloudbuildhelper.RollCL(
      message=res.stdout.get('commitMessage'),
      tbr=[],
      cc=res.stdout.get('cc'),
      commit=True)


def GenTests(api):
  yield (api.test(
      'basic', api.buildbucket.ci_build(),
      api.step_data(
          'read channels.json',
          api.file.read_json({
              'canaryPercent': 20,
              'tarballs': {
                  'luci-go/luci-milo-ui': {
                      'canary': '17105-0f99334',
                      'stable': '17105-0f99334',
                      'staging': '17140-f02aba0'
                  }
              }
          }),
      ),
      api.step_data(
          'read <tarball-version>.json',
          api.file.read_json({
              "location":
                  "gs://chops-public-tarballs-prod/gae/luci-go/luci-milo-ui/b99e32bb9c91c0df857db91b0b15bf6e2e6240907eed46d2242ac0c407b42572.tar.gz",
              "metadata": {
                  "date":
                      "2024-11-09T02:25:59.666653Z",
                  "links": {
                      "buildbucket":
                          "https://cr-buildbucket.appspot.com/build/8731783829329190193"
                  },
                  "source": {
                      "repo":
                          "https://chromium.googlesource.com/infra/luci/luci-go",
                      "revision":
                          "0dfb7071da8c77a0a55d1e1267fc5fc972158d15"
                  },
                  "sources": [{
                      "repository":
                          "https://chromium.googlesource.com/infra/luci/luci-go",
                      "revision":
                          "0dfb7071da8c77a0a55d1e1267fc5fc972158d15",
                      "sources": ["milo"]
                  }]
              },
              "sha256":
                  "b99e32bb9c91c0df857db91b0b15bf6e2e6240907eed46d2242ac0c407b42572",
              "version":
                  "17137-0dfb707"
          })),
      api.step_data(
          'promote.py',
          stdout=api.json.output({
              'cc': [
                  'person-1@google.com',
                  'person-2@google.com',
              ],
              'commitMessage': 'this is a commit message'
          })), api.step_data('git diff', retcode=1)))

  yield (api.test(
      'already_promoted',
      api.buildbucket.ci_build(),
      api.step_data(
          'read channels.json',
          api.file.read_json({
              'canaryPercent': 20,
              'tarballs': {
                  'luci-go/luci-milo-ui': {
                      'canary': '17140-f02aba0',
                      'stable': '17140-f02aba0',
                      'staging': '17140-f02aba0'
                  }
              }
          }),
      ),
  ))
