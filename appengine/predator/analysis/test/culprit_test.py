# Copyright 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from analysis.analysis_testcase import AnalysisTestCase
from analysis.culprit import Culprit


class CulpritTest(AnalysisTestCase):

  def testFieldsProperty(self):
    culprit = Culprit('', [], ['Blink>DOM'], None, [], None, 'core_algorithm',
                      True)
    self.assertEqual(
        culprit.fields,
        ('project', 'file_paths', 'components', 'buganizer_component_id',
         'suspected_cls', 'regression_range', 'algorithm', 'success'))

  def testToDictsDroppingEmptyFields(self):
    culprit = Culprit('', [], [], None, [], [], 'core_algorithm', True)
    self.assertTupleEqual(culprit.ToDicts(), ({
        'found': False
    }, {
        'suspect_count': 0,
        'found_suspects': False,
        'found_project': False,
        'found_file_paths': False,
        'found_components': False,
        'found_buganizer_component': False,
        'has_regression_range': False,
        'solution': 'core_algorithm',
        'success': True
    }))

  def testToDicts(self):
    cl = self.GetDummyChangeLog()
    culprit = Culprit('proj', ['path'], ['comp'], '1456190', [cl],
                      ['50.0.1234.1', '50.0.1234.2'], 'core_algorithm', True)
    self.assertTupleEqual(culprit.ToDicts(), ({
        'found': True,
        'regression_range': ['50.0.1234.1', '50.0.1234.2'],
        'suspected_project': 'proj',
        'suspected_file_paths': ['path'],
        'suspected_components': ['comp'],
        'suspected_buganizer_component_id': '1456190',
        'suspected_cls': [cl.ToDict()]
    }, {
        'suspect_count': 1,
        'found_suspects': True,
        'found_project': True,
        'found_file_paths': True,
        'found_components': True,
        'found_buganizer_component': True,
        'has_regression_range': True,
        'solution': 'core_algorithm',
        'success': True
    }))
