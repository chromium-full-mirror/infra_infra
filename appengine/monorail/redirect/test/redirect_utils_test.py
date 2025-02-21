# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import unittest
from unittest import mock

import werkzeug

from redirect import redirect_utils


class TestRedirectUtils(unittest.TestCase):

  @mock.patch('redirect.redirect_project_template.Get')
  def testNewIssueParams(self, fake_redirectProjectTemplate):
    fake_redirectProjectTemplate.return_value = None, None
    params = werkzeug.datastructures.MultiDict(
        [
            ('summary', 'this is a summary'),
            ('owner', 'test@google.com'),
            ('description', 'task'),
            ('cc', 'c1@google.com,c2@google.com'),
        ])
    expected = (
        'title=this+is+a+summary&description=task&'
        'cc=c1%40google.com%2Cc2%40google.com&assignee=test')

    get = redirect_utils.GetNewIssueParams(params, 'project')
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_project_template.Get')
  def testNewIssueParams_withSubComponent(self, fake_redirectProjectTemplate):
    fake_redirectProjectTemplate.return_value = None, None
    params = werkzeug.datastructures.MultiDict(
        [
            ('summary', 'this is a summary'),
            ('owner', 'test@google.com'),
            ('description', 'task'),
            ('cc', 'c1@google.com,c2@google.com'),
        ])
    expected = (
        'component=1590116&title=this+is+a+summary&description=task&'
        'cc=c1%40google.com%2Cc2%40google.com&assignee=test&noWizard=True')

    get = redirect_utils.GetNewIssueParams(params, 'boringssl')
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_project_template.Get')
  def testNewIssueRedirectWithTemplateParam(self, fake_redirectProjectTemplate):
    fake_redirectProjectTemplate.return_value = '1', '2'
    params = werkzeug.datastructures.MultiDict(
        [
            ('template', 'test'), ('summary', 'this is a summary'),
            ('owner', 'test@google.com')
        ])
    expected = 'component=1&template=2&title=this+is+a+summary&assignee=test'

    get = redirect_utils.GetNewIssueParams(params, 'project')
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_project_template.Get')
  def testNewIssueRedirectWithComponentsParam(
      self, fake_redirectProjectTemplate):
    fake_redirectProjectTemplate.return_value = '1', None
    params = werkzeug.datastructures.MultiDict(
        [
            ('components', 'test'), ('summary', 'this is a summary'),
            ('owner', 'test@google.com')
        ])
    expected = 'component=1&title=this+is+a+summary&assignee=test'

    get = redirect_utils.GetNewIssueParams(params, 'project')
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_project_template.Get')
  def testNewIssueRedirectWithComponentsParam(
      self, fake_redirectProjectTemplate):
    fake_redirectProjectTemplate.return_value = '1', None
    params = werkzeug.datastructures.MultiDict(
        [
            ('components', 'test'), ('summary', 'this is a summary'),
            ('owner', 'test@google.com')
        ])
    expected = 'component=1&title=this+is+a+summary&assignee=test'

    get = redirect_utils.GetNewIssueParams(params, 'project')
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_project_template.Get')
  def testNewIssueRedirectWithMutipleComponentsParam(
      self, fake_redirectProjectTemplate):

    def mock_redirect(_, arg):
      if arg == 'comp1':
        return 1, None
      elif arg == 'comp2':
        return 2, None
      return None, None

    fake_redirectProjectTemplate.side_effect = mock_redirect
    params = werkzeug.datastructures.MultiDict(
        [
            ('components', 'comp2, comp1'), ('summary', 'this is a summary'),
            ('owner', 'test@google.com')
        ])
    expected = 'component=2&title=this+is+a+summary&assignee=test'

    get = redirect_utils.GetNewIssueParams(params, 'project')
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_project_template.Get')
  def testNewIssueRedirectWithAllParam(self, fake_redirectProjectTemplate):

    def mock_redirect(_, arg):
      if arg == 'tmp1':
        return 111, 222
      elif arg == 'comp1':
        return 1, None
      elif arg == 'comp2':
        return 2, None
      return None, None

    fake_redirectProjectTemplate.side_effect = mock_redirect
    params = werkzeug.datastructures.MultiDict(
        [
            ('template', 'tmp1'), ('components', 'comp1, comp2'),
            ('summary', 'this is a summary'), ('owner', 'test@google.com')
        ])
    expected = (
      'component=111&template=222&title=this+is+a+summary&assignee=test')
    get = redirect_utils.GetNewIssueParams(params, 'project')
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_project_template.Get')
  def testNewIssueParamsWithNoValidValue(self, fake_redirectProjectTemplate):
    fake_redirectProjectTemplate.return_value = None, None
    params = werkzeug.datastructures.MultiDict([('test', 'this is a test')])
    expected = ''
    get = redirect_utils.GetNewIssueParams(params, 'project')
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_project_template.Get')
  def testNewIssueParamsWithNoWizard(self, fake_redirectProjectTemplate):
    fake_redirectProjectTemplate.return_value = None, None
    params = werkzeug.datastructures.MultiDict([('test', 'this is a test')])
    expected = 'component=1570784&noWizard=True'
    get = redirect_utils.GetNewIssueParams(params, 'dawn')
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_custom_labels.GetHotlist')
  def testNewIssueParamsWithCustomLabelsToHotlists(
      self, fake_redirect_custom_labels_to_hotlists):
    fake_redirect_custom_labels_to_hotlists.return_value = '12345'
    params = werkzeug.datastructures.MultiDict(
        [('test', 'this is a test'), ('labels', 'reward-topanel')])
    expected = 'hotlistIds=12345'
    get = redirect_utils.GetNewIssueParams(params, 'project')
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_custom_labels.GetCustomFieldMap')
  @mock.patch(
      'redirect.redirect_custom_labels.GetHotlist',
      mock.Mock(return_value=None))
  def testNewIssueParamsWithCustomLabelsToCustomFields(
      self, fake_redirect_custom_labels_to_custom_fields):
    fake_redirect_custom_labels_to_custom_fields.return_value = {
        'project:reward-':
            {
                'monorail_prefix': 'reward-',
                'custom_field_id': '1223135',
                'expected_value_type': 'numeric',
                'process_redirect_value': None
            }
    }
    params = werkzeug.datastructures.MultiDict(
        [('test', 'this is a test'), ('labels', 'reward-100')])
    expected = 'customFields=1223135%3A100'
    get = redirect_utils.GetNewIssueParams(params, 'project')
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_custom_labels.GetHotlist')
  def testGetSearchQuery(self, fake_redirect_custom_labels_to_hotlists):
    fake_redirect_custom_labels_to_hotlists.return_value = None
    params = werkzeug.datastructures.MultiDict(
        [('q', 'owner%3Ame%20has%3ARollout-Type')])
    expected = 'q=is%3Aopen+assignee%3A%28me%29'

    get = redirect_utils.GetSearchQuery('project', params)
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_custom_labels.GetHotlist')
  def testGetSearchQueryWithSubComponent(
      self, fake_redirect_custom_labels_to_hotlists):
    fake_redirect_custom_labels_to_hotlists.return_value = None
    params = werkzeug.datastructures.MultiDict([])
    expected = 'q=componentid%3A1590116%2B+is%3Aopen'

    get = redirect_utils.GetSearchQuery('boringssl', params)
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_custom_labels.GetHotlist')
  def testGetSearchQueryWithCustomLabelsToHotlists(
      self, fake_redirect_custom_labels_to_hotlists):
    fake_redirect_custom_labels_to_hotlists.return_value = '12345'
    params = werkzeug.datastructures.MultiDict(
        [('q', 'owner%3Ame%20has%3ARollout-Type%20label:reward-topanel')])
    expected = 'q=is%3Aopen+assignee%3A%28me%29+hotlistid%3A12345'

    get = redirect_utils.GetSearchQuery('project', params)
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_custom_labels.GetCustomFieldMap')
  @mock.patch(
      'redirect.redirect_custom_labels.GetHotlist',
      mock.Mock(return_value=None))
  def testGetSearchQueryWithCustomLabelsToCustomFields(
      self, fake_redirect_custom_labels_to_custom_fields):
    fake_redirect_custom_labels_to_custom_fields.return_value = {
        'project:reward-':
            {
                'monorail_prefix': 'reward-',
                'custom_field_id': '1223135',
                'expected_value_type': 'numeric',
                'process_redirect_value': None
            }
    }
    params = werkzeug.datastructures.MultiDict(
        [('q', 'owner%3Ame%20has%3ARollout-Type%20label:reward-100')])
    expected = 'q=is%3Aopen+assignee%3A%28me%29+customfield1223135%3A100'

    get = redirect_utils.GetSearchQuery('project', params)
    self.assertEqual(expected, get)

  @mock.patch('redirect.redirect_custom_labels.GetHotlist')
  def testGetSearchQueryWithCanValue(
      self, fake_redirect_custom_labels_to_hotlists):
    fake_redirect_custom_labels_to_hotlists.return_value = None
    params = werkzeug.datastructures.MultiDict([('can', 4)])
    expected = 'q=is%3Aopen+reporter%3A%28me%29'

    get = redirect_utils.GetSearchQuery('project', params)
    self.assertEqual(expected, get)
