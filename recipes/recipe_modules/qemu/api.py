# Copyright 2022 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from recipe_engine import recipe_api
from recipe_engine.recipe_api import Property
from textwrap import dedent

QEMU_PKG = 'infra/3pp/tools/qemu/linux-amd64'

QMP_HOST = 'localhost:4444'
SERIAL_HOST = 'localhost:4445'

QMP_SOCKET = 'tcp:' + QMP_HOST + ',server,nowait'
SERIAL_SOCKET = 'tcp:' + SERIAL_HOST + ',server,nowait'

PARTED = 'infra/3pp/tools/parted/linux-amd64'


class QEMUError(Exception):
  """ Base QEMU exception class """
  pass


class QEMUAPI(recipe_api.RecipeApi):
  """API to manage qemu VMs"""

  def __init__(self, *args, **kwargs):
    super(QEMUAPI, self).__init__(*args, **kwargs)
    self._install_dir = None
    self._workdir = None

  @property
  def path(self):
    return self._install_dir / 'bin'

  @property
  def disks(self):
    return self._workdir / 'disks'

  def init(self, version):
    """ Initialize the module, ensure that qemu exists on the system

    Note:
      - QEMU is installed in cache/qemu
      - Virtual disks are stored in cleanup/qemu/disks

    Args:
      * version: the cipd version tag for qemu
    """
    # create a directory to store qemu tools
    self._install_dir = self.m.path.cache_dir / 'qemu'
    # directory to store qemu workdata
    self._workdir = self.m.path.cleanup_dir.joinpath('qemu', 'workdir')
    self.m.file.ensure_directory(
        name='Ensure {}'.format(self.disks), dest=self.disks)

    # download the binaries to the install directory
    e = self.m.cipd.EnsureFile()
    # download latest if nothing is given
    e.add_package(QEMU_PKG, version if version else 'latest')
    e.add_package(PARTED, 'latest')
    self.m.cipd.ensure(self._install_dir, e, name="Download qemu")

  def cleanup_disks(self):  # pragma: nocover
    """ cleanup_disks deletes all the disks in the disks dir. This is meant to
    be used for cleanup after using the VM"""
    d_list = self.m.file.listdir('List all the disks', source=self.disks)
    for d in d_list:
      self.m.file.remove('Delete {}'.format(d), source=d)

  def create_disk(self, disk_name, fs_format='fat', min_size=0, include=None):
    """ create_disk creates a virtual disk with the given name, format and size.

    Optionally it is possible to specify a list of paths to copy to the disk.
    If the size is deemed to be too small to copy the files it might be
    increased to fit all the files.

    Args:
      * disk_name: name of the disk image file
      * fs_format: one of [exfat, ext3, fat, msdos, vfat, ext2, ext4, ntfs]
      * min_size: minimum size of the disk in Megabytes(1048576 bytes)
                  (bigger size used if required)
      * include: sequence of files and directories to copy to image
    """
    # Get the number of bytes
    min_size = min_size * 1048576
    if include:
      # mock output for testing
      test_res = dedent('''
                            123567   /usr/local/bin/fortune
                            123448   /usr/local/lib/libnolib.so
                            123565   /usr/local/lib/libs.so
                            13245243 total
                        ''')
      # use ls to estimate the size of all files
      res = self.m.step(
          name='Estimate size required for {}'.format(disk_name),
          # Use ls to get file size
          # s: print allocated size for each file
          # 1: list one file per line
          # R: list sub directories recursively
          # L: deference symbolic links
          # A: list all files, except implied . and ..
          # block-size: List size in bytes
          cmd=['ls', '-s1RLA', '--block-size=1'] + list(include.keys()),
          stdout=self.m.raw_io.output(),
          step_test_data=lambda: self.m.raw_io.test_api.stream_output(test_res))
      # Add stdout back to output
      res.presentation.logs['stdout'] = res.stdout
      calculations = ''
      failures = ''
      # Get list of file sizes, Ignore total at the end
      oplines = res.stdout.strip().splitlines()
      # Max FAT32 sector size is 32KB. Assume this is the case. Align to 32KB
      # https://support.microsoft.com/en-us/topic/default-cluster-size-for-ntfs-fat-and-exfat-9772e6f1-e31a-00d7-e18f-73169155af95
      f32_sector = 32768
      total_size = 0
      for idx, op in enumerate(oplines):
        op = op.strip()
        # skip empty lines
        if op == b'':
          continue
        # skip the total count
        if op.startswith(b'total'):
          continue
        if op.endswith(b':'):
          # can be the dir name followed by total
          if (idx + 1) < len(oplines) and oplines[idx + 1]:
            next_line = oplines[idx + 1].strip()
            if next_line.startswith(b'total'):
              continue
        tokens = op.split()
        if len(tokens) > 1:
          try:
            # must be a file listing of 'size name'
            file_size, file_name = tokens[0], tokens[1]
            f_size = int(file_size)
            # Assign the required number of sectors
            assigned_size = ((f_size // f32_sector) + 1) * f32_sector
            total_size += assigned_size
            calculations += '{}: {}\n'.format(
                file_name.decode('utf-8'), assigned_size)
          except Exception as err:
            failures += 'Cannot parse {}: {}'.format(op.decode('utf-8'), err)
        if len(tokens) != 2:
          failures += 'Unsure about {}'.format(op.decode('utf-8'))
      # bump up the size by 100% to ensure that all files can be copied
      total_size += total_size
      min_size = max(total_size, min_size)
      # Write back the logs to the list step
      calculations += 'Total size for disk: {}'.format(total_size)
      res.presentation.logs['calculations'] = calculations
      res.presentation.logs['failures'] = failures
    # create an empty disk
    self.create_empty_disk(disk_name, fs_format, min_size)
    if include:
      # copy files to the disk
      with self.m.step.nest(name='Copy files to {}'.format(disk_name)):
        loop_file, mount_loc = self.mount_disk_image(self.disks / disk_name)
        try:
          for src, dest in include.items():
            dest = mount_loc[0] + '/' + dest
            # Check if the path being copied is a dir
            is_dir_path = self.m.path.isdir(src)
            src = src if is_dir_path else self.m.path.dirname(src)
            dest = dest if is_dir_path else self.m.path.dirname(dest)
            self.m.file.copytree(
                name='Copy {}'.format(src), source=src, dest=dest)
        finally:
          self.unmount_disk_image(loop_file)

  def create_empty_disk(self, disk_name, fs_format, size):
    """ create_empty_disk creates an empty disk image and formats it

    Args:
      * disk_name: name of the disk image file
      * fs_format: one of [ext3, fat, ntfs]
      * size: size of the disk image in bytes
    """
    bs = 1024
    res = self.m.step(
        name='Check free space on disk for {}'.format(disk_name),
        cmd=['df', '--output=avail', '-B', bs, self.disks],
        stdout=self.m.raw_io.output())
    res.presentation.logs['stdout'] = res.stdout
    free_disk = int(res.stdout.splitlines()[-1].strip()) * bs
    if size > free_disk:
      raise self.m.step.StepFailure(
          'No space on the bot. Required {}, available {}'.format(
              size, free_disk))
    # Create disk of minimum 100 MiB. We can't create less than 32763.5 KiB
    # Because we can't format FAT32 on it. Use a minimum of 100 MiB drive. Just
    # so that we don't have to worry about overhead-space/data ratio. I didn't
    # come up with that number. That is the minimum partition size that
    # microsoft recommends for any partition.
    # https://docs.microsoft.com/en-us/windows-hardware/manufacture/desktop/configure-biosmbr-based-hard-drive-partitions?view=windows-11#system-partition
    if size < 104857600:  # pragma: no cover
      size = 104857600
    # replace the periods "." with underscore marks "_". This will be used to
    # create a powershell var later
    partition_name = disk_name.replace(".", "_")
    # First create a blank image for given size, We create a msdos style
    # partition table with one partition only
    self.m.step(
        name='Create blank image {}/{}'.format(self.disks, disk_name),
        cmd=[
            'python3',
            self.resource('mk_virt_disk.py'),
            '-pt',
            'gpt',
            '-ps',
            ' 4MiB',
            '-p',
            'type=primary,fs={},size={},name={}'.format(fs_format, size,
                                                        partition_name),
            # pass path to the parted binary
            '-g',
            self._install_dir.joinpath('sbin', 'parted'),
            self.disks / disk_name
        ])

  def mount_disk_image(self, disk, partitions=[1]):
    """ mount_disk_image mounts the given image and returns the mount location
    and loop file used for mounting

    Args:
      * disk: name of the disk image file
      * partitions: list of partitions to mount, If None attempt to mount the
      whole image

    Returns: loop file used for the disk and list of mount locations
    """
    # setup a loop device for the given disk image
    cmd = ['udisksctl', 'loop-setup', '-f', disk]
    if str(disk).endswith('iso') or str(disk).endswith('ISO'):
      # Cannot mount isos as rw
      cmd.append('-r')  # mount readonly
    res = self.m.step(
        name='Setup loop',
        cmd=cmd,
        stdout=self.m.raw_io.output(),
        step_test_data=lambda: self.m.raw_io.test_api.stream_output(
            'Mapped file {} to /dev/loop6'.format(disk)))
    loop_file = res.stdout.split()[-1].decode('UTF-8').strip('.')
    res.presentation.logs['stdout'] = res.stdout
    mounted_partitions = []
    mount_loc = []
    cmd = ['python3', self.resource('wait_for_file.py'), loop_file]
    res = self.m.step(name='Wait for mount to complete', cmd=cmd)
    res.presentation.logs['reason'] = 'Wait for loop to mount'
    try:
      if partitions:
        for partition in partitions:
          res = self.m.step(
              name='Mount loop',
              cmd=[
                  'udisksctl', 'mount', '-b',
                  loop_file + 'p{}'.format(partition)
              ],
              stdout=self.m.raw_io.output())
          mount_loc.append(res.stdout.split()[-1].decode('UTF-8').rstrip('.'))
          res.presentation.logs['stdout'] = res.stdout
          mounted_partitions.append(partition)
      else:
        # Might be a iso image. Mount the loop itself
        res = self.m.step(
            name='Mount loop',
            cmd=['udisksctl', 'mount', '-b', loop_file],
            stdout=self.m.raw_io.output())
        res.presentation.logs['stdout'] = res.stdout
        mount_loc.append(res.stdout.split()[-1].decode('UTF-8').strip('.'))

    except Exception as e:
      self.unmount_disk_image(loop_file, mounted_partitions)
      raise e

    return loop_file, mount_loc

  def unmount_disk_image(self, loop_file, partitions=[1]):
    """ unmount_disk_image unmounts the disk mounted using the given loop_file

    Args:
      * loop_file: Loop device used to mount the image
    """
    if partitions:
      for partition in partitions:
        # unmount the loop device
        self.m.step(
            name='Unmount loop',
            cmd=[
                'udisksctl', 'unmount', '-b',
                loop_file + 'p{}'.format(partition)
            ])
    else:
      # unmount the loop device
      self.m.step(
          name='Unmount loop', cmd=['udisksctl', 'unmount', '-b', loop_file])
    # delete the loop device
    self.m.step(
        name='Delete loop', cmd=['udisksctl', 'loop-delete', '-b', loop_file])

  def start_vm(self, arch, qemu_vm, kvm=False):
    """ start_vm starts a qemu vm

    QEMU is started with qemu_monitor running a qmp service. It also connects
    the serial port of the machine to a tcp port.

    Args:
     * arch: The arch that the VM should be based on
     * qemu_vm: QEMU_VM proto object containing all the config for starting
                the vm
     * kvm: If true then VM is run on hardware. It's emulated otherwise
    """
    host_arch = self.m.platform.arch
    if kvm:
      if (host_arch == 'arm' and (arch == 'amd64' or arch == 'x86')) or \
          (host_arch == 'intel' and arch == 'aarch64'):
        raise QEMUError('Cannot enable {} kvm on {}'.format(arch, host_arch))
    # Map the WIB config arch to qemu arch
    ARCH_TO_QEMU_ARCH = {'amd64': 'x86_64', 'aarch64': 'aarch64', 'x86': 'i386'}
    cmd = [
        self.path / 'qemu-system-{}'.format(ARCH_TO_QEMU_ARCH[arch]),
        '-qmp',
        QMP_SOCKET,  # start a qmp service
        '--serial',
        SERIAL_SOCKET,  # serial over tcp
        '-daemonize',  # run in background
        '-name',
        qemu_vm.name  # name the vm
    ]
    if kvm:
      # Enable kvm if it was asked for
      cmd += ['--enable-kvm']
    if qemu_vm.memory:
      # Add ram size if given
      cmd += ['-m', '{}M'.format(qemu_vm.memory)]
    if qemu_vm.cpu:
      # Add cpu config if given
      cmd += ['-cpu', qemu_vm.cpu]
    if qemu_vm.machine:
      # Add machine config if given
      cmd += ['-machine', qemu_vm.machine]
    if qemu_vm.smp:
      # Add smp config if given
      cmd += ['-smp', qemu_vm.smp]
    for dev in qemu_vm.device:
      # Add device configs if given
      cmd += ['-device', dev]
    for drive in qemu_vm.drives:
      # Add the file to be used for the drive
      drive_opts = 'file={},'.format(self.disks / drive.name)
      # Add an id to the drive. Useful in specifying device for it
      drive_opts += 'id={},'.format(drive.name)
      if drive.interface:
        # Add interface if given
        drive_opts += 'if={},'.format(drive.interface)
      if drive.media:
        # Add media if given
        drive_opts += 'media={},'.format(drive.media)
      if drive.fmt:
        # Add format if given
        drive_opts += 'format={},'.format(drive.fmt)
      if drive.readonly:
        # Add readonly if set
        drive_opts += 'readonly=on,'
      # Add drive to the cmd
      cmd += ['-drive', drive_opts]
    for extra_arg in qemu_vm.extra_args:
      # Add any extra args given
      cmd += [extra_arg]
    # Start the vm
    self.m.step(name='Start vm {}'.format(qemu_vm.name), cmd=cmd)

  def powerdown_vm(self, name):
    """ powerdown_vm sends a shutdown signal to the given VM. Similar to power
    button on a physical device

    Args:
      * name: name of the vm to shutdown

    Returns: True if powerdown signal was sent to VM. False otherwise
    """
    try:
      res = self.m.step(
          name='Powerdown {}'.format(name),
          cmd=[
              'python3',
              self.resource('qmp.py'), '-c', 'system_powerdown', '-s', QMP_HOST
          ],
          stdout=self.m.json.output())
      resp = res.stdout
      if 'Error' in resp['return'] and \
          'Connection refused' in resp['return']['Error']:
        # If we cannot connect to the VM. The response would be
        # {
        #   "return": {
        #       "Error": "[Errno 111] Connection refused"
        #   }
        # }
        # The VM has powered down.
        return True
      if not resp['return']:
        # If we send the powerdown signal successfully. The response will be
        # {
        #   "return": {}
        # }
        return True
      # return False in all other cases
      return False
    except Exception:
      # Failed to power down. Is it already powered down?
      # avoid raising exception
      return False

  def quit_vm(self, name):
    """ quit_vm sends a quit signal to the qemu process. Use this if your VM
    doesn't respond to powerdown signal.

    Args:
      * name: name of the vm to quit

    Returns: True if quit signal was sent to VM. False otherwise
    """
    try:
      res = self.m.step(
          name='Quit {}'.format(name),
          cmd=[
              'python3',
              self.resource('qmp.py'), '-c', 'quit', '-s', QMP_HOST
          ],
          stdout=self.m.json.output())
      resp = res.stdout
      if 'Error' in resp['return'] and \
          'Connection refused' in resp['return']['Error']:
        # If we cannot connect to the VM. The response would be
        # {
        #   "return": {
        #       "Error": "[Errno 111] Connection refused"
        #   }
        # }
        # The VM has powered down.
        return True
      if not resp['return']:
        # If we send the kill signal successfully. The response will be
        # {
        #   "return": {}
        # }
        return True
      # return false for all other cases
      return False
    except Exception:
      # Failed to power down. Is it already powered down?
      # avoid raising exception
      return False

  def vm_status(self, name):
    """ vm_status returns a dict describing the status of the vm. The return
    value is the QMP response to `query-status`

    Args:
      * name: name of the vm

    Returns: QMP json response for status query
    """
    try:
      res = self.m.step(
          name='Status {}'.format(name),
          cmd=[
              'python3',
              self.resource('qmp.py'), '-c', 'query-status', '-s', QMP_HOST
          ],
          stdout=self.m.json.output())
      return res.stdout
    except Exception as e:
      raise QEMUError(e)
