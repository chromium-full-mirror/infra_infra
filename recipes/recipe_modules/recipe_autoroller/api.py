# Copyright 2018 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import base64
from dataclasses import dataclass
import datetime
import re
import traceback
from typing import Optional
import urllib.parse

from google.protobuf import json_format as jsonpb

from recipe_engine import recipe_api
from PB.go.chromium.org.luci.buildbucket.proto import common as common_pb2
from PB.recipe_engine.recipes_cfg import (AutorollRecipeOptions, DepRepoSpecs,
                                          RepoSpec)
from PB.recipe_engine import result as result_pb2

_TRIVIAL_ROLL_HASHTAG = 'trivial-roll'
_NONTRIVIAL_ROLL_HASHTAG = 'nontrivial-roll'
_RECIPE_DEP_ROLL_HASHTAG = 'recipe-dep-roll'


class RepoData(object):
  _TIME_FORMAT = '%Y-%m-%dT%H:%M:%S'

  def __init__(self, issue, issue_url, trivial, last_roll_ts_utc):
    assert isinstance(issue, str)
    assert isinstance(issue_url, str)
    assert isinstance(trivial, bool)
    assert isinstance(last_roll_ts_utc, datetime.datetime)

    self.issue = issue
    self.issue_url = issue_url
    self.trivial = trivial
    self.last_roll_ts_utc = last_roll_ts_utc

  @classmethod
  def from_json(cls, obj):  # pragma: no cover
    return cls(
      obj['issue'],
      obj['issue_url'],
      obj['trivial'],
      datetime.datetime.strptime(obj['last_roll_ts_utc'], cls._TIME_FORMAT),
    )

  def to_json(self):
    return {
      'issue': self.issue,
      'issue_url': self.issue_url,
      'trivial': self.trivial,
      'last_roll_ts_utc': self.last_roll_ts_utc.strftime(self._TIME_FORMAT),
    }


COMMIT_MESSAGE_HEADER = ("""
This is an automated CL created by the recipe roller. This CL rolls
recipe changes from upstream projects (%(roll_projects)s) into this repository.

The build that created this CL was
https://ci.chromium.org/b/%(build_id)s
""")

NON_TRIVIAL_MESSAGE = ("""
Please review the expectation changes, and LGTM+CQ.
""")

COMMIT_MESSAGE_INFO = ("""
Please check the following references for more information:
- autoroller, https://chromium.googlesource.com/infra/luci/recipes-py/+/main/doc/workflow.md#autoroller
- rollback, https://chromium.googlesource.com/infra/luci/recipes-py/+/main/doc/workflow.md#rollback
- cross-repo dependencies, https://chromium.googlesource.com/infra/luci/recipes-py/+/main/doc/cross_repo.md

Use https://goo.gl/noib3a to file a bug.
""")

COMMIT_MESSAGE_FOOTER = ("""
Recipe-Tryjob-Bypass-Reason: Autoroller
Ignore-Freeze: Autoroller
Bugdroid-Send-Email: False
""")


# These are different results of a roll attempt:
#   - success means we have a working non-empty roll
#   - empty means the repo is using latest revision of its dependencies
#   - failure means there are roll candidates but none of them are suitable
#     for an automated roll
#   - skip means that the roll was skipped (not processed). This can happen if
#     the repo has a 'disable_message' in its autoroll_recipe_options.
ROLL_SUCCESS, ROLL_EMPTY, ROLL_FAILURE, ROLL_SKIP = range(4)


@dataclass
class _Status(object):
  code: int
  url: str = ""


_ROLL_STALE_THRESHOLD = datetime.timedelta(hours=2)


def _gs_path(project_url):
  return 'repo_metadata/%s' % base64.urlsafe_b64encode(
      project_url.encode()).decode()


def get_commit_message(roll_result, build_id):
  """Construct a roll commit message from 'recipes.py autoroll' result.
  """
  picked = roll_result['picked_roll_details']
  commit_infos = picked['commit_infos']
  deps = picked['spec']['deps']
  roll_projects = sorted(commit_infos.keys())
  trivial = roll_result['trivial']

  message = 'Roll recipe dependencies (%s).\n' % (
      'trivial' if trivial else 'nontrivial')

  message += COMMIT_MESSAGE_HEADER % dict(
      roll_projects=', '.join(roll_projects), build_id=build_id)

  if not trivial:
    message += NON_TRIVIAL_MESSAGE

  blame = []
  for project, commits in commit_infos.items():
    blame.append('')
    blame.append('%s:' % project)
    remote = deps[project]['url']
    if len(commits) == 1:
      blame.append('%s/+/%s' % (remote, commits[0]['revision']))
    else:
      blame.append('%s/+log/%s~..%s' %
                   (remote, commits[0]['revision'], commits[-1]['revision']))
    for commit in commits:
      blame.append('  %s (%s)' %
                   (commit['revision'][:7], commit['author_email']))
      message_lines = commit['message_lines']
      summary_line = '      %s' % message_lines[0] if message_lines else 'n/a'
      max_line_length = 72
      if len(summary_line) > max_line_length:
        summary_line = summary_line[:max_line_length - 3].rstrip() + '...'
      blame.append(summary_line)

  message += ''.join(l + '\n' for l in blame)
  message += COMMIT_MESSAGE_INFO
  message += COMMIT_MESSAGE_FOOTER
  return message


def get_summary_markdown(roll_results):
  links = []
  for result in roll_results:
    if not result.url:
      continue

    try:
      host = re.search(r'/([\w-]+)-review\.', result.url).group(1)
      number = re.search(r'/(\d+)$', result.url).group(1)
      links.append('[{}:{}]({})'.format(host, number, result.url))
    except AttributeError:  # pragma: no cover
      pass

  return result_pb2.RawResult(
      summary_markdown='\n'.join('* {}'.format(x) for x in links),
      status=common_pb2.SUCCESS,
  )


class RecipeAutorollerApi(recipe_api.RecipeApi):
  def roll_projects(self, projects, db_gcs_bucket):
    """Attempts to roll each project from the provided list.

    If rolling any of the projects leads to failures, other
    projects are not affected.

    Args:
      projects: list of tuples of
        project_id (string): id as found in recipes.cfg.
        project_url (string): Git repository URL of the project.
        db_gcs_bucket (string): The GCS bucket used as a database for previous
          roll attempts.
    """
    futures = []
    for project_id, project_url in projects:
      future = self.m.futures.spawn(self._roll_project, project_id, project_url,
                                    db_gcs_bucket)
      futures.append((project_id, future))

    failed_rolls = []
    for project_id, future in futures:
      if future.exception() is not None:
        failed_rolls.append(project_id)

    if failed_rolls:
      raise self.m.step.StepFailure(
          'Rolls failed for the following projects: {}'.format(
              ', '.join(failed_rolls)))

    results = [f.result() for _, f in futures]
    result_codes = [x.code for x in results]

    # Failures to roll are OK as long as at least one of the repos is moving
    # forward. For example, with repos with following dependencies:
    #
    #   A    <- B
    #   A, B <- C
    #
    # New commit in A repo will need to get rolled into B first. However,
    # it'd also appear as a candidate for C roll, leading to a failure there.
    if ROLL_FAILURE in result_codes and ROLL_SUCCESS not in result_codes:
      self.m.step.empty(
          'roll result',
          status=self.m.step.FAILURE,
          step_text='manual intervention needed: automated roll attempt failed')

    return get_summary_markdown(results)

  def _prepare_checkout(self, project_id, project_url):
    # Keep persistent checkout. Speeds up the roller for large repos
    # like chromium/src.
    workdir = self.m.path.cache_dir.joinpath('builder', 'recipe_autoroller',
                                             project_id)
    self.m.git.checkout(
        project_url, dir_path=workdir, submodules=False, ref='main')

    with self.m.context(cwd=workdir):
      # On LUCI user.email is already configured to match that of task service
      # account with which we'll be authenticating to Git/Gerrit.
      # Set a nicer name than service account's long email.
      self.m.git('config', 'user.name', 'recipe-roller')

      # Clean up possibly left over roll branch. Ignore errors.
      self.m.git('branch', '-D', 'roll', ok_ret='any')

      # git cl upload cannot work with detached HEAD, it requires a branch.
      with self.m.depot_tools.on_path():
        self.m.git('new-branch', 'roll', '--upstream', 'origin/main')

    return workdir

  def _check_previous_roll(self, project_url, workdir):
    # Check status of last known CL for this repo. Ensure there's always
    # at most one roll CL in flight.
    repo_data, cl_status = self._get_pending_cl_status(project_url, workdir)
    if repo_data:
      last_roll_elapsed = self.m.time.utcnow() - repo_data.last_roll_ts_utc

      # Allow trivial rolls in CQ to finish.
      if repo_data.trivial and cl_status == 'commit':
        if (last_roll_elapsed and
            last_roll_elapsed > _ROLL_STALE_THRESHOLD):
          self.m.step.empty(
              'stale roll',
              status=self.m.step.FAILURE,
              step_text='manual intervention needed: automated roll attempt is '
              'stale')

        return ROLL_SUCCESS

      # Allow non-trivial rolls to wait for review comments.
      if not repo_data.trivial and cl_status != 'closed':
        if (last_roll_elapsed and
            last_roll_elapsed > _ROLL_STALE_THRESHOLD):
          self.m.step.empty(
              'stale roll',
              status=self.m.step.FAILURE,
              step_text='manual intervention needed: automated roll attempt is '
              'stale')

        return ROLL_SUCCESS

      # TODO(phajdan.jr): detect staleness by creating CLs in a loop.
      # It's possible that the roller keeps creating new CLs (especially
      # trivial rolls), but they e.g. fail to land, causing staleness.

      # We're about to upload a new CL, so make sure the old one is closed.
      if cl_status != 'closed':
        with self.m.context(cwd=workdir):
          self.m.git_cl('set-close', ['--issue', repo_data.issue],
                        name='git cl set-close')
    return None

  def _roll_project(self, project_id, project_url, db_gcs_bucket):
    with self.m.step.nest(str(project_id)) as presentation:
      try:
        return self._roll_project_impl(
            project_id, project_url, db_gcs_bucket)
      except Exception:
        presentation.logs['exception'] = traceback.format_exc()
        raise

  def _roll_project_impl(self, project_id, project_url, db_gcs_bucket):
    # Keep persistent checkout. Speeds up the roller for large repos
    # like chromium/src.
    workdir = self._prepare_checkout(project_id, project_url)

    recipes_cfg_path = workdir.joinpath('infra', 'config', 'recipes.cfg')
    recipes_cfg = self.m.file.read_proto(
        'read recipes.cfg', recipes_cfg_path, RepoSpec, 'JSONPB',
        decoding_kwargs = {'ignore_unknown_fields': True})

    if reason := recipes_cfg.autoroll_recipe_options.disable_reason:
      rslt = self.m.step.empty('disabled', step_text=reason)
      rslt.presentation.status = self.m.step.WARNING
      return _Status(ROLL_SKIP)

    status = self._check_previous_roll(project_url, workdir)
    if status is not None:
      # This means that the previous roll is still going, or similar. In this
      # situation we're done with this repo, for now.
      return _Status(status)

    recipes_py = workdir.joinpath(recipes_cfg.recipes_path, 'recipes.py')
    roll_step = self.m.step('roll', [
        'vpython3', recipes_py, '--package', recipes_cfg_path,
        '-vv', 'autoroll', '--output-json',
        self.m.json.output()
    ])
    roll_result = roll_step.json.output

    if roll_result['success'] and roll_result['picked_roll_details']:
      issue_result = self._process_successful_roll(
          project_url, roll_step, workdir, recipes_py, recipes_cfg_path,
          db_gcs_bucket)
      return _Status(ROLL_SUCCESS, issue_result['issue_url'])

    num_rejected = roll_result['rejected_candidates_count']
    if not roll_result['roll_details'] and num_rejected == 0:
      roll_step.presentation.step_text += ' (already at latest revisions)'
      return _Status(ROLL_EMPTY)

    for i, roll_candidate in enumerate(roll_result['roll_details']):
      roll_step.presentation.logs['candidate #%d' % (i + 1)] = (
          self.m.json.dumps(roll_candidate['spec'], indent=2))

    return _Status(ROLL_FAILURE)

  def _process_successful_roll(self, project_url, roll_step, workdir,
                               recipes_py, recipes_cfg_path, db_gcs_bucket):
    """
    Args:
      roll_step - The StepResult of the actual roll command. This is used to
        adjust presentation and obtain the json output.
    """
    roll_result = roll_step.json.output
    picked_details = roll_result['picked_roll_details']

    spec = jsonpb.ParseDict(picked_details['spec'], RepoSpec())

    upload_args = ['--send-mail', '--hashtag', _RECIPE_DEP_ROLL_HASHTAG]
    if roll_result['trivial']:
      upload_args.extend(('--hashtag', _TRIVIAL_ROLL_HASHTAG))
      s = spec.autoroll_recipe_options.trivial
      opts = AutorollRecipeOptions.TrivialOptions
      if self.m.led.launched_by_led:
        # No-op to ensure that we require code coverage for this branch.
        # If launched by led then we shouldn't self-approve.
        pass
      elif s.self_approve_method == opts.CODE_REVIEW_1_APPROVE:
        upload_args.extend(['-o', '-l=Code-Review+1'])
      elif s.self_approve_method == opts.CODE_REVIEW_2_APPROVE:
        upload_args.extend(['-o', '-l=Code-Review+2'])
      elif s.self_approve_method == opts.NO_LABELS_APPROVE:
        # No-op to ensure that we require code coverage for this branch.
        pass
      else:
        upload_args.append('--set-bot-commit')

      if s.tbr_emails:
        upload_args.extend(['-r', self.m.random.choice(s.tbr_emails)])

      if s.automatic_commit:
        if self.m.led.launched_by_led:
          # Since we're not self-approving for led builds, don't CQ+2 either.
          upload_args.append('--cq-dry-run')
        else:
          upload_args.append('--use-commit-queue')
      else:
        if not spec.autoroll_recipe_options.no_owners:
          upload_args.append('--r-owners')
        if s.dry_run:
          upload_args.append('--cq-dry-run')
    else:
      upload_args.extend(('--hashtag', _NONTRIVIAL_ROLL_HASHTAG))
      s = spec.autoroll_recipe_options.nontrivial
      if s.extra_reviewer_emails:
        upload_args.append('--reviewers=%s' % ','.join(s.extra_reviewer_emails))
      if not spec.autoroll_recipe_options.no_owners:
        upload_args.append('--r-owners')

      if s.automatic_commit_dry_run:
        upload_args.append('--cq-dry-run')
      if s.set_autosubmit:
        upload_args.append('--enable-auto-submit')

    upload_args.extend(['--bypass-hooks', '-f'])

    commit_message = get_commit_message(roll_result,
                                        self.m.buildbucket.build.id)

    roll_step.presentation.logs['commit_message'] = commit_message.splitlines()
    if roll_result['trivial']:
      roll_step.presentation.step_text += ' (trivial)'
    else:
      roll_step.presentation.status = self.m.step.FAILURE

    dep_specs = None
    try:
      dep_specs = self.m.step(
          'get deps',
          [
              'vpython3',
              recipes_py,
              '--package',
              recipes_cfg_path,
              'dump_specs',
          ],
          stdout=self.m.proto.output(DepRepoSpecs, codec='JSONPB'),
          step_test_data=lambda: self.m.proto.test_api.output_stream(
              DepRepoSpecs(repo_specs={'recipe_engine': RepoSpec()})),
      ).stdout
    except self.m.step.StepFailure:
      # TODO(fxbug.dev/54380): delete this `except` after crrev.com/c/2252547
      # has rolled into all downstream repos that are rolled by an autoroller.
      pass

    cc_list = set()
    for dep, commits in picked_details['commit_infos'].items():
      if dep_specs:
        dep_spec = dep_specs.repo_specs[dep]
        if dep_spec.autoroll_recipe_options.no_cc_authors:
          continue
      for commit in commits:
        cc_list.add(commit['author_email'])
    if cc_list:
      upload_args.append('--cc=%s' % ','.join(sorted(cc_list)))

    with self.m.context(cwd=workdir):
      self.m.git('commit', '-a', '-m', 'roll recipes.cfg')
      self.m.git_cl.upload(
          commit_message, upload_args, name='git cl upload')
      issue_step = self.m.git_cl(
          'issue', ['--json', self.m.json.output()],
          name='git cl issue',
          step_test_data=lambda: self.m.json.test_api.output({
              'issue': 123456789,
              'issue_url': 'https://code-review.googlesource.com/123456789'}))
    issue_result = issue_step.json.output

    if not issue_result['issue'] or not issue_result['issue_url']:
      self.m.step.empty(
          'git cl upload failed',
          status=self.m.step.FAILURE,
          step_text='no issue metadata returned')

    repo_data = RepoData(
      str(issue_result['issue']),
      issue_result['issue_url'],
      roll_result['trivial'],
      self.m.time.utcnow(),
    )

    issue_step.presentation.links['Issue %s' % repo_data.issue] = (
        repo_data.issue_url)

    # TODO: b/40275665 - Stop uploading to GCS.
    self.m.gsutil.upload(
        self.m.json.input(repo_data.to_json()), db_gcs_bucket,
        _gs_path(project_url))

    return issue_result

  def _get_pending_cl_status(self, project_url, workdir):
    """Returns (current_repo_data, git_cl_status_string) of the last known
    roll CL for given repo.

    If no such CL has been recorded, returns (None, None).
    """
    url = urllib.parse.urlparse(project_url)
    host = f'{url.scheme}://{url.netloc}'
    host = host.replace(".googlesource.com", "-review.googlesource.com")
    project = url.path.lstrip('/').removesuffix('.git')

    # We're only considering the first change returned to see if CV is running,
    # but we will abandon additional matching changes.
    changes = self.m.gerrit.get_changes(
        host,
        query_params=(
            ('is', 'open'),
            ('owner', 'self'),
            ('project', project),
            ('hashtag', _RECIPE_DEP_ROLL_HASHTAG),
        ),
        name='find changes',
        limit=10,
        step_test_data=self.m.gerrit.test_api.get_empty_changes_response_data,
    )

    if not changes:
      return None, None

    # The created time is still a str and not a datetime but it should still
    # sort fine. Gerrit will sort these too, but we explicitly want to sort by
    # creation time, not the time the change was last modified. If those differ,
    # it's likely because a human touched a (possibly old) change.
    changes.sort(key=lambda x: x['created'], reverse=True)

    repo_data: Optional[RepoData] = None

    for i, change in enumerate(changes):
      number = change['_number']
      if i == 0:
        # Only consider the first one as possibly "current".
        created_str = re.sub(r'\.0+$', '', change['created'])
        created = datetime.datetime.strptime(created_str, '%Y-%m-%d %H:%M:%S')

        repo_data = RepoData(
            issue=str(number),
            issue_url=f'{host}/c/{project}/+/{number}',
            trivial=_TRIVIAL_ROLL_HASHTAG in change['hashtags'],
            last_roll_ts_utc=created,
        )

      else:
        # Just abandon other matching changes.
        self.m.gerrit.abandon_change(
            host, change=number, name=f'abandon {number}',
            message=self.m.buildbucket.build_url(),
        )

    step = self.m.step.empty('issue')
    step.presentation.links['Issue %s' % repo_data.issue] = repo_data.issue_url
    if repo_data.trivial:
      step.presentation.step_text += ' (trivial)'

    with self.m.context(cwd=workdir):
      status_result = self.m.git_cl(
          'status', ['--issue', repo_data.issue, '--field', 'status'],
          name='git cl status',
          stdout=self.m.raw_io.output_text(),
          step_test_data=lambda: self.m.raw_io.test_api.stream_output_text(
              'foo')).stdout.strip()
      self.m.step.active_result.presentation.step_text = status_result

    return repo_data, status_result
