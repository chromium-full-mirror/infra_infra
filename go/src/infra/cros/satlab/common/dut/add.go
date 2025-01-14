// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dut

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"infra/cros/satlab/common/dns"
	"infra/cros/satlab/common/dut/shivas"
	"infra/cros/satlab/common/satlabcommands"
	"infra/cros/satlab/common/services/build_service"
	"infra/cros/satlab/common/site"
	"infra/cros/satlab/common/utils/executor"
	"infra/cros/satlab/common/utils/misc"
)

type AddDUTResponse struct {
	// RackMsg response message from adding a rack
	RackMsg string
	// AssetMsg response message from adding a asset
	AssetMsg string
	// DUTMsg response message from adding a DUT
	DUTMsg string
}

// AddDUT contains all the commands for "satlab add dut" inherited from shivas.
//
// Keep this up to date with infra/cmd/shivas/ufs/subcmds/dut/add_dut.go
type AddDUT struct {
	SatlabID  string
	Namespace string

	NewSpecsFile             string
	Hostname                 string
	Asset                    string
	Servo                    string
	ServoSerial              string
	ServoSetupType           string
	ServoDockerContainerName string
	SubrailConfig            string
	LicenseTypes             []string
	LicenseIds               []string
	Pools                    []string
	Rpm                      string
	RpmOutlet                string

	IgnoreUFS        bool
	DeployTags       []string
	DeploymentTicket string
	Tags             []string
	State            string
	Description      string

	// Asset location fields
	Zone string
	Rack string
	Hive string

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

	// Machine specific fields
	Model string
	Board string

	// AssetType is the type of the asset, it always has a value of "dut"
	AssetType string
	// Address is the IP address of the DUT
	Address string
	// SkipDNS controls whether to modify the `/etc/dut_hosts/hosts` file on the dns container
	SkipDNS bool

	// qualifiedHostname is the hostname with the SatlabID prepended
	qualifiedHostname string
	// qualifiedServo is the servo with the SatlabID prepended
	qualifiedServo string
	// qualifiedRack is the rack with the SatlabID prepended
	qualifiedRack string
}

func (c *AddDUT) setupHive(hostBoxIdentifier string) {
	if c.Hive == "" {
		c.Hive = fmt.Sprintf("%s-%s", site.Satlab, hostBoxIdentifier)
	}
}

func (c *AddDUT) setupServo(hostBoxIdentifier string) bool {
	if c.Servo == "" && c.ServoSerial == "" {
		c.qualifiedServo = ""
		c.ServoDockerContainerName = ""
		return false
	}

	if c.Servo == "" {
		// If no servo configuration is provided, use
		// the docker_servod configuration
		c.qualifiedServo = site.MaybePrepend(
			site.Satlab,
			hostBoxIdentifier,
			fmt.Sprintf(
				"%s-%s",
				c.Hostname,
				"docker_servod:9999",
			),
		)
		if c.ServoDockerContainerName == "" {
			c.ServoDockerContainerName = site.MaybePrepend(
				site.Satlab,
				hostBoxIdentifier,
				fmt.Sprintf("%s-%s", c.Hostname, "docker_servod"),
			)
		}
	} else {
		c.qualifiedServo = site.MaybePrepend(site.Satlab, hostBoxIdentifier, c.Servo)
	}

	return true
}

func (c *AddDUT) setupPools(hostBoxIdentifier string) {
	if len(c.Pools) == 0 {
		defaultPool := fmt.Sprintf("%s-%s", site.Satlab, hostBoxIdentifier)
		c.Pools = append(c.Pools, defaultPool)
	}
}

var defaultRack = "rack"

func (c *AddDUT) setupRack() {
	if c.Rack == "" {
		c.Rack = defaultRack
	}
}

func (c *AddDUT) setupZone() {
	if c.Zone == "" {
		c.Zone = site.GetUFSZone()
	}
}

func (c *AddDUT) setupNamespace() {
	c.Namespace = site.GetNamespace(c.Namespace)
}

func (c *AddDUT) setupSatlabID(ctx context.Context, executor executor.IExecCommander) error {
	if c.SatlabID == "" {
		id, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, executor)
		if err != nil {
			return err
		}
		c.SatlabID = id
	}
	return nil
}

func (c *AddDUT) TriggerRun(
	ctx context.Context,
	executor executor.IExecCommander,
	writer io.Writer,
) error {
	if err := validateHostname(c.Hostname); err != nil {
		return err
	}
	if err := validateBoardAndModel(c.Board, c.Model); err != nil {
		return err
	}

	// setup Satlab ID
	c.setupSatlabID(ctx, executor)

	// setup namespace
	c.setupNamespace()

	// This function has a single defer block that inspects the return value err to see if it
	// is nil. This defer block does *not* set the err back to nil if it succeeds in cleaning up
	// the dut_hosts file. Instead, it creates a multierror with whatever errors it encountered.
	//
	// If we're going to add multiple defer blocks, a different strategy is needed to make sure that
	// they compose in the correct way.
	dockerHostBoxIdentifier, err := getDockerHostBoxIdentifier(ctx, executor, c.SatlabID)
	if err != nil {
		return errors.Annotate(err, "add dut").Err()
	}

	// setup Satlab Hive
	c.setupHive(dockerHostBoxIdentifier)

	// setup pools
	c.setupPools(dockerHostBoxIdentifier)
	// setup rack
	c.setupRack()
	// setup servo
	c.setupServo(dockerHostBoxIdentifier)
	// setup zone
	c.setupZone()

	if site.IsPartner() {
		if shouldCreateStableVersion(c.Board, c.Model) {
			service, err := build_service.New(ctx)
			if err != nil {
				logging.Errorf(ctx, "failed to set up new Moblab API build connector: %s", err.Error())
				return errors.Annotate(err, "new Moblab API").Err()
			}
			recoveryVersion, err := service.FindMostStableBuildByBoardAndModel(ctx, c.Board, c.Model)
			if err != nil {
				logging.Errorf(ctx, "failed to find the most stable build: %s", err.Error())
				return errors.Annotate(err, "find most stable build").Err()
			}

			err = misc.StageAndWriteLocalStableVersion(ctx, service, recoveryVersion, site.RecoveryVersionDirectory)
			if err != nil {
				logging.Errorf(ctx, "failed to write local stable version: %s", err.Error())
				return errors.Annotate(err, "stage and write local stable version").Err()
			}
		}
	}

	c.qualifiedHostname = site.MaybePrepend(site.Satlab, dockerHostBoxIdentifier, c.Hostname)
	c.qualifiedRack = site.MaybePrepend(site.Satlab, dockerHostBoxIdentifier, c.Rack)

	// The flag indicate the DUT has been deployed before.
	// We need to rollback the DNS. Otherwise the previous DUT's IP address
	// will replace the new IP address
	var exist bool
	if !c.SkipDNS {
		content, updateErr := dns.UpdateRecord(
			ctx,
			c.qualifiedHostname,
			c.Address,
		)
		if updateErr != nil {
			logging.Errorf(ctx, "failed to update DNS: %s", err.Error())
			return errors.Annotate(updateErr, "add dut").Err()
		}

		// Write the content back if we fail at a later step for any reason.
		defer (func() {
			// Err refers to the error for the function as a whole.
			// If it's non-nil, then a later step has failed and we need
			// to clean up after ourselves.
			if content == "" {
				// If the content is empty, do nothing because we either failed to
				// copy the contents of the file, or the file was empty originally.
				//
				// In either case, restoring the old contents could potentially lose
				// information.
				//
				// Do not modify the error value.
			} else if err != nil || exist {
				dnsErr := dns.SetDNSFileContent(content)
				reloadErr := dns.ForceReloadDNSMasqProcess()
				err = errors.NewMultiError(err, dnsErr, reloadErr)
			}
		})()
	}

	_, err = (&shivas.Rack{
		Name:      c.qualifiedRack,
		Namespace: c.Namespace,
		Zone:      c.Zone,
	}).CheckAndAdd(ctx, executor, writer)

	if err != nil {
		return err
	}

	_, err = (&shivas.Asset{
		Asset:     c.Asset,
		Rack:      c.qualifiedRack,
		Zone:      c.Zone,
		Model:     c.Model,
		Board:     c.Board,
		Namespace: c.Namespace,
		Type:      c.AssetType,
	}).CheckAndAdd(ctx, executor, writer)

	if err != nil {
		return err
	}

	exist, err = (&shivas.DUT{
		Namespace:  c.Namespace,
		Zone:       c.Zone,
		Name:       c.qualifiedHostname,
		Rack:       c.qualifiedRack,
		Servo:      c.qualifiedServo,
		ShivasArgs: makeAddShivasFlags(c),
	}).CheckAndAdd(ctx, executor, writer)

	if err != nil {
		return err
	}

	return nil
}

// MakeShivasFlags takes an add DUT command and serializes its flags in such
// a way that they will parse to same values.
func makeAddShivasFlags(c *AddDUT) flagmap {
	out := make(flagmap)

	// These other flags are inherited from shivas.
	if c.NewSpecsFile != "" {
		// Do nothing.
		// This flag is intentionally unsupported.
		// We tweak the names of fields therefore we cannot deploy
		// using a spec file.
	}
	if c.Zone != "" {
		//NOTE: Do not pass the zone.
		// If you add dut with a zone field, it tries to update the asset's zone.
		// This feature was added to make it easier for the labops for machine
		// migration from on zone to another. if want to you pass zone, then also
		// pass the rack information below.
	}
	if c.Rack != "" {
		// Do nothing.
		// The rack must be qualified when passed to shivas.
	}
	if c.Hostname != "" {
		// Do nothing. The hostname must be qualified when passed to
		// shivas.
	}
	if c.Asset != "" {
		out["asset"] = []string{c.Asset}
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
	if c.SubrailConfig != "" {
		out["subrail-config"] = []string{c.SubrailConfig}
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
	if c.IgnoreUFS {
		// This flag is unsupported.
	}
	if len(c.DeployTags) != 0 {
		out["deploy-tags"] = []string{strings.Join(c.DeployTags, ",")}
	}
	if len(c.Tags) != 0 {
		out["tags"] = []string{strings.Join(c.Tags, ",")}
	}
	if c.State != "" {
		// This flag is unsupported.
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
	if c.Model != "" {
		out["model"] = []string{c.Model}
	}
	if c.Board != "" {
		out["board"] = []string{c.Board}
	}
	out["namespace"] = []string{site.GetNamespace(c.Namespace)}
	if c.Hive != "" {
		out["hive"] = []string{c.Hive}
	}
	return out
}

var hostnameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

func validateHostname(hostname string) error {
	if len(hostname) > 32 {
		return errors.New("hostname must be 32 characters or less")
	}

	if !hostnameRegex.MatchString(hostname) {
		return errors.New("hostname must only contain a-z, 0-9, and -")
	}

	return nil
}

func validateBoardAndModel(board, model string) error {
	if board == "" {
		return errors.Reason("Please provide a board").Err()
	}
	if model == "" {
		return errors.Reason("Please provide a model").Err()
	}
	return nil
}

func shouldCreateStableVersion(board, model string) bool {
	localStableVersion := fmt.Sprintf("%s%s-%s.json", site.RecoveryVersionDirectory, board, model)
	_, err := os.Stat(localStableVersion)
	return err != nil
}
