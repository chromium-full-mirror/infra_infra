# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import unittest

from google.appengine.ext import ndb
from google.appengine.ext import testbed

from redirect import redirect_custom_labels


class TestRedirectCustomLabelsToHotlists(unittest.TestCase):

  def setUp(self):
    self.testbed = testbed.Testbed()
    self.testbed.activate()
    self.testbed.init_datastore_v3_stub()
    self.testbed.init_memcache_stub()
    ndb.get_context().clear_cache()

  def tearDown(self):
    self.testbed.deactivate()

  def testGetRedirectCustomLabels(self):
    redirector = redirect_custom_labels.RedirectCustomLabelsToHotlists
    redirector(
        ProjectName='a', MonorailLabel='test', HotlistId='12345',
        id='a:test').put()

    t = redirector.Get('a', 'test')
    self.assertEqual(t, '12345')

  def testGetRedirectCustomValueWithoutValue(self):
    redirector = redirect_custom_labels.RedirectCustomLabelsToHotlists

    t = redirector.Get('a', 'test')
    self.assertEqual(t, None)

  def testGetRedirectCustomValueOnlyReturnTheFirstMatch(self):
    # There should be only one match in db.
    # This may change if we decided to support multiple value mapping.
    redirector = redirect_custom_labels.RedirectCustomLabelsToHotlists
    redirector(
        ProjectName='a', MonorailLabel='test1', HotlistId='12345',
        id='a:test1').put()
    redirector(
        ProjectName='a', MonorailLabel='test2', HotlistId='23456',
        id='a:test2').put()
    t = redirector.Get('a', 'test1')
    self.assertEqual(t, '12345')


class TestRedirectToCustomFields(unittest.TestCase):

  def setUp(self):
    self.testbed = testbed.Testbed()
    self.testbed.activate()
    self.testbed.init_datastore_v3_stub()
    self.testbed.init_memcache_stub()
    ndb.get_context().clear_cache()

  def tearDown(self):
    self.testbed.deactivate()

  def testGetRedirectToCustomFields(self):
    redirector = redirect_custom_labels.RedirectToCustomFields
    redirector(
        ProjectName='a',
        MonorailPrefix='test-',
        CustomFieldId='12345',
        ExpectedValueType='numeric',
        id='a:test-1-').put()
    redirector(
        ProjectName='a',
        MonorailPrefix='test-',
        CustomFieldId='23456',
        ProcessRedirectValue='capitalize',
        id='a:test-2-').put()

    t = redirector.GetAll()
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
