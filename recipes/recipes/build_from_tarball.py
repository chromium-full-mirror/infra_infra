# Copyright 2018 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from recipe_engine import post_process

PYTHON_VERSION_COMPATIBILITY = 'PY3'

DEPS = [
    'depot_tools/gsutil',
    'recipe_engine/cipd',
    'recipe_engine/context',
    'recipe_engine/file',
    'recipe_engine/path',
    'recipe_engine/platform',
    'recipe_engine/properties',
    'recipe_engine/step',
]


def RunSteps(api):
  build_dir = api.path.start_dir / 'build_dir'
  try:
    version = api.properties['version']
    tar_filename = 'chromium-%s.tar.xz' % version
    tar_file = build_dir / tar_filename
    api.gsutil.download_url('gs://chromium-browser-official/' + tar_filename,
                            tar_file)
    api.step('Extract tarball.',
             ['tar', '-xJf', str(tar_file), '-C',
              str(build_dir)])
    src_dir = build_dir.joinpath('chromium-' + version)

    # Install ninja CIPD package.
    cipd_root = api.path.cache_dir / 'cipd'
    ensure_file = api.cipd.EnsureFile().add_package(
        'infra/3pp/tools/ninja/${platform}', 'version:2@1.8.2.chromium.3')
    api.cipd.ensure(cipd_root, ensure_file)

    with api.context(
        cwd=src_dir,
        env_suffixes={'PATH': [cipd_root]}):
      llvm_bin_dir = src_dir.joinpath('third_party', 'llvm-build',
                                      'Release+Asserts', 'bin')
      gn_bootstrap_env = {
          'CC': llvm_bin_dir / 'clang',
          'CXX': llvm_bin_dir.joinpath('clang++'),
          'AR': llvm_bin_dir / 'llvm-ar',
          'LDFLAGS': '-fuse-ld=lld',
      }

      gn_args = [
          'is_debug=false',
          'enable_nacl=false',
          'is_official_build=true',
          'enable_distro_version_check=false',

          # TODO(thomasanderson): Setting use_system_libjpeg shouldn't be
          # necessary when unbundling libjpeg.
          'use_system_libjpeg=true',
          'use_v8_context_snapshot=false',
      ]

      unbundle_libs = [
          'fontconfig',
          'freetype',
          'libdrm',
          'libjpeg',
          'libwebp',
          'opus',

          # https://crbug.com/731766
          # 'ffmpeg',

          # TODO(thomasanderson): Add ogg-dev to sysroots.
          # 'flac',

          # TODO(thomasanderson): Reenable once Debian unstable pulls in
          # harfbuzz 1.7.5 or later.
          # 'harfbuzz-ng',

          # The icu dev package is huge, so it's omitted from the sysroots.
          # 'icu',

          # https://crbug.com/752403#c10
          # 'libpng',

          # TODO(thomasanderson): Update the sysroot.
          # 'libvpx',

          # https://crbug.com/736026
          # 'libxml',

          # TODO(thomasanderson): Add libxml2-dev to sysroots.
          # 'libxslt',

          # Chrome passes c++ strings to re2, but the inline namespace used by
          # libc++ (std::__1::string) differs from the one re2 expects
          # (std::__cxx11::string), causing link failures.
          # 're2',

          # TODO(https://crbug.com/1412844): Fix snappy unbundle and reenable
          # when the fix rolls out to all channels.
          # 'snappy',

          # Use the yasm in third_party to prevent having to install yasm on the
          # bot.
          # 'yasm',

          # TODO(thomasanderson): Add libminizip-dev to sysroots.
          # 'zlib',
      ]

      api.step('Download sysroot.', [
          api.path.join(src_dir, 'build', 'linux', 'sysroot_scripts',
                        'install-sysroot.py'), '--arch=amd64'
      ])

      api.step('Build clang.', [
          api.path.join(src_dir, 'tools', 'clang', 'scripts', 'build.py'),
          '--skip-checkout', '--without-android', '--without-fuchsia'
      ])

      api.step('Build rustc.', [
          api.path.join(src_dir, 'tools', 'rust', 'build_rust.py'),
          '--skip-checkout'
      ])
      api.step('Build bindgen.',
               [api.path.join(src_dir, 'tools', 'rust', 'build_bindgen.py')])

      with api.context(env=gn_bootstrap_env):
        api.step('Bootstrap gn.', [
            api.path.join(src_dir, 'tools', 'gn', 'bootstrap', 'bootstrap.py'),
            '--gn-gen-args=%s' % ' '.join(gn_args), '--use-custom-libcxx'
        ])

      api.step('Download nodejs.', [
          api.path.join(src_dir, 'third_party', 'node', 'update_node_binaries')
      ])

      api.step('Unbundle libraries.', [
          api.path.join(src_dir, 'build', 'linux', 'unbundle',
                        'replace_gn_files.py'), '--system-libraries'
      ] + unbundle_libs)

      api.step('Build chrome.',
               ['ninja', '-C', 'out/Release', 'chrome/installer/linux'])
  finally:
    api.file.rmtree('Cleaning build dir.', build_dir)


def GenTests(api):
  yield (api.test('basic') + api.properties.generic(version='130.0.6723.117') +
         api.platform('linux', 64))
  yield (api.test('basic-with-js_type_check-gn-arg') +
         api.properties.generic(version='130.0.6723.117') +
         api.platform('linux', 64))
