// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gcloud

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"go.chromium.org/infra/libs/vmlab/api"
)

// TODO(b/250961857): implement gcloud provider
// gcloudInstanceApi implements api.InstanceApi. The struct itself doesn't need
// to be public.
type gcloudInstanceApi struct{}

type gcloudResponseInstance struct {
	Name              string
	Zone              string
	MachineType       string
	NetworkInterfaces []gcloudNetworkInterface
}

type gcloudNetworkInterface struct {
	AccessConfigs []gcloudAccessConfig
	NetworkIP     string
}

type gcloudAccessConfig struct {
	NatIP string
}

// An interface that wraps some function of os/exec to allow unittest mock.
type commander interface {
	GetCommandOutput(string, ...string) ([]byte, error)
}

type realCommander struct{}

func (c realCommander) GetCommandOutput(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).Output()
}

// A utility method that extracts explicit message from an error, especially for
// the error returned by exec.Cmd.Output().
func extractErrorMessage(err error) string {
	if ee, ok := err.(*exec.ExitError); ok {
		return string(ee.Stderr)
	}
	return err.Error()
}

// An interface that wraps some function of crypto/rand to allow unittest mock.
type randomGenerator interface {
	GetRandHex(l int) (string, error)
}

type realRandomGenerator struct{}

func (c realRandomGenerator) GetRandHex(l int) (string, error) {
	b := make([]byte, l/2+1)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b)[:l], nil
}

var execCommand commander = realCommander{}
var random randomGenerator = realRandomGenerator{}

var TAG_KEY_FORMAT = regexp.MustCompile(`[a-z][a-z\-_0-9]*`)
var TAG_VALUE_FORMAT = regexp.MustCompile(`[a-z\-_0-9]+`)

// New constructs a new api.InstanceApi with gcloud backend.
func New() (api.InstanceApi, error) {
	return &gcloudInstanceApi{}, nil
}

func extractBaseNameFromGceResourceUri(uri string) string {
	return uri[strings.LastIndex(uri, "/")+1:]
}

func checkGCloudConfigCommon(gcloudConfig *api.Config_GCloudBackend) error {
	if gcloudConfig.GetProject() == "" {
		return errors.New("project must be set")
	}

	if gcloudConfig.GetInstancePrefix() == "" {
		return errors.New("instance prefix must be set")
	}
	return nil
}

func checkGCloudConfigForCreate(gcloudConfig *api.Config_GCloudBackend) error {
	if err := checkGCloudConfigCommon(gcloudConfig); err != nil {
		return err
	}

	if gcloudConfig.GetZone() == "" {
		return errors.New("zone must be set")
	}
	if gcloudConfig.GetMachineType() == "" {
		return errors.New("machine type must be set")
	}
	if gcloudConfig.GetImage().GetName() == "" {
		return errors.New("image name must be set")
	}
	if gcloudConfig.GetImage().GetProject() == "" {
		return errors.New("image project must be set")
	}

	return nil
}

func (g *gcloudInstanceApi) Create(ctx context.Context, req *api.CreateVmInstanceRequest) (*api.VmInstance, error) {
	gcloudConfig := req.GetConfig().GetGcloudBackend()
	if gcloudConfig == nil {
		return nil, fmt.Errorf("invalid argument: bad backend: want gcloud, got %v", req.GetConfig())
	}
	if err := checkGCloudConfigForCreate(gcloudConfig); err != nil {
		return nil, fmt.Errorf("invalid config argument: %w", err)
	}
	tagsToSet := []string{}
	for tagKey, tagValue := range req.Tags {
		if !TAG_KEY_FORMAT.MatchString(tagKey) {
			return nil, fmt.Errorf("Tag key doesn't match format: %v", tagKey)
		}
		if !TAG_VALUE_FORMAT.MatchString(tagValue) {
			return nil, fmt.Errorf("Tag value doesn't match format: %v", tagValue)
		}
		tagsToSet = append(tagsToSet, tagKey+"="+tagValue)
	}

	gcloudArgs := []string{}

	// Instance name is a str that prefixes with instance_prefix, followed by a few alphabet or numeric characters.
	randName, err := random.GetRandHex(25)
	if err != nil {
		return nil, fmt.Errorf("failed to generate instance name: %w", err)
	}
	name := fmt.Sprintf("%s%s", gcloudConfig.GetInstancePrefix(), randName)
	gcloudArgs = append(gcloudArgs, "compute", "instances", "create", name)

	gcloudArgs = append(gcloudArgs,
		"--project="+gcloudConfig.GetProject(),
		"--image="+gcloudConfig.GetImage().GetName(), "--image-project="+gcloudConfig.GetImage().GetProject(),
		"--machine-type="+gcloudConfig.GetMachineType(), "--no-scopes",
		"--zone="+gcloudConfig.GetZone(), "--format=json")

	if gcloudConfig.GetNetwork() != "" {
		gcloudArgs = append(gcloudArgs, "--network="+gcloudConfig.GetNetwork())
	}
	if gcloudConfig.GetSubnet() != "" {
		gcloudArgs = append(gcloudArgs, "--subnet="+gcloudConfig.GetSubnet())
	}

	if !gcloudConfig.GetPublicIp() {
		gcloudArgs = append(gcloudArgs, "--no-address")
	}
	if len(tagsToSet) > 0 {
		// Sort the tags to make sure it has fixed command for unittest.
		sort.Strings(tagsToSet)
		gcloudArgs = append(gcloudArgs, "--labels="+strings.Join(tagsToSet, ","))
	}

	out, err := execCommand.GetCommandOutput("gcloud", gcloudArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to launch instance: %v", extractErrorMessage(err))
	}
	var gcloudResult []gcloudResponseInstance
	if err := json.Unmarshal(out, &gcloudResult); err != nil {
		return nil, fmt.Errorf("unable to parse gcloud result: %w\n output is: %v\n", err, string(out))
	}

	ipAddress := gcloudResult[0].NetworkInterfaces[0].NetworkIP
	if gcloudConfig.GetPublicIp() && !gcloudConfig.GetAlwaysSshInternalIp() {
		ipAddress = gcloudResult[0].NetworkInterfaces[0].AccessConfigs[0].NatIP
	}

	return &api.VmInstance{
		Name: name,
		Ssh: &api.AddressPort{
			Address: ipAddress,
			Port:    22,
		},
		Config: req.GetConfig(),
	}, nil
}

func (g *gcloudInstanceApi) Delete(ctx context.Context, ins *api.VmInstance) error {
	gcloudConfig := ins.GetConfig().GetGcloudBackend()
	if gcloudConfig == nil {
		return fmt.Errorf("invalid argument: bad backend: want gcloud, got %v", ins.GetConfig())
	}
	if ins.GetName() == "" {
		return errors.New("instance name must be set")
	}
	if gcloudConfig.GetProject() == "" {
		return errors.New("project must be set")
	}
	if gcloudConfig.GetZone() == "" {
		return errors.New("zone must be set")
	}

	gcloudArgs := []string{"compute", "instances", "delete", ins.GetName()}
	gcloudArgs = append(gcloudArgs,
		"--project="+gcloudConfig.GetProject(),
		"--zone="+gcloudConfig.GetZone(), "--quiet")

	_, err := execCommand.GetCommandOutput("gcloud", gcloudArgs...)
	if err != nil {
		return fmt.Errorf("failed to launch instance: %v", extractErrorMessage(err))
	}

	return nil
}

func (g *gcloudInstanceApi) List(ctx context.Context, req *api.ListVmInstancesRequest) ([]*api.VmInstance, error) {
	result := []*api.VmInstance{}

	gcloudConfig := req.GetConfig().GetGcloudBackend()

	if gcloudConfig == nil {
		return result, fmt.Errorf("invalid argument: bad backend: want gcloud, got %v", req.GetConfig())
	}
	if err := checkGCloudConfigCommon(gcloudConfig); err != nil {
		return result, fmt.Errorf("invalid config argument: %w", err)
	}

	filters := []string{}
	for tagKey, tagValue := range req.TagFilters {
		if !TAG_KEY_FORMAT.MatchString(tagKey) {
			return result, fmt.Errorf("Tag key doesn't match format: %v", tagKey)
		}
		if !TAG_VALUE_FORMAT.MatchString(tagValue) {
			return result, fmt.Errorf("Tag value doesn't match format: %v", tagValue)
		}
		filters = append(filters, fmt.Sprintf("labels.%s=%s", tagKey, tagValue))
	}
	sort.Strings(filters)
	filters = append([]string{
		"name~^" + gcloudConfig.GetInstancePrefix() + ".*",
		"zone:" + gcloudConfig.GetZone(),
	}, filters...)

	gcloudListArgs := []string{}
	gcloudListArgs = append(gcloudListArgs, "compute", "instances", "list",
		"--project="+gcloudConfig.GetProject(),
		"--filter", strings.Join(filters, " "), "--format", "json")
	listOuts, err := execCommand.GetCommandOutput("gcloud", gcloudListArgs...)
	if err != nil {
		return result, fmt.Errorf("failed to search instances: %v", extractErrorMessage(err))
	}

	var gcloudResult []gcloudResponseInstance
	if err := json.Unmarshal(listOuts, &gcloudResult); err != nil {
		return result, fmt.Errorf("unable to parse gcloud result: %w\n output is: %v\n", err, string(listOuts))
	}

	for _, instanceJson := range gcloudResult {
		ipAddress := instanceJson.NetworkInterfaces[0].NetworkIP
		if len(instanceJson.NetworkInterfaces[0].AccessConfigs) > 0 {
			ipAddress = gcloudResult[0].NetworkInterfaces[0].AccessConfigs[0].NatIP
		}

		instance := &api.VmInstance{
			Name: instanceJson.Name,
			Ssh: &api.AddressPort{
				Address: ipAddress,
				Port:    22,
			},
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &api.Config_GCloudBackend{
						Project:     gcloudConfig.GetProject(),
						Zone:        extractBaseNameFromGceResourceUri(instanceJson.Zone),
						MachineType: extractBaseNameFromGceResourceUri(instanceJson.MachineType),
					},
				},
			},
		}

		result = append(result, instance)
	}

	return result, nil
}
