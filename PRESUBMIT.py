# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Top-level presubmit script for infra.

See http://dev.chromium.org/developers/how-tos/depottools/presubmit-scripts for
details on the presubmit API built into gcl.
"""

import os

DISABLED_PYLINT_WARNINGS = [
    'no-init',                # Class has no __init__ method
    'super-init-not-called',  # __init__ method from base class is not called
    'cyclic-import',
    'unused-argument',
    'import-outside-toplevel',
    'inconsistent-return-statements',
    'no-member',
    'no-value-for-parameter',
    'stop-iteration-return',
    'subprocess-run-check',
    # TODO(crbug/1347377): Re-enable these checks.
    'no-else-return',
    'no-else-continue',
    'no-else-raise',
    'use-a-generator',
    'consider-using-in',
    'consider-iterating-dictionary',
    'arguments-differ',
    'useless-import-alias',
    'unused-variable',
    'pointless-statement',
    'redefined-outer-name',
    'line-too-long',
    'possibly-unused-variable',
    'useless-object-inheritance',
    'redefined-argument-from-local',
    'unnecessary-comprehension',
    'unnecessary-pass',
    'consider-using-set-comprehension',
    'dangerous-default-value',
    'redefined-builtin',
    'unidiomatic-typecheck',
    'multiple-statements',
    'logging-fstring-interpolation',
    'useless-super-delegation',
    'bare-except',
    'raising-format-tuple',
    'useless-return',
    'f-string-without-interpolation',
    'attribute-defined-outside-init',
    'try-except-raise',
    # Checks which should be re-enabled after Python 2 support is removed.
    'useless-object-inheritance',
    'super-with-arguments',
    'raise-missing-from',
    'keyword-arg-before-vararg',
]

DISABLED_PROJECTS = [
    # Taken care of by appengine/PRESUBMIT.py
    'appengine/*',
    'infra/services/lkgr_finder',

    # Don't bother pylinting (these could also move to .gitignore):
    '.*/__pycache__',
    '\.git',
    '\.wheelcache',
    '\.dockerbuild',
    'bootstrap/virtualenv-ext',
    'go/src/infra/tools/vpython/utils',
]

# List of third_party direcories.
THIRD_PARTY_DIRS = [
    r'appengine/third_party/.*',
    r'go/src/vendor/.*',
]

# List of directories to jshint.
JSHINT_PROJECTS = [
  'appengine/chromium_cq_status',
  'appengine/chromium_status',
  'appengine/milo',
]
# List of blacklisted directories that we don't want to jshint.
JSHINT_PROJECTS_BLACKLIST = THIRD_PARTY_DIRS

# Files that must not be modified (regex)
# Paths tested are relative to the directory containing this file.
# Ex: infra/libs/logs.py
NOFORK_PATHS = []


def CommandInGoEnv(input_api, output_api, name, cmd, kwargs, blocking=True):
  """Returns input_api.Command that wraps |cmd| with invocation to go/env.py.

  env.py makes golang tools available in PATH. It also bootstraps Golang dev
  environment if necessary.
  """
  if input_api.is_committing and blocking:
    error_type = output_api.PresubmitError
  else:
    error_type = output_api.PresubmitPromptWarning
  full_cmd = [
      input_api.python3_executable,
      input_api.os_path.join(input_api.change.RepositoryRoot(), 'go', 'env.py'),
  ]
  full_cmd.extend(cmd)
  return input_api.Command(
      name=name,
      cmd=full_cmd,
      kwargs=kwargs,
      message=error_type)


def GoCheckers(input_api, output_api):
  file_filter = lambda path: input_api.FilterSourceFile(
      path,
      files_to_check=[r'.*\.go$'],
      files_to_skip=THIRD_PARTY_DIRS + [r'.*\.pb\.go$', r'.*\.gen\.go$'])
  affected_files = sorted([
      f for f in input_api.AffectedFiles(
          include_deletes=False, file_filter=file_filter)
  ])
  if not affected_files:
    return []
  stdin = '\n'.join([f.AbsoluteLocalPath() for f in affected_files]).encode()

  tool_names = ['gofmt', 'govet']
  ret = []
  for tool_name in tool_names:
    cmd = [
        input_api.python3_executable,
        input_api.os_path.join(input_api.change.RepositoryRoot(), 'go',
                               'check.py'),
        tool_name,
    ]
    if input_api.verbose:
      cmd.append("--verbose")
    ret.append(
      CommandInGoEnv(
          input_api, output_api,
          name='Check %s (%d files)' % (tool_name, len(affected_files)),
          cmd=cmd,
          kwargs={'stdin': stdin}),
    )

    # Isinstance doesn't work since the GitChange class isn't accessible.
    if hasattr(input_api.change, 'UpstreamBranch'):
      since = ['--new-from-rev', input_api.change.UpstreamBranch()]
    else:
      since = []

    # In case of multiple files in the same directory, use dict to dedupe.
    dirs = {
        os.path.dirname(f.AbsoluteLocalPath()): os.path.dirname(f.LocalPath())
        for f in affected_files
    }
    for absolute, pretty in sorted(dirs.items()):
      ret.append(
          CommandInGoEnv(
              input_api,
              output_api,
              # TODO: --fix exists. Maybe we could either perform the fix for
              # them, or if the lint has a fix, inform them to run the command
              # with --fix.
              name=f'Check golint on {pretty}',
              # This command cannot be run from any directory.
              # Eg. for the package "infra/build/siso/build":
              # * `golangci-lint .` from `src/infra/build/sisa/build` works
              # * `golangci-lint siso/build` from `src/infra/build` works
              # * `golangci-lint build/siso/build` from `src/infra` works
              # * `golangci-lint infra/build/siso/build` from `src` doesn't

              # The easiest way to bypass this problem is to just always run
              # from the directory you're trying to lint.
              cmd=['golangci-lint', 'run', *since, '.'],
              kwargs={'cwd': absolute},
              # Revisit at some point after leaving on for a while.
              blocking=False))

  return ret


def GoCheckGoModTidy(input_api, output_api):
  def is_interesting(p):
    return p == 'DEPS' or p.endswith(('.go', 'go.mod', 'go.sum'))

  run = any(
      is_interesting(f.LocalPath())
      for f in input_api.change.AffectedFiles(include_deletes=True))
  if not run:
    return []

  # Don't run "go install ...", it may blow up with untidy go.mod.
  env = os.environ.copy()
  env['INFRA_GO_SKIP_TOOLS_INSTALL'] = '1'

  root = input_api.change.RepositoryRoot()
  return input_api.RunTests([
      CommandInGoEnv(
          input_api,
          output_api,
          name='Check go.mod tidiness',
          cmd=[
              input_api.python3_executable,
              os.path.join(
                  root,
                  'go',
                  'src',
                  'go.chromium.org',
                  'luci',
                  'scripts',
                  'check_go_mod_tidy.py',
              ),
              os.path.join(root, 'go', 'src', 'infra'),
          ],
          kwargs={'env': env}),
  ])


def IgnoredPaths(input_api): # pragma: no cover
  # This computes the list if repository-root-relative paths which are
  # ignored by .gitignore files. There is probably a faster way to do this.
  status_output = input_api.subprocess.check_output(
      ['git', 'status', '--porcelain', '--ignored'], text=True)
  statuses = [(line[:2], line[3:]) for line in status_output.splitlines()]
  return [
    input_api.re.escape(path) for (mode, path) in statuses
    if mode in ('!!', '??') and not path.endswith('.pyc')
  ]


def NoForkCheck(input_api, output_api): # pragma: no cover
  """Warn when a file that should not be modified is modified.

  This is useful when a file is to be moved to a different place
  and is temporarily copied to preserve backward compatibility. We don't
  want the original file to be modified.
  """
  black_list_re = [input_api.re.compile(regexp) for regexp in NOFORK_PATHS]
  offending_files = []
  for filename in input_api.AffectedTextFiles():
    if any(regexp.search(filename.LocalPath()) for regexp in black_list_re):
      offending_files.append(filename.LocalPath())
  if offending_files:
    return [output_api.PresubmitPromptWarning(
      'You modified files that should not be modified. Look for a NOFORK file\n'
      + 'in a directory above those files to get more context:\n%s'
      % '\n'.join(offending_files)
      )]
  return []


def BrokenLinksChecks(input_api, output_api):  # pragma: no cover
  """Complains if there are broken committed symlinks."""
  stdout = input_api.subprocess.check_output(['git', 'ls-files'], text=True)
  files = stdout.splitlines()
  output = []
  infra_root = input_api.PresubmitLocalPath()
  for filename in files:
    fullname = input_api.os_path.join(infra_root, filename)
    if (input_api.os_path.lexists(fullname)
        and not input_api.os_path.exists(fullname)):
      output.append(output_api.PresubmitError('Broken symbolic link: %s'
                                              % filename))
  return output


def PylintChecks(input_api, output_api, only_changed):  # pragma: no cover
  py2_files = [
      r'^bootstrap/.*\.py$',
      r'^build/.*\.py$',
      r'^infra/tools/bucket/.*\.py$',
      r'^infra/tools/dockerbuild/.*\.py$',
      r'^infra/tools/new_tool/.*\.py$',
      r'^infra/tools/zip_release_commits/.*\.py$',
      r'^go/src/infra/tools/cloudtail/.*\.py$',
  ]
  tests = []
  tests.extend(PylintChecksForPython3(
        input_api, output_api, exclude=py2_files))
  return tests


def PylintChecksForPython3(input_api, output_api, exclude):  # pragma: no cover
  files_to_skip = list(input_api.DEFAULT_FILES_TO_SKIP)
  files_to_skip += exclude
  files_to_skip += DISABLED_PROJECTS
  files_to_skip += IgnoredPaths(input_api)
  files_to_skip += input_api.ListSubmodules()
  files_to_skip += [
      r'.*_pb2\.py$',

      # Tests are expected to trigger pylint warnings
      r'go/src/infra/tricium/functions/pylint/test/.*\.py$',

      # TODO(phajdan.jr): pylint recipes-py code (http://crbug.com/617939).
      r'^recipes/recipes\.py$'
  ]

  return input_api.canned_checks.GetPylint(
      input_api,
      output_api,
      files_to_skip=files_to_skip,
      disabled_warnings=DISABLED_PYLINT_WARNINGS,
      version='2.7',
  )


def GetAffectedJsFiles(input_api, include_deletes=False):
  """Returns a list of absolute paths to modified *.js files."""
  infra_root = input_api.PresubmitLocalPath()
  whitelisted_paths = [
      input_api.os_path.join(infra_root, path)
      for path in JSHINT_PROJECTS]
  blacklisted_paths = [
      input_api.os_path.join(infra_root, path)
      for path in JSHINT_PROJECTS_BLACKLIST]

  def keep_whitelisted_files(affected_file):
    return any([
        affected_file.AbsoluteLocalPath().startswith(whitelisted_path) and
        all([
            not affected_file.AbsoluteLocalPath().startswith(blacklisted_path)
            for blacklisted_path in blacklisted_paths
        ])
        for whitelisted_path in whitelisted_paths
    ])
  return sorted(
      f.AbsoluteLocalPath()
      for f in input_api.AffectedFiles(
          include_deletes=include_deletes, file_filter=keep_whitelisted_files)
      if f.AbsoluteLocalPath().endswith('.js'))


def JshintChecks(input_api, output_api):  # pragma: no cover
  """Runs Jshint on all .js files under appengine/."""
  infra_root = input_api.PresubmitLocalPath()
  node_jshint_path = input_api.os_path.join(infra_root, 'node', 'jshint.py')

  tests = []
  for js_file in GetAffectedJsFiles(input_api):
    cmd = [input_api.python3_executable, node_jshint_path, js_file]
    tests.append(input_api.Command(
        name='Jshint %s' % js_file,
        cmd=cmd,
        kwargs={},
        message=output_api.PresubmitError))
  return tests


# string pattern, sequence of strings to show when pattern matches,
# error flag. True if match is a presubmit error, otherwise it's a warning.
_NON_INCLUSIVE_TERMS = (
    (
        # Note that \b pattern in python re is pretty particular. In this
        # regexp, 'class WhiteList ...' will match, but 'class FooWhiteList
        # ...' will not. This may require some tweaking to catch these cases
        # without triggering a lot of false positives. Leaving it naive and
        # less matchy for now.
        r'/(?i)\b((black|white)list|master|slave)\b',  # nocheck
        (
            'Please don\'t use blacklist, whitelist, '  # nocheck
            'master, or slave in your',  # nocheck
            'code and make every effort to use other terms. Using "// nocheck"',
            'at the end of the offending line will bypass this PRESUBMIT error',
            'but avoid using this whenever possible. Reach out to',
            'community@chromium.org if you have questions'),
        True),)


def CheckInclusiveLanguage(input_api, output_api):
  """Make sure that banned non-inclusive terms are not used."""
  results = []
  results.extend(
      input_api.canned_checks.CheckInclusiveLanguage(
          input_api,
          output_api,
          excluded_directories_relative_path=[
              'infra', 'inclusive_language_presubmit_exempt_dirs.txt'
          ],
          non_inclusive_terms=_NON_INCLUSIVE_TERMS))
  return results


def GoChecks(input_api, output_api):  # pragma: no cover
  # "go.mod" tidiness test needs to run first because if go.mod is untidy,
  # the rest of Go tests may blow up in a noisy way. Running this check also
  # bootstraps the go environment, but without tools. The tools are installed
  # later in `bootstrap go env` step.
  output = GoCheckGoModTidy(input_api, output_api)
  if any(x.fatal for x in output):
    return output, []

  # Collect all other potential Go tests
  tests = GoCheckers(input_api, output_api)
  if not tests:
    return output, []

  # depot_tools runs tests in parallel. If go env is not setup, each test will
  # attempt to bootstrap it simultaneously, which doesn't currently work
  # correctly.
  #
  # Because we use RunTests here, this will run immediately. The actual go
  # tests will run after this, assuming the bootstrap is successful.
  output.extend(
      input_api.RunTests([
          input_api.Command(
              name='bootstrap go env',
              cmd=[
                  input_api.python3_executable,
                  input_api.os_path.join(input_api.change.RepositoryRoot(),
                                         'go', 'bootstrap.py')
              ],
              kwargs={},
              message=output_api.PresubmitError)
      ]))
  if any(x.fatal for x in output):
    return output, []
  return output, tests


def CommonChecks(input_api, output_api):  # pragma: no cover
  output, tests = GoChecks(input_api, output_api)

  file_filter = lambda x: x.LocalPath() == 'infra/config/recipes.cfg'
  output.extend(
      input_api.canned_checks.CheckJsonParses(
          input_api, output_api, file_filter=file_filter))

  # Add non-go tests
  tests += JshintChecks(input_api, output_api)

  # Run all the collected tests
  if tests:
    output.extend(input_api.RunTests(tests))

  output.extend(BrokenLinksChecks(input_api, output_api))

  third_party_filter = lambda path: input_api.FilterSourceFile(
      path, files_to_skip=THIRD_PARTY_DIRS)
  output.extend(input_api.canned_checks.CheckGenderNeutral(
      input_api, output_api, source_file_filter=third_party_filter))
  output.extend(
      input_api.canned_checks.CheckPatchFormatted(
          input_api, output_api, check_clang_format=False))
  output.extend(
      input_api.canned_checks.PanProjectChecks(
          input_api, output_api, excluded_paths=[r'.*python_pb2/.*_pb2\.py$']))

  files_to_skip = list(input_api.DEFAULT_FILES_TO_SKIP) + THIRD_PARTY_DIRS + [
      r'.*pb[^/]*\.go$',
  ]
  files_to_check = list(input_api.DEFAULT_FILES_TO_CHECK) + [
      r'.+\.go$',
  ]

  output.extend(
      input_api.canned_checks.CheckLicense(
          input_api,
          output_api,
          source_file_filter=lambda x: input_api.FilterSourceFile(
              x, files_to_check=files_to_check, files_to_skip=files_to_skip)))

  return output


def CheckChangeOnUpload(input_api, output_api):  # pragma: no cover
  output = CommonChecks(input_api, output_api)
  output.extend(input_api.RunTests(
    PylintChecks(input_api, output_api, only_changed=True)))
  output.extend(NoForkCheck(input_api, output_api))
  output.extend(CheckInclusiveLanguage(input_api, output_api))
  return output


def CheckChangeOnCommit(input_api, output_api):  # pragma: no cover
  output = CommonChecks(input_api, output_api)
  output.extend(input_api.RunTests(
    PylintChecks(input_api, output_api, only_changed=False)))
  return output
