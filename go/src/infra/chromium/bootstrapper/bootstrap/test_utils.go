// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bootstrap

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/luciexe/exe"

	"go.chromium.org/infra/chromium/util"
)

func jsonToStruct(json string) *structpb.Struct {
	s := &structpb.Struct{}
	util.PanicOnError(protojson.Unmarshal([]byte(json), s))
	return s
}

func setPropertiesFromJson(build *buildbucketpb.Build, propsJson map[string]string) {
	props := make(map[string]any, len(propsJson))
	for key, p := range propsJson {
		s := &structpb.Value{}
		util.PanicOnError(protojson.Unmarshal([]byte(p), s))
		props[key] = s
	}
	util.PanicOnError(exe.WriteProperties(build.Input.Properties, props))
}

func setBootstrapPropertiesProperties(build *buildbucketpb.Build, propsJson string) {
	setPropertiesFromJson(build, map[string]string{
		"$bootstrap/properties": propsJson,
	})
}

func setBootstrapExeProperties(build *buildbucketpb.Build, propsJson string) {
	setPropertiesFromJson(build, map[string]string{
		"$bootstrap/exe": propsJson,
	})
}

func setBootstrapTriggerProperties(build *buildbucketpb.Build, propsJson string) {
	setPropertiesFromJson(build, map[string]string{
		"$bootstrap/trigger": propsJson,
	})
}

func getInput(build *buildbucketpb.Build) *Input {
	input, err := InputOptions{}.NewInput(build)
	util.PanicOnError(err)
	return input
}
