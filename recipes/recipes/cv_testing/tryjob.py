# Copyright 2021 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Recipe to test LUCI CQ/CV itself."""

from PB.recipes.infra.cv_testing import tryjob as pb
from PB.go.chromium.org.luci.common.proto.findings import findings as findings_pb

PYTHON_VERSION_COMPATIBILITY = 'PY2+3'

DEPS = [
    'recipe_engine/buildbucket',
    'recipe_engine/cv',
    'recipe_engine/findings',
    'recipe_engine/properties',
    'recipe_engine/step',
]

PROPERTIES = pb.Input


def RunSteps(api, properties):
  api.step.empty('1 step per recipe keeps a recipe engine crash away')
  if properties.reuse_own_mode_only:
    api.cv.allow_reuse_for(api.cv.run_mode)
  if properties.fail:
    raise api.step.StepFailure('tryjob wants to be red')
  if properties.infra_fail:
    raise api.step.InfraFailure('tryjob wants to be purple')
  if properties.emit_findings:
    finding = findings_pb.Finding(
        category='LUCIFindingsTestCategory',
        severity_level=findings_pb.Finding.SEVERITY_LEVEL_INFO,
        message='This is a test code finding')
    api.findings.populate_source_from_current_build(finding.location)
    finding.location.file_path = '/COMMIT_MSG'
    api.findings.upload_findings([finding])


def GenTests(api):
  def test(name, *args, **kwargs):
    return api.test(
        name,
        api.cv(run_mode=api.cv.DRY_RUN),
        *args,
        **kwargs,
    )

  yield test(
      'any-reuse',
  )
  yield test(
      'reuse-by-the-same-mode-only',
      api.properties(reuse_own_mode_only=True),
  )
  yield test(
      'fail',
      api.properties(fail=True),
      status='FAILURE',
  )
  yield test(
      'infra_fail',
      api.properties(infra_fail=True),
      status='INFRA_FAILURE',
  )
  yield test(
      'emit_findings',
      api.buildbucket.try_build(project='infra'),
      api.properties(emit_findings=True),
  )
