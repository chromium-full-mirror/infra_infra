// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"strings"
	"time"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// Supported modem types.
const (
	ModemTypeUnknown = tlw.Cellular_MODEM_TYPE_UNSPECIFIED
	ModemTypeL850    = tlw.Cellular_MODEM_TYPE_FIBOCOMM_L850GL
	ModemTypeNL668   = tlw.Cellular_MODEM_TYPE_NL668
	ModemTypeFM350   = tlw.Cellular_MODEM_TYPE_FM350
	ModemTypeFM101   = tlw.Cellular_MODEM_TYPE_FM101
	ModemTypeSC7180  = tlw.Cellular_MODEM_TYPE_QUALCOMM_SC7180
	ModemTypeSC7280  = tlw.Cellular_MODEM_TYPE_QUALCOMM_SC7280
	ModemTypeEM060   = tlw.Cellular_MODEM_TYPE_EM060
	ModemTypeRW101   = tlw.Cellular_MODEM_TYPE_RW101
	ModemTypeRW135   = tlw.Cellular_MODEM_TYPE_RW135
	ModemTypeLCUK54  = tlw.Cellular_MODEM_TYPE_LCUK54
)

// DeviceInfo provides a mapping between variant and modem type.
type DeviceInfo struct {
	ModemVariant string
	Board        string
	Modem        tlw.Cellular_ModemType
}

// Maps modem type enums in: go.chromium.org/chromiumos/config/proto/chromiumos/config/api/topology.proto
// to tlw.Cellular_ModemType.
var modemTypeMap = map[string]tlw.Cellular_ModemType{
	"0":  ModemTypeUnknown,
	"1":  ModemTypeL850,
	"2":  ModemTypeNL668,
	"3":  ModemTypeFM101,
	"4":  ModemTypeFM350,
	"5":  ModemTypeSC7180,
	"6":  ModemTypeSC7280,
	"7":  ModemTypeEM060,
	"8":  ModemTypeRW101,
	"9":  ModemTypeRW135,
	"10": ModemTypeLCUK54,
}

// This list is left as a fallback for older devices and images that do not contain
// modem-type information in their cros_config. No new models should be added here.
var (
	// KnownVariants mapping between variant and modem type.
	KnownVariants = map[string]DeviceInfo{
		"anahera_l850":       {"anahera_l850", "brya", ModemTypeL850},
		"brya_fm350":         {"brya_fm350", "brya", ModemTypeFM350},
		"brya_l850":          {"brya_l850", "brya", ModemTypeL850},
		"crota_fm101":        {"crota_fm101", "brya", ModemTypeFM101},
		"primus_l850":        {"primus_l850", "brya", ModemTypeL850},
		"redrix_fm350":       {"redrix_fm350", "brya", ModemTypeFM350},
		"redrix_l850":        {"redrix_l850", "brya", ModemTypeL850},
		"vell_fm350":         {"vell_fm350", "brya", ModemTypeFM350},
		"astronaut":          {"astronaut", "coral", ModemTypeL850},
		"krabby_fm101":       {"krabby_fm101", "corsola", ModemTypeFM101},
		"ponyta_fm101":       {"ponyta_fm101", "corsola", ModemTypeFM101},
		"rusty_fm101":        {"rusty_fm101", "corsola", ModemTypeFM101},
		"rusty_em060":        {"rusty_em060", "corsola", ModemTypeEM060},
		"steelix_fm101":      {"steelix_fm101", "corsola", ModemTypeFM101},
		"beadrix_nl668am":    {"beadrix_nl668am", "dedede", ModemTypeNL668},
		"blacktiplte":        {"blacktiplte", "coral", ModemTypeL850},
		"boten":              {"boten", "dedede", ModemTypeL850},
		"bugzzy_l850gl":      {"bugzzy_l850gl", "dedede", ModemTypeL850},
		"bugzzy_nl668am":     {"bugzzy_nl668am", "dedede", ModemTypeNL668},
		"cret":               {"cret", "dedede", ModemTypeL850},
		"drallion":           {"drallion", "drallion", ModemTypeL850},
		"drawper_l850gl":     {"drawper_l850gl", "dedede", ModemTypeL850},
		"kracko_nl668am":     {"kracko_nl668am", "dedede", ModemTypeNL668},
		"kracko_fm101_cat12": {"kracko_fm101_cat12", "dedede", ModemTypeFM101},
		"kracko_fm101_cat6":  {"kracko_fm101_cat6", "dedede", ModemTypeFM101},
		"metaknight":         {"metaknight", "dedede", ModemTypeL850},
		"sasuke":             {"sasuke", "dedede", ModemTypeL850},
		"sasuke_nl668am":     {"sasuke_nl668am", "dedede", ModemTypeNL668},
		"sasukette":          {"sasukette", "dedede", ModemTypeL850},
		"storo360_l850gl":    {"storo360_l850gl", "dedede", ModemTypeL850},
		"storo360_nl668am":   {"storo360_nl668am", "dedede", ModemTypeNL668},
		"storo_l850gl":       {"storo_l850gl", "dedede", ModemTypeL850},
		"storo_nl668am":      {"storo_nl668am", "dedede", ModemTypeNL668},
		"guybrush360_l850":   {"guybrush360_l850", "guybrush", ModemTypeL850},
		"guybrush_fm350":     {"guybrush_fm350", "guybrush", ModemTypeFM350},
		"nipperkin":          {"nipperkin", "guybrush", ModemTypeL850},
		"jinlon":             {"jinlon", "hatch", ModemTypeL850},
		"evoker_sc7280":      {"evoker_sc7280", "herobrine", ModemTypeSC7280},
		"herobrine_sc7280":   {"herobrine_sc7280", "herobrine", ModemTypeSC7280},
		"hoglin_sc7280":      {"hoglin_sc7280", "herobrine", ModemTypeSC7280},
		"piglin_sc7280":      {"piglin_sc7280", "herobrine", ModemTypeSC7280},
		"villager_sc7280":    {"villager_sc7280", "herobrine", ModemTypeSC7280},
		"zoglin_sc7280":      {"zoglin_sc7280", "herobrine", ModemTypeSC7280},
		"zombie_sc7280":      {"zombie_sc7280", "herobrine", ModemTypeSC7280},
		"gooey":              {"gooey", "keeby", ModemTypeL850},
		"nautiluslte":        {"nautiluslte", "nautilus", ModemTypeL850},
		"craask_fm101":       {"craask_fm101", "nissa", ModemTypeFM101},
		"gothrax_fm101":      {"gothrax_fm101", "nissa", ModemTypeFM101},
		"nivviks_fm101":      {"nivviks_fm101", "nissa", ModemTypeFM101},
		"pujjo_fm101":        {"pujjo_fm101", "nissa", ModemTypeFM101},
		"pujjoteen5_fm350":   {"pujjoteen5_fm350", "nissa", ModemTypeFM350},
		"quandiso_fm101":     {"quandiso_fm101", "nissa", ModemTypeFM101},
		"quandiso360_fm101":  {"quandiso360_fm101", "nissa", ModemTypeFM101},
		"uldren_fm101":       {"uldren_fm101", "nissa", ModemTypeFM101},
		"yavijo_fm101":       {"yavijo_fm101", "nissa", ModemTypeFM101},
		"yavilla_fm101":      {"yavilla_fm101", "nissa", ModemTypeFM101},
		"yavilly_fm101":      {"yavilly_fm101", "nissa", ModemTypeFM101},
		"dood":               {"dood", "octopus", ModemTypeL850},
		"droid":              {"droid", "octopus", ModemTypeL850},
		"fleex":              {"fleex", "octopus", ModemTypeL850},
		"garg":               {"garg", "octopus", ModemTypeL850},
		"rex_fm101":          {"rex_fm101", "rex", ModemTypeFM101},
		"rex_fm350":          {"rex_fm350", "rex", ModemTypeFM350},
		"arcada":             {"arcada", "sarien", ModemTypeL850},
		"sarien":             {"sarien", "sarien", ModemTypeL850},
		"starmie_fm101":      {"starmie_fm101", "staryu", ModemTypeFM101},
		"coachz":             {"coachz", "strongbad", ModemTypeSC7180},
		"quackingstick":      {"quackingstick", "strongbad", ModemTypeSC7180},
		"kingoftown":         {"kingoftown", "trogdor", ModemTypeSC7180},
		"lazor":              {"lazor", "trogdor", ModemTypeSC7180},
		"limozeen":           {"limozeen", "trogdor", ModemTypeSC7180},
		"pazquel":            {"pazquel", "trogdor", ModemTypeSC7180},
		"pazquel360":         {"pazquel360", "trogdor", ModemTypeSC7180},
		"skyrim_fm101":       {"skyrim_fm101", "skyrim", ModemTypeFM101},
		"vilboz":             {"vilboz", "zork", ModemTypeNL668},
		"vilboz360":          {"vilboz360", "zork", ModemTypeL850},
	}
)

// HasCellularVariant returns true if cellular modem is expected to exist on the DUT.
func HasCellularVariant(ctx context.Context, runner components.Runner) bool {
	const cmd = "cros_config /modem firmware-variant"
	if _, err := runner(ctx, 5*time.Second, cmd); err != nil {
		return false
	}
	return true
}

// GetModelVariant returns the model sub-variant for models that support multiple types of modems.
func GetModelVariant(ctx context.Context, runner components.Runner) string {
	const cmd = "cros_config /modem firmware-variant"
	out, err := runner(ctx, 5*time.Second, cmd)
	if err != nil {
		// If no variant is present on the DUT then return empty string.
		log.Errorf(ctx, "get model variant: failed to get variant from cros_config: %s", err.Error())
		return ""
	}
	return out
}

// GetModemTypeFromConfig tries to get the modem type from cros_config or returns unspecified if it's not populated.
func GetModemTypeFromConfig(ctx context.Context, runner components.Runner) tlw.Cellular_ModemType {
	const cmd = "cros_config /modem modem-type"
	if out, err := runner(ctx, 5*time.Second, cmd); err != nil {
		// Field is not required and may not be populated on all devices, so just return UNSPECIFIED.
		log.Errorf(ctx, "get modem type: failed to get modem type from cros_config: %s", err.Error())
		return tlw.Cellular_MODEM_TYPE_UNSPECIFIED
	} else if val, ok := modemTypeMap[strings.TrimSpace(out)]; ok {
		return val
	}
	return tlw.Cellular_MODEM_TYPE_UNSUPPORTED
}

// GetModemTypeFromVariant gets DUT's modem type from variant.
// Note: This only is only available as a fallback for devices that do not yet support GetModemTypeFromConfig.
func GetModemTypeFromVariant(variant string) tlw.Cellular_ModemType {
	device, ok := KnownVariants[variant]
	if !ok {
		return tlw.Cellular_MODEM_TYPE_UNSUPPORTED
	}
	return device.Modem
}
