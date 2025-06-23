// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	crosLabAPI "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/dutstate"
	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/components/cros/kernel"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// StorageState is a description of the DUT's storage state given the type of the DUT storage.
type StorageState string

const (
	// DUT storage state cannot be determined.
	StorageStateUndefined StorageState = "UNDEFINED"
	// DUT storage state is normal.
	StorageStateNormal StorageState = "NORMAL"
	// DUT storage state is warning.
	StorageStateWarning StorageState = "WARNING"
	// DUT storage state is critical.
	StorageStateCritical StorageState = "CRITICAL"
)

// storageSMART is used to store the processed information of both storage type and storage state
// after it reads from the storage-info-common.sh file on the DUT.
//
// supported storageType : MMC, NVME, SSD
// supported storageState: storageStateUndefined, storageStateNomral, storageStateWarning, storageStateCritical
type storageSMART struct {
	StorageType  crosLabAPI.StorageType
	StorageState StorageState
}

// ParseSMARTInfo reads the storage info from SMART.
// The info will be located as collection of lines
func ParseSMARTInfo(ctx context.Context, rawOutput string) (*storageSMART, error) {
	storageType, storageState, err := storageSMARTFieldValue(ctx, rawOutput)
	return &storageSMART{
		StorageType:  storageType,
		StorageState: storageState,
	}, errors.WrapIf(err, "parse smart info")
}

type storageStateFunc func(context.Context, []string) (StorageState, error)

var typeToStateFuncMap map[crosLabAPI.StorageType]storageStateFunc = map[crosLabAPI.StorageType]storageStateFunc{
	crosLabAPI.StorageType_SSD:  detectSSDState,
	crosLabAPI.StorageType_MMC:  detectMMCState,
	crosLabAPI.StorageType_NVME: detectNVMEState,
	crosLabAPI.StorageType_UFS:  detectUFSState,
}

// storageSMARTFieldValue takes the raw output from the command line and return the field value of the storageSMART struct.
func storageSMARTFieldValue(ctx context.Context, rawOutput string) (crosLabAPI.StorageType, StorageState, error) {
	rawOutput = strings.TrimSpace(rawOutput)
	if rawOutput == "" {
		return crosLabAPI.StorageType_UNSPECIFIED, StorageStateUndefined, errors.Reason("storageSMART field value: storage info is empty").Err()
	}
	storageInfoSlice := strings.Split(rawOutput, "\n")
	storageType, err := extractStorageType(ctx, storageInfoSlice)
	if err != nil {
		return crosLabAPI.StorageType_UNSPECIFIED, StorageStateUndefined, errors.Annotate(err, "storageSMART field value").Err()
	}
	funcToCall, typeInMap := typeToStateFuncMap[storageType]
	if !typeInMap {
		return storageType, StorageStateUndefined, nil
	}
	storageState, err := funcToCall(ctx, storageInfoSlice)
	if err != nil {
		return storageType, StorageStateUndefined, errors.Annotate(err, "storageSMART field value").Err()
	}
	return storageType, storageState, nil
}

const (
	// Example "SATA Version is: SATA 3.1, 6.0 Gb/s (current: 6.0 Gb/s)"
	ssdTypeStorageGlob = `SATA Version is:.*`
	// Example "   Extended CSD rev 1.7 (MMC 5.0)"
	mmcTypeStorageGlob = `\s*Extended CSD rev.*MMC (?P<version>\d+.\d+)`
	// Example "SMART/Health Information (NVMe Log 0x02, NSID 0xffffffff)"
	nvmeTypeStorageGlob = `.*NVMe Log .*`
	// Example "$ ufs-utils desc -a -p /dev/bsg/ufs-bsg0"
	ufsTypeStorageGlob = `\s*ufs-utils\s*`
)

// extractStorageType extracts the storage type information from storageInfoSlice.
// return error if the regular expression cannot compile.
func extractStorageType(ctx context.Context, storageInfoSlice []string) (crosLabAPI.StorageType, error) {
	log.Debugf(ctx, "Extracting storage type")
	ssdTypeRegexp, err := regexp.Compile(ssdTypeStorageGlob)
	if err != nil {
		return crosLabAPI.StorageType_UNSPECIFIED, errors.Annotate(err, "extract storage type").Err()
	}
	mmcTypeRegexp, err := regexp.Compile(mmcTypeStorageGlob)
	if err != nil {
		return crosLabAPI.StorageType_UNSPECIFIED, errors.Annotate(err, "extract storage type").Err()
	}
	nvmeTypeRegexp, err := regexp.Compile(nvmeTypeStorageGlob)
	if err != nil {
		return crosLabAPI.StorageType_UNSPECIFIED, errors.Annotate(err, "extract storage type").Err()
	}
	ufsTypeRegexp, err := regexp.Compile(ufsTypeStorageGlob)
	if err != nil {
		return crosLabAPI.StorageType_UNSPECIFIED, errors.Annotate(err, "extract storage type").Err()
	}
	for _, line := range storageInfoSlice {
		// check if storage type is SSD
		if ssdTypeRegexp.MatchString(line) {
			return crosLabAPI.StorageType_SSD, nil
		}
		// check if storage type is UFS
		if ufsTypeRegexp.MatchString(line) {
			return crosLabAPI.StorageType_UFS, nil
		}
		// check if storage type is MMC
		mMMC, err := regexpSubmatchToMap(mmcTypeRegexp, line)
		if err == nil {
			log.Infof(ctx, "Found line => "+line)
			if version, ok := mMMC["version"]; ok {
				log.Infof(ctx, "Found eMMC device, version: %s", version)
			}
			return crosLabAPI.StorageType_MMC, nil
		}
		// check if storage type is nvme
		if nvmeTypeRegexp.MatchString(line) {
			return crosLabAPI.StorageType_NVME, nil
		}
	}
	return crosLabAPI.StorageType_UNSPECIFIED, nil
}

const (
	// Field meaning and example line that have failing attribute
	// https://en.wikipedia.org/wiki/S.M.A.R.T.
	// ID# ATTRIBUTE_NAME     FLAGS    VALUE WORST THRESH FAIL RAW_VALUE
	// 184 End-to-End_Error   PO--CK   001   001   097    NOW  135
	ssdFailGlob            = `\s*(?P<param>\S+\s\S+)\s+[P-][O-][S-][R-][C-][K-](\s+\d{3}){3}\s+NOW`
	ssdRelocateSectorsGlob = `\s*\d\sReallocated_Sector_Ct\s*[P-][O-][S-][R-][C-][K-]\s*(?P<value>\d{3})\s*(?P<worst>\d{3})\s*(?P<thresh>\d{3})`
)

// detectSSDState read the info to detect state for SSD storage.
// return error if the regular expression cannot compile.
func detectSSDState(ctx context.Context, storageInfoSlice []string) (StorageState, error) {
	log.Infof(ctx, "Extraction metrics for SSD storage")
	ssdFailRegexp, err := regexp.Compile(ssdFailGlob)
	if err != nil {
		return StorageStateUndefined, errors.Annotate(err, "detect ssd state").Err()
	}
	ssdRelocateSectorsRegexp, err := regexp.Compile(ssdRelocateSectorsGlob)
	if err != nil {
		return StorageStateUndefined, errors.Annotate(err, "detect ssd state").Err()
	}
	var usedValue float64
	var errorFound bool
	var errorFoundLine string
	for _, line := range storageInfoSlice {
		if !errorFound {
			_, err := regexpSubmatchToMap(ssdFailRegexp, line)
			if err == nil {
				errorFound = true
				errorFoundLine = line
			}
		}
		mRelocate, err := regexpSubmatchToMap(ssdRelocateSectorsRegexp, line)
		if err == nil {
			log.Debugf(ctx, "Found warning line => %q", line)
			usedValue, _ = strconv.ParseFloat(mRelocate["value"], 64)
			// manufacture set default value 100, if number started to grow then it is time to mark it.
			if usedValue > 100 {
				break
			}
		}
	}
	metrics.DefaultActionAddObservations(ctx, metrics.NewStringObservation("storage_type", "ssd"))
	// metrics.DefaultActionAddObservations(ctx, metrics.NewInt64Observation("storage_end_of_life_signal", int64(eolValue)))
	metrics.DefaultActionAddObservations(ctx, metrics.NewInt64Observation("est_storage_life_used", int64(usedValue)))
	if errorFound {
		log.Debugf(ctx, "Found critical line => %q", errorFoundLine)
		return StorageStateCritical, nil
	} else if usedValue > 100 {
		return StorageStateWarning, nil
	}
	return StorageStateNormal, nil
}

const (
	// Regex used for MMS and SSD.
	// SSD will be applied later.
	// Ex: Device life time type A [DEVICE_LIFE_TIME_EST_TYP_A: 0x01]
	// 0x00~9 means 0-90% band
	// 0x0a means 90-100% band
	// 0x0b means over 100% band
	deviceLifeTimeGlob = `.*(?P<param>DEVICE_LIFE_TIME_EST_TYP_.)]?: 0x0(?P<val>\S)` // life time persentage
	// Ex "Pre EOL information [PRE_EOL_INFO: 0x01]"
	// 0x00 - not defined
	// 0x01 - Normal
	// 0x02 - Warning, consumed 80% of the reserved blocks
	// 0x03 - Urgent, consumed 90% of the reserved blocks
	devicePreEolGlob = `.*(?P<param>PRE_EOL_INFO)]?: 0x0(?P<val>\d)`
)

const (
	// Ex:
	// Device Health Descriptor: [Byte offset 0x3]: bDeviceLifeTimeEstA = 0x1
	// 0x0~9 means 0-90% band
	// 0xa means 90-100% band
	// 0xb means over 100% band
	ufsFailLifeGlob = `.*(?P<param>bDeviceLifeTimeEst.) = 0x(?P<val>\S)`
	// Ex:
	// Device Health Descriptor: [Byte offset 0x2]: bPreEOLInfo = 0x3
	// 0x0 - not defined
	// 0x1 - Normal
	// 0x2 - Warning, consumed 80% of the reserved blocks
	// 0x3 - Urgent, consumed 90% of the reserved blocks
	ufsFailEolGlob = `.*(?P<param>bPreEOLInfo) = 0x(?P<val>\d)`
)

// detectMMCState read the info to detect state for MMC storage.
// return error if the regular expression cannot compile.
func detectMMCState(ctx context.Context, storageInfoSlice []string) (StorageState, error) {
	return detectJedecState(ctx, "MMC", deviceLifeTimeGlob, devicePreEolGlob, storageInfoSlice)
}

// detectUFSState read the info to detect state for UFS storage.
// return error if the regular expression cannot compile.
func detectUFSState(ctx context.Context, storageInfoSlice []string) (StorageState, error) {
	return detectJedecState(ctx, "UFS", ufsFailLifeGlob, ufsFailEolGlob, storageInfoSlice)
}

// detectJedecState read the info to detect state for UFS or MMC storage.
// return error if the regular expression cannot compile.
func detectJedecState(ctx context.Context, ifaceName, jedecFailLifeGlob, jedecFailEolGlob string, storageInfoSlice []string) (StorageState, error) {
	log.Infof(ctx, "Extraction metrics for "+ifaceName+" storage")
	jedecFailLiveRegexp, err := regexp.Compile(jedecFailLifeGlob)
	if err != nil {
		return StorageStateUndefined, errors.Annotate(err, "detect %s state", ifaceName).Err()
	}
	jedecFailEolRegexp, err := regexp.Compile(jedecFailEolGlob)
	if err != nil {
		return StorageStateUndefined, errors.Annotate(err, "detect %s state", ifaceName).Err()
	}
	eolValue := 0
	lifeValue := 0
	for _, line := range storageInfoSlice {
		mLife, err := regexpSubmatchToMap(jedecFailLiveRegexp, line)
		if err == nil {
			param := mLife["val"]
			log.Debugf(ctx, "Found line for lifetime estimate => %q", line)
			var val int
			if param == "a" {
				val = 100
			} else if param == "b" {
				val = 101
			} else {
				parsedVal, parseIntErr := strconv.ParseInt(param, 10, 64)
				if parseIntErr != nil {
					log.Errorf(ctx, parseIntErr.Error())
				}
				val = int(parsedVal * 10)
			}
			if val > lifeValue {
				lifeValue = val
			}
			continue
		}
		mEol, err := regexpSubmatchToMap(jedecFailEolRegexp, line)
		if err == nil {
			param := mEol["val"]
			log.Debugf(ctx, "Found line for end-of-life => %q", line)
			parsedVal, parseIntErr := strconv.ParseInt(param, 10, 64)
			if parseIntErr != nil {
				log.Errorf(ctx, parseIntErr.Error())
			}
			eolValue = int(parsedVal)
		}
	}
	log.Debugf(ctx, "lifeValue: %d, eolValue: %d", lifeValue, eolValue)
	metrics.DefaultActionAddObservations(ctx, metrics.NewStringObservation("storage_type", ifaceName))
	metrics.DefaultActionAddObservations(ctx, metrics.NewInt64Observation("storage_end_of_life_signal", int64(eolValue)))
	metrics.DefaultActionAddObservations(ctx, metrics.NewInt64Observation("est_storage_life_used", int64(lifeValue)))
	// We determine storage state Critical(require replacement) based on end-of-life signal only
	// and estimation of lifespan only served as a supplemental information.
	if eolValue == 3 {
		return StorageStateCritical, nil
	} else if eolValue == 2 || lifeValue >= 90 {
		return StorageStateWarning, nil
	} else {
		return StorageStateNormal, nil
	}
}

const (
	// Ex "Percentage Used:         100%"
	nvmeFailGlob = `Percentage Used:\s+(?P<param>(\d{1,3}))%`
)

// detectNVMEState read the info to detect state for NVMe storage.
// return error if the regular expression cannot compile
func detectNVMEState(ctx context.Context, storageInfoSlice []string) (StorageState, error) {
	log.Infof(ctx, "Extraction metrics for NVMe storage")
	nvmeFailRegexp, err := regexp.Compile(nvmeFailGlob)
	if err != nil {
		return StorageStateUndefined, errors.Annotate(err, "detect nvme state").Err()
	}
	var usedValue = -1
	for _, line := range storageInfoSlice {
		m, err := regexpSubmatchToMap(nvmeFailRegexp, line)
		if err == nil {
			log.Debugf(ctx, "Found line for usage => %q", line)
			val, convertErr := strconv.ParseInt(m["param"], 10, 64)
			if convertErr == nil {
				usedValue = int(val)
			} else {
				log.Debugf(ctx, "Could not cast: %s to int", m["param"])
			}
			break
		}
	}
	metrics.DefaultActionAddObservations(ctx, metrics.NewStringObservation("storage_type", "nvme"))
	// metrics.DefaultActionAddObservations(ctx, metrics.NewInt64Observation("storage_end_of_life_signal", int64(eolValue)))
	metrics.DefaultActionAddObservations(ctx, metrics.NewInt64Observation("est_storage_life_used", int64(usedValue)))
	if usedValue < 91 {
		log.Infof(ctx, "NVME storage usage value: %v", usedValue)
		return StorageStateNormal, nil
	}
	return StorageStateWarning, nil
}

// regexpSubmatchToMap takes pattern of regex and the source string and returns
// the map containing the groups defined in the regex expression.
// Assumes the pattern can compile.
// return error if it doesn't find any match
func regexpSubmatchToMap(r *regexp.Regexp, source string) (map[string]string, error) {
	m := make(map[string]string)
	matches := r.FindStringSubmatch(source)
	if len(matches) < 1 {
		return m, errors.Reason("regexp submatch to map: no match found").Err()
	}
	// there is at least 1 match found
	names := r.SubexpNames()
	for i := range names {
		if i != 0 {
			m[names[i]] = matches[i]
		}
	}
	return m, nil
}

const (
	readStorageInfoCMD = ". /usr/share/misc/storage-info-common.sh; get_storage_info"
)

// storageStateMap maps state from storageState type to tlw.HardwareState type
var storageStateMap = map[StorageState]tlw.HardwareState{
	StorageStateNormal:    tlw.HardwareState_HARDWARE_NORMAL,
	StorageStateWarning:   tlw.HardwareState_HARDWARE_ACCEPTABLE,
	StorageStateCritical:  tlw.HardwareState_HARDWARE_NEED_REPLACEMENT,
	StorageStateUndefined: tlw.HardwareState_HARDWARE_UNSPECIFIED,
}

// AuditStorageSMART checks the storage using it SMART capabilities,
// and mark the DUT for replacement if needed.
//
// This is a helper function to encapsulate the logic for SMART-check
// and intended to be called from the exec, as well as any other place
// where such a check is required.
func AuditStorageSMART(ctx context.Context, r components.Runner, storage *tlw.Storage, dut *tlw.Dut) error {
	if storage == nil {
		return errors.Reason("audit storage smart: data is not present in dut info").Err()
	}
	log.Debugf(ctx, "audit storage smart: initial storage state: %s, initial dut state: %s", storage.State, dut.State)
	defer func() {
		log.Debugf(ctx, "audit storage smart: final storage state: %s, final dut state: %s", storage.State, dut.State)
	}()
	rawOutput, err := r(ctx, time.Minute, readStorageInfoCMD)
	if err != nil {
		return errors.Annotate(err, "audit storage smart").Err()
	}
	ss, err := ParseSMARTInfo(ctx, rawOutput)
	if err != nil {
		return errors.Annotate(err, "audit storage smart").Err()
	}
	log.Debugf(ctx, "Detected storage type: %q", ss.StorageType)
	log.Debugf(ctx, "Detected storage state: %q", ss.StorageState)
	convertedHardwareState, ok := storageStateMap[ss.StorageState]
	if !ok {
		return errors.Reason("audit storage smart: cannot find corresponding hardware state match in the map").Err()
	}
	switch convertedHardwareState {
	case tlw.HardwareState_HARDWARE_UNSPECIFIED:
		return errors.Reason("audit storage smart: DUT storage did not detected or state cannot extracted").Err()
	case tlw.HardwareState_HARDWARE_NEED_REPLACEMENT:
		log.Debugf(ctx, "Detected issue with storage on the DUT")
		storage.State = tlw.HardwareState_HARDWARE_NEED_REPLACEMENT
		log.Debugf(ctx, "Setting the DUT state: %q", string(dutstate.NeedsReplacement))
		dut.State = dutstate.NeedsReplacement
		dut.DutStateReason = tlw.DutStateReasonInternalStorageFailureFromSMARTInfo
		return errors.Reason("audit storage smart: hardware state need replacement").Err()
	default:
		storage.State = convertedHardwareState
		log.Debugf(ctx, "New storage state: %q", convertedHardwareState)
		return nil
	}
}

// isItTimeToRunROBadblocks determines if it is time to run the RO badblock again on this device.
//
// A read-only check can be run no more than once every 6 days.
func isItTimeToRunROBadblocks(ctx context.Context, metric metrics.Metrics, hostname string) error {
	// RO badblocks should not be run if the previous run was done
	// less than 6 days ago.
	if err := isItTimeToRunBadBlocks(ctx, metric, hostname, metrics.BadBlocksROExecutionKind, 6); err != nil {
		return errors.Annotate(err, "is it time to run ro badblocks").Err()
	}
	log.Debugf(ctx, "It is okay to run a read-only check, as the last check was performed a long time ago.")
	return nil
}

// isItTimeToRunRWBadblocks determines if it is time to run the RW badblock again on this device.
//
// A read-write check can be run no more than once every 60 days.
func isItTimeToRunRWBadblocks(ctx context.Context, metric metrics.Metrics, hostname string) error {
	// RW badblocks should not be run if the previous run was done
	// less than 60 days ago.
	if err := isItTimeToRunBadBlocks(ctx, metric, hostname, metrics.BadBlocksRWExecutionKind, 60); err != nil {
		return errors.Annotate(err, "is it time to run rw badblocks").Err()
	}
	log.Debugf(ctx, "It is okay to run a read-write check, as the last check was performed a long time ago.")
	return nil
}

// isItTimeToRunBadBlocks determines whether it is time to execute the badblocks check.
func isItTimeToRunBadBlocks(ctx context.Context, metric metrics.Metrics, hostname, metricsKind string, days int) error {
	if metric == nil {
		log.Debugf(ctx, "Is It Time To Run Badblocks: metrics not provided. Allowed to run!")
		return nil
	}
	karteQuery := &metrics.Query{
		Hostname:   hostname,
		ActionKind: metricsKind,
		Limit:      1,
	}
	queryRes, err := metric.Search(ctx, karteQuery)
	if err != nil {
		return errors.Annotate(err, "is it time to run badblocks").Err()
	}
	if len(queryRes.Actions) == 0 {
		// If no records for a previous badblock execution are
		// detected, we need to allow execution.
		log.Debugf(ctx, "Is It Time To Run Badblocks: No match of the metrics kind %q found in karte.", metricsKind)
		return nil
	}
	mostRecentExecutionTime := queryRes.Actions[0].StartTime
	now := time.Now()
	nextExecutionTime := mostRecentExecutionTime.AddDate(0, 0, days)
	if now.After(nextExecutionTime) {
		return nil
	}
	return errors.Reason("is it time to run badblocks: last run was on %v and we need wait %d days before run next one", mostRecentExecutionTime, days).Err()
}

// AuditMode represents the type of audit mode.
type AuditMode string

const (
	// "auto" represents that the type of bad-blocks check will be
	// determined automatically by the task.
	auditModeAuto AuditMode = "auto"
	// "not" means that the bad-blocks check will not be run at all. This
	// option is intended to make the functionality of recovery-lib (a.k.a
	// Paris) feature-complete w.r.t. legacy repair. However, unlike
	// legacy repair, this is not exercised in Paris because we do not run
	// badblocks check from repair at all.
	auditModeNot AuditMode = "not"
	// "rw" represents the read-write mode of bad-blocks check.
	auditModeRW AuditMode = "rw"
	// "ro" represents the read-only mode of bad-blocks check.
	auditModeRO AuditMode = "ro"
)

// Commands for RO and RW badblock execution.
var badBlockCommands = map[AuditMode]string{
	auditModeRW: "badblocks -e 100 -nsv -b 4096 %s",
	auditModeRO: "badblocks -e 100 -s -b 512 %s",
}

// runBadBlocksCheck executes the badblocks check on device.
func runBadBlocksCheck(ctx context.Context, mainStorage string, bbArgs *BadBlocksArgs) (string, error) {
	if badBlockCommands[bbArgs.AuditMode] == "" {
		return "", errors.Reason("run bad blocks check: unknown badblocks mode %q", bbArgs.AuditMode).Err()
	}
	badBlocksCmd := fmt.Sprintf(badBlockCommands[bbArgs.AuditMode], mainStorage)
	// Creating a metric is sufficient to create a record about this
	// invocation of the badblocks. Further, if the badblocks
	// invocation is for RW-mode, we need to create Karte records for
	// RO as we RW kinds following the logic in legacy
	// repair. Following this, the record for RO-mode is created here,
	// and that for RW-mode will be created upon checking for RW-mode
	// below.
	bbArgs.NewMetric(metrics.BadBlocksROExecutionKind)
	timeout := bbArgs.TimeoutRO
	if bbArgs.AuditMode == auditModeRW {
		timeout = bbArgs.TimeoutRW
		// For RW-mode, we need to create Karte records for RO as well
		// as RW modes. The RO-mode record has already been created
		// above, and now we create that for RW-mode of badblocks.
		bbArgs.NewMetric(metrics.BadBlocksRWExecutionKind)
	}
	log.Debugf(ctx, "Run Bad Blocks Check: executing command %q with timeout : %d", badBlocksCmd, timeout)
	cmdResult, err := bbArgs.Run(ctx, timeout, badBlocksCmd)
	if err != nil {
		return "", errors.Annotate(err, "run bad blocks check").Err()
	}
	if cmdResult != "" {
		// Following the logic from legacy repair, a non-empty result
		// from execution of badblocks is not good.
		log.Debugf(ctx, "Run Bad Blocks Check: non-empty result of badblocks command is %q", badBlocksCmd)
		return cmdResult, errors.Reason("run bad blocks check: badblocks output is non empty: %q", cmdResult).Err()
	}
	return "", nil
}

// BadBlocksArgs collects together all the parameters that are
// applicable for bad blocks execution.
type BadBlocksArgs struct {
	AuditMode AuditMode
	Run       components.Runner
	Storage   *tlw.Storage
	Dut       *tlw.Dut

	Metrics   metrics.Metrics
	NewMetric func(kind string) *metrics.Action

	TimeoutRW time.Duration
	TimeoutRO time.Duration
}

// CheckBadblocks executes the bad-blocks check on the storage device.
//
// It will also mark the DUT for replacement if required. Any
// execution of the badblocks command is recorded in Karte using
// metric kinds backblocks_ro_execution for RO-mode and
// backblocks_rw_execution for RW-mode check. These Karte metrics are
// used to decide whether it is time to again run the badblocks
// command.
func CheckBadblocks(ctx context.Context, bbArgs *BadBlocksArgs) error {
	usbBootErr := IsBootedFromExternalStorage(ctx, bbArgs.Run)
	switch bbArgs.AuditMode {
	case auditModeNot:
		// As also mentioned in the comment that introduced this flag,
		// this option is intended to make the functionality of
		// recovery-lib (a.k.a Paris) feature-complete w.r.t. legacy
		// repair. However, unlike legacy repair, this is not
		// exercised in Paris because we do not run badblocks check
		// from repair at all.
		log.Debugf(ctx, "Check Bad Blocks: audit mode : %q: skipping badblocks.", bbArgs.AuditMode)
		return nil
	case auditModeAuto:
		// The mode "auto" means that we will determine the
		// appropriate mode for badblocks ourselves.
		log.Debugf(ctx, "Determine if it's time to run a read-only badblocks check...")
		if err := isItTimeToRunROBadblocks(ctx, bbArgs.Metrics, bbArgs.Dut.Name); err == nil {
			bbArgs.AuditMode = auditModeRO
		} else {
			log.Debugf(ctx, "Fail to assign RO mode (non-critical): %s.", err)
		}
		// Here we might end up overwriting the AuditMode after also
		// setting it to "ro" above. This is intentional. The "rw"
		// badblocks is a stronger check, and if it is the right time
		// for it, it will supercede any "ro" checks.
		log.Debugf(ctx, "Determine if it's time to run a read-write badblocks check...")
		if usbBootErr != nil {
			log.Debugf(ctx, "Device is not booted from USB-stick, run RW impossible from the same storage.")
		} else if err := isItTimeToRunRWBadblocks(ctx, bbArgs.Metrics, bbArgs.Dut.Name); err == nil {
			bbArgs.AuditMode = auditModeRW
		} else {
			log.Debugf(ctx, "Fail to assign RW mode (non-critical): %s.", err)
		}
		// Check if we finalized audit mode.
		if bbArgs.AuditMode == auditModeAuto {
			// The audit mode did not flip to either RO, or RW modes.
			return nil
		}
		log.Debugf(ctx, "Check Bad Blocks: the finalized audit mode is :%q", bbArgs.AuditMode)
	case auditModeRO:
		// If user specifies the RO mode, we will accept it as-is and
		// will execute RO-made down below.
	case auditModeRW:
		// If user specifies the RW mode for badblocks, we can execute
		// it only if the device is booted from external device.
		if usbBootErr != nil {
			log.Debugf(ctx, "Device is not booted from USB-stick, run RW impossible from the same storage.")
			return nil
		}
	default:
		return errors.Reason("check bad blocks: unknown audit mode %q", bbArgs.AuditMode).Err()
	}
	mainStorage, err := DeviceMainStoragePath(ctx, bbArgs.Run)
	if err != nil {
		return errors.Annotate(err, "check bad blocks").Err()
	} else if mainStorage == "" {
		log.Debugf(ctx, "Check Bad Blocks: path to main storage is empty, hence cannot run any type of badblocks check (non-critical).")
		// We return without error if the path of main storage device
		// is empty. This following the logic in legacy repair.
		return nil
	}
	if bbArgs.AuditMode == auditModeRW {
		if out, err := runBadBlocksCheck(ctx, mainStorage, bbArgs); err != nil {
			if components.SSHErrorInternal.In(err) {
				log.Debugf(ctx, "Check Bad Blocks: RW bad blocks check command returned a negative error code, not setting needs replacement state for storage device.")
			} else if out != "" {
				bbArgs.Storage.State = tlw.HardwareState_HARDWARE_NEED_REPLACEMENT
				bbArgs.Dut.State = dutstate.NeedsReplacement
				log.Infof(ctx, "DUT state changed to: %q", bbArgs.Dut.State.String())
				bbArgs.Dut.DutStateReason = tlw.DutStateReasonInternalStorageFailureFromBadblocksCheck
				return errors.Annotate(err, "audit storage badblocks: bad blocks detected").Err()
			}
			return errors.Annotate(err, "audit storage badblocks").Err()
		}
	} else if bbArgs.AuditMode == auditModeRO {
		if out, err := runBadBlocksCheck(ctx, mainStorage, bbArgs); err != nil {
			if components.SSHErrorInternal.In(err) {
				log.Debugf(ctx, "Check Bad Blocks: RO bad blocks check command returned a negative error code, not setting needs replacement state for storage device.")
			} else if out != "" {
				bbArgs.Storage.State = tlw.HardwareState_HARDWARE_NEED_REPLACEMENT
				bbArgs.Dut.State = dutstate.NeedsReplacement
				bbArgs.Dut.DutStateReason = tlw.DutStateReasonInternalStorageFailureFromBadblocksCheck
				return errors.Annotate(err, "audit storage badblocks: bad blocks detected").Err()
			}
			return errors.Annotate(err, "audit storage badblocks").Err()
		}
	}
	return nil
}

// CheckPartitionHash checks the storage by copying the root partition
// to an inactive partition with /bin/dd, calculating a sha256 checksum
// on the partitions, and comparing the hashes. If the test fails,
// mark the DUT for repair/replacement.
//
// This is a helper function to encapsulate the logic and intended to be called
// from the exec, as well as any other place where such a check is required.
func CheckPartitionHash(ctx context.Context, r components.Runner, storage *tlw.Storage, dut *tlw.Dut, copyTimeout time.Duration, hashTimeout time.Duration) tlw.DutStateReason {
	activeKernel, nextKernel, diskBlock, activePartition, err := kernel.GetKernelData(ctx, r)
	if err != nil {
		log.Warningf(ctx, "audit storage partition hash: %w", err)
		return tlw.DutStateReasonEmpty
	}
	diskSuffix := strings.TrimPrefix(activePartition, diskBlock)
	partitionPrefix := strings.TrimSuffix(diskSuffix, strconv.Itoa(activeKernel.RootPartition))
	inactivePartition := fmt.Sprintf("%s%s%d", diskBlock, partitionPrefix, nextKernel.RootPartition)
	_, err = r(ctx, copyTimeout, fmt.Sprintf("/bin/dd if=%s of=%s bs=4M", activePartition, inactivePartition))
	if err != nil {
		log.Warningf(ctx, "audit storage partition hash: %w", err)
		return tlw.DutStateReasonEmpty
	}
	var activeHash, inactiveHash string
	var activeErr, inactiveErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		activeHash, activeErr = getPartitionHash(ctx, r, hashTimeout, activePartition)
	}()
	go func() {
		defer wg.Done()
		inactiveHash, inactiveErr = getPartitionHash(ctx, r, hashTimeout, inactivePartition)
	}()
	wg.Wait()
	if activeErr != nil {
		log.Warningf(ctx, "audit storage partition hash: %w", err)
		return tlw.DutStateReasonEmpty
	}
	if inactiveErr != nil {
		log.Warningf(ctx, "audit storage partition hash: %w", err)
		return tlw.DutStateReasonEmpty
	}
	if activeHash != inactiveHash {
		log.Debugf(ctx, "Partition hashes do not match.")
		return tlw.DutStateReasonInternalStoragePartitionHashesMismatch
	}
	log.Debugf(ctx, "Partition hashes match. No action needed.")
	return tlw.DutStateReasonEmpty
}

func getPartitionHash(ctx context.Context, r components.Runner, timeout time.Duration, partition string) (string, error) {
	out, err := r(ctx, timeout, fmt.Sprintf("/usr/bin/sha256sum %s", partition))
	if err != nil {
		return "", errors.Annotate(err, "get partition hash").Err()
	}
	hash, _, _ := strings.Cut(out, " ")
	return hash, nil
}
