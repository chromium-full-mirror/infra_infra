# Copyright 2015 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# pylint: disable=undefined-variable

# Crazy hack, because of appengine.
import os
import sys


# NOTE: There's `expect_tests_pretest.py` file nearby. It serves different
# purpose than this `.expect_tests_pretest.py`:
#
#  * `expect_tests_pretest.py` is symlinked into various appengine/<app> and
#    used in tests there.
#  * `.expect_tests_pretest.py` is used for tests in appengine_module
#    specifically.


def _fix_sys_path_for_appengine(pretest_filename):
  infra_base_dir = os.path.abspath(pretest_filename)
  pos = infra_base_dir.rfind('/infra/appengine')
  if pos == -1:
    return
  infra_base_dir = infra_base_dir[:pos + len('/infra')]

  # Remove the base infra directory from the path, since this isn't available
  # on appengine.
  if infra_base_dir in sys.path:
    sys.path.remove(infra_base_dir)

  # Add the google_appengine directory.
  sys.path.insert(
      0,
      os.path.join(infra_base_dir, 'cipd', 'gcloud', 'platform',
                   'google_appengine'))

  import dev_appserver as pretest_dev_appserver
  pretest_dev_appserver.fix_sys_path()

  # For `endpoints` vendored into luci/appengine/....
  sys.path.insert(
      0, os.path.join(infra_base_dir, 'luci', 'appengine', 'components'))
  from components import utils
  utils.import_third_party()


# Using pretest_filename is magic, because it is available in the locals() of
# the script which execfiles this file.
_fix_sys_path_for_appengine(pretest_filename)
