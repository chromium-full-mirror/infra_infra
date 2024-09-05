// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"infra/cmd/shivas/cmdhelp"
	"infra/cmd/shivas/utils"
	"infra/cros/satlab/common/site"
)

// Register flags inherited from shivas in place in the add DUT command.
// Keep this up to date with infra/cmd/shivas/ufs/subcmds/dut/add_dut.go
func registerAddShivasFlags(c *addDUTCmd) {
	c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)

	c.Flags.StringVar(&c.NewSpecsFile, "f", "", cmdhelp.DUTRegistrationFileText)
	c.Flags.StringVar(&c.SatlabID, "satlab-id", "", "the ID for the satlab in question")
	c.Flags.StringVar(&c.Namespace, "namespace", "", "namespace where data resides.")

	// Asset location fields
	c.Flags.StringVar(&c.Zone, "zone", site.GetUFSZone(), "Zone that the asset is in. "+cmdhelp.ZoneFilterHelpText)
	c.Flags.StringVar(&c.Rack, "rack", "", "Rack that the asset is in.")
	c.Flags.StringVar(&c.Hive, "hive", "", "Hive that the DUT belongs to. Example: satlab-abc123")

	// DUT/MachineLSE common fields
	c.Flags.StringVar(&c.Hostname, "name", "", "hostname of the DUT.")
	c.Flags.StringVar(&c.Asset, "asset", "", "asset tag of the machine.")
	c.Flags.StringVar(&c.Servo, "servo", "", "servo hostname and port as hostname:port. (port is assigned by UFS if missing)")
	c.Flags.StringVar(&c.ServoSerial, "servo-serial", "", "serial number for the servo. Can skip for Servo V3.")
	c.Flags.StringVar(&c.ServoSetupType, "servo-setup", "", "servo setup type. Allowed values are "+cmdhelp.ServoSetupTypeAllowedValuesString()+", UFS assigns REGULAR if unassigned.")
	c.Flags.StringVar(&c.ServoDockerContainerName, "servod-docker", "", "servod docker container name. Required if serovd is running on docker")
	c.Flags.StringVar(&c.SubrailConfig, "subrail-config", "", "power subrail config")
	c.Flags.Var(utils.CSVString(&c.Pools), "pools", "comma separated pools assigned to the DUT. 'satlab-<identifier>' is used if nothing is specified")
	c.Flags.Var(utils.CSVString(&c.LicenseTypes), "licensetype", cmdhelp.LicenseTypeHelpText)
	c.Flags.Var(utils.CSVString(&c.LicenseIds), "licenseid", "the name of the license type. Can specify multiple comma separated values.")
	c.Flags.StringVar(&c.Rpm, "rpm", "", "rpm assigned to the DUT.")
	c.Flags.StringVar(&c.RpmOutlet, "rpm-outlet", "", "rpm outlet used for the DUT.")
	c.Flags.BoolVar(&c.IgnoreUFS, "ignore-ufs", false, "skip updating UFS create a deploy task.")
	c.Flags.Var(utils.CSVString(&c.DeployTags), "deploy-tags", "comma separated tags for deployment task.")
	c.Flags.StringVar(&c.DeploymentTicket, "ticket", "", "the deployment ticket for this machine.")
	c.Flags.Var(utils.CSVString(&c.Tags), "tags", "comma separated tags.")
	c.Flags.StringVar(&c.State, "state", "", cmdhelp.StateHelp)
	c.Flags.StringVar(&c.Description, "desc", "", "description for the machine.")

	// ACS DUT fields
	c.Flags.Var(utils.CSVString(&c.Chameleons), "chameleons", cmdhelp.ChameleonTypeHelpText)
	c.Flags.Var(utils.CSVString(&c.Cameras), "cameras", cmdhelp.CameraTypeHelpText)
	c.Flags.Var(utils.CSVString(&c.Cables), "cables", cmdhelp.CableTypeHelpText)
	c.Flags.StringVar(&c.AntennaConnection, "antennaconnection", "", cmdhelp.AntennaConnectionHelpText)
	c.Flags.StringVar(&c.Router, "router", "", cmdhelp.RouterHelpText)
	c.Flags.StringVar(&c.Facing, "facing", "", cmdhelp.FacingHelpText)
	c.Flags.StringVar(&c.Light, "light", "", cmdhelp.LightHelpText)
	c.Flags.StringVar(&c.Carrier, "carrier", "", "name of the carrier.")
	c.Flags.BoolVar(&c.AudioBoard, "audioboard", false, "adding this flag will specify if audioboard is present")
	c.Flags.BoolVar(&c.AudioBox, "audiobox", false, "adding this flag will specify if audiobox is present")
	c.Flags.BoolVar(&c.Atrus, "atrus", false, "adding this flag will specify if atrus is present")
	c.Flags.BoolVar(&c.WifiCell, "wificell", false, "adding this flag will specify if wificell is present")
	c.Flags.BoolVar(&c.TouchMimo, "touchmimo", false, "adding this flag will specify if touchmimo is present")
	c.Flags.BoolVar(&c.CameraBox, "camerabox", false, "adding this flag will specify if camerabox is present")
	c.Flags.BoolVar(&c.Chaos, "chaos", false, "adding this flag will specify if chaos is present")
	c.Flags.BoolVar(&c.AudioCable, "audiocable", false, "adding this flag will specify if audiocable is present")
	c.Flags.BoolVar(&c.SmartUSBHub, "smartusbhub", false, "adding this flag will specify if smartusbhub is present")

	// Machine fields
	// crbug.com/1188488 showed us that it might be wise to add model/board during deployment if required.
	c.Flags.StringVar(&c.Model, "model", "", "model of the DUT undergoing deployment. If not given, HaRT data is used. Fails if model is not known for the DUT")
	c.Flags.StringVar(&c.Board, "board", "", "board of the DUT undergoing deployment. If not given, HaRT data is used. Fails if board is not known for the DUT")
}
