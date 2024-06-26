# Copyright 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Handler to update component classifier config."""

from collections import defaultdict
import json
import logging
import traceback

from google.appengine.api import users

from common.base_handler import BaseHandler, Permission
from common.model.crash_config import CrashConfig
from gae_libs.http.http_client_appengine import HttpClientAppengine

_BUGANIZER_COMPONENT_META_URL = \
  'https://storage.googleapis.com/chrome-metadata/metadata_reduced.json'


def _getMetaDataDict(client, url):
  code, meta_dict, _ = client.Get(url)
  if code != 200:
    return None
  try:
    meta_dict = json.loads(meta_dict)
  except Exception:  # pragma: no cover
    logging.error(traceback.format_exc())
    return None
  return meta_dict


def _getComponentInfo(owner_mappings):
  if not owner_mappings:
    return []
  component_dict = defaultdict(dict)
  for dir_name, component in list(owner_mappings['dir-to-component'].items()):
    if component_dict.get(component) == None:
      component_dict[component]['component'] = component
      component_dict[component]['dirs'] = []
      if owner_mappings['component-to-team'].get(component):
        component_dict[component]['team'] = (
            owner_mappings['component-to-team'].get(component))
    component_dict[component]['dirs'].append('src/' + dir_name)
  return list(component_dict.values())


def GetComponentClassifierConfig(config, http_client=HttpClientAppengine()):
  """Get component mapping information from owners files and convert in
  Predator input format.

  The main purpose is to get the latest component/team information from
  OWNERS files and convert into predator mapping input format.

  Args:
    config(dict): Configuration of component classifier.

  Returns:
    A dict of {'component_info': data}, where data is a list of dict in the
    form {'component': component name.
          'dirs': a list of directories maps to this component.
          'team': the team owns this component.}.
    """
  owner_mappings = _getMetaDataDict(http_client, config['owner_mapping_url'])
  buganizer_meta = _getMetaDataDict(http_client, _BUGANIZER_COMPONENT_META_URL)
  component_classifier_config = {
      'component_info':
          _getComponentInfo(owner_mappings),
      'top_n':
          config['top_n'],
      'owner_mapping_url':
          config['owner_mapping_url'],
      'buganzier_component_dict':
          buganizer_meta.get('dirs', {}) if buganizer_meta else {}
  }
  return component_classifier_config

class UpdateComponentConfig(BaseHandler):
  PERMISSION_LEVEL = Permission.APP_SELF

  def HandleGet(self):
    # Update component_classifier with latest component/team information.
    crash_config = CrashConfig.Get()
    new_component_config = GetComponentClassifierConfig(
        crash_config.component_classifier)

    if not new_component_config:  # pragma: no cover.
      return self.CreateError('Component Classifier Config Update Fail', 400)

    crash_config.Update(users.User('cron_admin@chromium.org'), True,
                        component_classifier=new_component_config)
