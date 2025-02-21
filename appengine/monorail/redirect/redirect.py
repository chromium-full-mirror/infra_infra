# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Redirect Middleware for Monorail.

Handles traffic redirection before hitting main monorail app.
"""
import json

import flask

from redirect import redirect_utils
from redirect import redirectissue


def GenerateRedirectApp():
  redirect_app = flask.Flask(__name__)

  def PreCheckHandler():
    # Should not redirect away from monorail if param set.
    r = flask.request
    no_redirect = 'no_tracker_redirect' in r.args or 'ntr' in r.args
    if no_redirect:
      flask.abort(404)
  redirect_app.before_request(PreCheckHandler)

  def IssueList(project_name: str):
    redirect_url = redirect_utils.GetRedirectURL(project_name)
    if redirect_url:
      query_string = redirect_utils.GetSearchQuery(
          project_name, flask.request.args)
      return flask.redirect(redirect_url + '/issues?' + query_string)
    flask.abort(404)

  redirect_app.route('/p/<string:project_name>/')(IssueList)
  redirect_app.route('/p/<string:project_name>/issues/')(IssueList)
  redirect_app.route('/p/<string:project_name>/issues/list')(IssueList)
  redirect_app.route('/p/<string:project_name>/issues/list_new')(IssueList)

  def IssueDetail(project_name: str):
    local_id = flask.request.args.get('id', type=int)
    if not local_id:
      flask.abort(404)

    redirect_url = _GenerateIssueDetailRedirectURL(local_id, project_name)
    if redirect_url:
      return flask.render_template('redirect.html', base_url=redirect_url)
    flask.abort(404)
  redirect_app.route('/p/<string:project_name>/issues/detail')(IssueDetail)

  def IssueCreate(project_name: str):
    redirect_url = redirect_utils.GetRedirectURL(project_name)
    if redirect_url:
      query_string = redirect_utils.GetNewIssueParams(
          flask.request.args, project_name)
      return flask.redirect(redirect_url + '/new?' + query_string)
    flask.abort(404)
  redirect_app.route('/p/<string:project_name>/issues/entry')(IssueCreate)
  redirect_app.route('/p/<string:project_name>/issues/entry_new')(IssueCreate)

  def IssueWizard(project_name: str):
    return flask.redirect('https://issues.chromium.org/issues/wizard')
  redirect_app.route('/p/<string:project_name>/issues/wizard')(IssueWizard)

  # RESTful API that maps Monorail local issue IDs to redirected issue IDs.
  def MappingApi(project_name: str, local_id: int) -> str:
    redirect_id = redirectissue.Get(project_name, local_id)
    if redirect_id:
      return str(redirect_id)
    flask.abort(404)
  redirect_app.route('/<string:project_name>/<int:local_id>')(MappingApi)

  def Handle404(e):
    return flask.render_template('404.html'), 404

  redirect_app.register_error_handler(404, Handle404)

  return redirect_app


def _GenerateIssueDetailRedirectURL(local_id: int, project_name: str):
  log = {
      'action': 'redirect/issue',
      'project': project_name,
      'original_id': local_id,
  }

  redirect_base_url = redirect_utils.GetRedirectURL(project_name)
  if not redirect_base_url:
    print(json.dumps({**log, 'type': 'unknown_project'}))
    return None

  if local_id >= redirect_utils.MIN_ISSUETRACKER_ISSUE_ID:
    print(json.dumps({**log, 'type': 'passthrough', 'redirect_id': local_id}))
    return redirect_base_url + '/' + str(local_id)

  tracker_id = redirectissue.Get(project_name, local_id)
  if not tracker_id:
    print(json.dumps({**log, 'type': 'unknown_issue'}))
    return None

  if tracker_id < redirect_utils.MIN_ISSUETRACKER_ISSUE_ID:
    print(json.dumps({**log, 'type': 'launch', 'redirect_id': tracker_id}))
    return redirect_utils.LAUNCH_BASE_URL + '/' + str(tracker_id)

  print(json.dumps({**log, 'type': 'issuetracker', 'redirect_id': tracker_id}))
  return redirect_base_url + '/' + str(tracker_id)
