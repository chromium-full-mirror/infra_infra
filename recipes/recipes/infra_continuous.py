# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from PB.go.chromium.org.luci.buildbucket.proto.common import FAILURE, SUCCESS
from PB.recipe_engine.result import RawResult

PYTHON_VERSION_COMPATIBILITY = "PY2+3"

DEPS = [
    'depot_tools/bot_update',
    'depot_tools/depot_tools',
    'depot_tools/gclient',
    'depot_tools/osx_sdk',
    'recipe_engine/buildbucket',
    'recipe_engine/context',
    'recipe_engine/defer',
    'recipe_engine/file',
    'recipe_engine/json',
    'recipe_engine/path',
    'recipe_engine/platform',
    'recipe_engine/properties',
    'recipe_engine/resultdb',
    'recipe_engine/runtime',
    'recipe_engine/step',
    'infra_checkout',
    'infra_cipd',
]


# Mapping from a builder name to a list of GOOS-GOARCH variants it should build
# CIPD packages for. 'native' means "do not cross-compile, build for the host
# platform". Targeting 'native' will also usually build non-go based packages.
#
# Additionally, a variant may have a sequence of options appended to it,
# separated by colons. e.g. 'VARIANT:option:option'. Currently the supported
# options are:
#   * 'test' - Run the tests. By default no tests are run.
#   * 'legacy' - Switch the builder that builds this variant to use go "legacy"
#     Go version 1.15.* instead of the "bleeding_edge" version. This is
#     primarily needed by builders targeting OSX amd64 that need to produce
#     binaries that can run on OSX 10.10 and OSX 10.11. What exact versions
#     correspond to "legacy" and "bleeding_edge" is defined in bootstrap.py in
#     infra.git. Note that this option applies to the entire builder (not only
#     the individual build variant).
#
# If the builder is not in this set, or the list of GOOS-GOARCH for it is empty,
# it won't be used for building CIPD packages.
#
# Only builders named '*-packager-*' builders will actually upload CIPD
# packages, while '*-continuous-*' builders merely verify that CIPD packages can
# be built.
#
# TODO(iannucci): make packager role explicit with `package=cipd_prefix` option.
# TODO(iannucci): remove this dict and put this all configuration as explicit
#    property inputs to the recipe :)
CIPD_PACKAGE_BUILDERS = {
    # bionic-64 is the primary builder for linux-amd64, and the rest just
    # cross-compile to different platforms (to speed up the overall cycle time
    # by doing stuff in parallel).
    'infra-continuous-bionic-64': [
        'native:test',
    ],
    'infra-continuous-jammy-64': [
        'linux-arm64',
    ],

    # 10.13 is the primary builder for darwin-amd64.
    'infra-continuous-mac-10.13-64': ['native:test:legacy',],
    'infra-continuous-mac-10.14-64': [],
    'infra-continuous-mac-10.15-64': ['darwin-arm64',],

    # Windows 64 bit builder runs and tests for both 64 && 32 bit.
    'infra-continuous-win10-64': [
        'native:test',
        'windows-386:test',
        'windows-arm64',  # cross compiled, so don't run tests.
    ],
    'infra-continuous-win11-64': [
        'native:test',
        'windows-386:test',
    ],

    # Internal builders, they use exact same recipe.
    'infra-internal-continuous-bionic-64': [
       'native:test',
       'linux-arm64',
    ],
    'infra-internal-continuous-win-64': [
        'native:test',
        'windows-386:test',
        'windows-arm64',  # cross compiled, so don't run tests.
    ],
    'infra-internal-continuous-mac-11-64': ['native:test:legacy',],

    # Builders that upload CIPD packages.
    #
    # In comments is approximate runtime for building and testing packages, per
    # platform (as of Feb 23 2021). We try to balance xc1 and xc2.
    'infra-packager-linux-64': [
        'native',  # ~120 sec
    ],
    'infra-packager-linux-xc1': [
        'dragonfly-amd64',
        'freebsd-386',
        'freebsd-amd64',
        'freebsd-arm64',
        'freebsd-riscv64',
        'illumos-amd64',
        'linux-386',  # ~60 sec
        'linux-arm',  # ~60 sec
        'linux-arm64',  # ~60 sec
        'openbsd-386',
        'openbsd-amd64',
        'openbsd-arm64',
        'openbsd-ppc64',
        'openbsd-riscv64',
        'solaris-amd64',
    ],
    'infra-packager-linux-xc2': [
        'aix-ppc64',  # ~5 sec
        'linux-loong64',
        'linux-mips',
        'linux-mips64',  # ~40 sec
        'linux-mips64le',  # ~40 sec
        'linux-mipsle',  # ~40 sec
        'linux-ppc64',  # ~40 sec
        'linux-ppc64le',  # ~40 sec
        'linux-riscv64',  # ~5 sec
        'linux-s390x',  # ~40 sec
        'netbsd-386',
        'netbsd-amd64',
        'netbsd-arm',
        'netbsd-arm64',
    ],
    'infra-packager-mac-64': [
        # NOTE: Mac packages need to be codesigned and so can ONLY be packaged
        # on Mac.
        'native:legacy',  # ~150 sec
    ],
    'infra-packager-mac-arm64': [
        # NOTE: Mac packages need to be codesigned and so can ONLY be packaged
        # on Mac.
        'native',  # ~150 sec
    ],
    'infra-packager-win-64': [
        'native',  # ~60 sec
        'windows-386',  # ~100 sec
        'windows-arm64',  # not timed
    ],
    'infra-internal-packager-linux-64': [
        'native',  # ~60 sec
        'linux-arm',  # ~30 sec
        'linux-arm64',  # ~30 sec
    ],
    'infra-internal-packager-mac-64': [
        # NOTE: Mac packages need to be codesigned and so can ONLY be packaged
        # on Mac.
        'native:legacy',  # ~40 sec
    ],
    'infra-internal-packager-mac-arm64': [
        # NOTE: Mac packages need to be codesigned and so can ONLY be packaged
        # on Mac.
        'native',  # ~40 sec
    ],
    'infra-internal-packager-win-64': [
        'native',  # ~60 sec
        'windows-386',  # ~40 sec
        'windows-arm64',  # not timed
    ],

    # Experimental builders to test new VMs. Copies of corresponding non-exp
    # builders.
    'infra-packager-linux-64-exp': [
        'native',
    ],
    'infra-packager-win-64-exp': [
        'native',
        'windows-386',
        'windows-arm64',
    ],
}

INFRA_INTERNAL_REPO = 'https://chrome-internal.googlesource.com/infra/infra_internal'
INFRA_REPO = 'https://chromium.googlesource.com/infra/infra'


def should_run_python_tests(api, builder_name):
  # Do not run python tests on packager builders, since most of them are
  # irrelevant to the produced packages. Relevant portion of tests will be run
  # from api.infra_cipd.test() below, when testing packages that pack python
  # code.
  return api.platform.arch != 'arm' and 'packager' not in builder_name


def RunSteps(api):
  buildername: str = api.buildbucket.builder_name
  if buildername.startswith((
      'infra-internal-continuous',
      'infra-internal-packager',
  )):
    project_name = 'infra_internal'
    repo_url = INFRA_INTERNAL_REPO
    gclient_config = 'infra_internal'
  elif buildername.startswith((
      'infra-continuous',
      'infra-packager',
  )):
    project_name = 'infra'
    repo_url = INFRA_REPO
    gclient_config = 'infra'
  else:  # pragma: no cover
    raise ValueError(
        'This recipe is not intended for builder %s. ' % buildername)

  # Use the latest bleeding edge version of Go unless asked for the legacy one.
  go_version_variant = 'bleeding_edge'
  for variant in CIPD_PACKAGE_BUILDERS.get(buildername, []):
    if 'legacy' in variant.split(':'):
      go_version_variant = 'legacy'
      break

  internal = project_name == 'infra_internal'
  co = api.infra_checkout.checkout(
      gclient_config_name=gclient_config,
      internal=internal,
      go_version_variant=go_version_variant)
  co.gclient_runhooks()

  # Whatever is checked out by bot_update. It is usually equal to
  # api.buildbucket.gitiles_commit.id except when the build was triggered
  # manually (commit id is empty in that case).
  rev = co.bot_update_step.properties['got_revision']
  return build_main(api, co, buildername, project_name, repo_url, rev)


def build_main(api, checkout, buildername, project_name, repo_url, rev):
  is_packager = 'packager' in buildername

  if should_run_python_tests(api, buildername):
    run_python_tests(api, checkout, project_name)

  # Some third_party go packages on OSX rely on cgo and thus a configured
  # clang toolchain.
  with api.osx_sdk('mac'), checkout.go_env():
    if not is_packager:
      with api.depot_tools.on_path():
        # Some go tests test interactions with depot_tools binaries, so put
        # depot_tools on the path.
        api.step(
            'infra go tests',
            api.resultdb.wrap([
                'vpython3', '-u',
                checkout.path / project_name / 'go' / 'test.py'
            ]))

    fails = []
    for plat in CIPD_PACKAGE_BUILDERS.get(buildername, []):
      options = plat.split(':')
      plat = options.pop(0)

      # Mac packages can only be produced by a mac builder.
      if 'darwin-' in plat and is_packager and not api.platform.is_mac:
        api.step.empty(
            "Invalid package configuration",
            status=api.step.EXCEPTION,
            step_text=(
              "Mac packages require codesign and can not run on %s."
              % (api.platform.name,)
            ),
        )

      if plat == 'native':
        goos, goarch = None, None
      else:
        goos, goarch = plat.split('-', 1)

      try:
        with api.infra_cipd.context(
            checkout.path / project_name, goos, goarch):
          if api.platform.is_mac:
            api.infra_cipd.build(api.properties.get('signing_identity'))
          else:
            api.infra_cipd.build()
          if 'test' in options:
            api.infra_cipd.test()
          if is_packager:
            if api.runtime.is_experimental:
              api.step('no CIPD package upload in experimental mode', cmd=None)
            else:
              api.infra_cipd.upload(api.infra_cipd.tags(repo_url, rev))
      except api.step.StepFailure:
        fails.append(plat)

  status = SUCCESS
  summary = []
  if fails:
    status = FAILURE
    summary.append('Failed to build some CIPD packages for platform(s):')
    summary.extend('  * %s' % f for f in fails)
  return RawResult(status=status, summary_markdown='\n'.join(summary))


def run_python_tests(api, checkout, project_name):
  deferred = []
  with api.context(cwd=checkout.path / project_name):
    # Run Linux tests everywhere, Windows tests only on public CI.
    if api.platform.is_linux or project_name == 'infra':
      deferred.append(
          api.defer(api.step, 'infra python tests',
                    ['vpython3', 'test.py', 'test']))
    if project_name == 'infra':
      # The infra project runs py3 tests separately.
      deferred.append(
          api.defer(api.step, 'infra python tests (py3)',
                    ['vpython3', 'test.py', 'test', '--py3']))

    if ((api.platform.is_linux or api.platform.is_mac) and
        project_name == 'infra'):
      for app in ['monorail', 'predator', 'findit']:
        cwd = checkout.path / project_name / 'appengine' / app
        if app == 'predator':
          cwd = cwd / 'app'
        with api.context(cwd=cwd):
          deferred.append(
              api.defer(api.step, '%s python tests' % app,
                        ['vpython3', 'test.py']))

    # Validate ccompute configs.
    if api.platform.is_linux and project_name == 'infra_internal':
      ccompute_config = checkout.path.joinpath(project_name, 'ccompute',
                                               'scripts', 'ccompute_config.py')
      deferred.append(
          api.defer(api.step, 'ccompute config test',
                    ['python3', ccompute_config, 'test']))

  api.defer.collect(deferred)


def GenTests(api):
  # This allows us to test potential misconfigurations. As long as this runs
  # before the yield statement for the test that needs it, it will be added to
  # the module global by the time the test runs.
  #
  # We put it here so that ALL tests have a consistent view of
  # CIPD_PACKAGE_BUILDERS.
  CIPD_PACKAGE_BUILDERS['infra-packager-TEST-linux-xcMac'] = [
    'darwin-arm64',
  ]

  def test(name,
           builder,
           repo,
           project,
           bucket,
           plat,
           is_experimental=False,
           arch='intel',
           status='SUCCESS'):
    return (api.test(name, status=status) + api.platform(plat, 64, arch) +
            api.runtime(is_experimental=is_experimental) +
            api.buildbucket.ci_build(
                project, bucket, builder, git_repo=repo, build_number=123))

  yield test('public-ci-linux', 'infra-continuous-bionic-64', INFRA_REPO,
             'infra', 'ci', 'linux')
  yield test(
      'public-ci-linux-arm64',
      'infra-continuous-bionic-64',
      INFRA_REPO,
      'infra',
      'ci',
      'linux',
      arch='arm')
  yield test('public-ci-win', 'infra-continuous-win10-64', INFRA_REPO, 'infra',
             'ci', 'win')

  yield test('internal-ci-linux', 'infra-internal-continuous-bionic-64',
             INFRA_INTERNAL_REPO, 'infra-internal', 'ci', 'linux')
  yield test('internal-ci-mac', 'infra-internal-continuous-mac-11-64',
             INFRA_INTERNAL_REPO, 'infra-internal', 'ci', 'mac')

  yield test('public-packager-mac', 'infra-packager-mac-64', INFRA_REPO,
             'infra', 'prod', 'mac')
  yield test(
      'public-packager-mac_experimental',
      'infra-packager-mac-64',
      INFRA_REPO,
      'infra',
      'prod',
      'mac',
      is_experimental=True)
  yield test('public-packager-mac_codesign', 'infra-packager-mac-64',
             INFRA_REPO, 'infra', 'prod', 'mac') + api.properties(
                 signing_identity='AAAAAAAAAAAAABBBBBBBBBBBBBXXXXXXXXXXXXXX')
  yield test('public-packager-mac-arm64_codesign', 'infra-packager-mac-arm64',
             INFRA_REPO, 'infra', 'prod', 'mac') + api.properties(
                 signing_identity='AAAAAAAAAAAAABBBBBBBBBBBBBXXXXXXXXXXXXXX')
  yield test('public-packager-win', 'infra-packager-win-64', INFRA_REPO,
             'infra', 'prod', 'win')

  yield test('internal-packager-linux', 'infra-internal-packager-linux-64',
             INFRA_INTERNAL_REPO, 'infra-internal', 'prod', 'linux')
  yield test('internal-packager-mac-arm64_codesign',
             'infra-internal-packager-mac-arm64', INFRA_INTERNAL_REPO,
             'infra_internal', 'prod', 'mac') + api.properties(
                 signing_identity='AAAAAAAAAAAAABBBBBBBBBBBBBXXXXXXXXXXXXXX')

  yield (
      test('packager-cipd-fail', 'infra-packager-linux-xc1', INFRA_REPO,
           'infra', 'prod', 'linux', status='FAILURE') +
      api.step_data('[GOOS:linux GOARCH:arm]cipd - build packages', retcode=1))

  yield test('cross-packager-mac-fail', 'infra-packager-TEST-linux-xcMac',
             INFRA_REPO, 'infra', 'prod', 'linux', status='INFRA_FAILURE')
