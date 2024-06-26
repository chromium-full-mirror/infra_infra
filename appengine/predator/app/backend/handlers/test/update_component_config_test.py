# Copyright 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from flask import Flask
import json
import mock

from backend.handlers.update_component_config import (
    GetComponentClassifierConfig)
from backend.handlers.update_component_config import UpdateComponentConfig
from common.model.crash_config import CrashConfig
from gae_libs.http.http_client_appengine import HttpClientAppengine
from gae_libs.testcase import TestCase


_MOCK_OWNERS_MAPPINGS = json.dumps({
  'component-to-team': {
      'compoA': 'team1@chromium.org',
      'compoB': 'team2@chromium.org',
      'compoD': 'team4@chromium.org'
    },
  'dir-to-component': {
      'dirA': 'compoA',
      'dirB': 'compoB',
      'dirC': 'compoB',
      'dirE': 'compoE',
    }
})


_MOCK_CONFIG = {
    'component_info': [
        {
            'dirs': ['src/dirA'],
            'component': 'compoA',
            'team': 'team1@chromium.org'
        },
        {
            'dirs': ['src/dirB', 'src/dirC'],
            'component': 'compoB',
            'team': 'team2@chromium.org'
        },
        {
            'dirs': ['src/dirE'],
            'component': 'compoE'
        },
    ],
    'owner_mapping_url': 'url',
    'top_n': 4,
    'buganzier_component_dict': {
        ".": {
            "monorail": {
                "project": "chromium"
            }
        },
        "android_webview": {
            "monorail": {
                "component": "Mobile>WebView"
            },
            "teamEmail": "android-webview-dev@chromium.org",
            "os": "ANDROID",
            "buganizerPublic": {
                "componentId": "1456456"
            }
        },
        "android_webview/test/components": {
            "monorail": {
                "component": "Test>WebView"
            },
            "buganizerPublic": {
                "componentId": "1457060"
            }
        },
        "apps": {
            "monorail": {
                "component": "Platform>Apps"
            },
            "teamEmail": "apps-dev@chromium.org",
            "buganizerPublic": {
                "componentId": "1456886"
            }
        },
        "ash": {
            "monorail": {
                "component": "UI>Shell"
            },
            "buganizerPublic": {
                "componentId": "1456399"
            }
        }
    },
}

_MOCK_CURRENT_CONFIG = {
    'component_info': [{
        'dirs': ['src/dirA'],
        'component': 'compoA',
        'team': 'team1@chromium.org'
    },],
    'owner_mapping_url': 'url',
    'top_n': 4,
    'buganzier_component_dict': {
        ".": {
            "monorail": {
                "project": "chromium"
            }
        },
        "android_webview": {
            "monorail": {
                "component": "Mobile>WebView"
            },
            "teamEmail": "android-webview-dev@chromium.org",
            "os": "ANDROID",
            "buganizerPublic": {
                "componentId": "1456456"
            }
        },
        "android_webview/test/components": {
            "monorail": {
                "component": "Test>WebView"
            },
            "buganizerPublic": {
                "componentId": "1457060"
            }
        },
        "apps": {
            "monorail": {
                "component": "Platform>Apps"
            },
            "teamEmail": "apps-dev@chromium.org",
            "buganizerPublic": {
                "componentId": "1456886"
            }
        },
        "ash": {
            "monorail": {
                "component": "UI>Shell"
            },
            "buganizerPublic": {
                "componentId": "1456399"
            }
        }
    },
}

_MOCK_BUGANIZER_COMPONENT_METADATA = """{"dirs":
       {".":
          {"monorail":{"project":"chromium"}},
        "android_webview":
          {"monorail":{"component":"Mobile>WebView"},
           "teamEmail":"android-webview-dev@chromium.org",
           "os":"ANDROID",
           "buganizerPublic":{"componentId":"1456456"}
           },
        "android_webview/test/components":
          {"monorail":{"component":"Test>WebView"},
           "buganizerPublic":{"componentId":"1457060"}
           }
        }
     }
"""


class DummyHttpClient(HttpClientAppengine):  # pragma: no cover.
  def __init__(self, config=None, response=None):
    super(DummyHttpClient, self).__init__()
    self.config = config or CrashConfig.Get().component_classifier
    self.response = response or _MOCK_OWNERS_MAPPINGS

  def Get(self, url):  # pylint: disable=W
    if 'metadata_reduced.json' in url:
      return 200, _MOCK_BUGANIZER_COMPONENT_METADATA, {}
    if 'owner_mapping_url' in self.config:
      return 200, self.response, {}
    else:
      return 500, {}, {}


class UpdateComponentConfigTest(TestCase):
  """Tests utility functions and ``CrashConfig`` handler."""
  app_module = Flask(__name__)
  app_module.add_url_rule(
      '/process/update-component-config',
      view_func=UpdateComponentConfig().Handle,
      methods=['GET'])

  def setUp(self):
    super(UpdateComponentConfigTest, self).setUp()
    self.http_client_for_git = self.GetMockHttpClient()

  def testGetComponentClassifierConfig(self):
    component_classifier_config = GetComponentClassifierConfig(
        _MOCK_CURRENT_CONFIG, DummyHttpClient({'owner_mapping_url': 'url'}))
    expected_components = _MOCK_CONFIG['component_info']
    components = component_classifier_config['component_info']
    self.assertCountEqual(components, expected_components)

  def testGetComponentClassifierConfigNoOWNERS(self):
    component_classifier_config = GetComponentClassifierConfig(
        _MOCK_CURRENT_CONFIG, DummyHttpClient(config={'top_n': 3}))
    self.assertEqual(0, len(component_classifier_config['component_info']))

  @mock.patch(
      'backend.handlers.update_component_config.GetComponentClassifierConfig')
  def testHandleGet(self, mocked_get_component_classifier_config):
    mocked_get_component_classifier_config.return_value = _MOCK_CONFIG
    response = self.test_app.get('/process/update-component-config',
                                 headers={'X-AppEngine-Cron': 'true'})
    self.assertEqual(response.status_int, 200)
    self.assertDictEqual(_MOCK_CONFIG, CrashConfig.Get().component_classifier)
