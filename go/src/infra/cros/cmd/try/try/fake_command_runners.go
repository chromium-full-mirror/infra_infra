// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package try

import (
	"fmt"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/internal/cmd"
)

// fakeBBBuildersRunner mocks stdout for `bb builders {projectBucket}`.
// projectBucket should normally be of the form "project/bucket", such as "chromeos/staging".
// retBuilders is a list of builders that should be returned, such as []string{"chromeos/firmware/firmware-eve-9584.B-branch"}.
func fakeBBBuildersRunner(projectBucket string, retBuilders []string) *cmd.FakeCommandRunner {
	return &cmd.FakeCommandRunner{
		ExpectedCmd: []string{"bb", "builders", projectBucket},
		Stdout:      strings.Join(retBuilders, "\n"),
	}
}

// fakeLEDGetBuilderRunner mocks stdout for `led get-builder {bucket}:{builder}`.
// projectBucket should normally be of the form "project/bucket", such as "chromeos/staging".
// builder should normally be a builder name like "staging-grunt-release-main".
// pass denotes whether the fake command should pass or fail.
func fakeLEDGetBuilderRunner(projectBucket, builder string, pass bool) *cmd.FakeCommandRunner {
	var stdout, stderr string
	var failError error
	if pass {
		stdout = `{
			"buildbucket": {
				"bbagent_args": {
					"build": {
						"input": {
							"properties": {
								"$chromeos/my_module": {
									"my_prop": 100
								},
								"my_other_prop": 101
							}
						},
						"infra": {
							"buildbucket": {
								"experiment_reasons": {
									"chromeos.cros_artifacts.use_gcloud_storage": 1
								}
							}
						}
					}
				}
			}
		}`
	} else {
		failError = errors.New("return code 1")
		stderr = "... not found ..."
	}
	return &cmd.FakeCommandRunner{
		ExpectedCmd: []string{
			"led",
			"get-builder",
			fmt.Sprintf("%s:%s", projectBucket, builder),
		},
		Stdout:      stdout,
		Stderr:      stderr,
		FailCommand: !pass,
		FailError:   failError,
	}
}
