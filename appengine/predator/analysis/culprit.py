# Copyright 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from collections import namedtuple


class Culprit(
    namedtuple('Culprit', [
        'project', 'file_paths', 'components', 'buganizer_component_id',
        'suspected_cls', 'regression_range', 'algorithm', 'success'
    ])):
  """The result of successfully identifying the culprit of a crash report.

  That is, this is what ``Predator.FindCultprit`` returns. It encapsulates
  all the information predator discovered during its various analyses.

  Args:
    project (str): the most-suspected project
    file_paths (list of str): the suspected file paths.
    components (list of str): the suspected crbug components.
    buganizer_component_id (str): the suspected buganizer componentID.
    suspected_cls (list of Suspects): the suspected suspected_cls.
    regression_range (tuple): a pair of the last-good and first-bad versions.
    algorithm (str): What algorithm was used to produce this object.
    success (bool): If we successfully ran the analysis process to get the
      culprit.
  """
  __slots__ = ()

  @property
  def fields(self):
    return self._fields

  # TODO(http://crbug/644476): better name for this method.
  def ToDicts(self):
    """Convert this object to a pair of anonymous dicts for JSON.

    Returns:
      (analysis_result_dict, tag_dict)
      The analysis result is a dict like below:
      {
          # Indicate if Predator found any suspects_cls, project,
          # components or regression_range.
          "found": true,
          "suspected_project": "chromium-v8", # Which project is most suspected.
          "suspected_file_paths": ["chrome/core/v8", "chrome/test/v8"]
          "feedback_url": "https://.."
          "suspected_cls": [
              {
                  "revision": "commit-hash",
                  "url": "https://chromium.googlesource.com/chromium/src/+/...",
                  "review_url": "https://codereview.chromium.org/issue-number",
                  "project_path": "third_party/pdfium",
                  "author": "who@chromium.org",
                  "time": "2015-08-17 03:38:16",
                  "reasons": [('MinDistance', 1, 'minimum distance is 0.'),
                              ('TopFrame', 0.9, 'top frame is2nd frame.')],
                  "changed_files": [
                      {"file": "file_name1.cc",
                       "blame_url": "https://...",
                       "info": "minimum distance (LOC) 0, frame #2"},
                      {"file": "file_name2.cc",
                       "blame_url": "https://...",
                       "info": "minimum distance (LOC) 20, frame #4"},
                      ...
                  ],
                  "confidence": 0.60
              },
              ...,
          ],
          "regression_range": [  # Detected regression range.
              "53.0.2765.0",
              "53.0.2766.0"
          ],
          "suspected_components": [  # A list of crbug components to file bugs.
              "Blink>JavaScript"
          ]
          "suspected_buganizer_component_id": 1456190
           # suspected buganizer componentID
      }

      The code review url might not always be available, because not all
      commits go through code review. In that case, commit url should
      be used instead.

      The tag dict are allowed key/value pairs to tag the analysis result
      for query and monitoring purpose on Predator side. For allowed keys,
      please refer to crash_analysis.py and fracas_crash_analysis.py:
        For results with normal culprit-finding algorithm: {
            'found_suspects': True,
            'has_regression_range': True,
            'solution': 'core_algorithm',
        }
        For results using git blame without a regression range: {
            'found_suspects': True,
            'has_regression_range': False,
            'solution': 'blame',
        }
        If nothing is found: {
            'found_suspects': False,
        }
    """
    result = {}
    result['found'] = (
        bool(self.project) or bool(self.file_paths) or bool(self.components) or
        bool(self.buganizer_component_id) or bool(self.suspected_cls) or
        bool(self.regression_range))
    if self.regression_range:
      result['regression_range'] = self.regression_range
    if self.project:
      result['suspected_project'] = self.project
    if self.file_paths:
      result['suspected_file_paths'] = self.file_paths
    if self.components:
      result['suspected_components'] = self.components
    if self.buganizer_component_id:
      result['suspected_buganizer_component_id'] = self.buganizer_component_id
    if self.suspected_cls:
      result['suspected_cls'] = [cl.ToDict() for cl in self.suspected_cls]

    tags = {
        'suspect_count': len(self.suspected_cls) if self.suspected_cls else 0,
        'found_suspects': bool(self.suspected_cls),
        'has_regression_range': bool(self.regression_range),
        'found_project': bool(self.project),
        'found_file_paths': len(self.file_paths) != 0,
        'found_components': bool(self.components),
        'found_buganizer_component': bool(self.buganizer_component_id),
        'solution': self.algorithm,
        'success': self.success,
    }

    return result, tags
