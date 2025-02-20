// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"go.chromium.org/infra/cmd/shivas/cmdhelp"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cros/satlab/common/site"
)

// RegisterUpdateShivasFlags registers the flags inherited from shivas.
func registerUpdateShivasFlags(c *updateDUTCmd) {
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)

	c.Flags.StringVar(&c.SatlabID, "satlab-id", "", "the ID for the satlab in question")
	c.Flags.StringVar(&c.Namespace, "namespace", "", "namespace where data resides.")

	c.Flags.StringVar(&c.NewSpecsFile, "f", "", cmdhelp.DUTUpdateFileText)

	c.Flags.StringVar(&c.Hostname, "name", "", "hostname of the DUT.")
	c.Flags.StringVar(&c.Machine, "asset", "", "asset tag of the DUT.")
	c.Flags.StringVar(&c.Servo, "servo", "", "servo hostname and port as hostname:port. Clearing this field will delete the servo in DUT. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.ServoSerial, "servo-serial", "", "serial number for the servo.")
	c.Flags.StringVar(&c.ServoSetupType, "servo-setup", "", "servo setup type. Allowed values are "+cmdhelp.ServoSetupTypeAllowedValuesString()+".")
	c.Flags.StringVar(&c.ServoFwChannel, "servo-fw-channel", "", "servo firmware channel. Allowed values are "+cmdhelp.ServoFwChannelAllowedValuesString()+".")
	c.Flags.StringVar(&c.ServoDockerContainerName, "servod-docker", "", "servo docker container name. Required if servod is running in docker.")
	c.Flags.Var(utils.CSVString(&c.Pools), "pools", "comma seperated pools. These will be appended to existing pools. "+cmdhelp.ClearFieldHelpText)
	c.Flags.Var(utils.CSVString(&c.LicenseTypes), "licensetype", cmdhelp.LicenseTypeHelpText)
	c.Flags.Var(utils.CSVString(&c.LicenseIds), "licenseid", "the name of the license type. Can specify multiple comma separated values. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.Rpm, "rpm", "", "rpm assigned to the DUT. Clearing this field will delete rpm. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.RpmOutlet, "rpm-outlet", "", "rpm outlet used for the DUT.")
	c.Flags.StringVar(&c.DeploymentTicket, "ticket", "", "the deployment ticket for this machine. "+cmdhelp.ClearFieldHelpText)
	c.Flags.Var(utils.CSVString(&c.Tags), "tags", "comma separated tags. You can only append new tags or delete all of them. "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.Description, "desc", "", "description for the machine. "+cmdhelp.ClearFieldHelpText)

	c.Flags.BoolVar(&c.ForceDeploy, "force-deploy", false, "forces a deploy task for all the updates.")
	c.Flags.Var(utils.CSVString(&c.DeployTags), "deploy-tags", "comma seperated tags for deployment task.")

	// ACS DUT fields
	c.Flags.Var(utils.CSVString(&c.Chameleons), "chameleons", cmdhelp.ChameleonTypeHelpText+". "+cmdhelp.ClearFieldHelpText)
	c.Flags.Var(utils.CSVString(&c.Cameras), "cameras", cmdhelp.CameraTypeHelpText+". "+cmdhelp.ClearFieldHelpText)
	c.Flags.Var(utils.CSVString(&c.Cables), "cables", cmdhelp.CableTypeHelpText+". "+cmdhelp.ClearFieldHelpText)
	c.Flags.StringVar(&c.AntennaConnection, "antennaconnection", "", cmdhelp.AntennaConnectionHelpText)
	c.Flags.StringVar(&c.Router, "router", "", cmdhelp.RouterHelpText)
	c.Flags.StringVar(&c.Facing, "facing", "", cmdhelp.FacingHelpText)
	c.Flags.StringVar(&c.Light, "light", "", cmdhelp.LightHelpText)
	c.Flags.StringVar(&c.Carrier, "carrier", "", "name of the carrier."+". "+cmdhelp.ClearFieldHelpText)
	c.Flags.BoolVar(&c.AudioBoard, "audioboard", false, "adding this flag will specify if audioboard is present")
	c.Flags.BoolVar(&c.AudioBox, "audiobox", false, "adding this flag will specify if audiobox is present")
	c.Flags.BoolVar(&c.Atrus, "atrus", false, "adding this flag will specify if atrus is present")
	c.Flags.BoolVar(&c.WifiCell, "wificell", false, "adding this flag will specify if wificell is present")
	c.Flags.BoolVar(&c.TouchMimo, "touchmimo", false, "adding this flag will specify if touchmimo is present")
	c.Flags.BoolVar(&c.CameraBox, "camerabox", false, "adding this flag will specify if camerabox is present")
	c.Flags.BoolVar(&c.Chaos, "chaos", false, "adding this flag will specify if chaos is present")
	c.Flags.BoolVar(&c.AudioCable, "audiocable", false, "adding this flag will specify if audiocable is present")
	c.Flags.BoolVar(&c.SmartUSBHub, "smartusbhub", false, "adding this flag will specify if smartusbhub is present")
}
