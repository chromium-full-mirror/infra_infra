// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rpm

import (
	"context"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// hasRpmInfoExec verifies if rpm info is present for DUT.
func hasRpmInfoExec(ctx context.Context, info *execs.ExecInfo) error {
	if r := info.GetChromeos().GetRpmOutlet(); r != nil {
		// TODO(otabek@): set fixed number to check and add accept argument value.
		if r.GetHostname() != "" && r.GetOutlet() != "" {
			return nil
		}
		r.State = tlw.RPMOutlet_MISSING_CONFIG
	}
	return errors.Reason("has rpm info: not present or incorrect").Err()
}

// rpmActionEnabled checks if rpm actions are in place for the rpm type
func rpmActionEnabled(ctx context.Context, info *execs.ExecInfo) error {
	if r := info.GetChromeos().GetRpmOutlet(); r != nil {
		switch r.GetType() {
		case tlw.RPMType_IP9850:
			fallthrough
		case tlw.RPMType_CPI:
			return errors.Reason("rpm action enabled: rpm actions disabled for rpm type").Err()
		case tlw.RPMType_SENTRY:
			fallthrough
		default:
			return nil
		}
	}
	return errors.Reason("rpm action enabled: rpm info missing").Err()
}

// rpmPowerCycleExec performs power cycle the device by RPM.
// This function use RPM service built-in cycle interface which has an 5 seconds interval between power state change.
func rpmPowerCycleExec(ctx context.Context, info *execs.ExecInfo) error {
	if err := info.RPMAction(ctx, info.GetDut().Name, info.GetChromeos().GetRpmOutlet(), tlw.RunRPMActionRequest_CYCLE); err != nil {
		return errors.Annotate(err, "rpm power cycle").Err()
	}
	log.Debugf(ctx, "RPM power cycle finished with success.")
	return nil
}

// rpmPowerOffExec performs power off the device by RPM.
func rpmPowerOffExec(ctx context.Context, info *execs.ExecInfo) error {
	if err := info.RPMAction(ctx, info.GetDut().Name, info.GetChromeos().GetRpmOutlet(), tlw.RunRPMActionRequest_OFF); err != nil {
		return errors.Annotate(err, "rpm power off").Err()
	}
	log.Debugf(ctx, "RPM power OFF finished with success.")
	return nil
}

// rpmPowerOffExec performs power on the device by RPM.
func rpmPowerOnExec(ctx context.Context, info *execs.ExecInfo) error {
	if err := info.RPMAction(ctx, info.GetDut().Name, info.GetChromeos().GetRpmOutlet(), tlw.RunRPMActionRequest_ON); err != nil {
		return errors.Annotate(err, "rpm power on").Err()
	}
	log.Debugf(ctx, "RPM power ON finished with success.")
	return nil
}

// hasRpmInfoDeviceExec verifies if rpm info is present for DUT.
func hasRpmInfoDeviceExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	deviceType := argsMap.AsString(ctx, "device_type", "")
	_, r, err := deviceHostnameAndRPMOutlet(info, deviceType)
	if err != nil {
		return errors.Annotate(err, "has rpm info: ").Err()
	}
	if r.GetHostname() != "" && r.GetOutlet() != "" {
		return nil
	}
	r.State = tlw.RPMOutlet_MISSING_CONFIG
	return errors.Reason("has rpm info: rpm for %s not present or incorrect", deviceType).Err()
}

// rpmPowerCycleDeviceExec performs power cycle the device by RPM.
// This function use RPM service built-in cycle interface which has an 5 seconds interval between power state change.
func rpmPowerCycleDeviceExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	deviceType := argsMap.AsString(ctx, "device_type", "")
	hostname, r, err := deviceHostnameAndRPMOutlet(info, deviceType)
	if err != nil {
		return errors.Annotate(err, "rpm power cycle:").Err()
	}
	if err := info.RPMAction(ctx, hostname, r, tlw.RunRPMActionRequest_CYCLE); err != nil {
		return errors.Annotate(err, "rpm power cycle ").Err()
	}
	log.Debugf(ctx, "RPM power cycle %s finished with success.", deviceType)
	return nil
}

// rpmPowerOffDeviceExec performs power off the device by RPM.
func rpmPowerOffDeviceExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	deviceType := argsMap.AsString(ctx, "device_type", "")
	hostname, r, err := deviceHostnameAndRPMOutlet(info, deviceType)
	if err != nil {
		return errors.Annotate(err, "rpm power off:").Err()
	}
	if err := info.RPMAction(ctx, hostname, r, tlw.RunRPMActionRequest_OFF); err != nil {
		return errors.Annotate(err, "rpm power off:").Err()
	}
	log.Debugf(ctx, "RPM power OFF %s finished with success.", deviceType)
	return nil

}

// rpmPowerOffExec performs power on the device by RPM.
func rpmPowerOnDeviceExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	deviceType := argsMap.AsString(ctx, "device_type", "")
	hostname, r, err := deviceHostnameAndRPMOutlet(info, deviceType)
	if err != nil {
		return errors.Annotate(err, "rpm power on:").Err()
	}
	if err := info.RPMAction(ctx, hostname, r, tlw.RunRPMActionRequest_ON); err != nil {
		return errors.Annotate(err, "rpm power dut on").Err()
	}
	log.Debugf(ctx, "RPM power ON %s finished with success.", deviceType)
	return nil
}

// rpmPowerOffExec performs power on the device by RPM.
func rpmSetStateExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	deviceType := argsMap.AsString(ctx, "device_type", "")
	newStateString := strings.ToUpper(argsMap.AsString(ctx, "state", ""))
	var newState tlw.RPMOutlet_State
	if s, ok := tlw.RPMOutlet_State_value[newStateString]; ok && tlw.RPMOutlet_State(s) != tlw.RPMOutlet_UNSPECIFIED {
		newState = tlw.RPMOutlet_State(s)
	} else {
		return errors.Reason("set rpm state: not provided or incorrect %q", newStateString).Err()
	}
	hostname, r, err := deviceHostnameAndRPMOutlet(info, deviceType)
	if err != nil {
		return errors.Annotate(err, "set rpm state").Err()
	}
	r.State = newState
	log.Debugf(ctx, "RPM of %q now have state %q.", hostname, r.State.String())
	return nil
}

// activeChameleon finds active chameleon related to the executed plan.
func activeChameleon(info *execs.ExecInfo) (*tlw.Chameleon, error) {
	if c := info.GetChromeos().GetChameleon(); c != nil {
		if c.GetName() == info.GetActiveResource() {
			return c, nil
		}
	}
	return nil, errors.Reason("chameleon: chameleon %q not found", info.GetActiveResource()).Err()
}

// deviceHostnameAndRPMOutlet gets device hostname and its RPMOutlet given device type.
func deviceHostnameAndRPMOutlet(info *execs.ExecInfo, deviceType string) (string, *tlw.RPMOutlet, error) {
	switch deviceType {
	case "":
		return "", nil, errors.Reason("device hostname and rpmoutlet: device type not specified").Err()
	case "dut":
		if info.GetChromeos().GetRpmOutlet() == nil {
			return "", nil, errors.Reason("device hostname and rpmoutlet for %q: not specified", deviceType).Err()
		}
		return info.GetDut().Name, info.GetChromeos().GetRpmOutlet(), nil
	case "chameleon":
		c, err := activeChameleon(info)
		if err != nil {
			return "", nil, errors.Annotate(err, "device hostname and rpmoutlet for %q:", deviceType).Err()
		}
		if c.GetRPMOutlet() == nil {
			return "", nil, errors.Reason("device hostname and rpmoutlet for %q: not specified", deviceType).Err()
		}
		return c.GetName(), c.GetRPMOutlet(), nil
	case "dolos":
		dolos := info.GetChromeos().GetDolos()
		if dolos == nil {
			return "", nil, errors.Reason("no dolos information found %q:", deviceType).Err()
		}
		if dolos.GetRpmOutlet() == nil {
			return "", nil, errors.Reason("device hostname and rpmoutlet for %q: not specified", deviceType).Err()
		}
		// Return serial cable number as it is the only unique identifier for dolos there is no hostname.
		return dolos.GetSerialCable(), dolos.GetRpmOutlet(), nil
	default:
		return "", nil, errors.Reason("device hostname and rpmoutlet: %q incorrect device type", deviceType).Err()
	}
}

func init() {
	// TODO(bniche@): retire non device execs after device execs are fully integrated.
	execs.Register("has_rpm_info", hasRpmInfoExec)
	execs.Register("rpm_action_enabled", rpmActionEnabled)
	execs.Register("rpm_power_cycle", rpmPowerCycleExec)
	execs.Register("rpm_power_off", rpmPowerOffExec)
	execs.Register("rpm_power_on", rpmPowerOnExec)

	execs.Register("device_has_rpm_info", hasRpmInfoDeviceExec)
	execs.Register("device_rpm_power_cycle", rpmPowerCycleDeviceExec)
	execs.Register("device_rpm_power_off", rpmPowerOffDeviceExec)
	execs.Register("device_rpm_power_on", rpmPowerOnDeviceExec)
	execs.Register("set_rpm_state", rpmSetStateExec)
}
