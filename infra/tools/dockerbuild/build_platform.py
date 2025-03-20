# Copyright 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import collections
import platform
import subprocess
import sys

_MANYLINUX_ENV = {
    'LDSHARED': '/opt/rh/devtoolset-10/root/usr/bin/gcc -pthread -shared'
}


class Platform(
    collections.namedtuple(
        'Platform',
        (
            # The name of the platform.
            'name',

            # The value to pass to e.g. `./configure --host ...`
            'cross_triple',

            # The Python wheel ABI.
            'wheel_abi',

            # Tuple of Python wheel platforms. Must have at least one.
            #
            # This is used for local wheel naming. Wheels are named universally
            # within CIPD packages. Changing this will not impact CIPD package
            # contents, but will affect the locally generated intermediate wheel
            # names.
            'wheel_plat',

            # The "dockcross" base image (can be None).
            'dockcross_base',

            # The tag to pull for the dockcross_base image
            # (None if not using dockcross).
            'dockcross_tag',

            # The name of the dockerbuild docker image we have built
            # for this platform. The same image may be shared between
            # several Platform tuples. May be None if this platform
            # does not use Docker.
            'dockerbuild_image',

            # The OpenSSL "Configure" script build target.
            'openssl_target',

            # Do Python wheels get packaged on PyPi for this platform?
            'packaged',

            # The name of the CIPD platform to use.
            'cipd_platform',

            # Extra environment variables to set when building wheels on this
            # platform.
            'env',

            # Whether wheels are built for this platform by default.
            'default',
        ))):

  @property
  def dockcross_base_image(self):
    if not self.dockcross_base:
      return None
    return 'dockcross/%s' % (self.dockcross_base,)

  @property
  def dockcross_image_tag(self):
    return 'infra-dockerbuild-%s' % (self.name,)

  @property
  def universal(self):
    return 'any' in self.wheel_plat


ALL = {
    p.name: p for p in (
        Platform(
            name='linux-armv6-py3.8',
            cross_triple='armv6-unknown-linux-gnueabihf',
            wheel_abi='cp38',
            wheel_plat=('linux_armv6l', 'linux_armv7l', 'linux_armv8l',
                        'linux_armv9l'),
            dockcross_base='linux-armv6-lts',
            dockcross_tag='20230222-162287d',
            dockerbuild_image='linux-armv6-py3',
            openssl_target='linux-armv4',
            packaged=False,
            cipd_platform='linux-armv6l',
            env={},
            default=False,
        ),
        Platform(
            name='linux-armv6-py3.11',
            cross_triple='armv6-unknown-linux-gnueabihf',
            wheel_abi='cp311',
            wheel_plat=('linux_armv6l', 'linux_armv7l', 'linux_armv8l',
                        'linux_armv9l'),
            dockcross_base='linux-armv6-lts',
            dockcross_tag='20230222-162287d',
            dockerbuild_image='linux-armv6-py3',
            openssl_target='linux-armv4',
            packaged=False,
            cipd_platform='linux-armv6l',
            env={},
            default=True,
        ),
        Platform(
            name='linux-arm64-py3.8',
            cross_triple='aarch64-unknown-linux-gnu',
            wheel_abi='cp38',
            wheel_plat=('linux_arm64', 'linux_aarch64'),
            dockcross_base='linux-arm64-lts',
            dockcross_tag='20230222-162287d',
            dockerbuild_image='linux-arm64-py3',
            openssl_target='linux-aarch64',
            packaged=False,
            cipd_platform='linux-arm64',
            env={},
            default=False,
        ),
        Platform(
            name='linux-arm64-py3.11',
            cross_triple='aarch64-unknown-linux-gnu',
            wheel_abi='cp311',
            wheel_plat=('linux_arm64', 'linux_aarch64'),
            dockcross_base='linux-arm64-lts',
            dockcross_tag='20230222-162287d',
            dockerbuild_image='linux-arm64-py3',
            openssl_target='linux-aarch64',
            packaged=False,
            cipd_platform='linux-arm64',
            env={},
            default=True,
        ),
        Platform(
            name='linux-riscv64-py3.8',
            cross_triple='riscv64-linux-gnu',
            wheel_abi='cp38',
            wheel_plat=('linux_riscv64'),
            dockcross_base='linux-riscv64',
            dockcross_tag='20240104-6eda627',
            dockerbuild_image='linux-riscv64-py3',
            openssl_target='linux-riscv64',
            packaged=False,
            cipd_platform='linux-riscv64',
            env={},
            default=False,
        ),
        Platform(
            name='linux-riscv64-py3.11',
            cross_triple='riscv64-linux-gnu',
            wheel_abi='cp311',
            wheel_plat=('linux_riscv64'),
            dockcross_base='linux-riscv64',
            dockcross_tag='20240104-6eda627',
            dockerbuild_image='linux-riscv64-py3',
            openssl_target='linux-riscv64',
            packaged=False,
            cipd_platform='linux-riscv64',
            env={},
            default=False,
        ),
        Platform(
            name='manylinux-x64-py3.8',
            cross_triple='x86_64-linux-gnu',
            wheel_abi='cp38',
            wheel_plat=('manylinux2014_x86_64',),
            dockcross_base='manylinux2014-x64',
            dockcross_tag='20230222-162287d',
            dockerbuild_image='manylinux-x64-py3',
            openssl_target='linux-x86_64',
            packaged=True,
            cipd_platform='linux-amd64',
            env=_MANYLINUX_ENV,
            default=False,
        ),
        Platform(
            name='manylinux-x64-py3.11',
            cross_triple='x86_64-linux-gnu',
            wheel_abi='cp311',
            wheel_plat=('manylinux2014_x86_64',),
            dockcross_base='manylinux2014-x64',
            dockcross_tag='20230222-162287d',
            dockerbuild_image='manylinux-x64-py3',
            openssl_target='linux-x86_64',
            packaged=False,  # Most wheels not available on pypi.org
            cipd_platform='linux-amd64',
            env=_MANYLINUX_ENV,
            default=True,
        ),
        Platform(
            name='mac-x64-py3.8',
            cross_triple='',
            wheel_abi='cp38',
            wheel_plat=('macosx_10_11_x86_64',),
            dockcross_base=None,
            dockcross_tag=None,
            dockerbuild_image=None,
            openssl_target='darwin64-x86_64-cc',
            packaged=True,
            cipd_platform='mac-amd64',
            env={
                # Necessary for some wheels to build. See for instance:
                # https://github.com/giampaolo/psutil/issues/1832
                'ARCHFLAGS': '-arch x86_64',
                'MACOSX_DEPLOYMENT_TARGET': '10.13'
            },
            default=False,
        ),
        Platform(
            name='mac-x64-py3.11',
            cross_triple='',
            wheel_abi='cp311',
            wheel_plat=('macosx_10_11_x86_64', 'macosx_11_0_universal2'),
            dockcross_base=None,
            dockcross_tag=None,
            dockerbuild_image=None,
            openssl_target='darwin64-x86_64-cc',
            packaged=True,
            cipd_platform='mac-amd64',
            env={
                # Necessary for some wheels to build. See for instance:
                # https://github.com/giampaolo/psutil/issues/1832
                'ARCHFLAGS': '-arch x86_64',
                'MACOSX_DEPLOYMENT_TARGET': '10.13'
            },
            default=True,
        ),
        Platform(
            name='mac-arm64-py3.8',
            cross_triple='',
            wheel_abi='cp38',
            wheel_plat=('macosx_11_0_arm64',),
            dockcross_base=None,
            dockcross_tag=None,
            dockerbuild_image=None,
            openssl_target='darwin64-arm64-cc',
            # TODO: See whether this can be enabled now that Python 3.8.10
            # supports mac-arm64.
            packaged=False,
            cipd_platform='mac-arm64',
            env={
                # Necessary for some wheels to build. See for instance:
                # https://github.com/giampaolo/psutil/issues/1832
                'ARCHFLAGS': '-arch arm64',
                'MACOSX_DEPLOYMENT_TARGET': '11.0'
            },
            default=False,
        ),
        Platform(
            name='mac-arm64-py3.11',
            cross_triple='',
            wheel_abi='cp311',
            wheel_plat=('macosx_11_0_arm64', 'macosx_11_0_universal2'),
            dockcross_base=None,
            dockcross_tag=None,
            dockerbuild_image=None,
            openssl_target='darwin64-arm64-cc',
            packaged=True,
            cipd_platform='mac-arm64',
            env={
                # Necessary for some wheels to build. See for instance:
                # https://github.com/giampaolo/psutil/issues/1832
                'ARCHFLAGS': '-arch arm64',
                'MACOSX_DEPLOYMENT_TARGET': '11.0'
            },
            default=True,
        ),
        Platform(
            name='windows-x86-py3.8',
            cross_triple='',
            wheel_abi='cp38',
            wheel_plat=('win32',),
            dockcross_base=None,
            dockcross_tag=None,
            dockerbuild_image=None,
            openssl_target='Cygwin-x86',
            packaged=True,
            cipd_platform='windows-386',
            env={},
            default=False,
        ),
        Platform(
            name='windows-x86-py3.11',
            cross_triple='',
            wheel_abi='cp311',
            wheel_plat=('win32',),
            dockcross_base=None,
            dockcross_tag=None,
            dockerbuild_image=None,
            openssl_target='Cygwin-x86',
            packaged=True,
            cipd_platform='windows-386',
            env={},
            default=True,
        ),
        Platform(
            name='windows-x64-py3.8',
            cross_triple='',
            wheel_abi='cp38',
            wheel_plat=('win_amd64',),
            dockcross_base=None,
            dockcross_tag=None,
            dockerbuild_image=None,
            openssl_target='Cygwin-x86_64',
            packaged=True,
            cipd_platform='windows-amd64',
            env={},
            default=False,
        ),
        Platform(
            name='windows-x64-py3.11',
            cross_triple='',
            wheel_abi='cp311',
            wheel_plat=('win_amd64',),
            dockcross_base=None,
            dockcross_tag=None,
            dockerbuild_image=None,
            openssl_target='Cygwin-x86_64',
            packaged=True,
            cipd_platform='windows-amd64',
            env={},
            default=True,
        ),
        Platform(
            name='universal',
            cross_triple='',
            wheel_abi='none',
            wheel_plat=('any',),
            dockcross_base=None,
            dockcross_tag=None,
            dockerbuild_image=None,
            openssl_target=None,
            packaged=True,
            cipd_platform=None,
            env={},
            default=True,
        ),
    )
}


# Detect whether we're on an ARM64 Mac running emulated x86_64 Python.
# In this situation, we still consider ARM64 the native platform.
def _CheckTranslated():
  if sys.platform != 'darwin':
    return False

  try:
    output = subprocess.check_output(
        ["/usr/sbin/sysctl", "-n", "sysctl.proc_translated"], text=True,
        stderr=subprocess.STDOUT)
    return output[0] == '1'
  except subprocess.CalledProcessError:
    # The call will fail on x86_64 Macs.
    return False


NAMES = sorted(ALL.keys())
PACKAGED = [p for p in ALL.values() if p.packaged]
ALL_LINUX = [p.name for p in ALL.values() if 'linux' in p.name]
UNIVERSAL = [p.name for p in ALL.values() if 'universal' in p.name]
ALL_PY311 = [p.name for p in ALL.values() if p.name.endswith('-py3.11')]
ALL_MAC_X64 = [p.name for p in ALL.values() if p.name.startswith('mac-x64-')]
ALL_MAC_ARM64 = [
    p.name for p in ALL.values() if p.name.startswith('mac-arm64-')
]
ALL_MAC = [p.name for p in ALL.values() if p.name.startswith('mac-')]
_IS_TRANSLATED = _CheckTranslated()


def NativeMachine():
  machine = platform.machine()
  if sys.platform == 'darwin' and machine == 'x86_64' and _IS_TRANSLATED:
    machine = 'arm64'
  return machine


def NativePlatforms():
  # Every supported OS can build universal wheels.
  plats = [ALL[u] for u in UNIVERSAL]

  # Identify our native platforms.
  if sys.platform == 'darwin':
    mac_plats = {'x86_64': ALL_MAC_X64, 'arm64': ALL_MAC_ARM64}
    return plats + [ALL[p] for p in mac_plats[NativeMachine()]]
  elif sys.platform == 'win32':
    return plats + [
        ALL['windows-x86-py3.8'], ALL['windows-x86-py3.11'],
        ALL['windows-x64-py3.8'], ALL['windows-x64-py3.11']
    ]
  elif sys.platform.startswith('linux'):
    # Linux platforms are built with docker, so Linux doesn't support any
    # non-universal platforms natively.
    return plats
  raise ValueError('Cannot identify native image for %r-%r.' %
                   (sys.platform, NativeMachine()))
