// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package misc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/models"
	"go.chromium.org/infra/cros/satlab/common/services/build_service"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/parser"
)

// StageAndWriteLocalStableVersion stages a recovery image to partner bucket and writes the associated rv metadata locally.
func StageAndWriteLocalStableVersion(
	ctx context.Context,
	service build_service.IBuildService,
	rv *models.RecoveryVersion,
	path string,
) error {
	buildVersion := strings.Split(rv.OsImage, "-")[1]
	fwImageBuildVersion, err := parser.ExtractFwImageBuildVersionFrom(rv.FwImage)
	if err != nil {
		return errors.Annotate(err, "get firmware image build version").Err()
	}
	bucket := site.GetGCSImageBucket()
	if bucket == "" {
		return errors.New("GCS_IMAGE_BUCKET not found")
	}
	if _, err := service.StageBuild(ctx, rv.Board, rv.Model, buildVersion, bucket, build_service.Release); err != nil {
		return errors.Annotate(err, "stage stable version image to bucket").Err()
	}
	buildArtifact, err := service.StageBuild(ctx, rv.Board, rv.Model, fwImageBuildVersion, bucket, build_service.Firmware)
	if err != nil {
		return errors.Annotate(err, "stage stable version firmware image to bucket").Err()
	}
	// FwImage can be in different locations, staging will return the correct place.
	rv.FwImage = buildArtifact.GetPath()
	if err := WriteLocalStableVersion(rv, path); err != nil {
		return errors.Annotate(err, "write local stable version").Err()
	}
	return nil
}

// WriteLocalStableVersion saves a recovery version to the specified directory and creates the directory if necessary.
func WriteLocalStableVersion(recoveryVersion *models.RecoveryVersion, path string) error {

	// Check if recoveryVersions directory created
	if _, err := os.Stat(path); err != nil {
		return err
	}

	fname := fmt.Sprintf("%s%s-%s.json", path, recoveryVersion.Board, recoveryVersion.Model)
	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	// close file on exit and check for its returned error
	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

	rv, err := json.MarshalIndent(recoveryVersion, "", " ")
	if err != nil {
		return errors.Annotate(err, "marshal recovery version").Err()
	}
	if _, err := f.Write(rv); err != nil {
		return err
	}
	fmt.Println("Recovery Version written locally: ", string(rv))
	return nil
}

// StableVersionFromFile reads stable version from file.
func StableVersionFromFile(board string, model string) (*models.RecoveryVersion, error) {
	fname := fmt.Sprintf("%s%s-%s.json", site.RecoveryVersionDirectory, board, model)
	f, err := os.ReadFile(fname)
	if err != nil {
		return &models.RecoveryVersion{}, errors.Annotate(err, "read stable version file").Err()
	}
	rv := &models.RecoveryVersion{}
	if err := json.Unmarshal([]byte(f), rv); err != nil {
		return &models.RecoveryVersion{}, errors.Annotate(err, "unmarshal json").Err()
	}
	return rv, nil
}

// MakeTempFile makes a temporary file.
func MakeTempFile(content string) (string, error) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		return "", errors.Annotate(err, "makeTempFile").Err()
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return "", errors.Annotate(err, "makeTempFile").Err()
	}
	if err := os.WriteFile(name, []byte(content), 0o077); err != nil {
		return "", errors.Annotate(err, "makeTempFile").Err()
	}
	return name, nil
}

// TrimOutput trims trailing whitespace from command output.
func TrimOutput(output []byte) string {
	if len(output) == 0 {
		return ""
	}
	return strings.TrimRight(string(output), "\n\t")
}

// AskConfirmation asks users a question for Y/N answer.
func AskConfirmation(s string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/n]: ", s)
		response, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" || response == "Y" {
			return true, nil
		} else if response == "n" || response == "no" || response == "N" {
			return false, nil
		}
	}
}

// GetEnv is helper to get env variables and falling back if not set.
func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// BoolVal fetches a bool val from a string. Returns false if unable to parse.
func BoolVal(val string) bool {
	b, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}
	return b
}

// StrTestArgsToMap converts a TestArgs string into map[string]string.
func StrTestArgsToMap(str string) map[string]string {
	res := make(map[string]string)
	for _, s := range strings.Fields(str) {
		keyVal := strings.Split(s, "=")
		if len(keyVal) != 2 {
			fmt.Printf("Warning! Not valid testArgs format. Got %s, want key=value\n", s)
			continue
		}
		res[keyVal[0]] = keyVal[1]
	}
	return res
}

// MapTestArgsToStr converts a TestArgs map into string.
func MapTestArgsToStr(m map[string]string) string {
	res := new(bytes.Buffer)
	for key, val := range m {
		fmt.Fprintf(res, "%s=%s ", key, val)
	}
	return strings.TrimSpace(res.String())
}

// RemovePrefixFromTestArgs removes requested prefix from testArgs key.
// It also removes args without requested prefix.
func RemovePrefixFromTestArgs(m map[string]string, prefix string) map[string]string {
	res := map[string]string{}
	for key, val := range m {
		newKey, found := strings.CutPrefix(key, prefix)
		if !found {
			fmt.Printf("Warning! Not valid testArgs prefix. Got %s, want %s%[1]s\n", key, prefix)
			continue
		}
		res[newKey] = val
	}
	return res
}

func IsCustomBuild(build string) bool {
	return strings.ContainsAny(build, "-_")
}
