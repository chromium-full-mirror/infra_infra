# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import unittest
from unittest import mock

from redirect import redirect_project_template
from redirect.test import datastore_stub


class TestRedirectCustomValue(unittest.TestCase):

  @mock.patch(
      'google.cloud.datastore.Client',
      datastore_stub.MakeClient(
          {
              datastore_stub.Key(
                  'RedirectProjectTemplate', 'a:default template'):
                  {
                      'ProjectName': 'a',
                      'MonorailTemplateName': 'default template',
                      'RedirectComponentID': '123',
                      'RedirectTemplateID': '456',
                  }
          }))
  def testGetRedirectProjectTemplate(self):
    (t, v) = redirect_project_template.Get('a', 'default template')
    self.assertEqual(t, '123')
    self.assertEqual(v, '456')

  @mock.patch('google.cloud.datastore.Client', datastore_stub.Client)
  def testGetRedirectProjectTemplateWithoutValue(self):
    (t, v) = redirect_project_template.Get('a', 'default template')
    self.assertEqual(t, None)
    self.assertEqual(v, None)
