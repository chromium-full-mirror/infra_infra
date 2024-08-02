// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/config/go/test/dut"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"

	"infra/libs/fleet/boxster/swarming"
	"infra/libs/skylab/common/heuristics"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufsdevice "infra/unifiedfleet/api/v1/models/chromeos/device"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsmanufacturing "infra/unifiedfleet/api/v1/models/chromeos/manufacturing"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/config"
	"infra/unifiedfleet/app/external"
	"infra/unifiedfleet/app/model/configuration"
	ufsds "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/model/inventory"
	"infra/unifiedfleet/app/model/registration"
	"infra/unifiedfleet/app/model/state"
	"infra/unifiedfleet/app/util"
	"infra/unifiedfleet/app/util/osutil"
)

const (
	// Servo port ranges from 9980 to 9999
	// https://chromium.googlesource.com/chromiumos/third_party/hdctools/+/refs/heads/master/servo/servod.py#50
	// However, as there are devices with servo ports < 9980. Limit the validation to 9900.
	servoPortMax = 9999
	servoPortMin = 9000
)

var defaultPools = []string{"DUT_POOL_QUOTA"}

// CreateDUT creates ChromeOSMachineLSE entities for a DUT.
//
// Creates one MachineLSE for DUT and updates another MachineLSE for the
// Labstation(with new Servo info from DUT)
func CreateDUT(ctx context.Context, machinelse *ufspb.MachineLSE) (*ufspb.MachineLSE, error) {
	f := func(ctx context.Context) error {
		hc := getHostHistoryClient(machinelse)
		machinelses := []*ufspb.MachineLSE{machinelse}

		// Get machine to get zone and rack info for machinelse table indexing
		machine, err := GetMachine(ctx, machinelse.GetMachines()[0])
		if err != nil {
			return errors.Annotate(err, "Unable to get machine %s", machinelse.GetMachines()[0]).Err()
		}

		// Validate input
		if err := validateCreateMachineLSE(ctx, machinelse, nil, machine); err != nil {
			return errors.Annotate(err, "Validation error - Failed to Create ChromeOSMachineLSEDUT").Err()
		}

		// Verify RPM fields
		if rpm := machinelse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetRpm(); rpm != nil {
			rpmFieldsAllEmpty := (rpm.GetPowerunitName() == "" && rpm.GetPowerunitOutlet() == "")
			rpmFieldsAllFull := (rpm.GetPowerunitName() != "" && rpm.GetPowerunitOutlet() != "")
			if !(rpmFieldsAllEmpty || rpmFieldsAllFull) {
				return errors.Annotate(err, "Validation error - Must specify RPM host and outlet").Err()
			}
		}

		oldMachine := proto.Clone(machine).(*ufspb.Machine)
		machine.ResourceState = ufspb.State_STATE_SERVING
		setOutputField(ctx, machine, machinelse)

		// Check if the DUT has Servo information.
		// Update Labstation MachineLSE with new Servo info unless it's ServoV3. ServoV3 devices don't have labstation info.
		newServo := machinelse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo()
		if newServo != nil && newServo.GetServoHostname() != "" && !util.ServoV3HostnameRegex.MatchString(newServo.GetServoHostname()) && newServo.GetDockerContainerName() == "" {
			// Check if the Labstation MachineLSE exists in the system.
			labstationMachinelse, err := getLabstationMachineLSE(ctx, newServo.GetServoHostname())
			if err != nil {
				return errors.Annotate(err, "Validation error - Cannot get labstation").Err()
			}
			// Clone a copy for logging.
			oldLabstationMachineLseCopy := proto.Clone(labstationMachinelse).(*ufspb.MachineLSE)
			// Client to log labstation changes.
			hcLabstation := getHostHistoryClient(labstationMachinelse)
			// Check if the servo port is assigned, If missing assign a new one.
			if err := assignServoPortIfMissing(labstationMachinelse, newServo); err != nil {
				return err
			}
			// Check if the ServoHostName and ServoPort are already in use
			_, err = validateServoInfoForDUT(ctx, newServo, machinelse.GetName())
			if err != nil {
				return err
			}
			// Clean servo type and servo topology as that will be updated from SSW.
			cleanPreDeployFields(newServo)
			// Update the Labstation MachineLSE with new Servo information.
			// Append the new Servo entry to the Labstation
			if err := appendServoEntryToLabstation(ctx, newServo, labstationMachinelse); err != nil {
				return err
			}
			machinelses = append(machinelses, labstationMachinelse)
			// Log labstation changes to history client.
			hcLabstation.LogMachineLSEChanges(oldLabstationMachineLseCopy, labstationMachinelse)
			if err := hc.SaveChangeEvents(ctx); err != nil {
				return err
			}
		}

		// Validate Dolos host when necessary.
		if err := dolosHostValidation(ctx, machinelse, nil); err != nil {
			return err
		}

		// BatchUpdate both DUT (and its machine), and Labstation
		if _, err := registration.BatchUpdateMachines(ctx, []*ufspb.Machine{machine}); err != nil {
			return errors.Annotate(err, "Fail to update machine %s", machine.GetName()).Err()
		}
		hc.LogMachineChanges(oldMachine, machine)
		machinelse.ResourceState = ufspb.State_STATE_REGISTERED
		_, err = inventory.BatchUpdateMachineLSEs(ctx, machinelses)
		if err != nil {
			return errors.Annotate(err, "Failed to BatchUpdate MachineLSEs").Err()
		}
		hc.LogMachineLSEChanges(nil, machinelse)

		// Update states
		if err := hc.stUdt.addLseStateHelper(ctx, machinelse, machine); err != nil {
			return err
		}
		return hc.SaveChangeEvents(ctx)
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "Failed to create MachineLSE in datastore: %s", err)
		return nil, err
	}
	return machinelse, nil
}

// UpdateDUT updates a chrome OS DUT.
//
// updates one MachineLSE for DUT and updates Labstation MachineLSE
// (with new Servo info from DUT). If DUT is connected to the same
// Labstation but different port, The servo entry in Labstation is updated.
// If DUT is connected to a different labstation, then old servo info of DUT
// is removed from old Labstation and new servo info from the DUT is added
// to the new labstation.
func UpdateDUT(ctx context.Context, machinelse *ufspb.MachineLSE, mask *field_mask.FieldMask) (*ufspb.MachineLSE, error) {
	for i, f := range mask.GetPaths() {
		if !heuristics.LooksLikeFieldMask(f) {
			return nil, status.Errorf(codes.InvalidArgument, "Update DUT - mask field #%d %q is not valid", i, f)
		}
	}
	f := func(ctx context.Context) error {
		hc := getHostHistoryClient(machinelse)

		// Get the existing MachineLSE(DUT).
		oldMachinelse, err := inventory.GetMachineLSE(ctx, machinelse.GetName())
		if err != nil {
			return errors.Annotate(err, "Failed to get existing MachineLSE").Err()
		}
		// Validate that we are updating a DUT. Will lead to segfault later otherwise.
		if oldMachinelse.GetChromeosMachineLse() == nil || oldMachinelse.GetChromeosMachineLse().GetDeviceLse().GetDut() == nil {
			return status.Errorf(codes.Aborted, "%s is not a DUT. Cannot update", machinelse.GetName())
		}

		// Validate the input. Not passing the update mask as there is a different validation for dut.
		if err := validateUpdateMachineLSE(ctx, oldMachinelse, machinelse, nil); err != nil {
			return errors.Annotate(err, "Validation error - Failed to update ChromeOSMachineLSEDUT").Err()
		}

		var machine *ufspb.Machine

		// Validate the update mask and process it.
		if mask != nil && len(mask.Paths) > 0 {
			machine, err = GetMachine(ctx, oldMachinelse.GetMachines()[0])
			if err != nil {
				return errors.Annotate(err, "unable to get machine %s", oldMachinelse.GetMachines()[0]).Err()
			}
			if err := validateUpdateMachineLSEDUTMask(ctx, mask, oldMachinelse, machinelse, machine); err != nil {
				return err
			}
			machinelse, err = processUpdateMachineLSEUpdateMask(ctx, proto.Clone(oldMachinelse).(*ufspb.MachineLSE), machinelse, mask)
			if err != nil {
				return err
			}
		} else {
			// Full update, Machines cannot be empty.
			if len(machinelse.GetMachines()) > 0 {
				if len(oldMachinelse.GetMachines()) == 0 {
					return errors.Reason("DUT in invalid state. Delete DUT and recreate").Err()
				}
				if machine, err = GetMachine(ctx, machinelse.GetMachines()[0]); err != nil {
					return err
				}
				// Check if the machines have been changed.
				if machinelse.GetMachines()[0] != oldMachinelse.GetMachines()[0] {
					setOutputField(ctx, machine, machinelse)
				}
			} else {
				// Empty machines field, Invalid update.
				return status.Error(codes.InvalidArgument, "machines field cannot be empty/nil.")
			}
			// Copy state if its not updated.
			if machinelse.GetResourceState() == ufspb.State_STATE_UNSPECIFIED {
				machinelse.ResourceState = oldMachinelse.GetResourceState()
			}
			// Verify LogicalZone
			if err := validateMachineLSELogicalZone(machinelse, machine); err != nil {
				return err
			}
		}

		machinelses := []*ufspb.MachineLSE{machinelse}

		// Extract old and new servo.
		newServo := machinelse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo()
		oldServo := oldMachinelse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo()

		var servoUpdated bool

		// Check if servo is being updated. We can avoid updating servo otherwise.
		if mask != nil && len(mask.Paths) > 0 {
			servoUpdated = util.ContainsAnyStrings(mask.Paths,
				"dut.servo.hostname",
				"dut.servo.port",
				"dut.servo.serial",
				"dut.servo.type",
				"dut.servo.topology",
				"dut.servo.fwchannel",
				"dut.servo.dockerContainer",
			)
		} else {
			// Update servo on full update
			servoUpdated = true
		}

		// Validate Dolos host when necessary.
		if err := dolosHostValidation(ctx, machinelse, mask); err != nil {
			return err
		}

		// Common refs to avoid multiple queries
		var newLabstationMachinelse *ufspb.MachineLSE
		var hcNewLabstation *HistoryClient

		// Remove the old Servo info from labstation record. Unless it's servo V3 device or servod is running in docker instance
		if servoUpdated && oldServo != nil && oldServo.GetServoHostname() != "" && !util.ServoV3HostnameRegex.MatchString(oldServo.GetServoHostname()) && oldServo.GetDockerContainerName() == "" {
			// Remove the servo from the labstation
			oldLabstationMachinelse, err := inventory.GetMachineLSE(ctx, oldServo.GetServoHostname())
			if err != nil && !util.IsNotFoundError(err) {
				// Avoid returning error if we don't find the labstation
				return err
			}
			if oldLabstationMachinelse != nil {
				// Copy for logging history
				oldLabstationMachineLseCopy := proto.Clone(oldLabstationMachinelse).(*ufspb.MachineLSE)
				hcOldLabstation := getHostHistoryClient(oldLabstationMachinelse)

				// Remove servo from labstation
				if err := removeServoEntryFromLabstation(ctx, oldServo, oldLabstationMachineLseCopy); err != nil {
					return err
				}

				// Log servo removal
				hcOldLabstation.LogMachineLSEChanges(oldLabstationMachinelse, oldLabstationMachineLseCopy)

				// Updating on the same labstation. Don't save the lse yet
				if newServo != nil && newServo.GetServoHostname() == oldServo.GetServoHostname() {
					// If we are updating on the same labstation, avoid updating yet
					newLabstationMachinelse = oldLabstationMachineLseCopy
					hcNewLabstation = hcOldLabstation
				} else {
					// Record labstation changes
					if err := hcOldLabstation.SaveChangeEvents(ctx); err != nil {
						return err
					}
					machinelses = append(machinelses, oldLabstationMachineLseCopy)
				}
			}
		}

		// Process newServo and getNewLabstationMachinelse if available. Don't care about labstation if it's a ServoV3 device or servod is running on docker
		if servoUpdated && newServo != nil && newServo.GetServoHostname() != "" && !util.ServoV3HostnameRegex.MatchString(newServo.GetServoHostname()) && newServo.GetDockerContainerName() == "" {
			if newLabstationMachinelse == nil {
				// Check if the Labstation MachineLSE exists in the system first. No use doing anything if it doesn't exist.
				newLabstationMachinelse, err = getLabstationMachineLSE(ctx, newServo.GetServoHostname())
				if err != nil {
					return err
				}
				// For logging new Labstation changes.
				hcNewLabstation = getHostHistoryClient(newLabstationMachinelse)
			}

			// Check if a servo port is assigned. Assign one if its not.
			if err := assignServoPortIfMissing(newLabstationMachinelse, newServo); err != nil {
				return err
			}

			// Check if the ServoHostName and ServoPort are already in use.
			_, err = validateServoInfoForDUT(ctx, newServo, machinelse.GetName())
			if err != nil {
				return err
			}

			// Make a copy to log changes for the labstation.
			newLabstationMachinelseCopy := proto.Clone(newLabstationMachinelse).(*ufspb.MachineLSE)

			// Append the newServo entry of DUT to the newLabstationMachinelse.
			if err := appendServoEntryToLabstation(ctx, newServo, newLabstationMachinelse); err != nil {
				return err
			}

			hcNewLabstation.LogMachineLSEChanges(newLabstationMachinelseCopy, newLabstationMachinelse)
			if err := hcNewLabstation.SaveChangeEvents(ctx); err != nil {
				return err
			}
			machinelses = append(machinelses, newLabstationMachinelse)
		}

		// BatchUpdate both DUT and Labstation(s)
		_, err = inventory.BatchUpdateMachineLSEs(ctx, machinelses)
		if err != nil {
			logging.Errorf(ctx, "Failed to BatchUpdate ChromeOSMachineLSEDUTs %s", err)
			return err
		}
		hc.LogMachineLSEChanges(oldMachinelse, machinelse)

		// Update state changes.
		dutState := machinelse.GetResourceState()
		if err := hc.stUdt.updateStateHelper(ctx, dutState); err != nil {
			return err
		}
		return hc.SaveChangeEvents(ctx)
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "Failed to update MachineLSE DUT in datastore: %s", err)
		return nil, errors.Annotate(err, "UpdateDUT - failed transaction").Err()
	}
	return machinelse, nil
}

// dolosHostValidation validate(e.g. the given host is exists) Dolos host upon an update.
// In our lab, Dolos will share labstation with servos, so it's checking the existence of a given labstation.
func dolosHostValidation(ctx context.Context, newMachineLse *ufspb.MachineLSE, mask *field_mask.FieldMask) error {
	dolosMasks := []string{
		"dut.dolos.hostname",
		"dut.dolos.serial.cable",
		"dut.dolos.serial.usb",
		"dut.dolos.rpm.host",
		"dut.dolos.rpm.outlet",
	}
	// We do not need to perform validation when any masks exist but no Dolos specific masks found in there.
	if mask != nil && len(mask.Paths) > 0 && !util.ContainsAnyStrings(mask.Paths, dolosMasks...) {
		return nil
	}
	// We need to ensure that the host machine(labstation) of dolos exists.
	newDolos := newMachineLse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetDolos()
	if newDolos != nil && newDolos.GetHostname() != "" {
		if _, err := getLabstationMachineLSE(ctx, newDolos.GetHostname()); err != nil {
			return errors.Annotate(err, "Validation error - Cannot get Dolos host(labstation)").Err()
		}
	}
	return nil
}

// assignServoPortIfMissing assigns a servo port to the given servo
// if it's missing. Returns error if the port is out of range.
func assignServoPortIfMissing(labstation *ufspb.MachineLSE, newServo *chromeosLab.Servo) error {
	// If servo port is assigned, nothing is modified.
	if newServo.GetServoPort() != 0 {
		// If the servo is assigned in an invalid range return error
		if port := newServo.GetServoPort(); int(port) > servoPortMax || int(port) < servoPortMin {
			return errors.Reason("Servo port %v is invalid. Valid servo port range [%v, %v]", port, servoPortMax, servoPortMin).Err()
		}
		return nil
	}
	// If servo is  a servo v3 host then assign port 9999
	// TODO(anushruth): Avoid hostname regex by querying machine.
	if util.ServoV3HostnameRegex.MatchString(newServo.GetServoHostname()) {
		newServo.ServoPort = int32(servoPortMax)
		return nil
	}
	ports := make(map[int32]struct{}) // set of ports
	servos := labstation.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetServos()
	for _, servo := range servos {
		ports[servo.GetServoPort()] = struct{}{} // Assign an empty struct. Note: Empty structs don't take memory
	}
	for idx := int32(servoPortMax); idx >= int32(servoPortMin); idx-- {
		// Assign the highest port available to the servo
		if _, ok := ports[idx]; !ok {
			newServo.ServoPort = int32(idx)
			return nil
		}
	}
	return errors.Reason("Unable to assign a servo port. Check if %s has ports available", labstation.GetHostname()).Err()
}

// validateDeviceConfig checks if the corresponding device config exists in IV2
//
// Checks if the device configuration is known by querying IV2. Returns error if the device config doesn't exist.
func validateDeviceConfig(ctx context.Context, dut *ufspb.Machine) error {
	var devCfgIds []*ufsdevice.ConfigId
	devConfigID, err := extractDeviceConfigID(dut)
	if err != nil {
		return err
	}
	devCfgIds = append(devCfgIds, devConfigID)
	if fallBackDevConfigID := getFallbackDeviceConfigID(devConfigID); fallBackDevConfigID != nil {
		devCfgIds = append(devCfgIds, fallBackDevConfigID)
	}

	devCfgClient, err := GetDeviceConfigClient(ctx)
	if err != nil {
		return err
	}

	resp, err := devCfgClient.DeviceConfigsExists(ctx, devCfgIds)

	if err != nil {
		return errors.Annotate(err, "Device config validation failed").Err()
	}
	for i := range resp {
		if resp[i] {
			return nil
		}
	}
	errStr := fmt.Sprintf("No device config for platform %q, model %q, config (%+v)", devConfigID.GetPlatformId(), devConfigID.GetModelId(), devConfigID)
	return status.Error(codes.InvalidArgument, errStr)
}

// extractDeviceConfigID returns a corresponding ConfigID object from machine.
func extractDeviceConfigID(dut *ufspb.Machine) (*ufsdevice.ConfigId, error) {
	crosMachine := dut.GetChromeosMachine()
	if crosMachine == nil {
		return nil, errors.Reason("Invalid machine type. Not a chrome OS machine").Err()
	}
	// Convert the build target and model to lower case to avoid mismatch due to case.
	buildTarget := strings.ToLower(crosMachine.GetBuildTarget())
	model := strings.ToLower(crosMachine.GetModel())
	devConfigID := &ufsdevice.ConfigId{
		PlatformId: &ufsdevice.PlatformId{
			Value: buildTarget,
		},
		ModelId: &ufsdevice.ModelId{
			Value: model,
		},
	}
	sku := strings.ToLower(crosMachine.GetSku())
	if sku != "" {
		devConfigID.VariantId = &ufsdevice.VariantId{
			Value: sku,
		}
	}
	return devConfigID, nil
}

func getFallbackDeviceConfigID(oldConfigID *ufsdevice.ConfigId) *ufsdevice.ConfigId {
	if oldConfigID.GetVariantId().GetValue() != "" {
		fallbackID := proto.Clone(oldConfigID).(*ufsdevice.ConfigId)
		fallbackID.VariantId = nil
		return fallbackID
	}
	return nil
}

// cleanPreDeployFields clears servo type and topology.
func cleanPreDeployFields(servo *chromeosLab.Servo) {
	servo.ServoType = ""
	servo.ServoTopology = nil
}

// validateUpdateMachineLSEDUTMask validates the input mask for the given machineLSE.
//
// Assumes that dut and mask aren't empty. This is because this function is not called otherwise.
func validateUpdateMachineLSEDUTMask(ctx context.Context, mask *field_mask.FieldMask, oldMachinelse, machinelse *ufspb.MachineLSE, machine *ufspb.Machine) error {
	var servo *chromeosLab.Servo
	var rpm *chromeosLab.OSRPM
	var dolos *chromeosLab.Dolos

	// GetDut should return an object. Otherwise UpdateDUT isn't called
	dut := machinelse.GetChromeosMachineLse().GetDeviceLse().GetDut()
	if peripherals := dut.GetPeripherals(); peripherals != nil {
		servo = peripherals.GetServo()
		rpm = peripherals.GetRpm()
		dolos = peripherals.GetDolos()
	}

	maskSet := make(map[string]struct{}) // Set of all the masks
	for _, path := range mask.Paths {
		maskSet[path] = struct{}{}
	}
	// validate the give field mask
	for _, path := range mask.Paths {
		switch path {
		case "name":
			return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - name cannot be updated, delete and create a new machinelse instead.")
		case "update_time":
			return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - update_time cannot be updated, it is a output only field.")
		case "machines":
			if machinelse.GetMachines() == nil || len(machinelse.GetMachines()) == 0 || machinelse.GetMachines()[0] == "" {
				return status.Error(codes.InvalidArgument, "machines field cannot be empty/nil.")
			}
		case "dut.hostname":
			return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - hostname cannot be updated, delete and create a new dut.")
		case "dut.servo.hostname":
			if _, ok := maskSet["dut.servo.port"]; servo.GetServoHostname() == "" && ok && servo.GetServoPort() != int32(0) {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Cannot update servo port. Servo host is being reset.")
			}
			if _, ok := maskSet["dut.servo.serial"]; servo.GetServoHostname() == "" && ok && servo.GetServoSerial() != "" {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Cannot update servo serial. Servo host is being reset.")
			}
			if _, ok := maskSet["dut.servo.setup"]; servo.GetServoHostname() == "" && ok {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Cannot update servo setup. Servo host is being reset.")
			}
			if _, ok := maskSet["dut.servo.fwchannel"]; servo.GetServoHostname() == "" && ok {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Cannot update servo firmware channel. Servo host is being reset.")
			}
		case "dut.rpm.host":
			// Check for deletion of the host. Outlet cannot be updated if host is deleted.
			if _, ok := maskSet["dut.rpm.outlet"]; ok && rpm.GetPowerunitName() == "" && rpm.GetPowerunitOutlet() != "" {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Deleting rpm host deletes everything. Cannot update outlet.")
			}
		case "dut.rpm.outlet":
			// Check for deletion of rpm outlet. This should not be possible without deleting the host.
			if _, ok := maskSet["dut.rpm.host"]; rpm.GetPowerunitOutlet() == "" && (!ok || (ok && rpm.GetPowerunitName() != "")) {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Cannot remove rpm outlet. Please delete rpm.")
			}
		case "dut.rpm.type":
			// Check that an RPM exists for this device
			if _, ok := maskSet["dut.rpm.host"]; (ok && rpm.GetPowerunitName() == "") || (!ok && oldMachinelse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetRpm().GetPowerunitName() == "") {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Cannot update rpm type. Existing rpm not found or will be deleted.")
			}
			// Check for a non-default RPM type. For now, log a warning.
			// We don't return an error to avoid breaking any existing workflows.
			if rpm.GetPowerunitType() == chromeosLab.OSRPM_TYPE_UNKNOWN {
				logging.Infof(ctx, "Updating OSRPM Type with default/unknown value for DUT %s", dut.GetHostname())
			}
		case "logicalZone":
			if err := validateMachineLSELogicalZone(machinelse, machine); err != nil {
				return err
			}
		case "dut.dolos.hostname":
			if _, ok := maskSet["dut.dolos.serial.cable"]; dolos.GetHostname() == "" && ok && dolos.GetSerialCable() != "" {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Cannot update dolos serial cable. Dolos host is being reset.")
			}
			if _, ok := maskSet["dut.dolos.serial.usb"]; dolos.GetHostname() == "" && ok && dolos.GetSerialUsb() != "" {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Cannot update dolos serial usb. Dolos host is being reset.")
			}
			if _, ok := maskSet["dut.dolos.rpm.outlet"]; dolos.GetHostname() == "" && ok && dolos.GetRpm().GetPowerunitOutlet() != "" {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Cannot update dolos rpm outlet. Dolos host is being reset.")
			}
			if _, ok := maskSet["dut.dolos.rpm.host"]; dolos.GetHostname() == "" && ok && dolos.GetRpm().GetPowerunitName() != "" {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Cannot update dolos rpm host. Dolos host is being reset.")
			}
		case "dut.dolos.rpm.host":
			if _, ok := maskSet["dut.rpm.rpm.outlet"]; ok && dolos.GetRpm().GetPowerunitName() == "" && dolos.GetRpm().GetPowerunitOutlet() != "" {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Deleting rpm host deletes everything. Cannot update outlet.")
			}
		case "dut.dolos.rpm.outlet":
			if _, ok := maskSet["dut.dolos.rpm.host"]; dolos.GetRpm().GetPowerunitOutlet() == "" && (!ok || (ok && dolos.GetRpm().GetPowerunitName() != "")) {
				return status.Error(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - Cannot remove dolos rpm outlet. Please delete dolos rpm.")
			}

		case "deploymentTicket":
		case "tags":
		case "description":
		case "resourceState":
		case "dut.pools":
		case "dut.licenses":
		case "dut.hive":
		case "dut.subrailConfig":
		case "dut.servo.port":
		case "dut.servo.serial":
		case "dut.servo.setup":
		case "dut.servo.fwchannel":
		case "dut.servo.type":
		case "dut.servo.topology":
		case "dut.servo.dockerContainer":
		case "dut.chameleon.type":
		case "dut.chameleon.audioboard":
		case "dut.camera.type":
		case "dut.audio.box":
		case "dut.audio.atrus":
		case "dut.audio.cable":
		case "dut.cable.type":
		case "dut.wifi.antennaconn":
		case "dut.wifi.wificell":
		case "dut.wifi.router":
		case "dut.touch.mimo":
		case "dut.carrier":
		case "dut.starfishSlotMapping":
		case "dut.chaos":
		case "dut.camerabox":
		case "dut.camerabox.facing":
		case "dut.camerabox.light":
		case "dut.usb.smarthub":
		case "dut.modeminfo":
		case "dut.siminfo":
		case "dut.dolos.serial.cable":
		case "dut.dolos.serial.usb":
			// valid fields, nothing to validate.
		default:
			return status.Errorf(codes.InvalidArgument, "validateUpdateMachineLSEDUTUpdateMask - unsupported update mask path %q", path)
		}
	}
	return nil
}

// processUpdateMachineLSEUpdateMask process the update mask and returns the machine lse with updated parameters.
func processUpdateMachineLSEUpdateMask(ctx context.Context, oldMachineLse, newMachineLse *ufspb.MachineLSE, mask *field_mask.FieldMask) (*ufspb.MachineLSE, error) {
	// Extract all the peripherals to avoid doing it for every update in loop.
	var oldServo, newServo *chromeosLab.Servo
	var oldRPM, newRPM *chromeosLab.OSRPM
	var oldDolos, newDolos *chromeosLab.Dolos
	oldDut := oldMachineLse.GetChromeosMachineLse().GetDeviceLse().GetDut()
	newDut := newMachineLse.GetChromeosMachineLse().GetDeviceLse().GetDut()
	if oldDut != nil {
		if oldPeripherals := oldDut.GetPeripherals(); oldPeripherals != nil {
			// Assign empty structs to avoid panics
			oldServo = oldPeripherals.GetServo()
			if oldServo == nil {
				oldServo = &chromeosLab.Servo{}
			}
			oldRPM = oldPeripherals.GetRpm()
			if oldRPM == nil {
				oldRPM = &chromeosLab.OSRPM{}
			}
			oldDolos = oldPeripherals.GetDolos()
			if oldDolos == nil {
				oldDolos = &chromeosLab.Dolos{}
			}
			if oldDolos.GetRpm() == nil {
				oldDolos.Rpm = &chromeosLab.OSRPM{}
			}
		}
	}
	if newDut != nil {
		if newPeripherals := newDut.GetPeripherals(); newPeripherals != nil {
			// Assign empty structs to avoid panics
			newServo = newPeripherals.GetServo()
			if newServo == nil {
				newServo = &chromeosLab.Servo{}
			}
			newRPM = newPeripherals.GetRpm()
			if newRPM == nil {
				newRPM = &chromeosLab.OSRPM{}
			}
			newDolos = newPeripherals.GetDolos()
			if newDolos == nil {
				newDolos = &chromeosLab.Dolos{}
			}
		}
	}
	for _, path := range mask.Paths {
		switch path {
		case "machines":
			// Get machine to get zone and rack info for machinelse table indexing
			machine, err := GetMachine(ctx, newMachineLse.GetMachines()[0])
			if err != nil {
				return oldMachineLse, errors.Annotate(err, "Unable to get machine %s", newMachineLse.GetMachines()[0]).Err()
			}
			oldMachineLse.Machines = newMachineLse.GetMachines()
			// Check permission
			if err := util.CheckPermission(ctx, util.InventoriesUpdate, machine.GetRealm()); err != nil {
				return oldMachineLse, err
			}
			setOutputField(ctx, machine, oldMachineLse)
		case "mlseprototype":
			oldMachineLse.MachineLsePrototype = newMachineLse.GetMachineLsePrototype()
		case "resourceState":
			// Avoid setting unspecified state.
			if newMachineLse.GetResourceState() != ufspb.State_STATE_UNSPECIFIED {
				oldMachineLse.ResourceState = newMachineLse.GetResourceState()
			}
		case "tags":
			if tags := newMachineLse.GetTags(); len(tags) > 0 {
				// Regular tag updates append to the existing tags.
				oldMachineLse.Tags = mergeTags(oldMachineLse.GetTags(), newMachineLse.GetTags())
			} else {
				// Updating tags without any input clears the tags.
				oldMachineLse.Tags = nil
			}
		case "description":
			oldMachineLse.Description = newMachineLse.Description
		case "deploymentTicket":
			oldMachineLse.DeploymentTicket = newMachineLse.GetDeploymentTicket()
		case "logicalZone":
			oldMachineLse.LogicalZone = newMachineLse.GetLogicalZone()
		default:
			if strings.HasPrefix(path, "dut") {
				if strings.HasPrefix(path, "dut.servo") {
					processUpdateMachineLSEServoMask(oldServo, newServo, path)
					continue
				}
				if strings.HasPrefix(path, "dut.rpm") {
					processUpdateMachineLSERPMMask(oldRPM, newRPM, path)
					continue
				}
				if strings.HasPrefix(path, "dut.dolos") {
					processUpdateMachineLSEDolosMask(oldDolos, newDolos, path)
					continue
				}
				processUpdateMachineLSEDUTMask(oldDut, newDut, path)
			}
		}
	}
	if oldServo.GetServoHostname() != "" {
		oldDut.GetPeripherals().Servo = oldServo
	} else { // Reset servo if the servo host is reset.
		oldDut.GetPeripherals().Servo = nil
	}
	if oldRPM.GetPowerunitName() != "" {
		oldDut.GetPeripherals().Rpm = oldRPM
	} else { // Reset RPM if the rpm host is reset.
		oldDut.GetPeripherals().Rpm = nil
	}
	if oldDolos.GetHostname() != "" {
		oldDut.GetPeripherals().Dolos = oldDolos
	} else { // Reset Dolos if the dolos host is reset.
		if oldDut.GetPeripherals().GetDolos() != nil {
			oldDut.GetPeripherals().GetDolos().Rpm = nil
		}
		oldDut.GetPeripherals().Dolos = nil
	}
	if oldDut.GetPeripherals().GetDolos() != nil {
		if oldDolos.GetRpm().GetPowerunitName() != "" {
			oldDut.GetPeripherals().GetDolos().Rpm = oldDolos.GetRpm()
		} else {
			oldDut.GetPeripherals().GetDolos().Rpm = nil
		}
	}
	// return existing/old machinelse with new updated values.
	return oldMachineLse, nil
}

// processUpdateMachineLSEUDTMask returns updated dut with the new parameters from the mask.
func processUpdateMachineLSEDUTMask(oldDut, newDut *chromeosLab.DeviceUnderTest, path string) {
	switch path {
	case "dut.pools":
		// Append/Clear the pools given.
		if len(newDut.GetPools()) > 0 {
			oldDut.Pools = util.AppendUniqueStrings(oldDut.GetPools(), newDut.GetPools()...)
		} else {
			// Clear all the pools assigned if nothing is given.
			oldDut.Pools = nil
		}
	case "dut.licenses":
		// Clean up all licenses if new input licenses are empty
		if newDut.GetLicenses() == nil || len(newDut.GetLicenses()) == 0 {
			oldDut.Licenses = nil
		} else {
			oldDut.Licenses = append(oldDut.GetLicenses(), newDut.GetLicenses()...)
		}
	case "dut.hive":
		oldDut.Hive = newDut.GetHive()
	case "dut.subrailConfig":
		oldDut.SubrailConfig = newDut.GetSubrailConfig()
	case "dut.carrier":
		oldDut.GetPeripherals().Carrier = newDut.GetPeripherals().GetCarrier()
	case "dut.starfishSlotMapping":
		oldDut.GetPeripherals().StarfishSlotMapping = newDut.GetPeripherals().GetStarfishSlotMapping()
	case "dut.chaos":
		oldDut.GetPeripherals().Chaos = newDut.GetPeripherals().GetChaos()
	case "dut.usb.smarthub":
		oldDut.GetPeripherals().SmartUsbhub = newDut.GetPeripherals().GetSmartUsbhub()
	case "dut.camera.type":
		// Copy the cameras list to oldDut.
		oldDut.GetPeripherals().ConnectedCamera = newDut.GetPeripherals().GetConnectedCamera()
	case "dut.cable.type":
		// Copy the cable list to oldDut.
		oldDut.GetPeripherals().Cable = newDut.GetPeripherals().GetCable()
	case "dut.touch.mimo":
		oldDut.GetPeripherals().Touch = newDut.GetPeripherals().GetTouch()
	case "dut.camerabox":
		oldDut.GetPeripherals().Camerabox = newDut.GetPeripherals().GetCamerabox()
	case "dut.chameleon.type":
		if oldDut.GetPeripherals().GetChameleon() == nil {
			oldDut.GetPeripherals().Chameleon = &chromeosLab.Chameleon{}
		}
		if newDut.GetPeripherals().GetChameleon() != nil && len(newDut.GetPeripherals().GetChameleon().GetChameleonPeripherals()) > 0 {
			for _, ctype := range newDut.GetPeripherals().GetChameleon().GetChameleonPeripherals() {
				// Copy chameleon peripherals. Avoid copying invalid values.
				if ctype != chromeosLab.ChameleonType_CHAMELEON_TYPE_INVALID {
					oldDut.GetPeripherals().GetChameleon().ChameleonPeripherals = append(oldDut.GetPeripherals().GetChameleon().ChameleonPeripherals, ctype)
				}
			}
		} else {
			// Deleting chameleon
			oldDut.GetPeripherals().Chameleon = nil
		}
	case "dut.chameleon.audioboard":
		if oldDut.GetPeripherals().GetChameleon() == nil {
			oldDut.GetPeripherals().Chameleon = &chromeosLab.Chameleon{}
		}
		if newDut.GetPeripherals().GetChameleon() != nil {
			oldDut.GetPeripherals().GetChameleon().AudioBoard = newDut.GetPeripherals().GetChameleon().GetAudioBoard()
		} else {
			// Deleting chameleon
			oldDut.GetPeripherals().Chameleon = nil
		}
	case "dut.wifi.antennaconn":
		if oldDut.GetPeripherals().GetWifi() == nil {
			oldDut.GetPeripherals().Wifi = &chromeosLab.Wifi{}
		}
		// If newDut doesn't have wifi. Ignore the assignment
		if newDut.GetPeripherals().GetWifi() != nil {
			oldDut.GetPeripherals().GetWifi().AntennaConn = newDut.GetPeripherals().GetWifi().GetAntennaConn()
		} else {
			// Deleting wifi
			oldDut.GetPeripherals().Wifi = nil
		}
	case "dut.wifi.wificell":
		if oldDut.GetPeripherals().GetWifi() == nil {
			oldDut.GetPeripherals().Wifi = &chromeosLab.Wifi{}
		}
		// If newDut doesn't have wifi. Ignore the assignment
		if newDut.GetPeripherals().GetWifi() != nil {
			oldDut.GetPeripherals().GetWifi().Wificell = newDut.GetPeripherals().GetWifi().GetWificell()
		} else {
			// Deleting wifi
			oldDut.GetPeripherals().Wifi = nil
		}
	case "dut.wifi.router":
		if oldDut.GetPeripherals().GetWifi() == nil {
			oldDut.GetPeripherals().Wifi = &chromeosLab.Wifi{}
		}
		// If newDut doesn't have wifi. Ignore the assignment
		if newDut.GetPeripherals().GetWifi() != nil {
			oldDut.GetPeripherals().GetWifi().Router = newDut.GetPeripherals().GetWifi().GetRouter()
		} else {
			// Deleting wifi
			oldDut.GetPeripherals().Wifi = nil
		}
	case "dut.audio.box":
		if oldDut.GetPeripherals().GetAudio() == nil {
			oldDut.GetPeripherals().Audio = &chromeosLab.Audio{}
		}
		if newDut.GetPeripherals().GetAudio() != nil {
			oldDut.GetPeripherals().GetAudio().AudioBox = newDut.GetPeripherals().GetAudio().GetAudioBox()
		} else {
			// Delete audio
			oldDut.GetPeripherals().Audio = nil
		}
	case "dut.audio.atrus":
		if oldDut.GetPeripherals().GetAudio() == nil {
			oldDut.GetPeripherals().Audio = &chromeosLab.Audio{}
		}
		if newDut.GetPeripherals().GetAudio() != nil {
			oldDut.GetPeripherals().GetAudio().Atrus = newDut.GetPeripherals().GetAudio().GetAtrus()
		} else {
			// Delete audio
			oldDut.GetPeripherals().Audio = nil
		}
	case "dut.audio.cable":
		if oldDut.GetPeripherals().GetAudio() == nil {
			oldDut.GetPeripherals().Audio = &chromeosLab.Audio{}
		}
		if newDut.GetPeripherals().GetAudio() != nil {
			oldDut.GetPeripherals().GetAudio().AudioCable = newDut.GetPeripherals().GetAudio().GetAudioCable()
		} else {
			// Delete audio
			oldDut.GetPeripherals().Audio = nil
		}
	case "dut.camerabox.facing":
		if oldDut.GetPeripherals().GetCameraboxInfo() == nil {
			oldDut.GetPeripherals().CameraboxInfo = &chromeosLab.Camerabox{}
		}
		if newDut.GetPeripherals().GetCameraboxInfo() != nil {
			oldDut.GetPeripherals().CameraboxInfo.Facing = newDut.GetPeripherals().GetCameraboxInfo().GetFacing()
		} else {
			oldDut.GetPeripherals().CameraboxInfo = nil
		}
	case "dut.camerabox.light":
		if oldDut.GetPeripherals().GetCameraboxInfo() == nil {
			oldDut.GetPeripherals().CameraboxInfo = &chromeosLab.Camerabox{}
		}
		if newDut.GetPeripherals().GetCameraboxInfo() != nil {
			oldDut.GetPeripherals().CameraboxInfo.Light = newDut.GetPeripherals().GetCameraboxInfo().GetLight()
		} else {
			oldDut.GetPeripherals().CameraboxInfo = nil
		}
	case "dut.modeminfo":
		if oldDut.Modeminfo == nil {
			oldDut.Modeminfo = newDut.GetModeminfo()
		} else if newDut.GetModeminfo() != nil {
			if newDut.GetModeminfo().GetType() != chromeosLab.ModemType_MODEM_TYPE_UNSPECIFIED {
				oldDut.Modeminfo.Type = newDut.GetModeminfo().GetType()
			}
			if newDut.GetModeminfo().GetImei() != "" {
				oldDut.Modeminfo.Imei = newDut.GetModeminfo().GetImei()
			}
			if newDut.GetModeminfo().GetSupportedBands() != "" {
				oldDut.Modeminfo.SupportedBands = newDut.GetModeminfo().GetSupportedBands()
			}
			if newDut.GetModeminfo().GetSimCount() != 0 {
				oldDut.Modeminfo.SimCount = newDut.GetModeminfo().GetSimCount()
			}
		} else {
			oldDut.Modeminfo = nil
		}
	case "dut.siminfo":
		if newDut.GetSiminfo() == nil {
			oldDut.Siminfo = nil
		} else {
			oldDut.Siminfo = newDut.GetSiminfo()
		}
	}
}

// processUpdateMachineLSEServoMask returns servo with new updated params from the mask.
func processUpdateMachineLSEServoMask(oldServo, newServo *chromeosLab.Servo, path string) {
	switch path {
	case "dut.servo.hostname":
		oldServo.ServoHostname = newServo.GetServoHostname()
	case "dut.servo.port":
		oldServo.ServoPort = newServo.GetServoPort()
	case "dut.servo.serial":
		oldServo.ServoSerial = newServo.GetServoSerial()
	case "dut.servo.type":
		oldServo.ServoType = newServo.GetServoType()
	case "dut.servo.setup":
		oldServo.ServoSetup = newServo.GetServoSetup()
	case "dut.servo.topology":
		oldServo.ServoTopology = newServo.GetServoTopology()
	case "dut.servo.fwchannel":
		oldServo.ServoFwChannel = newServo.GetServoFwChannel()
	case "dut.servo.dockerContainer":
		oldServo.DockerContainerName = newServo.GetDockerContainerName()
	}
}

// processUpdateMacineLSERPMMask returns rpm with new updated params from the mask
func processUpdateMachineLSERPMMask(oldRPM, newRPM *chromeosLab.OSRPM, path string) {
	switch path {
	case "dut.rpm.host":
		oldRPM.PowerunitName = newRPM.GetPowerunitName()
	case "dut.rpm.outlet":
		oldRPM.PowerunitOutlet = newRPM.GetPowerunitOutlet()
	case "dut.rpm.type":
		oldRPM.PowerunitType = newRPM.GetPowerunitType()
	}
}

// processUpdateMachineLSEDolosMask returns dolos with new updated params from the mask.
func processUpdateMachineLSEDolosMask(oldDolos, newDolos *chromeosLab.Dolos, path string) {
	switch path {
	case "dut.dolos.hostname":
		oldDolos.Hostname = newDolos.GetHostname()
	case "dut.dolos.serial.cable":
		oldDolos.SerialCable = newDolos.GetSerialCable()
	case "dut.dolos.serial.usb":
		oldDolos.SerialUsb = newDolos.GetSerialUsb()
	case "dut.dolos.rpm.host":
		oldDolos.GetRpm().PowerunitName = newDolos.GetRpm().GetPowerunitName()
	case "dut.dolos.rpm.outlet":
		oldDolos.GetRpm().PowerunitOutlet = newDolos.GetRpm().GetPowerunitOutlet()
	}
}

// GetChromeOSDeviceData returns ChromeOSDeviceData for the given id/hostname from InvV2 and UFS.
func GetChromeOSDeviceData(ctx context.Context, id, hostname string) (*ufspb.ChromeOSDeviceData, error) {
	var lse *ufspb.MachineLSE
	var err error
	if hostname != "" {
		logging.Debugf(ctx, "getting full configs for host %s", hostname)
		lse, err = GetMachineLSE(ctx, hostname)
		if err != nil {
			return nil, err
		}
		if len(lse.GetMachines()) != 0 {
			id = lse.GetMachines()[0]
		}
	} else {
		logging.Debugf(ctx, "getting full configs for machine %s", id)
		machinelses, err := inventory.QueryMachineLSEByPropertyName(ctx, "machine_ids", id, false)
		if err != nil {
			return nil, err
		}
		if len(machinelses) == 0 {
			return nil, status.Error(codes.NotFound, fmt.Sprintf("DUT not found for asset id %s", id))
		}
		lse = machinelses[0]
	}
	dutState, err := state.GetDutState(ctx, id)
	if err != nil {
		logging.Warningf(ctx, "DutState for %s not found. Error: %s", id, err)
	}
	machine, err := GetMachine(ctx, id)
	if err != nil {
		logging.Errorf(ctx, "Machine for %s not found. Error: %s", id, err)
		return &ufspb.ChromeOSDeviceData{
			LabConfig: lse,
		}, nil
	}
	devCfgClient, err := GetDeviceConfigClient(ctx)
	if err != nil {
		logging.Errorf(ctx, "Failed to create DeviceConfigClient. Error: %s", err)
		return &ufspb.ChromeOSDeviceData{
			LabConfig: lse,
		}, nil
	}
	devConfig, err := getDeviceConfig(ctx, devCfgClient, machine)
	if err != nil {
		logging.Warningf(ctx, "DeviceConfig for %s not found. Error: %s", id, err)
	}
	updateDeviceConfigWithAssetInfo(ctx, id, devConfig)
	isStable, err := getStability(ctx, machine.GetChromeosMachine().GetModel())
	if err != nil {
		logging.Warningf(ctx, "stability cannot be set. Error: %s", err)
	}

	// Fetch hwid data at last as it may retry and finally exceed the ctx deadline, which
	// causes the following operations using ctx fails.
	hwidClient, err := GetHwidClient(ctx)
	if err != nil {
		logging.Errorf(ctx, "Failed to get HwidClient. Error: %s", err)
		return nil, err
	}

	hwid := machine.GetChromeosMachine().GetHwid()
	hwidData, err := GetHwidData(ctx, hwidClient, hwid)
	if err != nil {
		logging.Warningf(ctx, "Hwid data for %s not found. Error: %s", hwid, err)
	}

	var mfgConfig *ufsmanufacturing.ManufacturingConfig
	if hwidData != nil {
		mfgConfig, err = configuration.ParseHwidDataIntoMfgCfg(hwidData)
		if err != nil {
			logging.Warningf(ctx, "ManufacturingConfig for %s not found. Error: %s", hwid, err)
		}
	}

	var schedulableLabels map[string]*ufspb.SchedulableLabelValues
	enableUFSSchedulableLabels := config.Get(ctx).GetEnableBoxsterLabels()
	if enableUFSSchedulableLabels {
		schedulableLabels, err = getSchedulableLabels(ctx, machine, lse, dutState, hwidData)
		if err != nil {
			logging.Warningf(ctx, "SchedulableLabels not found. Error: %s", err)
		}
	}

	data := &ufspb.ChromeOSDeviceData{
		LabConfig:                         lse,
		Machine:                           machine,
		DeviceConfig:                      devConfig,
		ManufacturingConfig:               mfgConfig,
		HwidData:                          hwidData,
		DutState:                          dutState,
		SchedulableLabels:                 schedulableLabels,
		RespectAutomatedSchedulableLabels: enableUFSSchedulableLabels,
	}
	dutV1, err := osutil.AdaptToV1DutSpec(data)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Cannot AdaptToV1DutSpec %s", err)
	}
	// Set stability additionally
	dutV1.GetCommon().GetLabels().Stability = &isStable
	data.DutV1 = dutV1
	return data, nil
}

// getDeviceConfig get device config form InvV2
func getDeviceConfig(ctx context.Context, devCfgClient external.DeviceConfigClient, machine *ufspb.Machine) (*ufsdevice.Config, error) {
	devConfigID, err := extractDeviceConfigID(machine)
	if err != nil {
		return nil, err
	}
	return devCfgClient.GetDeviceConfig(ctx, devConfigID)
}

func getStability(ctx context.Context, model string) (bool, error) {
	stability, err := configuration.GetDeviceStability(ctx, model)
	if err == nil && stability != nil && stability.GetStability() == dut.DeviceStability_UNSTABLE {
		return false, nil
	}
	// Return true for any failed case to make sure no models are false negative.
	return true, err
}

func updateDeviceConfigWithAssetInfo(ctx context.Context, id string, devConfig *ufsdevice.Config) {
	// TODO(b/308477445): currently AssetInfo does not work for partners as it
	// is based off of HaRT data tied to an Asset ID not relevant for partner
	// DUTs. As a result, we want to fallback to device configs data.
	if util.GetDatastoreNamespace(ctx) == util.OSPartnerNamespace {
		return
	}
	// Device config can be nil, do a check here to avoid nil pointer panic later.
	if devConfig == nil {
		return
	}
	asset, err := GetAsset(ctx, id)
	if err != nil {
		logging.Warningf(ctx, "Asset for %s not found. Error: %s", id, err)
	}
	if asset != nil {
		var features []ufsdevice.Config_HardwareFeature
		if asset.GetInfo().GetTouchScreen() {
			features = append(features, ufsdevice.Config_HARDWARE_FEATURE_TOUCHSCREEN)
		}
		if asset.GetInfo().GetFingerprintSensor() {
			features = append(features, ufsdevice.Config_HARDWARE_FEATURE_FINGERPRINT)
		}
		for _, f := range devConfig.HardwareFeatures {
			if f == ufsdevice.Config_HARDWARE_FEATURE_TOUCHSCREEN {
				continue
			}
			if f == ufsdevice.Config_HARDWARE_FEATURE_FINGERPRINT {
				continue
			}
			features = append(features, f)
		}
		devConfig.HardwareFeatures = features
	}
}

// getSchedulableLabels gets Swarming schedulable labels based on DutAttributes.
func getSchedulableLabels(ctx context.Context, machine *ufspb.Machine, lse *ufspb.MachineLSE, state *chromeosLab.DutState, hwidData *ufspb.HwidData) (map[string]*ufspb.SchedulableLabelValues, error) {
	if crosMachine := machine.GetChromeosMachine(); crosMachine == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "machine cannot be empty.")
	}

	logging.Debugf(ctx, "getSchedulableLabels: listing all DutAttributes")
	attrs, err := configuration.ListDutAttributes(ctx, false)
	if err != nil {
		return nil, err
	}

	// Get FlatConfig id ${board}-${model}-${model:sku}
	logging.Debugf(ctx, "getSchedulableLabels: generate FlatConfig id from cros machine")
	fcId, err := configuration.GenerateFCIdFromCrosMachine(machine)
	if err != nil {
		return nil, err
	}

	// TODO (b/263153839): Pending redesign of FlatConfig conversion.
	//
	// logging.Debugf(ctx, "getSchedulableLabels: get FlatConfig by id %s", fcId)
	// fc, err := configuration.GetFlatConfig(ctx, fcId)
	// if err != nil && status.Code(err) != codes.NotFound {
	// 	return nil, err
	// }

	logging.Debugf(ctx, "getSchedulableLabels: generating Swarming dimensions")
	ufsLabels := make(swarming.Dimensions)
	for _, dutAttr := range attrs {
		labelsMap, err := Convert(ctx, dutAttr, nil, lse, state)
		if err != nil {
			logging.Warningf(ctx, "could not get label values for %s %s: %s", fcId, dutAttr.GetId().GetValue(), err)
			continue
		}
		for k, v := range labelsMap {
			ufsLabels[k] = v
		}
	}
	if err != nil {
		return nil, err
	}

	logging.Debugf(ctx, "getSchedulableLabels: generating HWID data labels for Swarming")
	hwidlabelsMap, err := ConvertHwidDataLabels(ctx, hwidData)
	if err != nil {
		return nil, err
	}
	for k, v := range hwidlabelsMap {
		ufsLabels[k] = v
	}

	logging.Debugf(ctx, "getSchedulableLabels: creating SchedulableLabels map for %s labels", len(ufsLabels))
	schedulableLabels := make(map[string]*ufspb.SchedulableLabelValues)
	for k, v := range ufsLabels {
		schedulableLabels[k] = &ufspb.SchedulableLabelValues{
			LabelValues: v,
		}
	}
	return schedulableLabels, nil
}

func getRPMNamePortForOSMachineLSE(lse *ufspb.MachineLSE) (string, string) {
	if lse.GetChromeosMachineLse() != nil {
		if lse.GetChromeosMachineLse().GetDeviceLse().GetDut() != nil {
			return lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetRpm().GetPowerunitName(),
				lse.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetRpm().GetPowerunitOutlet()
		} else if lse.GetChromeosMachineLse().GetDeviceLse().GetLabstation() != nil {
			return lse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetRpm().GetPowerunitName(),
				lse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetRpm().GetPowerunitOutlet()
		}
	}
	return "", ""
}

// renameDUT deletes the dut with oldName and creates one with newName. Use inside a transaction.
func renameDUT(ctx context.Context, oldName, newName string, lse *ufspb.MachineLSE, machine *ufspb.Machine) (*ufspb.MachineLSE, error) {
	// Check if we can rename the host
	if err := validateRenameDUT(ctx, oldName, newName, lse); err != nil {
		return nil, err
	}
	hc := getHostHistoryClient(lse)
	newLse := proto.Clone(lse).(*ufspb.MachineLSE)
	// Delete the old host record
	if err := inventory.DeleteMachineLSE(ctx, oldName); err != nil {
		return nil, errors.Annotate(err, "Failed to remove machineLSE").Err()
	}
	// Delete old state record for host. Avoid deleting machine state.
	if err := hc.stUdt.deleteLseStateHelper(ctx, lse, nil); err != nil {
		return nil, errors.Annotate(err, "Fail to delete lse-related states").Err()
	}
	if err := hc.SaveChangeEvents(ctx); err != nil {
		return nil, errors.Annotate(err, "Failed to log changes").Err()
	}
	// Update the host name
	newLse.Name = newName
	newLse.Hostname = newName
	newLse.GetChromeosMachineLse().GetDeviceLse().GetDut().Hostname = newName

	if _, err := inventory.BatchUpdateMachineLSEs(ctx, []*ufspb.MachineLSE{newLse}); err != nil {
		return nil, errors.Annotate(err, "Failed to add MachineLSE").Err()
	}
	// Update states
	if err := hc.stUdt.addLseStateHelper(ctx, newLse, machine); err != nil {
		return nil, err
	}

	hc.LogMachineLSEChanges(lse, newLse)
	if err := hc.SaveChangeEvents(ctx); err != nil {
		return nil, errors.Annotate(err, "Failed to save history").Err()
	}
	return newLse, nil
}

func validateRenameDUT(ctx context.Context, oldName, newName string, lse *ufspb.MachineLSE) error {
	// Check if it's part of scheduling unit. It might be possible to rename these in future.
	schedulingUnits, err := inventory.QuerySchedulingUnitByPropertyNames(ctx, map[string]string{"machinelses": oldName}, true)
	if err != nil {
		return errors.Annotate(err, "failed to query SchedulingUnit for machinelses %s", oldName).Err()
	}
	if len(schedulingUnits) > 0 {
		return status.Errorf(codes.FailedPrecondition, fmt.Sprintf("DUT %s is associated with SchedulingUnit %s. It's not possible to rename this at the moment", oldName, schedulingUnits[0].GetName()))
	}
	return nil
}

// GetDUTConnectedToServo returns machineLSE of DUT connected to the servo
func GetDUTConnectedToServo(ctx context.Context, servo *chromeosLab.Servo) (*ufspb.MachineLSE, error) {
	servoID := ufsds.GetServoID(servo.GetServoHostname(), servo.GetServoPort())
	dut, err := inventory.QueryMachineLSEByPropertyName(ctx, "servo_id", servoID, false)
	if err != nil {
		return nil, err
	}
	// Return dut if we have one
	if len(dut) == 1 {
		return dut[0], nil
	}
	if len(dut) > 1 {
		return nil, status.Errorf(codes.Internal, "Misconfigured DUTs. Muiltple DUTS(%d) found with same servo config", len(dut))
	}
	return nil, nil
}

// UpdateRecoveryData updates data from recovery
// It updates machine/asset, Peripherals(servo, wifirouter,...) and Dut's resourceState
func UpdateRecoveryData(ctx context.Context, req *ufsAPI.UpdateDeviceRecoveryDataRequest) error {
	err := validateUpdateRecoveryData(ctx, req.GetHostname())
	if err != nil {
		return errors.Annotate(err, "Validation error - Failed call UpdateRecoveryData").Err()
	}

	if err := checkDutIdAndHostnameAreAssociated(ctx, req.GetDeviceId(), req.GetHostname()); err != nil {
		logging.Errorf(ctx, "updateRecoveryData chrome device id and hostname are not associated: %s", err.Error())
	}
	switch req.GetResourceType() {
	case ufsAPI.UpdateDeviceRecoveryDataRequest_RESOURCE_TYPE_ATTACHED_DEVICE:
		if err := updateRecoveryResourceState(ctx, req.GetHostname(), req.GetResourceState()); err != nil {
			logging.Errorf(ctx, "updateRecoveryData unable to update resource state", err.Error())
			return err
		}
	case ufsAPI.UpdateDeviceRecoveryDataRequest_RESOURCE_TYPE_CHROMEOS_DEVICE:
		if err := updateRecoveryDutData(ctx, req.GetDeviceId(), req.GetChromeos().GetDutData()); err != nil {
			logging.Errorf(ctx, "updateRecoveryData unable to update dut data", err.Error())
			return err
		}
		if err := updateRecoveryLabData(ctx, req.GetHostname(), req.GetResourceState(), req.GetChromeos().GetLabData()); err != nil {
			logging.Errorf(ctx, "updateRecoveryData unable to update lab data", err.Error())
			return err
		}
		if _, err := UpdateDutState(ctx, req.GetChromeos().GetDutState()); err != nil {
			logging.Errorf(ctx, "updateRecoveryData unable to update dut state", err.Error())
			return err
		}
	default:
		return errors.Reason("updateRecoveryData: unknown resource type (%s)", req.GetResourceType()).Err()
	}
	return nil
}

func validateUpdateRecoveryData(ctx context.Context, hostname string) error {
	lse, err := inventory.GetMachineLSE(ctx, hostname)
	if err != nil {
		return fmt.Errorf("failed to fetch MachineLSE: %s", err)
	}

	if err := util.CheckPermission(ctx, util.ConfigurationsUpdate, lse.GetRealm()); err != nil {
		logging.Infof(ctx, "User %s missing permission in realm %s for UpdateRecoveryData", auth.CurrentIdentity(ctx), lse.GetRealm())
		return err
	}

	return nil
}

// UpdateTestData updates data from test data.
// It updates only different type of states.
func UpdateTestData(ctx context.Context, req *ufsAPI.UpdateTestDataRequest) error {
	err := validateUpdateTestData(ctx, req.GetHostname())
	if err != nil {
		return errors.Annotate(err, "Validation error - Failed call UpdateTestData").Err()
	}

	if err := checkDutIdAndHostnameAreAssociated(ctx, req.GetDeviceId(), req.GetHostname()); err != nil {
		logging.Errorf(ctx, "UpdateTestData chrome device id and hostname are not associated: %s", err.Error())
	}

	logging.Debugf(ctx, "UpdateTestData received masks: %v", req.GetUpdateMask().GetPaths())
	maskSet := make(map[string]bool) // Set of all the masks
	for _, path := range req.GetUpdateMask().GetPaths() {
		maskSet[path] = true
	}
	if maskSet["dut.state"] {
		if err := updateRecoveryResourceState(ctx, req.GetHostname(), req.GetResourceState()); err != nil {
			logging.Errorf(ctx, "UpdateTestData unable to update resource state", err.Error())
			return err
		}
	}
	if chromeos := req.GetChromeosData(); chromeos != nil {
		if _, err := UpdateDutStateWithMasks(ctx, maskSet, chromeos.GetDutState()); err != nil {
			logging.Errorf(ctx, "UpdateTestData unable to update dut state", err.Error())
			return err
		}
	} else if android := req.GetAndroidData(); android != nil {
		// Do nothing as no states for that case.
	} else {
		logging.Debugf(ctx, "UpdateTestData not device type specific data to update!")
	}
	return nil
}

func checkDutIdAndHostnameAreAssociated(ctx context.Context, dutId string, hostname string) error {
	lses, err := inventory.QueryMachineLSEByPropertyName(ctx, "machine_ids", dutId, true)
	if err != nil {
		return errors.Annotate(err, "failed to query host for machine id %s", dutId).Err()
	}
	if len(lses) != 1 {
		return status.Errorf(codes.FailedPrecondition, "there should be exactly 1 machinelse associated id. (%d,%s).", len(lses), dutId)
	}
	if lses[0].GetName() != hostname {
		return status.Errorf(codes.FailedPrecondition, "chromeos device id(%s) associated hostname(%s) does not match(%s).", dutId, lses[0].GetName(), hostname)
	}
	return nil
}

func validateUpdateTestData(ctx context.Context, hostname string) error {
	lse, err := inventory.GetMachineLSE(ctx, hostname)
	if err != nil {
		return fmt.Errorf("failed to fetch MachineLSE: %s", err)
	}

	if err := util.CheckPermission(ctx, util.ConfigurationsUpdate, lse.GetRealm()); err != nil {
		logging.Infof(ctx, "User %s missing permission in realm %s for UpdateTestData", auth.CurrentIdentity(ctx), lse.GetRealm())
		return err
	}

	return nil
}
