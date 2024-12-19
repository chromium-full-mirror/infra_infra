# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import unittest

from google.appengine.ext import ndb
from google.appengine.ext import testbed

from redirect import redirect_project_template


class TestRedirectCustomValue(unittest.TestCase):

  def setUp(self):
    self.testbed = testbed.Testbed()
    self.testbed.activate()
    self.testbed.init_datastore_v3_stub()
    self.testbed.init_memcache_stub()
    ndb.get_context().clear_cache()

  def tearDown(self):
    self.testbed.deactivate()

  def testGetRedirectProjectTemplate(self):
    redirectProjectTemplate = redirect_project_template.RedirectProjectTemplate
    redirectProjectTemplate(
        ProjectName='a',
        MonorailTemplateName='default template',
        RedirectComponentID='123',
        RedirectTemplateID='456',
        id='a:default template').put()

    (t, v) = redirectProjectTemplate.Get('a', 'default template')
    self.assertEqual(t, '123')
    self.assertEqual(v, '456')

  def testGetRedirectProjectTemplateWithoutValue(self):
    redirectProjectTemplate = redirect_project_template.RedirectProjectTemplate

    (t, v) = redirectProjectTemplate.Get('a', 'default template')
    self.assertEqual(t, None)
    self.assertEqual(v, None)
