# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Redirect Middleware for Monorail.

Handles traffic redirection before hitting main monorail app.
"""
from __future__ import print_function
from __future__ import division
from __future__ import absolute_import

import flask
from redirect import redirect_utils
from redirect import redirectissue


class RedirectMiddleware(object):

  def __init__(self, main_app, redirect_app):
    self._main_app = main_app
    self._redirect_app = redirect_app

  def __call__(self, environ, start_response):
    # Run the redirect app first.
    response = flask.Response.from_app(self._redirect_app, environ)
    if response.status_code == 404:
      # If it returns 404, run the main app.
      return self._main_app(environ, start_response)
    # Otherwise, return the response from the redirect app.
    app_iter, status, headers = response.get_wsgi_response(environ)
    start_response(status, headers)
    return app_iter


def GenerateRedirectApp():
  redirect_app = flask.Flask(__name__)

  def PreCheckHandler():
    # Should not redirect away from monorail if param set.
    r = flask.request
    no_redirect = 'no_tracker_redirect' in r.args or 'ntr' in r.args
    if no_redirect:
      flask.abort(404)
  redirect_app.before_request(PreCheckHandler)

  def IssueList(project_name):
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

  def IssueDetail(project_name):
    local_id = flask.request.args.get('id', type=int)
    if not local_id:
      flask.abort(404)

    redirect_url = _GenerateIssueDetailRedirectURL(local_id, project_name)
    if redirect_url:
      return flask.render_template('redirect.html', base_url=redirect_url)
    flask.abort(404)
  redirect_app.route('/p/<string:project_name>/issues/detail')(IssueDetail)

  def IssueCreate(project_name):
    redirect_url = redirect_utils.GetRedirectURL(project_name)
    if redirect_url:
      query_string = redirect_utils.GetNewIssueParams(
          flask.request.args, project_name)
      return flask.redirect(redirect_url + '/new?' + query_string)
    flask.abort(404)
  redirect_app.route('/p/<string:project_name>/issues/entry')(IssueCreate)
  redirect_app.route('/p/<string:project_name>/issues/entry_new')(IssueCreate)

  def IssueWizard(project_name):
    return flask.redirect('https://issues.chromium.org/issues/wizard')
  redirect_app.route('/p/<string:project_name>/issues/wizard')(IssueWizard)

  # RESTful API that maps Monorail local issue IDs to redirected issue IDs.
  def MappingApi(project_name: str, local_id: int) -> str:
    redirect_id = redirectissue.RedirectIssue.Get(project_name, local_id)
    if redirect_id:
      return redirect_id
    flask.abort(404)
  redirect_app.route('/<string:project_name>/<int:local_id>')(MappingApi)

  return redirect_app


def _GenerateIssueDetailRedirectURL(local_id, project_name):
  redirect_base_url = redirect_utils.GetRedirectURL(project_name)
  if not redirect_base_url:
    return None

  if local_id > redirect_utils.MAX_MONORAIL_ISSUE_ID:
    return redirect_base_url + '/' + str(local_id)

  tracker_id = redirectissue.RedirectIssue.Get(project_name, local_id)
  if tracker_id:
    return redirect_base_url + '/' + tracker_id
  return None
