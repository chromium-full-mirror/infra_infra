# Copyright 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import datetime
import json
from typing import Union

from google.protobuf import json_format

from recipe_engine import recipe_test_api

from PB.recipe_engine.recipes_cfg import RepoSpec, AutorollRecipeOptions

BOT_COMMIT_APPROVE = AutorollRecipeOptions.TrivialOptions.BOT_COMMIT_APPROVE

class RecipeAutorollerTestApi(recipe_test_api.RecipeTestApi):
  _TBR_EMAILS = ('foo@bar.example.com', 'meep@example.com')
  _EXTRA_REVIEWERS = ('foo@chromium.org', 'foo@bar.example.com',
                      'meep@example.com')

  def repo_spec(self,
                tbr_emails=_TBR_EMAILS,
                extra_reviewers=_EXTRA_REVIEWERS,
                disable_reason='',
                self_approve_method=BOT_COMMIT_APPROVE,
                trivial_commit=True,
                trivial_dryrun=False,
                nontrivial_dryrun=True,
                nontrivial_autosubmit=True,
                include_autoroll_options=True,
                no_cc_authors=False,
                recipes_path=""):
    spec = RepoSpec(api_version=2)
    spec.deps['recipe_engine'].url = (
      'https://chromium.googlesource.com/infra/luci/recipes-py')
    spec.recipes_path = recipes_path
    if include_autoroll_options:
      trivial = spec.autoroll_recipe_options.trivial
      trivial.tbr_emails.extend(tbr_emails)
      trivial.automatic_commit = trivial_commit
      trivial.dry_run = trivial_dryrun
      trivial.self_approve_method = self_approve_method

      nontrivial = spec.autoroll_recipe_options.nontrivial
      nontrivial.extra_reviewer_emails.extend(extra_reviewers)
      nontrivial.automatic_commit_dry_run = nontrivial_dryrun
      nontrivial.set_autosubmit = nontrivial_autosubmit

      spec.autoroll_recipe_options.disable_reason = disable_reason
      spec.autoroll_recipe_options.no_cc_authors = no_cc_authors
    return spec

  def roll_data(self,
                project,
                spec=None,
                success=True,
                trivial=True,
                empty=False,
                num_commits=1):
    """Returns mock roll and recipes.cfg data for |project|."""
    if spec is None:
      spec = self.repo_spec()
    if empty:
      success = False

    ret = self.empty_test_data() + self.recipe_cfg(project, spec)
    if spec.autoroll_recipe_options.disable_reason:
      return ret

    commit_infos = []
    for i in range(num_commits):
      commit_infos.append({
          'author_email': 'foo@chromium.org',
          'message_lines': [
              ('some commit summary that is too long to fit in a single '
               'line in Gerrit'),
              ('R=bar@chromium.org,baz@chromium.org,invalid1,'
               'invalid2@chromium'),
              'BUG=123,456',
          ],
          'revision': '%d23abc' % (i + 1),
      })

    picked_roll_details = {
        'commit_infos': {
            'recipe_engine': commit_infos,
        },
        'spec': json_format.MessageToDict(spec),
    }

    roll_result = {
      'success': success,
      'trivial': trivial if success else None,
      'picked_roll_details': picked_roll_details if success else None,
    }
    roll_result['rejected_candidates_count'] = 0
    if empty:
      roll_result['roll_details'] = []
    else:
      roll_result['roll_details'] = [picked_roll_details]
      if not success:
        roll_result['rejected_candidates_count'] = 1

    ret += self.step_data('%s.roll' % project, self.m.json.output(roll_result))
    return ret

  def gerrit_change(self, number: Union[str, int], trivial: bool,
                    timestamp: datetime.datetime):
    return self.m.gerrit.gerrit_change_data(
        number,
        hashtags=['trivial-roll' if trivial else 'nontrivial-roll'],
        created=timestamp.strftime('%Y-%m-%d %H:%M:%S.000000000'))

  def gerrit_changes(self, project: str, changes: list):
    return self.override_step_data(
        f'{project}.gerrit find changes',
        self.m.gerrit.get_multiple_changes_response_data(changes))

  def roll_status(self, project: str, status: str):
    return self.step_data(f'{project}.git cl status',
                          self.m.raw_io.stream_output_text(status))

  def repo_data(self, project: str, trivial: bool, status: str,
                timestamp: datetime.datetime):
    changes = [
        self.gerrit_change(222222, trivial, timestamp),
        self.gerrit_change(111111, trivial,
                           timestamp - datetime.timedelta(days=1))
    ]

    return self.gerrit_changes(project, changes) + self.roll_status(
        project, status)

  def recipe_cfg(self, project, spec=None):
    """Returns mock recipes.cfg data (only) for |project|.

    This is used for tests which abort between the 'read recipes.cfg' step and
    the 'roll' step (e.g. which read repo state and decide to quit early).

    For "normal" test runs, you'll want to use roll_data() from this
    RecipeTestApi, which includes this step data automatically.
    """
    if spec is None:
      spec = self.repo_spec()
    return self.override_step_data(
      '%s.read recipes.cfg' % project, self.m.file.read_proto(spec))
