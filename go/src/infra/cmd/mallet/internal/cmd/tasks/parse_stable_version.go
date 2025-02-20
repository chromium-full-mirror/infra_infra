// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/maruel/subcommands"

	"go.chromium.org/chromiumos/infra/proto/go/lab_platform"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/errors"

	commonFlags "go.chromium.org/infra/cmd/mallet/internal/cmd/cmdlib"
	"go.chromium.org/infra/cmd/mallet/internal/site"
	"go.chromium.org/infra/cmdsupport/cmdlib"
)

var ParseStableVersion = &subcommands.Command{
	UsageLine: "parse-stable-version",
	ShortDesc: "Parse stable-version file for startlark.",
	CommandRun: func() subcommands.CommandRun {
		c := &parseStableVersionRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.CommonFlags.Register(&c.Flags)
		c.envFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.targetFile, "target", "", "Path to target file. Example: `/usr/local/google/home/{user}/chromeos/infra/config/lab_platform/stable_version_data/stable_versions.cfg`")
		c.Flags.StringVar(&c.compareFile, "compare", "", "Path to comparing file. Example: `/usr/local/google/home/{user}/chromeos/infra/config/lab_platform/generated/stable_versions.cfg`")
		return c
	},
}

type parseStableVersionRun struct {
	subcommands.CommandRunBase
	commonFlags.CommonFlags
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	targetFile  string
	compareFile string
}

// Run initiates execution of local recovery.
func (c *parseStableVersionRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// innerRun executes internal logic of control.
func (c *parseStableVersionRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if c.targetFile == "" {
		return errors.Reason("target file is not provided").Err()
	}
	target, err := readFile(c.targetFile)
	if err != nil {
		return errors.Annotate(err, "load target file").Err()
	}
	targetData := generateMethods(target)
	if c.compareFile != "" {
		cmp, err := readFile(c.compareFile)
		if err != nil {
			return errors.Annotate(err, "load compare file").Err()
		}
		cmpData := generateMethods(cmp)
		if len(cmpData) != len(targetData) {
			return errors.Reason("protos has diff length").Err()
		}
		for i, v := range cmpData {
			if v != targetData[i] {
				return errors.Reason("record diff: %q!=%q", v, targetData[i]).Err()
			}
		}
		log.Println("Files are equal!")
	} else {
		log.Println("Generated methods")
		log.Printf("\n %v\n", targetData)
	}
	return nil
}

func readFile(file string) (*lab_platform.StableVersions, error) {
	jsonFile, err := os.Open(file)
	if err != nil {
		return nil, errors.Annotate(err, "read file").Err()
	}
	defer jsonFile.Close()
	scv := &lab_platform.StableVersions{}
	if err := jsonpb.Unmarshal(jsonFile, scv); err != nil {
		return nil, errors.Annotate(err, "read file").Err()
	}
	log.Printf("Finished parsing %q!\n", file)
	return scv, nil
}

func generateMethods(src *lab_platform.StableVersions) []string {
	versions := make(map[string]*Version)
	keys := make(map[string]*Key)
	for _, item := range src.GetCros() {
		key := Key{
			Board: item.GetKey().GetBuildTarget().GetName(),
			Model: item.GetKey().GetModelId().GetValue(),
		}
		k := fmt.Sprintf("%s|%s", key.Board, key.Model)
		keys[k] = &key
		if v, ok := versions[k]; ok {
			v.Os = item.GetVersion()
		} else {
			versions[k] = &Version{
				Os: item.GetVersion(),
			}
		}
	}
	for _, item := range src.GetFaft() {
		key := Key{
			Board: item.GetKey().GetBuildTarget().GetName(),
			Model: item.GetKey().GetModelId().GetValue(),
		}
		k := fmt.Sprintf("%s|%s", key.Board, key.Model)
		keys[k] = &key
		if v, ok := versions[k]; ok {
			v.FwImage = item.GetVersion()
		} else {
			versions[k] = &Version{
				FwImage: item.GetVersion(),
			}
		}
	}
	for _, item := range src.GetFirmware() {
		key := Key{
			Board: item.GetKey().GetBuildTarget().GetName(),
			Model: item.GetKey().GetModelId().GetValue(),
		}
		k := fmt.Sprintf("%s|%s", key.Board, key.Model)
		keys[k] = &key
		if v, ok := versions[k]; ok {
			v.Fw = item.GetVersion()
		} else {
			versions[k] = &Version{
				Fw: item.GetVersion(),
			}
		}
	}
	keyNames := make([]string, 0, len(versions))
	for k := range keys {
		keyNames = append(keyNames, k)
	}
	sort.Strings(keyNames)
	r := make([]string, 0, len(keyNames))
	for _, key := range keyNames {
		v := versions[key]
		k := keys[key]
		r = append(r, fmt.Sprintf("addVersion(board=%q, model=%q, os=%q, fw=%q, fw_image=%q) \n", k.Board, k.Model, v.Os, v.Fw, v.FwImage))
	}
	return r
}

type Key struct {
	Board string
	Model string
}
type Version struct {
	Os      string
	Fw      string
	FwImage string
}
