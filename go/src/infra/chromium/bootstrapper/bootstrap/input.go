// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bootstrap

import (
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/led/ledcmd"
	"go.chromium.org/luci/luciexe/exe"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
)

// InputOptions provides options that are inputs to the bootstrapping process.
type InputOptions struct {
	// Polymorphic changes the bootstrapper's behavior to support polymorphic builders: builders
	// that bootstrap properties for other builders. Polymorphic builders will prefer the
	// build's properties over the properties in the properties file with the expectation that
	// any properties that the polymorphic builder is setting its definition are required for
	// proper operation of the builder (e.g. recipe).
	Polymorphic bool
	// PropertiesOptional changes the bootstrapper's behavior to not fail if
	// the $bootstrap/properties property is not set or if the identified
	// file does not exist at the revision being bootstrapped.
	PropertiesOptional bool
}

// Input provides the relevant details from the build input to the operations
// that prepare a bootstrapped executable to run. It is safe to share a single
// instance between multiple operations that take Input.
type Input struct {
	// commits are the gitiles commits that the bootstrapper can examine when bootstrapping the
	// properties. The considered commits will be the commit that is specified in the build's
	// input if one is present and any commits specified by the $bootstrap/trigger property.
	commits []*buildbucketpb.GitilesCommit
	// changes are the gerrit changes that the bootstrapper can examine when bootstrapping the
	// properties. The considered changes will be the changes in the build's input if any are
	// present.
	changes []*buildbucketpb.GerritChange
	// buildProperties are the properties set in the build's input.
	buildProperties *structpb.Struct
	// buildRequestedProperties are the properties requested when the build was scheduled.
	buildRequestedProperties *structpb.Struct
	// ledEditedProperties are the properties that were modified by led.
	ledEditedProperties *structpb.Struct
	// ledRemovedProperties are the properties that were removed by led.
	ledRemovedProperties []string
	// shadowBuild indicates whether the build is a shadow build i.e. an lead real build
	shadowBuild bool
	// polymorphic changes the behavior to support polymoprhic builders by prioritizing build
	// properties over properties from the properties file
	polymorphic bool
	// propertiesOptional prevents a lack of $bootstrap/properties property or the properties
	// file from being treated as an error.
	propertiesOptional bool
	// propsProperties is the value of the build's $bootstrap/properties property, if present.
	// This controls where the properties for the builder are looked up.
	propsProperties *BootstrapPropertiesProperties
	// exeProperties is the value of the build's $bootstrap/exe property. This controls the
	// executable that the bootstrapper will launch.
	exeProperties *BootstrapExeProperties
	// casRecipeBundle is the value of the build's led_cas_recipe_bundle property, if present.
	// This will be set when using "led edit-recipe-bundle" and will override the bundle to use
	// for the executable.
	casRecipeBundle *apipb.CASReference
}

// NewInput creates a new Input, returning an error if build input fails
// validation.
//
// The build input can fail to validate for the following reasons:
//   - The $bootstrap/properties property is not set and
//     o.PropertiesOptional is false.
//   - The $bootstrap/properties is set, but does not contain a valid
//     BootstrapPropertiesProperties message.
//   - The $bootstrap/exe property is not set.
//   - The $bootstrap/exe property is set, but does not contain a valid
//     BootstrapExeProperties message.
func (o InputOptions) NewInput(build *buildbucketpb.Build) (*Input, error) {
	properties := build.GetInput().GetProperties()
	if properties == nil {
		properties = &structpb.Struct{}
	}

	requestedProperties := build.GetInfra().GetBuildbucket().GetRequestedProperties()

	shadowBuild := false
	ledProps := properties.Fields["$recipe_engine/led"].GetStructValue()
	if ledProps != nil {
		shadowBucket := ledProps.Fields["shadowed_bucket"].GetStringValue()
		if shadowBucket != "" {
			shadowBuild = true
		}
	}

	// Check for the presence of required properties
	exeProperties := &BootstrapExeProperties{}
	propsToParse := map[string]any{
		"$bootstrap/exe": exeProperties,
	}

	var propsProperties *BootstrapPropertiesProperties
	addPropsProperties := func() {
		propsProperties = &BootstrapPropertiesProperties{}
		propsToParse["$bootstrap/properties"] = propsProperties
	}
	if !o.PropertiesOptional {
		addPropsProperties()
	}

	missingProps := make([]string, 0, len(propsToParse))
	for k := range propsToParse {
		if _, ok := properties.Fields[k]; !ok {
			missingProps = append(missingProps, k)
		}
	}
	if len(missingProps) != 0 {
		sort.Strings(missingProps)
		return nil, errors.Reason("the following required properties are not set: %s", strings.Join(missingProps, ", ")).Err()
	}

	if o.PropertiesOptional {
		if _, ok := properties.Fields["$bootstrap/properties"]; ok {
			addPropsProperties()
		}
	}
	triggerProperties := &BootstrapTriggerProperties{}
	propsToParse["$bootstrap/trigger"] = triggerProperties
	casRecipeBundle := &apipb.CASReference{}
	propsToParse[ledcmd.CASRecipeBundleProperty] = casRecipeBundle
	ledEditedProperties := &structpb.Struct{}
	propsToParse["led_edited_properties"] = ledEditedProperties
	var ledRemovedProperties []string
	propsToParse["led_removed_properties"] = &ledRemovedProperties

	if err := exe.ParseProperties(properties, propsToParse); err != nil {
		return nil, errors.Annotate(err, "failed to parse properties").Err()
	}

	if propsProperties != nil {
		if err := validate(propsProperties, "$bootstrap/properties"); err != nil {
			return nil, errors.Annotate(err, "failed to validate $bootstrap/properties property").Err()
		}
	}
	if err := validate(exeProperties, "$bootstrap/exe"); err != nil {
		return nil, errors.Annotate(err, "failed to validate $bootstrap/exe property").Err()
	}

	if casRecipeBundle.Digest == nil {
		casRecipeBundle = nil
	}
	// When led CAS recipe bundle property is set, it also shows up in led_edited_properties,
	// this leads to us applying the led CAS recipe bundle property because we reapply
	// everything in led_edited_properties, so remove it
	delete(ledEditedProperties.Fields, ledcmd.CASRecipeBundleProperty)
	if len(ledEditedProperties.Fields) == 0 {
		ledEditedProperties = nil
	}

	commits := []*buildbucketpb.GitilesCommit{}
	if build.Input.GitilesCommit != nil {
		commits = append(commits, proto.Clone(build.Input.GitilesCommit).(*buildbucketpb.GitilesCommit))
	}
	commits = append(commits, triggerProperties.Commits...)

	changes := make([]*buildbucketpb.GerritChange, len(build.Input.GerritChanges))
	for i, change := range build.Input.GerritChanges {
		changes[i] = proto.Clone(change).(*buildbucketpb.GerritChange)
	}

	properties = proto.Clone(properties).(*structpb.Struct)
	for k := range propsToParse {
		delete(properties.Fields, k)
		delete(requestedProperties.GetFields(), k)
	}

	// The bootstrapped executable doesn't need the led_builder_is_bootstrapped property set
	delete(properties.Fields, "led_builder_is_bootstrapped")

	input := &Input{
		commits:                  commits,
		changes:                  changes,
		buildProperties:          properties,
		buildRequestedProperties: requestedProperties,
		ledEditedProperties:      ledEditedProperties,
		ledRemovedProperties:     ledRemovedProperties,
		shadowBuild:              shadowBuild,
		polymorphic:              o.Polymorphic,
		propertiesOptional:       o.PropertiesOptional,
		propsProperties:          propsProperties,
		exeProperties:            exeProperties,
		casRecipeBundle:          casRecipeBundle,
	}
	return input, nil
}
