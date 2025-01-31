# Copyright 2021 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from PB.recipes.infra.windows_image_builder import (offline_winpe_customization
                                                    as winpe)
from PB.recipes.infra.windows_image_builder import actions
from PB.recipes.infra.windows_image_builder import windows_image_builder as wib
from PB.recipes.infra.windows_image_builder import sources
from PB.recipes.infra.windows_image_builder import windows_iso as winiso
from PB.recipes.infra.windows_image_builder import vm
from PB.recipes.infra.windows_image_builder import drive
from PB.recipes.infra.windows_image_builder import (online_windows_customization
                                                    as owc)

from recipe_engine.post_process import (
    DropExpectation,
    StatusFailure,
    StatusSuccess,
    StepCommandContains,
    StepCommandRE,
)

from textwrap import dedent

#    Step data mock methods. Use these to mock the step outputs

# _gcs_stat is the mock output of gsutil stat command
_gcs_stat = """
{}:
    Creation time:          Tue, 12 Oct 2021 00:32:06 GMT
    Update time:            Tue, 12 Oct 2021 00:32:06 GMT
    Storage class:          STANDARD
    Content-Length:         658955236
    Content-Type:           application/octet-stream
    Metadata:
        orig:               {}
    Hash (crc32c):          oaYUgQ==
    Hash (md5):             +W9+CqZbFtYTZrUrDPltMw==
    ETag:                   CJOHnM3Pw/MCEAE=
    Generation:             1633998726431635
    Metageneration:         1
"""


def NEST(*args):
  """ NEST generates nested names for steps """
  return '.'.join(args)


def NEST_CONFIG_STEP(image):
  """ generate config step name for nesting """
  return 'execute config {}'.format(image)


def NEST_WINDOWS_ISO_CUSTOMIZATION_STEP(customization):
  """ NEST_WINDOWS_ISO_CUSTOMIZATION_STEP returns step name for the same"""
  return 'Windows iso customization {}'.format(customization)


def NEST_ONLINE_WINDOWS_CUSTOMIZATION_STEP(customization):
  """ NEST_ONLINE_WINDOWS_CUSTOMIZATION_STEP returns step name for the same"""
  return 'Execute online windows customization {}'.format(customization)


def NEST_ONLINE_CUSTOMIZATION_STEP(on_cust):
  """ NEST_ONLINE_CUSTOMIZATION_STEP returns step name for the same."""
  return 'Execute online customization {}'.format(on_cust)


def NEST_ONLINE_CUSTOMIZATION_DEINIT_STEP(on_cust):
  """ NEST_ONLINE_CUSTOMIZATION_DEINIT_STEP returns step name for the same."""
  return 'Deinit online customization {}'.format(on_cust)


def NEST_ONLINE_ACTION_STEP(oa):
  """ NEST_ONLINE_ACTION_STEP returns step name for the same."""
  return 'Execute online action {}'.format(oa)


def NEST_WINPE_CUSTOMIZATION_STEP(customization):
  """ generate winpe customization step name for nesting """
  return 'offline winpe customization {}'.format(customization)


def NEST_WINPE_INIT_STEP(arch, customization):
  """ generate winpe init step nesting names """
  return 'Init WinPE image modification {}'.format(
      arch) + ' in [CLEANUP]\\{}\\workdir'.format(customization)


def NEST_WINPE_DEINIT_STEP():
  """ generate winpe deinit step nesting names """
  return 'Deinit WinPE image modification'


def NEST_PROCESS_CUST():
  """ generate process customization header"""
  return 'Process the customizations'


def NEST_PIN_SRCS(cust):
  """ generate Pin Src step nesting name """
  return 'Pin resources from {}'.format(cust)


def NEST_DOWNLOAD_ALL_SRC(cust):
  """ Download all available packages step name"""
  return 'Download resources for {}'.format(cust)


def NEST_UPLOAD_CUST_OUTPUT(cust):
  """ Upload all gcs artifacts step name"""
  return 'Upload the output of {}'.format(cust)


def NEST_BOOT_VM(vm_name):
  """ NEST_BOOT_VM returns step name for the same."""
  return 'Boot {}'.format(vm_name)


def json_res(api, success=True, err_msg='Failed step'):
  """ generate a api.json object to mock outputs """
  if success:
    return api.json.output({'results': {'Success': success,}})
  return api.json.output({
      'results': {
          'Success': success,
          'Command': 'powershell',
          'ErrorInfo': {
              'Message': err_msg,
          },
      }
  })


def pwsh_json_res(
    api,
    output,
    error,
    logs,
    success=True,
    retcode=0,
):
  """ generate a api.json object to mock outputs """
  return api.raw_io.output(
      api.json.dumps({
          'Success': success,
          'Output': output,
          'Logs': logs,
          'Error': error,
          'PWD': 'C:\\Users\\Spongebob\\Documents',
          'RetCode': retcode,
      }))


def MOCK_WPE_INIT_DEINIT_SUCCESS(api, key, arch, image, customization):
  """ mock all the winpe init and deinit steps """
  return GEN_WPE_MEDIA(api, arch, image, customization) + \
        MOUNT_WIM(api, arch, image, customization) + \
        UMOUNT_WIM(api, image, customization) + \
        DEINIT_WIM_ADD_CFG_TO_ROOT(api, key, image, customization) + \
        CHECK_UMOUNT_WIM(api, image, customization)


def MOCK_WPE_INIT_DEINIT_FAILURE(api, arch, image, customization):
  """ mock all the winpe init and deinit steps on an action failure """
  return  GEN_WPE_MEDIA(api, arch, image, customization) + \
         MOUNT_WIM(api, arch, image, customization) + \
         UMOUNT_WIM( api, image, customization) + \
         CHECK_UMOUNT_WIM(api, image, customization, save=False)


def MOCK_CUST_IMG_WPE_INIT_DEINIT_SUCCESS(api, key, arch, image, customization):
  """ mock all the winpe init and deinit steps """
  return  MOUNT_WIM(api, arch, image, customization) + \
          UMOUNT_WIM(api, image, customization) + \
          DEINIT_WIM_ADD_CFG_TO_ROOT(api, key, image, customization) + \
          CHECK_UMOUNT_WIM(api, image, customization)


def MOCK_CUST_OUTPUT(api, url, success=True):
  retcode = 1
  if success:
    retcode = 0  #pragma: nocover
  return api.step_data(
      'gsutil stat {}'.format(url),
      api.raw_io.stream_output(_gcs_stat.format(url, url)),
      retcode=retcode,
  )


def GEN_WPE_MEDIA(api, arch, image, customization, success=True):
  """ Mock winpe media generation step """
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(customization),
          NEST_WINPE_INIT_STEP(arch, customization),
          'PowerShell> Gen WinPE media for {}'.format(arch)),
      stdout=json_res(api, success))


def MOUNT_WIM(api, arch, image, customization, success=True):
  """ mock mount winpe wim step """
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(customization),
          NEST_WINPE_INIT_STEP(arch, customization),
          'PowerShell> Mount wim to [CLEANUP]\\{}\\workdir\\mount'.format(
              customization)),
      stdout=json_res(api, success))


def UMOUNT_WIM(api, image, customization, success=True):
  """ mock unmount winpe wim step """
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(customization),
          NEST_WINPE_DEINIT_STEP(),
          'PowerShell> Unmount wim at [CLEANUP]\\{}\\workdir\\mount'.format(
              customization)),
      stdout=json_res(api, success))


def DEINIT_WIM_ADD_CFG_TO_ROOT(api, key, image, customization, success=True):
  """ mock add cfg to root step in wpe deinit """
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(customization),
          NEST_WINPE_DEINIT_STEP(),
          'PowerShell> Add cfg [CLEANUP]\\configs\\{}.cfg'.format(key)),
      stdout=json_res(api, success))


def GIT_PIN_FILE(api, cust, refs, path, data, success=True):
  """ mock git pin file step """
  retcode = 1
  if success:
    retcode = 0
  return api.step_data(
      NEST(
          NEST_PROCESS_CUST(),
          NEST_PIN_SRCS(cust),
          'gitiles log: ' + '{}/{}'.format(refs, path),
      ),
      api.gitiles.make_log_test_data(data, n=3 if data else 0),
      retcode=retcode,
  )


def GIT_DOWNLOAD_FILE(api, cust, repo, refs, path, success=True):
  """ mock git checkout commit step """
  retcode = 1
  if success:  # pragma: nocover
    retcode = 0
  return api.step_data(
      NEST(
          NEST_DOWNLOAD_ALL_SRC(cust),
          'Download {}/+/{}/{}'.format(repo, refs, path),
          'git checkout ({})'.format(path),
      ),
      retcode=retcode,
  )


def GCS_PIN_FILE(api, cust, url, pin_url='', success=True):
  """ mock gcs pin file action"""
  retcode = 1
  if success:
    retcode = 0
  if not pin_url:
    pin_url = url
  return api.step_data(
      NEST(NEST_PROCESS_CUST(), NEST_PIN_SRCS(cust),
           'gsutil stat {}'.format(url)),
      api.raw_io.stream_output(_gcs_stat.format(url, pin_url)),
      retcode=retcode,
  )


def GCS_DOWNLOAD_FILE(api, cust, bucket, source, success=True):
  """ mock gcs download file action"""
  retcode = 1
  if success:
    retcode = 0
  return api.step_data(
      NEST(
          NEST_DOWNLOAD_ALL_SRC(cust),
          'gsutil download gs://{}/{}'.format(bucket, source)),
      retcode=retcode,
  )


def ADD_FILE(api, image, customization, url, success=True):
  """ mock add file to image step """
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(customization),
          'PowerShell> Add file {}'.format(url)),
      stdout=json_res(api, success))


def INSTALL_FILE(api, name, image, customization, success=True):
  """ mock install file to image step """
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(customization),
          'PowerShell> Install package {}'.format(name)),
      stdout=json_res(api, success))


def INSTALL_DRIVER(api, name, image, customization, success=True):
  """ mock install driver to image step """
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(customization),
          'PowerShell> Install driver {}'.format(name)),
      stdout=json_res(api, success))


def EDIT_REGISTRY(api, name, image, customization, success=True):
  """ mock registry edit action step"""
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(customization),
          'PowerShell> Edit Offline Registry Key Features and Property {}'
          .format(name)),
      stdout=json_res(api, success))


def VM_POWERSHELL_EXEC(api,
                       image,
                       customization,
                       title,
                       output,
                       error,
                       logs,
                       retcode=0,
                       success=True):
  """ mock the powershell execution in the vm step """
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image),
          NEST_ONLINE_WINDOWS_CUSTOMIZATION_STEP(customization),
          NEST_ONLINE_CUSTOMIZATION_STEP('windows_cust'),
          NEST_ONLINE_ACTION_STEP('work_block'),
          'Powershell> {}'.format(title)),
      stdout=pwsh_json_res(
          api, output, error, logs, retcode=retcode, success=success))


def ADD_FILE_VM(api, image, customization, name, retcode=0, success=True):
  return VM_POWERSHELL_EXEC(
      api,
      image,
      customization,
      'Add File: {}'.format(name),
      '-----ROBOCOPY-----',
      '' if success else '-------ROBOCOPY----\nERROR',
      logs={},
      retcode=retcode,
      success=success)


def POWERSHELL_EXPR_VM(api,
                       image,
                       customization,
                       name,
                       output,
                       error='',
                       retcode=0,
                       success=True):
  return VM_POWERSHELL_EXEC(
      api,
      image,
      customization,
      name,
      output,
      error,
      logs={},
      retcode=retcode,
      success=success)


def POWERSHELL_EXPR_TIMEOUT(api, image, customization, title):
  return VM_POWERSHELL_EXEC(
      api,
      image,
      customization,
      title,
      output='Timeout: Waited too long',
      error='Timeout: Waited n seconds',
      logs=(),
      success=False)


def SHUTDOWN_VM(api, image, customization, vm_name, retcode=0):
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image),
          NEST_ONLINE_WINDOWS_CUSTOMIZATION_STEP(customization),
          NEST_ONLINE_CUSTOMIZATION_STEP('windows_cust'),
          NEST_ONLINE_CUSTOMIZATION_DEINIT_STEP('windows_cust'),
          'Shutting down {}'.format(vm_name),
          'Powershell> Shutdown {}'.format(vm_name)),
      stdout=pwsh_json_res(
          api,
          output='',
          error='',
          logs=None,
          retcode=retcode,
          success=not bool(retcode)))


def STARTUP_VM(api, image, customization, vm_name, success=True):
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image),
          NEST_ONLINE_WINDOWS_CUSTOMIZATION_STEP(customization),
          NEST_ONLINE_CUSTOMIZATION_STEP('windows_cust'),
          'Boot {}'.format(vm_name), 'Powershell> Wait for boot up'),
      stdout=(pwsh_json_res(
          api,
          output='Tuesday, January 31, 2023 10:30:56 AM' if success else '',
          error='' if success else 'Timeout',
          logs=None,
          retcode=0,
          success=success)))


def STATUS_VM(api, image, customization, vm_name, running=False):
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image),
          NEST_ONLINE_WINDOWS_CUSTOMIZATION_STEP(customization),
          NEST_ONLINE_CUSTOMIZATION_STEP('windows_cust'),
          NEST_ONLINE_CUSTOMIZATION_DEINIT_STEP('windows_cust'),
          'Shutting down {}'.format(vm_name), 'Status {}'.format(vm_name)),
      stdout=api.json.output({
          'return': {
              'running': True,
              'singlestep': False,
              'status': 'running'
          }
      }) if running else api.json.output(
          {'return': {
              'Error': '[Errno i111] Connection refused',
          }}))


def POWERDOWN_VM(api, image, customization, vm_name, success=False):
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image),
          NEST_ONLINE_WINDOWS_CUSTOMIZATION_STEP(customization),
          NEST_ONLINE_CUSTOMIZATION_STEP('windows_cust'),
          NEST_ONLINE_CUSTOMIZATION_DEINIT_STEP('windows_cust'),
          'Shutting down {}'.format(vm_name), 'Powerdown {}'.format(vm_name)),
      stdout=api.json.output({'return': {}})
      if success else api.json.output({'return': {
          'Error': 'QMP ERROR',
      }}))


def QUIT_VM(api, image, customization, vm_name, success=True):
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image),
          NEST_ONLINE_WINDOWS_CUSTOMIZATION_STEP(customization),
          NEST_ONLINE_CUSTOMIZATION_STEP('windows_cust'),
          NEST_ONLINE_CUSTOMIZATION_DEINIT_STEP('windows_cust'),
          'Shutting down {}'.format(vm_name), 'Quit {}'.format(vm_name)),
      stdout=api.json.output({'return': {}})
      if success else api.json.output({'return': {
          'Error': 'QMP FAILURE',
      }}))


def DISK_SPACE(api,
               image,
               customization,
               vm_name,
               disk,
               size=27815012,
               success=True):
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image),
          NEST_ONLINE_WINDOWS_CUSTOMIZATION_STEP(customization),
          NEST_ONLINE_CUSTOMIZATION_STEP('windows_cust'), NEST_BOOT_VM(vm_name),
          'Check free space on disk for {}'.format(disk)),
      api.raw_io.stream_output(
          dedent('''Avail
                         {}
                 '''.format(size))),
      retcode=0 if success else 1)


def MOUNT_DISK(api, image, customization, vm_name, disk, success=True):
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image),
          NEST_ONLINE_WINDOWS_CUSTOMIZATION_STEP(customization),
          NEST_ONLINE_CUSTOMIZATION_STEP('windows_cust'), NEST_BOOT_VM(vm_name),
          'Copy files to {}.Mount loop'.format(disk)),
      api.raw_io.stream_output('Mounted /dev/loop6 at /media/chrome-bot/test'),
      retcode=0 if success else 1)


def MOUNT_DISK_ISO(api, image, customization, disk, success=True):
  return api.step_data(
      NEST(
          NEST_CONFIG_STEP(image),
          NEST_WINDOWS_ISO_CUSTOMIZATION_STEP(customization),
          'Copy {} to staging'.format(disk), 'Mount loop'),
      api.raw_io.stream_output('Mounted /dev/loop6 at /media/chrome-bot/test'),
      retcode=0 if success else 1)


#    Assert methods to validate that a certain step was run


def CHECK_UMOUNT_WIM(api, image, customization, save=True):
  """
      Post check that the wim was unmounted with either save or discard
  """
  args = ['.*'] * 10  # ignore matching the first 10 terms
  # check the last option
  if not save:
    args.append('-Discard')
  else:
    args.append('-Save')
  return api.post_process(
      StepCommandRE,
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(customization),
          NEST_WINPE_DEINIT_STEP(),
          'PowerShell> Unmount wim at [CLEANUP]\\{}\\workdir\\mount'.format(
              customization)), args)


def CHECK_GCS_UPLOAD(api, image, cust, source, destination, orig=''):
  """
      Post check the upload to GCS
  """
  if not orig:
    orig = destination
  args = ['.*'] * 11
  args[7] = 'x-goog-meta-orig:{}'.format(orig)  # ensure the orig meta url
  args[9] = source  # ensure the correct local src
  args[10] = destination  # ensure upload to correct location
  return api.post_process(
      StepCommandRE,
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(cust),
          NEST_WINPE_DEINIT_STEP(), NEST_UPLOAD_CUST_OUTPUT(cust),
          'gsutil upload {}'.format(destination)), args)


def CHECK_INSTALL_CAB(api, image, customization, action, args=None):
  """
      Post check for installation
  """
  wild_card = ['.*'] * 11
  if args:
    wild_card.append(*args)
  return api.post_process(
      StepCommandRE,
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(customization),
          'PowerShell> Install package {}'.format(action)), wild_card)


def CHECK_INSTALL_DRIVER(api, image, customization, action, args=None):
  """
      Post check for installation
  """
  wild_card = ['.*'] * 11
  if args:
    wild_card.append(*args)
  return api.post_process(
      StepCommandRE,
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(customization),
          'PowerShell> Install driver {}'.format(action)), wild_card)


def CHECK_CIPD_UPLOAD(api, image, cust, dest):
  """
      Post check the upload to GCS
  """
  package = dest.cipd_src.package
  platform = dest.cipd_src.platform
  step_name = NEST(
      NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(cust),
      NEST_WINPE_DEINIT_STEP(), NEST_UPLOAD_CUST_OUTPUT(cust),
      'create {}/{}'.format(package, platform))

  result = api.post_process(StepCommandContains, step_name,
                            ['-ref', dest.cipd_src.refs])

  for tag, value in dest.tags.items():
    result += api.post_process(StepCommandContains, step_name,
                              ['-tag', '{}:{}'.format(tag, value)])

  return result


def CHECK_ADD_FILE(api, image, cust, url, dest):
  """
      Check add file step
  """
  wild_card = ['.*'] * 14
  wild_card[3] = 'robocopy'
  wild_card[11] = dest

  return api.post_process(
      StepCommandRE,
      NEST(
          NEST_CONFIG_STEP(image), NEST_WINPE_CUSTOMIZATION_STEP(cust),
          'PowerShell> Add file {}'.format(url)), wild_card)


def CHECK_DEBUG_SLEEP(api, image, cust, ocust='windows_cust', time=300):
  """
      Check debug sleep step
  """
  wild_card = ['sleep', str(time)]

  return api.post_process(
      StepCommandRE,
      NEST(
          NEST_CONFIG_STEP(image), NEST_ONLINE_WINDOWS_CUSTOMIZATION_STEP(cust),
          NEST_ONLINE_CUSTOMIZATION_STEP(ocust),
          'Debug sleep for {} seconds'.format(time)), wild_card)


#   Generate proto configs helper functions


def WPE_IMAGE(image,
              arch,
              customization,
              sub_customization,
              action_list,
              up_dests=None,
              image_src=None,
              mode=wib.CustomizationMode.CUST_NORMAL):
  """ generates a winpe customization image """
  return wib.Image(
      name=image,
      arch=arch,
      customizations=[
          wib.Customization(
              mode=mode,
              offline_winpe_customization=winpe.OfflineWinPECustomization(
                  name=customization,
                  image_src=image_src,
                  image_dests=up_dests,
                  offline_customization=[
                      actions.OfflineAction(
                          name=sub_customization, actions=action_list)
                  ]))
      ])


def WIN_IMAGE(image,
              arch,
              customization,
              vm_config,
              action_list,
              win_config=None,
              mode=wib.CustomizationMode.CUST_NORMAL):
  """ generates a winpe customization image """
  return wib.Image(
      name=image,
      arch=arch,
      customizations=[
          wib.Customization(
              mode=mode,
              online_windows_customization=owc.OnlineWinCustomization(
                  name=customization,
                  online_customizations=[
                      owc.OnlineCustomization(
                          name='windows_cust',
                          win_vm_config=win_config,
                          vm_config=vm_config,
                          online_actions=[
                              actions.OnlineAction(
                                  name='work_block', actions=action_list)
                          ])
                  ]))
      ])


def VM_CONFIG(
    name,
    drives,
    machine='virt,virulization=on,highmem=off',
    cpu='cortex-a57',
    smp='cores=8',
    memory=8192,
    extra_args=('-device usb-kbd', '--device usb-mouse'),
    device=(),
    version='latest',
):
  return vm.VM(
      qemu_vm=vm.QEMU_VM(
          name=name,
          version=version,
          drives=drives,
          machine=machine,
          cpu=cpu,
          smp=smp,
          memory=memory,
          device=device,
          extra_args=list(extra_args)))


def VM_DRIVE(name,
             ip,
             op,
             interface='ide',
             media='disk',
             fmt='raw',
             readonly=False,
             size=65536,
             filesystem='fat'):
  return drive.Drive(
      name=name,
      input_src=ip,
      output_dests=op,
      interface=interface,
      media=media,
      fmt=fmt,
      readonly=readonly,
      size=size,
      filesystem=filesystem)


def WIN_ISO(
    image,
    arch,
    name,
    base_image=sources.Src(
        gcs_src=sources.GCSSrc(
            bucket='chrome-gce-images', source='WIN-ISO/win10_vanilla.iso')),
    boot_image=sources.Src(
        gcs_src=sources.GCSSrc(
            bucket='chrome-gce-images', source='WIN-WIM/win10_gce.wim')),
    copy_files=(winiso.CopyArtifact(
        artifact=sources.Src(
            gcs_src=sources.GCSSrc(
                bucket='chrome-gce-images', source='WIB-ONLINE-CACHE/st.zip')),
        mount=True,
        source='sources/install.wim'),
                winiso.CopyArtifact(
                    artifact=sources.Src(
                        gcs_src=sources.GCSSrc(
                            bucket='chrome-gce-images',
                            source='WIN-WIM/win10_bootstrap_wim.zip')),
                    source='sources/boot.wim',
                    dest='sources'),
                winiso.CopyArtifact(
                    artifact=sources.Src(
                        gcs_src=sources.GCSSrc(
                            bucket='chrome-win-soft',
                            source='openssh/ssh.msi')),
                    mount=False,
                    dest='sources'),
                winiso.CopyArtifact(
                    artifact=sources.Src(
                        cipd_src=sources.CIPDSrc(
                            package='infra/chrome/windows/wallpapers',
                            refs='latest',
                            platform='windows-amd64')),
                    mount=False,
                    dest='sources'),
                winiso.CopyArtifact(
                    artifact=sources.Src(
                        cipd_src=sources.CIPDSrc(
                            package='infra/chrome/nopompt_boot',
                            refs='latest',
                            platform='windows-amd64')),
                    source='noprompt_boot.efi',
                    mount=False,
                    dest='sources'),
                winiso.CopyArtifact(
                    artifact=sources.Src(
                        gcs_src=sources.GCSSrc(
                            bucket='chrome-gce-images',
                            source='WIN-CACHE/win10_chrome.iso')),
                    source='media/sources/install.wim',
                    mount=True,
                    dest='sources')),
    uploads=(),
    unpacked_uploads=()):
  return wib.Image(
      name=image,
      arch=arch,
      customizations=[
          wib.Customization(
              windows_iso_customization=winiso.WinISOImage(
                  name=name,
                  base_image=base_image,
                  boot_image=boot_image,
                  copy_files=copy_files,
                  uploads=uploads,
                  unpacked_uploads=unpacked_uploads,
              ))
      ])
