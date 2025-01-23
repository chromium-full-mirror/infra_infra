# Copyright 2022 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from collections import OrderedDict
from recipe_engine import post_process
from PB.recipe_engine.result import RawResult
from google.protobuf.struct_pb2 import Struct

from PB.recipes.infra.windows_image_builder import input as input_pb
from PB.recipes.infra.windows_image_builder import actions
from PB.recipes.infra.windows_image_builder import vm
from PB.recipes.infra.windows_image_builder import drive
from PB.recipes.infra.windows_image_builder import dest
from PB.recipes.infra.windows_image_builder import sources
from PB.recipes.infra.windows_image_builder import windows_vm
from PB.recipes.infra.windows_image_builder import windows_iso
from PB.recipes.infra.windows_image_builder \
    import windows_image_builder as wib
from PB.recipes.infra.windows_image_builder \
    import offline_winpe_customization as owc
from PB.recipes.infra.windows_image_builder \
    import online_windows_customization as onwc
from PB.go.chromium.org.luci.buildbucket.proto \
  import builds_service as bs_pb2
from PB.go.chromium.org.luci.buildbucket.proto \
  import build as b_pb2
from PB.go.chromium.org.luci.buildbucket.proto \
  import common as common_pb2

from RECIPE_MODULES.infra.windows_scripts_executor \
    import test_helper as t

DEPS = [
    'depot_tools/bot_update',
    'depot_tools/gclient',
    'depot_tools/gitiles',
    'depot_tools/tryserver',
    'recipe_engine/context',
    'recipe_engine/file',
    'recipe_engine/json',
    'recipe_engine/path',
    'recipe_engine/platform',
    'recipe_engine/properties',
    'recipe_engine/proto',
    'recipe_engine/step',
    'recipe_engine/buildbucket',
    'recipe_engine/raw_io',
    'recipe_engine/runtime',
    'windows_adk',
    'windows_scripts_executor',
]

PYTHON_VERSION_COMPATIBILITY = 'PY3'

PROPERTIES = input_pb.Inputs

################################ TEST DATA ####################################

TEST_IMAGE = wib.Image(
    name='test',
    arch=wib.ARCH_X86,
    customizations=[
        wib.Customization(
            offline_winpe_customization=owc.OfflineWinPECustomization(
                name='test_cust',
                offline_customization=[
                    actions.OfflineAction(
                        name='tests',
                        actions=[
                            actions.Action(
                                add_file=actions.AddFile(
                                    name='add_psovercom',
                                    src=sources.Src(
                                        git_src=sources.GITSrc(
                                            repo='https://winimage.gsrc.com/r',
                                            src='images/PSOverCom.ps1',
                                            ref='HEAD')))),
                            actions.Action(
                                add_file=actions.AddFile(
                                    name='add_startnet',
                                    src=sources.Src(
                                        git_src=sources.GITSrc(
                                            repo='https://winimage.gsrc.com/r',
                                            src='images/startnet.cmd',
                                            ref='HEAD')))),
                        ])
                ])),
        wib.Customization(
            windows_iso_customization=windows_iso.WinISOImage(
                name='bimage',
                base_image=sources.Src(
                    cipd_src=sources.CIPDSrc(
                        package='infra_internal/labs/images/windows/10/22h2',
                        refs='latest',
                        platform='windows-amd64',
                        filename='Win10.iso')),
                copy_files=[
                    windows_iso.CopyArtifact(
                        artifact=sources.Src(
                            local_src='image(test)-cust(test_cust)-output'),
                        mount=True,
                        source='sources/boot/boot.wim',
                    )
                ],
            )),
        wib.Customization(
            online_windows_customization=onwc.OnlineWinCustomization(
                name='test_win',
                online_customizations=[
                    onwc.OnlineCustomization(
                        name='test_boot1',
                        vm_config=vm.VM(
                            qemu_vm=vm.QEMU_VM(
                                name='squidward',
                                version='latest',
                                smp='cores=8',
                                memory=8192,
                                device=['ide-cd,drive=newWin.iso'],
                                drives=[
                                    drive.Drive(
                                        name='WinXP.iso',
                                        input_src=sources.Src(
                                            local_src='image(test)-'
                                            'cust(bimage)-output'),
                                        interface='none',
                                        media='cdrom',
                                        readonly=True),
                                    drive.Drive(
                                        name='system.img',
                                        output_dests=[
                                            dest.Dest(
                                                gcs_src=sources.GCSSrc(
                                                    bucket='chrome-gce-images',
                                                    source='tests/sys.img',
                                                ))
                                        ],
                                        interface='none',
                                        media='drive',
                                        size=1234546,
                                        filesystem='fat')
                                ])),
                        win_vm_config=windows_vm.WindowsVMConfig(
                            boot_time=300, shutdown_time=300),
                    )
                ]))
    ])

TEST_ISO_IMAGE = wib.Image(
    name='test',
    arch=wib.ARCH_X86,
    customizations=[
        wib.Customization(
            windows_iso_customization=windows_iso.WinISOImage(
                name='bimage',
                base_image=sources.Src(
                    cipd_src=sources.CIPDSrc(
                        package='infra_internal/labs/images/windows/10/22h2',
                        refs='latest',
                        platform='windows-amd64',
                        filename='Win10.iso')),
                copy_files=[
                    windows_iso.CopyArtifact(
                        artifact=sources.Src(
                            local_src='image(test)-cust(test_cust)-output'),
                        mount=True,
                        source='sources/boot/boot.wim',
                    )
                ],
            )),
    ])

TESTS = {'test1.cfg': TEST_IMAGE, 'test2.cfg': TEST_ISO_IMAGE}
DIR_DATA = {
    'tests/basic': ['test1.cfg'],
    'tests/collision': ['test1.cfg', 'test2.cfg']
}


def mock_tests(config):
  if config in TESTS.keys():
    return TESTS[config]
  return None  #pragma: no cover


def mock_lsdir(path):
  if path in DIR_DATA.keys():
    return DIR_DATA[path]
  return None  #pragma: no cover


############################## TEST DATA END ##################################


def url_title(build):
  """ url_title is a helper function to display the customization
      name over the build link in schedule process.
      Returns string formatted with builder name and customization
  """
  props = build.input.properties
  return props['name']


def RunSteps(api, inputs):
  """This recipe runs image builder for a given user config."""

  configs = []
  try_job = False
  if not inputs.config_path:
    raise api.step.StepFailure("`config_path` is a required property")

  refs = 'origin/main'
  if inputs.refs:
    refs = inputs.refs
  builder_named_cache = api.path.cache_dir / 'builder'

  with api.step.nest('read user config') as c:
    # check if it is a try build
    cl_ref = api.tryserver.gerrit_change_fetch_ref
    # download the configs repo
    api.gclient.set_config('infradata_config')
    if cl_ref:
      try_job = True
      api.gclient.c.solutions[0].revision = cl_ref
    else:
      api.gclient.c.solutions[0].revision = refs
    with api.context(cwd=builder_named_cache):
      api.bot_update.ensure_checkout()
      api.gclient.runhooks()
      # split the string on '/' as luci scheduler passes a unix path and this
      # recipe is expected to run on windows ('\')
      cfg_path = builder_named_cache.joinpath('infra-data-config',
                                              *inputs.config_path.split('/'))

      # Recursively call the offline.py recipe with all configs
      cfgs = api.file.listdir(
          "Read all the configs",
          cfg_path,
          test_data=mock_lsdir(inputs.config_path))
      reqs = []
      for cfg in cfgs:
        if str(cfg).endswith('.cfg'):
          try:
            configs.append(
                api.file.read_proto(
                    name='Reading ' + inputs.config_path,
                    source=cfg,
                    msg_class=wib.Image,
                    codec='TEXTPB',
                    test_proto=mock_tests(api.path.basename(cfg))))
          except ValueError as e:  #pragma: no cover
            _, name = api.path.split(cfg)
            summary = c.step_summary_text
            summary += 'Failed to read {}: {} <br>'.format(name, e)
            c.step_summary_text = summary

  if not configs:
    # If there are no config files, exit
    return  #pragma: no cover

  # initialize the recipe_module
  api.windows_scripts_executor.init(try_job)

  # collect all the customizations from all the configs
  custs = []
  for config in configs:
    custs.extend(api.windows_scripts_executor.init_customizations(config))

  if try_job:
    api.windows_scripts_executor.trim_uploads(custs)

  # Get all the inputs required. This will be used to determine if we have
  # to cache any images in online customization
  inputs = []
  for cust in custs:
    for ip in cust.inputs:
      if ip.WhichOneof('src') == 'local_src':
        inputs.append(ip.local_src)

  # process all the customizations (pin artifacts, generate hash)
  api.windows_scripts_executor.process_customizations(custs, {}, inputs)

  # Dict mapping the customization object key to list of customizations
  # corresponding to a customization. This ensures that we don't miss
  # executing a customization if its an exact copy of another. We only
  # execute one. But show links to both
  key_cust_map = OrderedDict()
  for cust in custs:
    if cust.get_key() in key_cust_map:
      raise Exception('{} and {} are duplicate customizations'.format(
          cust.id, key_cust_map[cust.get_key()].id))
    # Update the key map
    key_cust_map[cust.get_key()] = cust

  # triggered_custs contains the list of cust keys that have been triggered
  triggered_custs = set()
  # list of cust keys that failed to build
  failed_custs = set()
  # list of cust keys that had infra failure
  infra_failed_custs = set()
  # list of cust keys that were cancelled
  cancelled_custs = set()
  # list of cust keys that were built
  built_custs = set()
  # mapping from build_id to keys
  build_id_keys = {}
  # Summary text for the build
  summary = ''
  with api.step.nest('Execute customizations') as e:
    # Get all the images that can be executed at this time
    executions = api.windows_scripts_executor.get_executable_configs(custs)
    while executions:
      # list of builds to wait for
      blds = []
      # execute the customizations that need to be executed
      for builder, images in executions.items():
        for img, key_list in images:
          # collect tags to add to the build request
          tags = {}
          for key in key_list:
            cust = key_cust_map[key]
            tags[cust.id] = key
          # convert image to json config
          props = api.json.loads(api.proto.encode(img, 'JSONPB'))
          req = api.buildbucket.schedule_request(
              builder=builder,
              properties=props,
              tags=api.buildbucket.tags(**tags),
          )
          triggered_custs = triggered_custs.union(key_list)

          # schedule all the builds
          builds = api.buildbucket.schedule([req], url_title_fn=url_title)
          blds.append(builds[0].id)
          # Record all the keys associated with the build id
          build_id_keys[builds[0].id] = key_list
          for key in key_list:
            cust = key_cust_map[key]
            # Add a link to the cust build
            e.links['{}/{}'.format(
                cust.id,
                key)] = api.buildbucket.build_url(build_id=builds[0].id)
      # wait for all the triggered builds to complete
      build_map = api.buildbucket.collect_builds(
          [i for i in blds],
          step_name='waiting for builds to complete',
          timeout=21600)
      for build_id, build in build_map.items():
        # Collect all 4 terminal build status
        if build.status == common_pb2.Status.FAILURE:
          failed_custs = failed_custs.union(build_id_keys[build_id])
        if build.status == common_pb2.Status.CANCELED:
          cancelled_custs = cancelled_custs.union(build_id_keys[build_id])
        if build.status == common_pb2.Status.INFRA_FAILURE:
          infra_failed_custs = infra_failed_custs.union(build_id_keys[build_id])
        if build.status == common_pb2.Status.SUCCESS:
          built_custs = built_custs.union(build_id_keys[build_id])

      # Avoid triggering the builds again. (In case they failed)
      rcusts = [cust for cust in custs if cust.get_key() not in triggered_custs]
      # generate the new set of images that can be built
      executions = api.windows_scripts_executor.get_executable_configs(rcusts)

    summary = 'Summary:<br>'
    if failed_custs:
      summary += 'Failed:<br>'
      for cust in custs:
        if cust.get_key() in failed_custs:
          summary += '{}/{}<br>'.format(cust.id, cust.get_key())
    if infra_failed_custs:
      summary += 'InfraFailure:<br>'
      for cust in custs:
        if cust.get_key() in infra_failed_custs:
          summary += '{}/{}<br>'.format(cust.id, cust.get_key())
    if cancelled_custs:
      summary += 'Canceled:<br>'
      for cust in custs:
        if cust.get_key() in cancelled_custs:
          summary += '{}/{}<br>'.format(cust.id, cust.get_key())
    if built_custs:
      summary += 'Built:<br>'
      for cust in custs:
        if cust.get_key() in built_custs:
          summary += '{}/{}<br>'.format(cust.id, cust.get_key())
    not_built = set()
    for cust in custs:
      if cust.get_key() not in triggered_custs:
        not_built.add(cust.get_key())
    if not_built:
      summary += 'Did not build:<br>'
      for cust in custs:
        if cust.get_key() in not_built:
          summary += '{}/{}<br>'.format(cust.id, cust.get_key())
    e.step_summary_text = summary

  if len(summary) > 4000:  # pragma: nocover
    # Truncate the summary to 4000 bytes. As it fails otherwise
    summary = summary[:3967] + '...'
  if failed_custs or infra_failed_custs or cancelled_custs:
    # looks like we failed a few builds
    return RawResult(status=common_pb2.FAILURE, summary_markdown=summary)
  else:
    # looks like everything executed properly, return result
    return RawResult(status=common_pb2.SUCCESS, summary_markdown=summary)


def GenTests(api):

  key_wim = '0ba325f4cf5356b9864719365a807f2c9d48bf882d333149cebd9d1ec0b64e7b'
  key_win = '0f796362b84871b7a0d65e9c3f3d00685614441a3490f64fb4b2a391b4fb9fc4'
  key_iso = '2cb3344a7ae9c8e2772563ad8244a1bd99062f629d7c50ecc48e3d0e32974d7d'
  system = 'boot(test_boot1)-drive(system.img)-output.zip'
  image = 'test'
  cust = 'test_cust'


  # Mock schedule requests batch response for Wim builder
  prop_wim = b_pb2.Build.Input()
  prop_wim.properties['name'] = image
  prop_wim.properties['customizations'] = [{
      'offline_winpe_customization': {
          'name': 'test_cust'
      },
  }]

  # Mock schedule requests batch response for windows builder
  prop_win = b_pb2.Build.Input()
  prop_win.properties['name'] = image
  prop_win.properties['customizations'] = [{
      'online_windows_customization': {
          'name': 'test_win',
      },
      'windows_iso_customization': {
          'name': 'bimage',
      }
  }]
  BATCH_RESPONSE_WIM = bs_pb2.BatchResponse(responses=[
      dict(
          schedule_build=dict(
              builder=dict(builder='wib-wim-customization'),
              input=prop_wim,
              id=1234567890123456789)),
  ])

  BATCH_RESPONSE_WIN = bs_pb2.BatchResponse(responses=[
      dict(
          schedule_build=dict(
              builder=dict(builder='wib-windows-customization'),
              input=prop_win,
              id=9016911228971028736,
          )),
  ])

  def MOCK_CUST_OUTPUT(api, file, success=True):
    retcode = 1
    if success:
      retcode = 0
    url = 'gs://chrome-gce-images/{}'.format(file)
    return api.step_data(
        'Execute customizations.gsutil stat {}'.format(url),
        api.raw_io.stream_output(t._gcs_stat.format(url, url)),
        retcode=retcode,
    )

  # Test the happy path for the scheduler. We give scheduler the TEST_IMAGE
  # as input. As there are 3 customizations in that image with one dependent on
  # the other. It is expected that the scheduler will schedule the WinPE builder
  # first (wib-wim-customization) followed by the Windows customization
  # builder (for the remaining 2 customizations).
  yield (
      api.test('basic_scheduled', api.platform('win', 64)) + api.properties(
          input_pb.Inputs(config_path="tests/basic", refs='origin/main')) +
      api.properties.environ(input_pb.EnvProperties(MAX_CUST_BATCH_SIZE="5")) +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/PSOverCom.ps1', 'HEAD') +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/startnet.cmd', 'HEAD') +
      # Mock the check for output existence. Twice for wim (as output of
      # test_cust and input for bimage), twice for iso and once for system.img
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip'.format(key_wim), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ONLINE-CACHE/{}-{}'.format(key_win, system),
                       False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (2)'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip (2)'.format(key_wim), False) +
      # mock schedule output to test builds scheduled
      api.buildbucket.simulated_schedule_output(
          BATCH_RESPONSE_WIM,
          step_name='Execute customizations.buildbucket.schedule') +
      # mock collecting the build status
      api.buildbucket.simulated_collect_output(
          [
              api.buildbucket.ci_build_message(
                  build_id=1234567890123456789, status='SUCCESS'),
          ],
          step_name='Execute customizations.waiting for builds to complete') +
      # mock wim output check to show it exists. (wim build was successful)
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip (3)'.format(key_wim), True) +
      # mock check for iso and img. Show it doesn't exist.
      MOCK_CUST_OUTPUT(api, 'WIB-ONLINE-CACHE/{}-{} (2)'.format(
          key_win, system), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (3)'.format(key_iso), False) +
      # mock the windows customization schedule
      api.buildbucket.simulated_schedule_output(
          BATCH_RESPONSE_WIN,
          step_name='Execute customizations.buildbucket.schedule (2)') +
      api.buildbucket.simulated_collect_output(
          [
              api.buildbucket.ci_build_message(
                  build_id=9016911228971028736, status='SUCCESS'),
          ],
          step_name='Execute customizations.waiting for builds to complete (2)')
      + api.post_process(post_process.StatusSuccess) +
      api.post_process(post_process.DropExpectation))

  yield (
      api.test(
          'basic_scheduled_try_job',
          api.buildbucket.try_build(
              'infradata-config',
              'linux',
              git_repo='https://chromium.googlesource.com/chromium/src',
              change_number=91827,
              patch_set=1), api.platform('win', 64)) +
      api.properties(
          input_pb.Inputs(config_path="tests/basic", refs='origin/main')) +
      api.properties.environ(input_pb.EnvProperties(MAX_CUST_BATCH_SIZE="5")) +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/PSOverCom.ps1', 'HEAD') +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/startnet.cmd', 'HEAD') +
      # Mock the check for output existence. Twice for wim (as output of
      # test_cust and input for bimage), twice for iso and once for system.img
      MOCK_CUST_OUTPUT(api, 'WIB-WIM-TRY/{}.zip'.format(key_wim), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO-TRY/{}.iso'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO-TRY/{}.iso (2)'.format(key_iso), True) +
      MOCK_CUST_OUTPUT(api, 'WIB-WIM-TRY/{}.zip (2)'.format(key_wim), False) +
      # mock schedule output to test builds scheduled
      api.buildbucket.simulated_schedule_output(
          BATCH_RESPONSE_WIM,
          step_name='Execute customizations.buildbucket.schedule') +
      # mock collecting the build status
      api.buildbucket.simulated_collect_output(
          [
              api.buildbucket.ci_build_message(
                  build_id=1234567890123456789, status='SUCCESS'),
          ],
          step_name='Execute customizations.waiting for builds to complete') +
      api.post_process(post_process.StatusSuccess) +
      api.post_process(post_process.DropExpectation))

  # Test failure on one of the cust. This should fail the recipe
  yield (
      api.test(
          'basic_partial_failure', api.platform('win', 64), status="FAILURE") +
      api.properties.environ(input_pb.EnvProperties(MAX_CUST_BATCH_SIZE="5")) +
      api.properties(input_pb.Inputs(config_path="tests/basic")) +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/PSOverCom.ps1', 'HEAD') +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/startnet.cmd', 'HEAD') +
      # Mock the check for output existence. Twice for wim (as output of
      # test_cust and input for bimage), twice for iso and once for system.img
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip'.format(key_wim), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ONLINE-CACHE/{}-{}'.format(key_win, system),
                       False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (2)'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip (2)'.format(key_wim), False) +
      # mock schedule output to test builds scheduled
      api.buildbucket.simulated_schedule_output(
          BATCH_RESPONSE_WIM,
          step_name='Execute customizations.buildbucket.schedule') +
      # mock collecting the build status
      api.buildbucket.simulated_collect_output(
          [
              api.buildbucket.ci_build_message(
                  build_id=1234567890123456789, status='SUCCESS'),
          ],
          step_name='Execute customizations.waiting for builds to complete') +
      # mock wim output check to show it exists. (wim build was successful)
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip (3)'.format(key_wim), True) +
      # mock check for iso and img. Show it doesn't exist.
      MOCK_CUST_OUTPUT(api, 'WIB-ONLINE-CACHE/{}-{} (2)'.format(
          key_win, system), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (3)'.format(key_iso), False) +
      # mock the windows customization schedule
      api.buildbucket.simulated_schedule_output(
          BATCH_RESPONSE_WIN,
          step_name='Execute customizations.buildbucket.schedule (2)') +
      api.buildbucket.simulated_collect_output(
          [
              api.buildbucket.ci_build_message(
                  build_id=9016911228971028736, status='FAILURE'),
          ],
          step_name='Execute customizations.waiting for builds to complete (2)')
      # img file doesn't exist as it failed to build
      + api.post_process(post_process.StatusFailure) +
      api.post_process(post_process.DropExpectation))

  # Test builds not scheduled. If all the outputs exist, we don't need to
  # schedule a build.
  yield (
      api.test('basic_no_scheduled', api.platform('win', 64)) +
      api.properties(input_pb.Inputs(config_path="tests/basic")) +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/PSOverCom.ps1', 'HEAD') +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/startnet.cmd', 'HEAD') +
      # mock all three outputs as exists
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip'.format(key_wim), True) +
      MOCK_CUST_OUTPUT(api, 'WIB-ONLINE-CACHE/{}-{}'.format(key_win, system),
                       True) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso'.format(key_iso), True) +
      api.post_process(post_process.StatusSuccess) +
      api.post_process(post_process.DropExpectation))

  # Test failure due to duplicate customizations. TEST_ISO_IMAGE with TEST_IMAGE
  # repeats a customization.
  yield (
      api.test('basic_failure', api.platform('linux', 64)) +
      api.properties(input_pb.Inputs(config_path="tests/collision")) +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/PSOverCom.ps1', 'HEAD') +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/startnet.cmd', 'HEAD') +
      # expect exception as a customization was redefined
      api.expect_exception('Exception') +
      api.post_process(post_process.StatusException) +
      api.post_process(post_process.DropExpectation))

  # Test failure of a build that was scheduled.
  yield (
      api.test(
          'basic_scheduled_failure', api.platform('win', 64), status="FAILURE")
      + api.properties(input_pb.Inputs(config_path="tests/basic")) +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/PSOverCom.ps1', 'HEAD') +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/startnet.cmd', 'HEAD') +
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip'.format(key_wim), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ONLINE-CACHE/{}-{}'.format(key_win, system),
                       False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (2)'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (3)'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip (2)'.format(key_wim), False) +
      # mock schedule output to test builds scheduled state
      api.buildbucket.simulated_schedule_output(
          BATCH_RESPONSE_WIM,
          step_name='Execute customizations.buildbucket.schedule') +
      api.buildbucket.simulated_collect_output(
          [
              api.buildbucket.ci_build_message(
                  build_id=1234567890123456789, status='FAILURE'),
          ],
          step_name='Execute customizations.waiting for builds to complete') +
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip (3)'.format(key_wim), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ONLINE-CACHE/{}-{} (2)'.format(
          key_win, system), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (4)'.format(key_iso), False) +
      api.post_process(post_process.StatusFailure) +
      api.post_process(post_process.DropExpectation))

  # Test cancellation of a build that was scheduled.
  yield (
      api.test(
          'basic_scheduled_cancellation',
          api.platform('win', 64),
          status="FAILURE") +
      api.properties(input_pb.Inputs(config_path="tests/basic")) +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/PSOverCom.ps1', 'HEAD') +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/startnet.cmd', 'HEAD') +
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip'.format(key_wim), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ONLINE-CACHE/{}-{}'.format(key_win, system),
                       False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (2)'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (3)'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip (2)'.format(key_wim), False) +
      # mock schedule output to test builds scheduled state
      api.buildbucket.simulated_schedule_output(
          BATCH_RESPONSE_WIM,
          step_name='Execute customizations.buildbucket.schedule') +
      api.buildbucket.simulated_collect_output(
          [
              api.buildbucket.ci_build_message(
                  build_id=1234567890123456789, status='CANCELED'),
          ],
          step_name='Execute customizations.waiting for builds to complete') +
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip (3)'.format(key_wim), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ONLINE-CACHE/{}-{} (2)'.format(
          key_win, system), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (4)'.format(key_iso), False) +
      api.post_process(post_process.StatusFailure) +
      api.post_process(post_process.DropExpectation))

  # Test failure of a build that was scheduled.
  yield (
      api.test(
          'basic_scheduled_infra_failure',
          api.platform('win', 64),
          status="FAILURE") +
      api.properties(input_pb.Inputs(config_path="tests/basic")) +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/PSOverCom.ps1', 'HEAD') +
      t.GIT_PIN_FILE(api, 'test_cust', 'HEAD', 'images/startnet.cmd', 'HEAD') +
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip'.format(key_wim), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ONLINE-CACHE/{}-{}'.format(key_win, system),
                       False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (2)'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip (2)'.format(key_wim), False) +
      # mock schedule output to test builds scheduled state
      api.buildbucket.simulated_schedule_output(
          BATCH_RESPONSE_WIM,
          step_name='Execute customizations.buildbucket.schedule') +
      api.buildbucket.simulated_collect_output(
          [
              api.buildbucket.ci_build_message(
                  build_id=1234567890123456789, status='INFRA_FAILURE'),
          ],
          step_name='Execute customizations.waiting for builds to complete') +
      MOCK_CUST_OUTPUT(api, 'WIB-WIM/{}.zip (3)'.format(key_wim), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ONLINE-CACHE/{}-{} (2)'.format(
          key_win, system), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (3)'.format(key_iso), False) +
      MOCK_CUST_OUTPUT(api, 'WIB-ISO/{}.iso (4)'.format(key_iso), False) +
      api.post_process(post_process.StatusFailure) +
      api.post_process(post_process.DropExpectation))

  # test failure when run without a config file path.
  yield (api.test(
      'run_without_config_path', api.platform('win', 64), status="FAILURE") +
         api.properties(input_pb.Inputs(config_path="",),) +
         api.post_process(post_process.StatusFailure) +
         api.post_process(post_process.DropExpectation))
