# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from datetime import timedelta

from PB.go.chromium.org.luci.buildbucket.proto.common import GerritChange

DEPS = [
    'recipe_engine/buildbucket',
    'recipe_engine/file',
    'recipe_engine/json',
    'recipe_engine/path',
    'recipe_engine/platform',
    'recipe_engine/raw_io',
    'recipe_engine/step',
    'recipe_engine/time',
    'depot_tools/git',
    'cloudbuildhelper',
]

GAE_REPO_URL = 'https://chrome-internal.googlesource.com/infradata/gae'

# Use the "stable" channel to track old releases.
# "stable" releases are 2 weeks behind canary releases.
STABLE_RELEASE_DELAY_WEEK = 2


def RunSteps(api):
  assert api.platform.is_linux, 'Unsupported platform, only Linux is supported.'

  # Checkout the repo with GAE deployment configs.
  api.cloudbuildhelper.do_roll(
      repo_url=GAE_REPO_URL,
      root=api.path.cache_dir / 'builder' / 'gae',
      callback=lambda gae_dir: _promote_past_canary_to_stable(api, gae_dir))


def _promote_past_canary_to_stable(api, gae_dir):
  # Check the current LUCI UI versions.
  channel_json_file_rel = api.path.join('apps', 'luci-milo', 'channels.json')
  channel_json_file_abs = gae_dir / channel_json_file_rel
  tot_channel_json = api.file.read_json('read channels.json',
                                        channel_json_file_abs)
  tot_luci_ui_versions = tot_channel_json.get('tarballs').get(
      'luci-go/luci-milo-ui')
  tot_stable_version = tot_luci_ui_versions.get('stable')
  tot_stable_version_num = int(tot_stable_version.split('-', 2)[0])
  tot_canary_version = tot_luci_ui_versions.get('canary')
  tot_canary_version_num = int(tot_canary_version.split('-', 2)[0])

  # Get the LUCI UI canary version released a few weeks ago.
  past_timestamp = (api.time.utcnow() -
                    timedelta(weeks=STABLE_RELEASE_DELAY_WEEK)).isoformat()
  past_commit = api.git(
      'rev-list',
      '-n',
      '1',
      f'--before="{past_timestamp}"',
      'HEAD',
      stdout=api.raw_io.output_text()).stdout.strip()
  past_channel_json_text = api.git.cat_file_at_commit(
      channel_json_file_rel, past_commit,
      stdout=api.raw_io.output_text()).stdout
  past_channel_json = api.json.loads(past_channel_json_text)
  past_luci_ui_versions = past_channel_json.get('tarballs').get(
      'luci-go/luci-milo-ui')
  past_canary_version = past_luci_ui_versions.get('canary')
  past_canary_version_num = int(past_canary_version.split('-', 2)[0])

  target_stable_version = past_canary_version
  target_stable_version_num = past_canary_version_num

  # Do not make the stable version newer than the canary version.
  # If the canary version released a few weeks ago was since rolled back (e.g.
  # due to security issues), we do not want to serve it as a stable version a
  # few weeks later.
  if target_stable_version_num > tot_canary_version_num:
    target_stable_version = tot_canary_version
    target_stable_version_num = tot_canary_version_num

  # Do not roll the stable version back.
  # This allows us to purge vulnerable versions from future automatic stable
  # releases by manually bumping the stable version to a version with no
  # subsequent vulnerable releases.
  if target_stable_version_num <= tot_stable_version_num:
    return None

  # Roll the current stable version to the target version.
  res = api.step(
      name='promote.py',
      cmd=[
          gae_dir / 'scripts' / 'promote.py', '--stable', '--version',
          target_stable_version, '--json', 'luci-milo', '--tarballs',
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
                      'canary': '17404-55af652',
                      'stable': '17380-5010687',
                      'staging': '17404-55af652'
                  }
              }
          }),
      ),
      api.step_data(
          'git rev-list',
          stdout=api.raw_io.output_text(
              '98015b440da77cd986cb5722ed29894d29f2cf7f')),
      api.step_data(
          'git cat-file 98015b440da77cd986cb5722ed29894d29f2cf7f:apps/luci-milo/channels.json',
          stdout=api.raw_io.output_text('''
          {
            "canaryPercent": 20,
            "tarballs": {
              "luci-go/luci-milo-ui": {
                "canary": "17387-371b1db",
                "stable": "17312-aa02ee8",
                "staging": "17387-371b1db"
              }
            }
          }
          ''')),
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
      'canary_was_rolled_back', api.buildbucket.ci_build(),
      api.step_data(
          'read channels.json',
          api.file.read_json({
              'canaryPercent': 20,
              'tarballs': {
                  'luci-go/luci-milo-ui': {
                      'canary': '17404-55af652',
                      'stable': '17380-5010687',
                      'staging': '17404-55af652'
                  }
              }
          }),
      ),
      api.step_data(
          'git rev-list',
          stdout=api.raw_io.output_text(
              '98015b440da77cd986cb5722ed29894d29f2cf7f')),
      api.step_data(
          'git cat-file 98015b440da77cd986cb5722ed29894d29f2cf7f:apps/luci-milo/channels.json',
          stdout=api.raw_io.output_text('''
          {
            "canaryPercent": 20,
            "tarballs": {
              "luci-go/luci-milo-ui": {
                "canary": "17405-eee4e15",
                "stable": "17312-aa02ee8",
                "staging": "17405-eee4e15"
              }
            }
          }
          ''')),
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
      'stable_newer_than_past_canary', api.buildbucket.ci_build(),
      api.step_data(
          'read channels.json',
          api.file.read_json({
              'canaryPercent': 20,
              'tarballs': {
                  'luci-go/luci-milo-ui': {
                      'canary': '17404-55af652',
                      'stable': '17380-5010687',
                      'staging': '17404-55af652'
                  }
              }
          }),
      ),
      api.step_data(
          'git rev-list',
          stdout=api.raw_io.output_text(
              '98015b440da77cd986cb5722ed29894d29f2cf7f')),
      api.step_data(
          'git cat-file 98015b440da77cd986cb5722ed29894d29f2cf7f:apps/luci-milo/channels.json',
          stdout=api.raw_io.output_text('''
          {
            "canaryPercent": 20,
            "tarballs": {
              "luci-go/luci-milo-ui": {
                "canary": "17312-aa02ee8",
                "stable": "17312-aa02ee8",
                "staging": "17312-aa02ee8"
              }
            }
          }
          '''))))
