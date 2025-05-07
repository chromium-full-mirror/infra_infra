# Copyright 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

PYTHON_VERSION_COMPATIBILITY = "PY2+3"

DEPS = [
    'depot_tools/git',
    'infra_checkout',
    'recipe_engine/buildbucket',
    'recipe_engine/cipd',
    'recipe_engine/context',
    'recipe_engine/file',
    'recipe_engine/nodejs',
    'recipe_engine/path',
    'recipe_engine/platform',
    'recipe_engine/properties',
    'recipe_engine/raw_io',
    'recipe_engine/resultdb',
    'recipe_engine/step',
    'depot_tools/bot_update',
    'depot_tools/gclient',
]


def RunSteps(api):
  assert api.platform.is_linux, 'Unsupported platform, only Linux is supported.'
  cl = api.buildbucket.build.input.gerrit_changes[0]
  project_name = cl.project
  assert project_name in ('infra/infra', 'infra/infra_internal',
                          'infra/luci/luci-go'), ('unknown project: "%s"' %
                                                  project_name)
  path = api.path.cache_dir / 'builder'
  api.file.ensure_directory('ensure builder dir', path)

  patch_root = project_name.split('/')[-1]
  with api.context(cwd=path):
    api.gclient.set_config(patch_root.replace("-", "_"))
    api.bot_update.ensure_checkout(patch_root=patch_root)
    api.gclient.runhooks()

  # Project => how to run its tests.
  runner = {
      'infra/infra': RunInfraFrontendTests,
      'infra/infra_internal': RunInfraInternalFrontendTests,
      'infra/luci/luci-go': RunLuciGoTests,
  }[project_name]
  repo_checkout_root = api.path.checkout_dir
  if project_name == 'infra/luci/luci-go':
    repo_checkout_root = api.path.checkout_dir / 'go/src/go.chromium.org/luci'

  # Read the desired nodejs version from <repo>/build/NODEJS_VERSION.
  version = api.file.read_text(
      'read NODEJS_VERSION',
      repo_checkout_root.joinpath('build', 'NODEJS_VERSION'),
      test_data='6.6.6\n',
  ).strip().lower()

  # Bootstrap nodejs at that version and run tests.
  with api.nodejs(version):
    runner(api, repo_checkout_root)


def RunInfraInternalFrontendTests(api, root_path):
  """This function runs UI tests in `infra_internal` project.
  """

  # Add your infra_internal tests here following this example:
  # cwd = api.path.checkout_dir.joinpath('path', 'to', 'ui', 'root')
  # RunFrontendTests(api, env, cwd, 'myapp')
  # `myapp` is the name that will show up in the step.

  testhaus = root_path.joinpath('go', 'src', 'infra_internal', 'appengine',
                                'testhaus', 'frontend', 'ui')
  RunNpmInstall(api, testhaus, 'testhaus')
  RunFrontendBuildAndLint(api, testhaus, 'testhaus')
  RunFrontendTests(api, testhaus, 'testhaus')

  cwd = root_path.joinpath('go', 'src', 'infra_internal', 'appengine', 'spike',
                           'appengine', 'frontend', 'ui')
  RunNpmInstall(api, cwd, 'spike')
  RunFrontendTests(api, cwd, 'spike')


def RunInfraFrontendTests(api, root_path):
  """This function runs the UI tests in `infra` project.
  """

  cwd = root_path.joinpath('go', 'src', 'infra', 'appengine', 'dashboard',
                           'frontend')
  RunNpmInstall(api, cwd, 'chopsdash')
  RunFrontendTests(api, cwd, 'chopsdash')

  cwd = root_path.joinpath('go', 'src', 'infra', 'appengine',
                           'chrome-test-health', 'frontend')
  RunNpmInstall(api, cwd, 'chrome-test-health')
  RunFrontendTests(api, cwd, 'chrome-test-health')


def RunLuciGoTests(api, root_path):
  """This function runs UI tests in the `luci-go` project.
  If the UI project is configured to test a build and be linted,
  then this function will run those commands on the UI too.
  """
  with api.context(cwd=root_path):
    if _has_changed_files(api, 'milo/ui'):
      RunMiloUITests(api, root_path)
    if _has_changed_files(api, 'swarming/server/ui2'):
      RunSwarmingUITests(api, root_path)


def _has_changed_files(api, path):
  result = api.m.git(
      'diff',
      '--name-only',
      '--cached',
      path,
      name='get change list on %s' % path,
      stdout=api.m.raw_io.output_text())
  files = result.stdout.splitlines()

  result.presentation.logs['change list'] = sorted(files)
  return len(files) > 0


def RunMiloUITests(api, root_path):
  cwd = root_path.joinpath('milo', 'ui')
  with api.step.nest('milo-ui'):
    RunNpmInstall(api, cwd, 'milo')
    BuildAndLintFrontend(api, cwd, 'milo')
    RunFrontendTests(
        api,
        cwd,
        'milo',
        module_name='infra/luci/luci-go > //milo/ui:jest_tests')


def RunNpmInstall(api, cwd, app_name):
  with api.context(cwd=cwd):
    api.step(('%s npm install' % app_name), [
      'npm', 'ci', '--loglevel', 'silly'])


def BuildAndLintFrontend(api, cwd, app_name):
  """Build and run ESLint on a UI.
     To use this step your UI must have:
       1. A `Makefile` that contains a `build` command.
       2. A `lint` npm script in the `package.json` file.
  """

  with api.context(cwd=cwd):
    api.step(('%s lint' % app_name), ['make', 'lint'])
    api.step(('%s build' % app_name), ['make', 'build'])


def RunFrontendTests(api, cwd, app_name, module_name=None):
  with api.context(cwd=cwd):
    if module_name:
      # ResultDB integration enabled.
      api.step(('%s test' % app_name),
               api.resultdb.wrap(['npm', 'run', 'test'],
                                 module_name=module_name,
                                 module_scheme='jest'))
    else:
      api.step(('%s test' % app_name), ['npm', 'run', 'test'])


def RunSwarmingUITests(api, root_path):
  ui_dir = root_path.joinpath('swarming', 'server', 'ui2')
  with api.context(cwd=ui_dir):
    api.step('swarming ui tests', ['make', 'presubmit_test'])


def RunFrontendBuildAndLint(api, cwd, app_name):
  with api.context(cwd=cwd):
    api.step(('%s lint' % app_name), ['npm', 'run', 'lint'])
    api.step(('%s build' % app_name), ['npm', 'run', 'build'])

def GenTests(api):

  def _step_data_changed_files(directory, files):
    return api.step_data(
        'get change list on %s' % directory,
        api.raw_io.stream_output_text('\n'.join(files)),
        stream='stdout')

  yield (
      api.test('basic') +
      api.buildbucket.try_build(project='infra/infra'))
  yield (
      api.test('basic-internal') +
      api.buildbucket.try_build(project='infra/infra_internal'))
  yield (api.test('basic-luci-go-milo') +
         api.buildbucket.try_build(project='infra/luci/luci-go') +
         _step_data_changed_files('milo/ui', ['milo/ui/foo.js']))
  yield (api.test('basic-luci-go-swarming') +
         api.buildbucket.try_build(project='infra/luci/luci-go') +
         _step_data_changed_files('swarming/server/ui2',
                                  ['swarming/server/ui2/foo.js']))
