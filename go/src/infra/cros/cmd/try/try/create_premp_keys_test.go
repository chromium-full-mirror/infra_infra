// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package try

import (
	"context"
	"fmt"
	"os"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	bapipb "go.chromium.org/chromiumos/infra/proto/go/chromite/api"

	"infra/cros/internal/assert"
	"infra/cros/internal/cmd"
	bb "infra/cros/lib/buildbucket"
)

func TestValidate_createPreMPKeysRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Test the good workflow
	cmdRunner := fakeBBBuildersRunner("chromeos/staging", []string{"staging-key-manager"})
	f := createPreMPKeysRun{
		tryRunBase: tryRunBase{
			cmdRunner: cmdRunner,
			bbClient:  bb.NewClient(cmdRunner, nil, nil),
		},
		buildTarget: "atlas",
		bug:         1337,
	}
	assert.NilError(t, f.validate(ctx))

	// No build target provided.
	f.buildTarget = ""
	assert.NonNilError(t, f.validate(ctx))

	// No bug provided.
	f.buildTarget = "atlas"
	f.bug = 0
	assert.NonNilError(t, f.validate(ctx))
}

type createPreMPKeysTestConfig struct {
	buildTarget string
	dryrun      bool
	production  bool
	addLoem     bool
}

func doCreatePreMPKeysTest(t *testing.T, tc *createPreMPKeysTestConfig) {
	t.Helper()
	propsFile, err := os.CreateTemp("", "input_props")
	defer os.Remove(propsFile.Name())
	assert.NilError(t, err)

	f := &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			bb.FakeWhichRunner("bb", 0),
			bb.FakeAuthInfoRunner("bb", 0),
			bb.FakeWhichRunner("led", 0),
			bb.FakeAuthInfoRunner("led", 0),
			bb.FakeAuthInfoRunnerSuccessStdout("led", "sundar@google.com"),
		},
	}
	expectedBucket := "chromeos/staging"
	expectedBuilder := "staging-key-manager"
	if tc.production {
		expectedBucket = "chromeos/release"
		expectedBuilder = "key-manager"
	}
	f.CommandRunners = append(
		f.CommandRunners,
		*fakeLEDGetBuilderRunner(expectedBucket, expectedBuilder, true),
	)
	expectedAddCmd := []string{"bb", "add", fmt.Sprintf("%s/%s", expectedBucket, expectedBuilder)}
	expectedAddCmd = append(expectedAddCmd, "-t", "tryjob-launcher:sundar@google.com")

	expectedAddCmd = append(expectedAddCmd, "-p", fmt.Sprintf("@%s", propsFile.Name()))
	if !tc.dryrun {
		f.CommandRunners = append(f.CommandRunners, bb.FakeBBAddRunner(expectedAddCmd, "12345"))
	}

	r := createPreMPKeysRun{
		tryRunBase: tryRunBase{
			cmdRunner:  f,
			dryrun:     tc.dryrun,
			production: tc.production,
		},
		propsFile:   propsFile,
		buildTarget: tc.buildTarget,
		bug:         4201337,
		addLoem:     tc.addLoem,
	}
	ret := r.Run(nil, nil, nil)
	assert.IntsEqual(t, ret, Success)

	properties, err := bb.ReadStructFromFile(propsFile.Name())
	assert.NilError(t, err)

	// Check that the requests are populated correctly.
	jsonRequest, err := properties.GetFields()["create_premp_keys_request"].GetStructValue().MarshalJSON()
	assert.NilError(t, err)
	var createPreMPKeysRequest bapipb.CreatePreMPKeysRequest
	err = protojson.Unmarshal([]byte(jsonRequest), &createPreMPKeysRequest)
	assert.NilError(t, err)

	assert.StringsEqual(
		t,
		createPreMPKeysRequest.BuildTarget.Name,
		tc.buildTarget,
	)

	bugId := properties.GetFields()["bug"].GetNumberValue()
	assert.IntsEqual(
		t,
		int(bugId),
		4201337,
	)

	assert.BoolsEqual(
		t,
		createPreMPKeysRequest.AddLoem,
		tc.addLoem,
	)
}

func TestCreatePreMPKeys_dryrun(t *testing.T) {
	t.Parallel()
	doCreatePreMPKeysTest(t, &createPreMPKeysTestConfig{
		buildTarget: "atlas",
		dryrun:      true,
	})
}

func TestCreatePreMPKeys_production_success(t *testing.T) {
	t.Parallel()
	doCreatePreMPKeysTest(t, &createPreMPKeysTestConfig{
		buildTarget: "atlas",
		production:  true,
	})
}

func TestCreatePreMPKeys_staging_success(t *testing.T) {
	t.Parallel()
	doCreatePreMPKeysTest(t, &createPreMPKeysTestConfig{
		buildTarget: "atlas",
	})
}

func TestCreatePreMPKeys_addloem(t *testing.T) {
	t.Parallel()
	doCreatePreMPKeysTest(t, &createPreMPKeysTestConfig{
		buildTarget: "atlas",
		addLoem:     true,
	})
}
