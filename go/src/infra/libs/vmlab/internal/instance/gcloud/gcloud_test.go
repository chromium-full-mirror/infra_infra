// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gcloud

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/infra/libs/vmlab/api"
)

type fakeRandomGenerator struct{}

func (c fakeRandomGenerator) GetRandHex(l int) (string, error) {
	return "aaaaaa", nil
}

type mockCommander struct {
	Commands *[][]string
	Outputs  *[][]byte
}

func (c mockCommander) GetCommandOutput(command string, args ...string) ([]byte, error) {
	// Unused field are deleted.
	if c.Commands != nil {
		thisCommand := []string{}
		thisCommand = append(thisCommand, command)
		thisCommand = append(thisCommand, args...)
		*c.Commands = append(*c.Commands, thisCommand)
	}
	CurrentOutput := (*c.Outputs)[0]
	(*c.Outputs) = (*c.Outputs)[1:]
	return CurrentOutput, nil
}

var INSTNACE_WITH_PUBLIC_IP = []byte(`
  {
    "name": "vmlab-1112312312",
    "machineType": "https://www.googleapis.com/compute/v1/projects/vmlab-project/zones/us-west2-a/machineTypes/n2-standard-4",
    "zone": "https://www.googleapis.com/compute/v1/projects/vmlab-project/zones/us-west-2",
    "networkInterfaces": [
      {
        "accessConfigs": [
          {
            "natIP": "8.8.8.8"
          }
        ],
        "networkIP": "192.168.0.1"
      }
    ]
  }
`)

var INSTNACE_WITHOUT_PUBLIC_IP = []byte(`
  {
    "name": "vmlab-1234123481",
    "machineType": "https://www.googleapis.com/compute/v1/projects/vmlab-project/zones/us-west2-a/machineTypes/n2-standard-8",
    "zone": "https://www.googleapis.com/compute/v1/projects/vmlab-project/zones/us-west-3",
    "networkInterfaces": [
      {
        "networkIP": "192.168.0.2"
      }
    ]
  }
`)

var CREATE_SUCCESS_OUTPUT = []byte(fmt.Sprintf("[\n%s\n]", string(INSTNACE_WITH_PUBLIC_IP)))

var LIST_SUCCESS_OUTPUT = []byte(fmt.Sprintf("[\n%s,\n%s\n]", string(INSTNACE_WITH_PUBLIC_IP), string(INSTNACE_WITHOUT_PUBLIC_IP)))

var EMPTY_OUTPUT = []byte(``)

func TestCreateWithPublicIpAddress(t *testing.T) {
	gcloud, _ := New()
	mockExecCommand := mockCommander{
		Commands: &[][]string{},
		Outputs: &[][]byte{
			CREATE_SUCCESS_OUTPUT,
		},
	}
	execCommand = mockExecCommand
	random = fakeRandomGenerator{}
	instance, err := gcloud.Create(
		context.Background(),
		&api.CreateVmInstanceRequest{
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &api.Config_GCloudBackend{
						Project:        "vmlab-project",
						Zone:           "us-west-2",
						MachineType:    "n2-standard-4",
						InstancePrefix: "vmlab-",
						Network:        "default",
						Subnet:         "default",
						PublicIp:       true,
						Image: &api.GceImage{
							Project: "imagestorage-project",
							Name:    "betty-arc-r-release-r110-111111111111",
						},
					},
				},
			},
		})
	expectedCommand := []string{
		"gcloud", "compute", "instances", "create", "vmlab-aaaaaa",
		"--project=vmlab-project", "--image=betty-arc-r-release-r110-111111111111",
		"--image-project=imagestorage-project", "--machine-type=n2-standard-4", "--no-scopes",
		"--zone=us-west-2", "--format=json", "--network=default", "--subnet=default"}
	if diff := cmp.Diff(*mockExecCommand.Commands, [][]string{expectedCommand}); diff != "" {
		t.Errorf("Executed wrong command: %v", diff)
	}
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !strings.HasPrefix(instance.GetName(), "vmlab-") {
		t.Errorf("Instance name incorrect: %v", instance)
	}
	expectedSshTarget := &api.AddressPort{Address: "8.8.8.8", Port: 22}
	if diff := cmp.Diff(instance.GetSsh(), expectedSshTarget, protocmp.Transform()); diff != "" {
		t.Errorf("Got wrong ssh target: %v Diff is:\n%v", instance.GetSsh(), diff)
	}
}

func TestCreateWithPublicIpAddressAndSshInteralIp(t *testing.T) {
	gcloud, _ := New()
	mockExecCommand := mockCommander{
		Commands: &[][]string{},
		Outputs: &[][]byte{
			CREATE_SUCCESS_OUTPUT,
		},
	}
	execCommand = mockExecCommand
	random = fakeRandomGenerator{}
	instance, _ := gcloud.Create(
		context.Background(),
		&api.CreateVmInstanceRequest{
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &api.Config_GCloudBackend{
						Project:             "vmlab-project",
						Zone:                "us-west-2",
						MachineType:         "n2-standard-4",
						InstancePrefix:      "vmlab-",
						Network:             "default",
						Subnet:              "default",
						PublicIp:            true,
						AlwaysSshInternalIp: true,
						Image: &api.GceImage{
							Project: "imagestorage-project",
							Name:    "betty-arc-r-release-r110-111111111111",
						},
					},
				},
			},
		})
	expectedSshTarget := &api.AddressPort{Address: "192.168.0.1", Port: 22}
	if diff := cmp.Diff(instance.GetSsh(), expectedSshTarget, protocmp.Transform()); diff != "" {
		t.Errorf("Got wrong ssh target: %v Diff is:\n%v", instance.GetSsh(), diff)
	}
}

func TestCreateWithInternalIpAddress(t *testing.T) {
	gcloud, _ := New()
	mockExecCommand := mockCommander{
		Commands: &[][]string{},
		Outputs: &[][]byte{
			CREATE_SUCCESS_OUTPUT,
		},
	}
	execCommand = mockExecCommand
	random = fakeRandomGenerator{}
	instance, err := gcloud.Create(
		context.Background(),
		&api.CreateVmInstanceRequest{
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &api.Config_GCloudBackend{
						Project:        "vmlab-project",
						Zone:           "us-west-2",
						MachineType:    "n2-standard-4",
						InstancePrefix: "vmlab-",
						Network:        "default",
						Subnet:         "default",
						PublicIp:       false,
						Image: &api.GceImage{
							Project: "imagestorage-project",
							Name:    "betty-arc-r-release-r110-111111111111",
						},
					},
				},
			},
		})
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	expectedCommand := []string{
		"gcloud", "compute", "instances", "create", "vmlab-aaaaaa",
		"--project=vmlab-project", "--image=betty-arc-r-release-r110-111111111111",
		"--image-project=imagestorage-project", "--machine-type=n2-standard-4", "--no-scopes",
		"--zone=us-west-2", "--format=json", "--network=default", "--subnet=default",
		"--no-address"}
	if diff := cmp.Diff(*mockExecCommand.Commands, [][]string{expectedCommand}); diff != "" {
		t.Errorf("Executed wrong command: %v", diff)
	}
	if !strings.HasPrefix(instance.GetName(), "vmlab-") {
		t.Errorf("Instance name incorrect: %v", instance)
	}
	expectedSshTarget := &api.AddressPort{Address: "192.168.0.1", Port: 22}
	if diff := cmp.Diff(instance.GetSsh(), expectedSshTarget, protocmp.Transform()); diff != "" {
		t.Errorf("Got wrong ssh target: %v Diff is:\n%v", instance.GetSsh(), diff)
	}
}

func TestCreateWithTags(t *testing.T) {
	gcloud, _ := New()
	mockExecCommand := mockCommander{
		Commands: &[][]string{},
		Outputs: &[][]byte{
			CREATE_SUCCESS_OUTPUT,
		},
	}
	execCommand = mockExecCommand
	random = fakeRandomGenerator{}
	instance, err := gcloud.Create(
		context.Background(),
		&api.CreateVmInstanceRequest{
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &api.Config_GCloudBackend{
						Project:        "vmlab-project",
						Zone:           "us-west-2",
						MachineType:    "n2-standard-4",
						InstancePrefix: "vmlab-",
						Network:        "default",
						Subnet:         "default",
						PublicIp:       false,
						Image: &api.GceImage{
							Project: "imagestorage-project",
							Name:    "betty-arc-r-release-r110-111111111111",
						},
					},
				},
			},
			Tags: map[string]string{
				"swarming-bot": "vm-bot-1",
				"label-model":  "betty",
			},
		})
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	expectedCommand := []string{
		"gcloud", "compute", "instances", "create", "vmlab-aaaaaa",
		"--project=vmlab-project", "--image=betty-arc-r-release-r110-111111111111",
		"--image-project=imagestorage-project", "--machine-type=n2-standard-4", "--no-scopes",
		"--zone=us-west-2", "--format=json", "--network=default", "--subnet=default",
		"--no-address", "--labels=label-model=betty,swarming-bot=vm-bot-1"}
	if diff := cmp.Diff(*mockExecCommand.Commands, [][]string{expectedCommand}); diff != "" {
		t.Errorf("Executed wrong command: %v", diff)
	}
	if !strings.HasPrefix(instance.GetName(), "vmlab-") {
		t.Errorf("Instance name incorrect: %v", instance)
	}
	expectedSshTarget := &api.AddressPort{Address: "192.168.0.1", Port: 22}
	if diff := cmp.Diff(instance.GetSsh(), expectedSshTarget, protocmp.Transform()); diff != "" {
		t.Errorf("Got wrong ssh target: %v Diff is:\n%v", instance.GetSsh(), diff)
	}
}

func TestCreateMissingProject(t *testing.T) {
	gcloud, _ := New()
	_, err := gcloud.Create(
		context.Background(),
		&api.CreateVmInstanceRequest{
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &api.Config_GCloudBackend{},
				},
			},
		})
	if err.Error() != "invalid config argument: project must be set" {
		t.Errorf("expect project not set error, but got: %s", err.Error())
	}
}

func TestDeleteMissingZone(t *testing.T) {
	gcloud, _ := New()
	err := gcloud.Delete(
		context.Background(),
		&api.VmInstance{
			Name: "instance1",
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &api.Config_GCloudBackend{
						Project: "vmlab-project",
					},
				},
			},
		})
	if err.Error() != "zone must be set" {
		t.Errorf("expect zone not set error, but got: %s", err.Error())
	}
}

func TestDelete(t *testing.T) {
	gcloud, _ := New()
	mockExecCommand := mockCommander{
		Commands: &[][]string{},
		Outputs:  &[][]byte{EMPTY_OUTPUT},
	}
	execCommand = mockExecCommand
	err := gcloud.Delete(
		context.Background(),
		&api.VmInstance{
			Name: "instance1",
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &api.Config_GCloudBackend{
						Project: "vmlab-project",
						Zone:    "us-west-2",
					},
				},
			},
		})
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	expectedCommand := []string{
		"gcloud", "compute", "instances", "delete", "instance1",
		"--project=vmlab-project", "--zone=us-west-2", "--quiet"}
	if diff := cmp.Diff(*mockExecCommand.Commands, [][]string{expectedCommand}); diff != "" {
		t.Errorf("Executed wrong command: %v", diff)
	}
}

func TestList(t *testing.T) {
	gcloud, _ := New()
	mockExecCommand := mockCommander{
		Commands: &[][]string{},
		Outputs: &[][]byte{
			LIST_SUCCESS_OUTPUT,
		},
	}
	execCommand = mockExecCommand
	instances, err := gcloud.List(
		context.Background(),
		&api.ListVmInstancesRequest{
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &api.Config_GCloudBackend{
						Project:        "vmlab-project",
						Zone:           "us-west-2",
						MachineType:    "n2-standard-4",
						InstancePrefix: "gcetest-",
					},
				},
			},
			TagFilters: map[string]string{
				"swarming-bot": "vm-bot-1",
				"label-model":  "betty",
			},
		})
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	expectedCommands := [][]string{
		// List Commands
		{
			"gcloud", "compute", "instances", "list",
			"--project=vmlab-project", "--filter", "name~^gcetest-.* zone:us-west-2 labels.label-model=betty labels.swarming-bot=vm-bot-1", "--format", "json",
		},
	}
	if diff := cmp.Diff(*mockExecCommand.Commands, expectedCommands); diff != "" {
		t.Errorf("Executed wrong command: %v", diff)
	}

	expectedInstances := []*api.VmInstance{
		{
			Name: "vmlab-1112312312",
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &api.Config_GCloudBackend{
						Project:     "vmlab-project",
						MachineType: "n2-standard-4",
						Zone:        "us-west-2",
					},
				},
			},
			Ssh: &api.AddressPort{
				Address: "8.8.8.8",
				Port:    22,
			},
		},
		{
			Name: "vmlab-1234123481",
			Config: &api.Config{
				Backend: &api.Config_GcloudBackend{
					GcloudBackend: &api.Config_GCloudBackend{
						Project:     "vmlab-project",
						MachineType: "n2-standard-8",
						Zone:        "us-west-3",
					},
				},
			},
			Ssh: &api.AddressPort{
				Address: "192.168.0.2",
				Port:    22,
			},
		},
	}
	if diff := cmp.Diff(instances, expectedInstances, protocmp.Transform()); diff != "" {
		t.Errorf("Wrong instances returned: %v", diff)
	}
}
