# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Utils for redirect."""
import urllib
from werkzeug.datastructures import MultiDict
from redirect import redirect_project_template

from tracker import tracker_constants
from tracker import tracker_bizobj
from redirect import redirect_custom_labels

PROJECT_REDIRECT_MAP = {
    'angleproject': 'https://issues.angleproject.org',
    'aomedia': 'https://aomedia.issues.chromium.org',
    'boringssl': 'https://issues.chromium.org',
    'chromedriver': 'https://issues.chromium.org',
    'chromium': 'https://issues.chromium.org',
    'crashpad': 'https://issues.chromium.org',
    'dawn': 'https://issues.chromium.org',
    'fuchsia': 'https://issues.fuchsia.dev',
    'gerrit': 'https://issues.gerritcodereview.com',
    'git': 'https://git.issues.gerritcodereview.com',
    'libyuv': 'https://libyuv.issues.chromium.org',
    'pdfium': 'https://issues.chromium.org',
    'pigweed': 'https://issues.pigweed.dev',
    'skia': 'https://issues.skia.org',
    'tint': 'https://issues.chromium.org',
    'v8': 'https://issues.chromium.org',
    'webrtc': 'https://issues.webrtc.org',
}

# Only contain the project that is a subcomponent of the tracker.
PROJECT_COMPONENT_ID_MAP = {
    'boringssl': 1590116,
    'chromedriver': 1608258,
    'crashpad': 1605005,
    'dawn': 1570784,
    'pdfium': 1586257,
    'tint': 1571063,
    'v8': 1456824,
}

MAX_MONORAIL_ISSUE_ID = 10000000

TRACKER_SEARCH_KEY_MAP = {
    'cc': 'cc',
    'owner': 'assignee',
    'commentby': 'commenter',
    'reporter': 'reporter',
    'is': 'is',
}

VALID_IS_SEARCH_VALUE = ['open', 'starred']

# Chromium's component ID in Buganizer.
DEFAULT_COMPONENT_ID = '1363614'

def GetRedirectURL(project_name):
  return PROJECT_REDIRECT_MAP.get(project_name, None)


def GetNewIssueParams(params: MultiDict, project_name: str):
  # Use a tuple, because we can have multiple values for a param.
  new_issue_params = []

  has_append_component = False
  # Get component and template id.
  template_name = params.get('template', type=str)
  if template_name:
    redirect_component_id, redirect_template_id = (
        redirect_project_template.RedirectProjectTemplate.Get(
            project_name, template_name))
    if redirect_component_id:
      new_issue_params.append(('component', redirect_component_id))
      has_append_component = True
    if redirect_template_id:
      new_issue_params.append(('template', redirect_template_id))

  # Get components param.
  if 'components' in params:
    components_str = params.get('components', type=str)
    components = [component.strip() for component in components_str.split(",")]
    # Get buganizer component id base on components param if no component exist.
    # The redirect is base on first value on the list.
    if components and not has_append_component:
      component = components[0]
      redirect_component_id, _ = (
          redirect_project_template.RedirectProjectTemplate.Get(
              project_name, component))
      if redirect_component_id:
        new_issue_params.append(('component', redirect_component_id))
        has_append_component = True
      # TODO(b/283983843): set up the components customfiled while redirect.

  # Select the proper component id
  # if the project is a subcomponent of the tracker.
  if not has_append_component:
    project_component_id = PROJECT_COMPONENT_ID_MAP.get(project_name, '')
    if project_component_id:
      new_issue_params.append(('component', project_component_id))

  if params.get('summary', type=str):
    new_issue_params.append(('title', params.get('summary', type=str)))

  if (params.get('description', type=str) or params.get('comment', type=str)):
    new_issue_params.append(
        (
            'description', (
                params.get('description', type=str) or
                params.get('comment', type=str))))

  if params.get('cc', type=str):
    new_issue_params.append(('cc', params.get('cc', type=str)))

  if params.get('owner', type=str):
    new_issue_params.append(
        ('assignee', params.get('owner', type=str).split('@')[0]))

  if params.get('labels', type=str):
    labels = params.get('labels', type=str).split(',')
    for label in labels:
      field_label, field_value = _GetCustomLabelsRedirectInfo(
          'new', project_name, label)
      if field_label and field_value:
        new_issue_params.append((field_label, field_value))

  # TODO(b/283983843): redirect when custom field settled. (components)
  return urllib.parse.urlencode(new_issue_params)

def GetSearchQuery(project_name, params):
  component_id = PROJECT_COMPONENT_ID_MAP.get(project_name, '')

  if not params and not component_id:
    return ''

  search_conds = []

  # can param is the default search query used in monorail.
  # Each project can customize the canned queries.
  # (eg.can=41013401 in Monorail is the Triage Queue.)
  # For redirect we will just support the build in can query as the first step.
  # TODO(b/283983843): support customized can query as needed.
  can_param = params.get(
      'can', type=int, default=tracker_constants.OPEN_ISSUES_CAN)
  # TODO(b/283983843): move the BuiltInQuery to redirect folder.
  default_search_string = tracker_bizobj.GetBuiltInQuery(can_param)
  for cond in default_search_string.split(' '):
    search_conds.append(cond)

  # q param is the user defined search query.
  if params.get('q', type=str):
    search_string = urllib.parse.unquote(params.get('q', type=str))
    for cond in search_string.split(' '):
      search_conds.append(cond)

  query_string = ''
  if component_id:
    query_string += 'componentid:' + str(component_id) + '+'

  for cond in search_conds:
    condition_pair = _ConvertSearchCondition(project_name, cond)
    if condition_pair:
      (k, v) = condition_pair
      query_string += ' {0}:{1}'.format(k, v)
  return urllib.parse.urlencode({'q': query_string.strip()})


# Convert monorail search conditions to tracker search conditions.
def _ConvertSearchCondition(project_name, cond):
  cond_pair = []
  # In monorail the search condition can be either ':' or '='.
  if ':' in cond:
    cond_pair = cond.split(':')
  if '=' in cond:
    cond_pair = cond.split('=')

  if len(cond_pair) != 2:
    return None
  # '-' stand for NOT.
  pre = '-' if cond_pair[0].startswith('-') else ''
  key_val = cond_pair[0][1:] if cond_pair[0].startswith('-') else cond_pair[0]

  k, v = _GenerateTrackerSearchKeyValuePair(project_name, key_val, cond_pair[1])
  if not k or not v:
    return None

  return pre + k, v


# Convert the search value to tracker search format.
def _GenerateTrackerSearchKeyValuePair(project_name, key, value):
  if len(value) == 0:
    return None, None

  if key == 'label':
    field_label, field_value = _GetCustomLabelsRedirectInfo(
        'search', project_name, value)
    if field_label and field_value:
      return field_label, field_value

  # If the value is not store in datastore check the general filter set.
  new_key = TRACKER_SEARCH_KEY_MAP.get(key, None)
  if not new_key:
    return None, None

  if new_key == 'is':
    return new_key, value if value in VALID_IS_SEARCH_VALUE else None

  new_value = value.replace(',', '|')
  return new_key, '({})'.format(new_value)


def _GetCustomLabelsRedirectInfo(request_type, project_name, label):
  hotlist_id = redirect_custom_labels.RedirectCustomLabelsToHotlists.Get(
      project_name, label)

  if not hotlist_id:
    # Check if the label is mapped to a custom field.
    return _GetCustomLabelsToCustomFieldRedirectInfo(
        request_type, project_name, label)
  else:
    if request_type == 'search':
      return 'hotlistid', hotlist_id
    elif request_type == 'new':
      return 'hotlistIds', hotlist_id
    return None, None


def _GetCustomLabelsToCustomFieldRedirectInfo(
    request_type, project_name, label):
  custom_fields_map = redirect_custom_labels.RedirectToCustomFields.GetAll()

  lookup_key = project_name + ":" + label
  matches = [
      key for key in custom_fields_map.keys() if lookup_key.startswith(key)
  ]
  if len(matches) == 1:
    matched_key = matches[0]
  elif len(matches) == 2:
    if len(matches[0]) > len(matches[1]):
      matched_key = matches[0]
    else:
      matched_key = matches[1]
  else:
    return None, None

  field_id = custom_fields_map.get(matched_key).get('custom_field_id')

  monorail_prefix = custom_fields_map.get(matched_key).get('monorail_prefix')
  if not monorail_prefix:
    return None, None
  field_value = label[len(monorail_prefix):]

  expected_value_type = custom_fields_map.get(matched_key).get(
      'expected_value_type')
  if expected_value_type == 'numeric':
    if not field_value.isnumeric():
      return None, None

  process_redirect_value = custom_fields_map.get(matched_key).get(
      'process_redirect_value')
  if process_redirect_value == 'capitalize':
    field_value = field_value.capitalize()
  elif process_redirect_value == 'uppercase':
    field_value = field_value.uppercase()
  elif process_redirect_value == 'append -early':
    field_value = field_value + '-early'
  elif process_redirect_value == 'Qualified':
    field_value = 'Qualified'

  if field_id != '' and field_value != '':
    if request_type == 'search':
      return 'customfield' + field_id, field_value
    elif request_type == 'new':
      return 'customFields', field_id + ':' + field_value
    return None, None

  return None, None
