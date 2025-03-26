// Copyright 2025 The Chromium Authors
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

	"go.chromium.org/infra/cros/internal/assert"
	"go.chromium.org/infra/cros/internal/cmd"
	bb "go.chromium.org/infra/cros/lib/buildbucket"
)

func TestValidate_createAccessoryKeysRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Test the good workflow
	cmdRunner := fakeBBBuildersRunner("chromeos/staging", []string{"staging-key-manager"})
	f := createAccessoryKeysRun{
		tryRunBase: tryRunBase{
			cmdRunner: cmdRunner,
			bbClient:  bb.NewClient(cmdRunner, nil, nil),
		},
		buildTarget: "atlas",
		accessory:   "kelpie",
		bug:         1337,
	}
	assert.NilError(t, f.validate(ctx))

	// No build target provided.
	f.buildTarget = ""
	assert.NonNilError(t, f.validate(ctx))

	// No accessory provided.
	f.buildTarget = "atlas"
	f.accessory = ""
	assert.NonNilError(t, f.validate(ctx))

	// No bug provided.
	f.buildTarget = "atlas"
	f.bug = 0
	assert.NonNilError(t, f.validate(ctx))
}

type createAccessoryKeysTestConfig struct {
	buildTarget string
	accessory   string
	keyVersion  int
	isPreMp     bool
	dryrun      bool
	production  bool
}

func doCreateAccessoryKeysTest(t *testing.T, tc *createAccessoryKeysTestConfig) {
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

	r := createAccessoryKeysRun{
		tryRunBase: tryRunBase{
			cmdRunner:  f,
			dryrun:     tc.dryrun,
			production: tc.production,
		},
		propsFile:   propsFile,
		buildTarget: tc.buildTarget,
		accessory:   tc.accessory,
		keyVersion:  int32(tc.keyVersion),
		isPreMp:     tc.isPreMp,
		bug:         4201337,
	}
	ret := r.Run(nil, nil, nil)
	assert.IntsEqual(t, ret, Success)

	properties, err := bb.ReadStructFromFile(propsFile.Name())
	assert.NilError(t, err)

	// Check that the requests are populated correctly.
	jsonRequest, err := properties.GetFields()["create_accessory_keys_request"].GetStructValue().MarshalJSON()
	assert.NilError(t, err)
	var createAccessoryKeysRequest bapipb.CreateAccessoryKeyRequest
	err = protojson.Unmarshal([]byte(jsonRequest), &createAccessoryKeysRequest)
	assert.NilError(t, err)

	assert.StringsEqual(
		t,
		createAccessoryKeysRequest.BuildTarget.Name,
		tc.buildTarget,
	)

	assert.StringsEqual(
		t,
		createAccessoryKeysRequest.Accessory,
		tc.accessory,
	)

	assert.BoolsEqual(
		t,
		createAccessoryKeysRequest.IsPreMp,
		tc.isPreMp,
	)

	keyVersion := properties.GetFields()["keyVersion"]
	if keyVersion != nil {
		assert.IntsEqual(
			t,
			int(keyVersion.GetNumberValue()),
			tc.keyVersion,
		)
	}

	bugId := properties.GetFields()["bug"].GetNumberValue()
	assert.IntsEqual(
		t,
		int(bugId),
		4201337,
	)
}

func TestCreateAccessoryKeys_dryrun(t *testing.T) {
	t.Parallel()
	doCreateAccessoryKeysTest(t, &createAccessoryKeysTestConfig{
		buildTarget: "atlas",
		accessory:   "kelpie",
		dryrun:      true,
	})
}

func TestCreateAccessoryKeys_production_success(t *testing.T) {
	t.Parallel()
	doCreateAccessoryKeysTest(t, &createAccessoryKeysTestConfig{
		buildTarget: "atlas",
		accessory:   "kelpie",
		production:  true,
	})
}

func TestCreateAccessoryKeys_preMp_production_success(t *testing.T) {
	t.Parallel()
	doCreateAccessoryKeysTest(t, &createAccessoryKeysTestConfig{
		buildTarget: "atlas",
		isPreMp:     true,
		accessory:   "kelpie",
		production:  true,
	})
}

func TestCreateAccessoryKeys_incrementedVersion_production_success(t *testing.T) {
	t.Parallel()
	doCreateAccessoryKeysTest(t, &createAccessoryKeysTestConfig{
		buildTarget: "atlas",
		keyVersion:  4,
		accessory:   "kelpie",
		production:  true,
	})
}

func TestCreateAccessoryKeys_staging_success(t *testing.T) {
	t.Parallel()
	doCreateAccessoryKeysTest(t, &createAccessoryKeysTestConfig{
		buildTarget: "atlas",
		accessory:   "kelpie",
	})
}
