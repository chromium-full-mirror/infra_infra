// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"strings"

	"go.chromium.org/infra/cros/satlab/common/dut/shivas"
	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

type UpdateDUT struct {
	// DUT specification inputs.
	NewSpecsFile             string
	Hostname                 string
	Machine                  string
	Servo                    string
	ServoSerial              string
	ServoSetupType           string
	ServoFwChannel           string
	ServoDockerContainerName string
	Pools                    []string
	LicenseTypes             []string
	LicenseIds               []string
	Rpm                      string
	RpmOutlet                string
	DeploymentTicket         string
	Tags                     []string
	Description              string

	// Deploy task inputs.
	ForceDeploy bool
	DeployTags  []string

	// ACS DUT fields
	Chameleons        []string
	Cameras           []string
	AntennaConnection string
	Router            string
	Cables            []string
	Facing            string
	Light             string
	Carrier           string
	AudioBoard        bool
	AudioBox          bool
	Atrus             bool
	WifiCell          bool
	TouchMimo         bool
	CameraBox         bool
	Chaos             bool
	AudioCable        bool
	SmartUSBHub       bool

	// For use in determining if a flag is set
	FlagInputs map[string]bool

	// SatlabID the Satlab ID from the Command Line
	// if it is empty it will get the ID from the enviornment
	SatlabID string

	// Namespace for executing the `shivas`
	Namespace string
}

type flagmap = map[string][]string

func makeUpdateShivasFlags(c *UpdateDUT) flagmap {
	out := make(flagmap)

	if c.NewSpecsFile != "" {
		// do nothing
	}
	if c.Hostname != "" {
		// Do nothing. The hostname must be qualified when
		// passed to shivas.
	}
	if c.Machine != "" {
		// Do nothing. The asset tag must be qualified when passed to shivas.
	}
	if c.Servo != "" {
		// Do nothing.
		// The servo must be qualified when passed to shivas.
	}
	if c.ServoSerial != "" {
		out["servo-serial"] = []string{c.ServoSerial}
	}
	if c.ServoSetupType != "" {
		out["servo-setup"] = []string{c.ServoSetupType}
	}
	if c.ServoDockerContainerName != "" {
		out["servod-docker"] = []string{c.ServoDockerContainerName}
	}
	if len(c.Pools) != 0 {
		out["pools"] = []string{strings.Join(c.Pools, ",")}
	}
	if len(c.LicenseTypes) != 0 {
		out["licensetype"] = []string{strings.Join(c.LicenseTypes, ",")}
	}
	if c.Rpm != "" {
		out["rpm"] = []string{c.Rpm}
	}
	if c.RpmOutlet != "" {
		out["rpm-outlet"] = []string{c.RpmOutlet}
	}
	if len(c.DeployTags) != 0 {
		out["deploy-tags"] = []string{strings.Join(c.DeployTags, ",")}
	}
	if len(c.Tags) != 0 {
		out["tags"] = []string{strings.Join(c.Tags, ",")}
	}
	if c.Description != "" {
		out["desc"] = []string{c.Description}
	}
	if len(c.Chameleons) != 0 {
		out["chameleons"] = []string{strings.Join(c.Chameleons, ",")}
	}
	if len(c.Cameras) != 0 {
		out["cameras"] = []string{strings.Join(c.Cameras, ",")}
	}
	if len(c.Cables) != 0 {
		out["cables"] = []string{strings.Join(c.Cables, ",")}
	}
	if c.AntennaConnection != "" {
		out["antennaconnection"] = []string{c.AntennaConnection}
	}
	if c.Router != "" {
		out["router"] = []string{c.Router}
	}
	if c.Facing != "" {
		out["facing"] = []string{c.Facing}
	}
	if c.Light != "" {
		out["light"] = []string{c.Light}
	}
	if c.Carrier != "" {
		out["carrier"] = []string{c.Carrier}
	}
	if c.AudioBoard {
		out["audioboard"] = []string{}
	}
	if c.AudioBox {
		out["audiobox"] = []string{}
	}
	if c.Atrus {
		out["atrus"] = []string{}
	}
	if c.WifiCell {
		out["wificell"] = []string{}
	}
	if c.TouchMimo {
		out["touchmimo"] = []string{}
	}
	if c.CameraBox {
		out["camerabox"] = []string{}
	}
	if c.Chaos {
		out["chaos"] = []string{}
	}
	if c.AudioCable {
		out["audiocable"] = []string{}
	}
	if c.SmartUSBHub {
		out["smartusbhub"] = []string{}
	}
	if c.Namespace != "" {
		out["namespace"] = []string{c.Namespace}
	} else {
		flags := site.EnvFlags{}
		out["namespace"] = []string{flags.GetNamespace()}
	}

	return out
}

func (c *UpdateDUT) TriggerRun(ctx context.Context, executor executor.IExecCommander) error {
	var err error
	if c.SatlabID == "" {
		c.SatlabID, err = satlabcommands.GetDockerHostBoxIdentifier(ctx, executor)
		if err != nil {
			return err
		}
	}

	hostId, err := getDockerHostBoxIdentifier(ctx, executor, c.SatlabID)
	if err != nil {
		return err
	}

	qualifiedHostname := site.MaybePrepend(site.Satlab, hostId, c.Hostname)
	args := makeUpdateShivasFlags(c)

	return (&shivas.DUTUpdater{
		Name:     qualifiedHostname,
		Executor: executor,
	}).Update(ctx, args)
}
