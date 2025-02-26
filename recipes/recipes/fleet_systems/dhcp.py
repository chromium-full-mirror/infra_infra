# Copyright 2021 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Test chrome-golo repo DHCP configs using dhcpd binaries via docker."""

from recipe_engine import post_process

PYTHON_VERSION_COMPATIBILITY = 'PY3'

DEPS = [
    'depot_tools/bot_update',
    'depot_tools/gclient',
    'depot_tools/tryserver',
    'infra/docker',
    'recipe_engine/buildbucket',
    'recipe_engine/file',
    'recipe_engine/json',
    'recipe_engine/path',
    'recipe_engine/platform',
    'recipe_engine/properties',
    'recipe_engine/raw_io',
    'recipe_engine/step',
]

_IMAGE_TEMPLATE = 'fleet_systems/dhcp/%s:latest'

# These image versions align with the docker image repository here:
# https://console.cloud.google.com/gcr/images/chops-public-images-prod/GLOBAL/fleet_systems/dhcp
#
# The images are generated from configs here:
# https://chromium.googlesource.com/infra/infra/+/main/build/images/daily/fleet_systems/dhcp

_ZONE_OS_MAP_FILE = 'services/dhcpd/zone_os_map.json'


def _GetZonesToTest(api, zone_os_map):
  """Iterate over files in the CL to determine which zones need testing."""
  change_repo_url = api.m.tryserver.gerrit_change_repo_url
  dhcp_dirs = [f'{directory}/dhcpd' for directory in ['configs', 'services']]
  patch_root = api.gclient.get_gerrit_patch_root()
  zones_to_test = set()

  assert patch_root, f'local path is not configured for {change_repo_url}'

  for f in api.m.tryserver.get_files_affected_by_patch(patch_root):
    for zone in zone_os_map:
      if any([f'{dhcp_dir}/{zone}/' in f for dhcp_dir in dhcp_dirs]):
        zones_to_test.add(zone)
  return zones_to_test


def _PullDockerImages(api, oses_by_zone):
  """Pull docker images needed for testing."""
  api.docker.login(server='gcr.io', project='chops-public-images-prod')

  # Flatten the lists of oses.
  os_versions = set()
  for oses in oses_by_zone.values():
    os_versions.update(oses)

  for os_version in sorted(os_versions):
    image = _IMAGE_TEMPLATE % os_version
    try:
      api.docker.pull(image)
    except api.step.StepFailure:
      raise api.step.InfraFailure(
          f'Image {image} does not exist in the container registry.')


def RunSteps(api):
  oses_by_zone = {}

  assert api.platform.is_linux, 'Unsupported platform, only Linux is supported.'
  api.docker.ensure_installed()

  api.gclient.set_config('chrome_golo')
  api.bot_update.ensure_checkout()
  api.gclient.runhooks()

  # Read a file in the repo that defines zones and their dhcp servers.
  zone_os_map = api.file.read_json(f'read {_ZONE_OS_MAP_FILE}',
                                   api.path.checkout_dir / _ZONE_OS_MAP_FILE)

  zones_to_test = _GetZonesToTest(api, zone_os_map)
  if not zones_to_test:
    api.step.empty('CL does not contain DHCP changes')
    return

  for zone in zones_to_test:
    oses_by_zone[zone] = zone_os_map[zone]

  _PullDockerImages(api, oses_by_zone)

  # Test each zone/os combination as necessary.
  for zone, oses in sorted(oses_by_zone.items()):
    for os in sorted(oses):
      api.docker.run(
          image=_IMAGE_TEMPLATE % os,
          step_name=f'DHCP config test for {zone} on {os}',
          cmd_args=[zone],
          dir_mapping=[(api.path.checkout_dir, '/src')])


def GenTests(api):
  no_image_for_os_version = '420.04'
  no_image_for_zone = 'test_zone_image_missing'
  no_oses_for_zone = 'test_zone_no_oses'
  test_zone = 'test_zone'
  test_zone_os_versions = ['22.04', '24.04']
  zone_not_in_zone_os_map = 'missing_test_zone'
  zone_os_map = {
      test_zone: test_zone_os_versions,
      no_oses_for_zone: [],
      no_image_for_zone: [no_image_for_os_version],
  }

  def changed_files(test_zone):
    test_file = f'services/dhcpd/{test_zone}/foo'
    t = api.override_step_data(
        'git diff to analyze patch', stdout=api.raw_io.output(test_file))
    t += api.path.exists(
        api.path.checkout_dir.joinpath('chrome_golo', test_file))
    return t

  yield api.test(
      'chrome_golo_dhcp',
      api.properties(),
      api.buildbucket.try_build(),
      changed_files(test_zone),
      api.override_step_data(f'read {_ZONE_OS_MAP_FILE}',
                             api.file.read_json(zone_os_map)),
      api.override_step_data('docker pull %s' % _IMAGE_TEMPLATE %
                             test_zone_os_versions[0]),
      api.override_step_data('docker pull %s' % _IMAGE_TEMPLATE %
                             test_zone_os_versions[1]),
      api.override_step_data(
          f'DHCP config test for {test_zone} on {test_zone_os_versions[0]}'),
      api.override_step_data(
          f'DHCP config test for {test_zone} on {test_zone_os_versions[1]}'),
      api.post_process(post_process.StatusSuccess),
      api.post_process(post_process.DropExpectation),
  )

  yield api.test(
      'chrome_golo_dhcp_missing_compatible_image_version',
      api.properties(),
      api.buildbucket.try_build(),
      changed_files(no_image_for_zone),
      api.override_step_data(f'read {_ZONE_OS_MAP_FILE}',
                             api.file.read_json(zone_os_map)),
      api.override_step_data(
          'docker pull %s' % _IMAGE_TEMPLATE % no_image_for_os_version,
          retcode=1),
      api.expect_status('INFRA_FAILURE'),
      api.post_process(post_process.StatusException),
      api.post_process(post_process.DropExpectation),
  )

  yield api.test(
      'chrome_golo_dhcp_no_dhcp_files_changed',
      api.properties(),
      api.buildbucket.try_build(),
      changed_files('not_a_dhcp_change'),
      api.post_process(post_process.StatusSuccess),
      api.post_process(post_process.DropExpectation),
  )

  yield api.test(
      'chrome_golo_dhcp_no_oses_defined_for_zone',
      api.properties(),
      api.buildbucket.try_build(),
      changed_files(no_oses_for_zone),
      api.post_process(post_process.StatusSuccess),
      api.post_process(post_process.DropExpectation),
  )

  yield api.test(
      'chrome_golo_dhcp_zone_not_in_zone_os_map',
      api.properties(),
      api.buildbucket.try_build(),
      changed_files(zone_not_in_zone_os_map),
      api.post_process(post_process.StatusSuccess),
      api.post_process(post_process.DropExpectation),
  )
