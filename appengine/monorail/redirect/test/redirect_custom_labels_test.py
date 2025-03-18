# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import unittest
from unittest import mock

from redirect import redirect_custom_labels
from redirect.test import datastore_stub


class TestRedirectCustomLabelsToHotlists(unittest.TestCase):

  @mock.patch(
      'google.cloud.datastore.Client',
      datastore_stub.MakeClient(
          {
              datastore_stub.Key('RedirectCustomLabelsToHotlists', 'a:test'):
                  {
                      'ProjectName': 'a',
                      'MonorailLabel': 'test',
                      'HotlistId': '12345'
                  },
          }))
  def testGetRedirectCustomLabelsAsdf(self):
    t = redirect_custom_labels.GetHotlist('a', 'test')
    self.assertEqual(t, '12345')

  @mock.patch('google.cloud.datastore.Client', datastore_stub.Client)
  def testGetRedirectCustomValueWithoutValue(self):
    t = redirect_custom_labels.GetHotlist('a', 'unknown')
    self.assertEqual(t, None)

  @mock.patch(
      'google.cloud.datastore.Client',
      datastore_stub.MakeClient(
          {
              datastore_stub.Key('RedirectCustomLabelsToHotlists', 'a:test1'):
                  {
                      'ProjectName': 'a',
                      'MonorailLabel': 'test1',
                      'HotlistId': '12345'
                  },
              datastore_stub.Key('RedirectCustomLabelsToHotlists', 'a:test2'):
                  {
                      'ProjectName': 'a',
                      'MonorailLabel': 'test1',
                      'HotlistId': '23456'
                  },
          }))
  def testGetRedirectCustomValueOnlyReturnTheFirstMatch(self):
    # There should be only one match in db.
    # This may change if we decided to support multiple value mapping.
    t = redirect_custom_labels.GetHotlist('a', 'test1')
    self.assertEqual(t, '12345')


class TestRedirectToCustomFields(unittest.TestCase):

  @mock.patch(
      'google.cloud.datastore.Client',
      datastore_stub.MakeClient(
          {
              datastore_stub.Key('RedirectToCustomFields', 'a:test-1-'):
                  {
                      'ProjectName': 'a',
                      'MonorailPrefix': 'test-',
                      'CustomFieldId': '12345',
                      'ExpectedValueType': 'numeric',
                  },
              datastore_stub.Key('RedirectToCustomFields', 'a:test-2-'):
                  {
                      'ProjectName': 'a',
                      'MonorailPrefix': 'test-',
                      'CustomFieldId': '23456',
                      'ProcessRedirectValue': 'capitalize',
                  },
          }))
  def testGetRedirectToCustomFields(self):
    t = redirect_custom_labels.GetCustomFieldMap()
    self.assertEqual(
        t, {
            'a:test-1-':
                {
                    'monorail_prefix': 'test-',
                    'custom_field_id': '12345',
                    'expected_value_type': 'numeric',
                    'process_redirect_value': None
                },
            'a:test-2-':
                {
                    'monorail_prefix': 'test-',
                    'custom_field_id': '23456',
                    'expected_value_type': None,
                    'process_redirect_value': 'capitalize'
                }
        })
