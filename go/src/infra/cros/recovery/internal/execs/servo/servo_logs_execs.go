// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	base_error "errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/logger"
)

// Permissions is the default file permissions for log files.
// Currently, we allow everyone to read and write and nobody to execute.
const defaultFilePermissions fs.FileMode = 0666

var (
	logTimeRg             = regexp.MustCompile(`log.([\d-.]*).(INFO|DEBUG|WARNING)`)
	servoLogTimeLayout    = "2006-01-02--15-04-05.000"
	servodLatestDebugGlob = "/var/log/servod_%d/latest.DEBUG"
	servodStarLogTimeFile = "servo_started_log_time"
)

// extractTimeFromServoLog extracts time from servod log filename.
func extractTimeFromServoLog(filename string, log logger.Logger) (*time.Time, error) {
	if re := logTimeRg.FindStringSubmatch(filename); len(re) == 3 {
		return parseServodLogTime(re[1], log)
	}
	// Skip files like `latest.DEBUG`.
	return nil, errors.Reason("extract time from servo logs: skip as time not is part of name %q", filename).Err()
}

// parseServodLogTime parses time from string based on servod logs layout format.
func parseServodLogTime(rawTime string, log logger.Logger) (*time.Time, error) {
	t, err := time.Parse(servoLogTimeLayout, rawTime)
	if err != nil {
		return nil, errors.Annotate(err, "extract time from servo logs").Err()
	}
	return &t, nil
}

// getServodLogDir finds servod logs directory on servo-host.
func getServodLogDir(ctx context.Context, run components.Runner, servoPort int, log logger.Logger) (string, error) {
	latestDebugFile := fmt.Sprintf(servodLatestDebugGlob, servoPort)
	// Keep it as a single command as container doesn't accepting `realpath` as a command.
	// For single command we use `sh -c` to execute them.
	output, err := run(ctx, 30*time.Second, fmt.Sprintf("realpath %s", latestDebugFile))
	if err != nil {
		return "", errors.Annotate(err, "get time of latest servod log").Err()
	}
	output, err = run(ctx, 30*time.Second, "dirname", output)
	return output, errors.Annotate(err, "get time of latest servod log").Err()
}

// getLatestServodLogTime extract time of latest servod logs.
func getLatestServodLogTime(ctx context.Context, run components.Runner, servoPort int, log logger.Logger) (*time.Time, error) {
	latestDebugFile := fmt.Sprintf(servodLatestDebugGlob, servoPort)
	// Keep it as a single command as container doesn't accepting `realpath` as a command.
	// For single command we use `sh -c` to execute them.
	output, err := run(ctx, 30*time.Second, fmt.Sprintf("realpath %s", latestDebugFile))
	if err != nil {
		return nil, errors.Annotate(err, "get time of latest servod log").Err()
	}
	return extractTimeFromServoLog(output, log)
}

// getServosStartTime gets cached latest start time.
func getServosStartTime(ctx context.Context, logRoot string, servodPort int, run components.Runner, log logger.Logger) (*time.Time, error) {
	f := filepath.Join(logRoot, servodStarLogTimeFile)
	if _, err := os.Stat(f); base_error.Is(err, os.ErrNotExist) {
		// path/to/whatever does exist
		return nil, errors.Annotate(err, "collect servod logs").Err()
	}
	content, err := os.ReadFile(f)
	if err != nil {
		return nil, errors.Annotate(err, "collect servod logs").Err()
	}
	if len(content) == 0 {
		return nil, errors.Reason("collect servod logs: content is empty").Err()
	}
	return parseServodLogTime(string(content), log)
}

// regServoLogsStartPointExec cache latest servod start-time by logs.
func regServoLogsStartPointExec(ctx context.Context, info *execs.ExecInfo) error {
	sh := info.GetChromeos().GetServo()
	if sh.GetName() == "" {
		return errors.Reason("reg servo logs start point: servo is not present as part of dut info").Err()
	}
	logRoot := info.GetLogRoot()
	servod := info.NewServod()
	run := info.NewRunner(sh.GetName())
	log := info.NewLogger()
	f := filepath.Join(logRoot, servodStarLogTimeFile)
	if _, err := os.Stat(f); !base_error.Is(err, os.ErrNotExist) {
		// path/to/whatever does exist
		log.Debugf("The file %q is already exist!", f)
		return nil
	}
	t, err := getLatestServodLogTime(ctx, run, servod.Port(), log)
	if err != nil {
		return errors.Annotate(err, "reg servo logs start point").Err()
	}
	log.Debugf("Latest servod logs time: %v", t)
	os.WriteFile(f, []byte(t.Format(servoLogTimeLayout)), defaultFilePermissions)
	return nil
}

// collectServodLogsExec collects servod logs from servo-host.
func collectServodLogsExec(ctx context.Context, info *execs.ExecInfo) error {
	sh := info.GetChromeos().GetServo()
	if sh.GetName() == "" {
		return errors.Reason("collect servo logs: servo is not present as part of dut info").Err()
	}
	resource := sh.GetName()
	run := info.NewRunner(resource)
	log := info.NewLogger()
	logRoot := info.GetLogRoot()
	servod := info.NewServod()
	servoLogDir := filepath.Join(logRoot, resource)
	if err := exec.CommandContext(ctx, "mkdir", "-p", servoLogDir).Run(); err != nil {
		return errors.Annotate(err, "collect servod logs").Err()
	}
	servoLogsDir, err := getServodLogDir(ctx, run, servod.Port(), log)
	if err != nil {
		return errors.Annotate(err, "collect servod logs").Err()
	}
	log.Debugf("Servod logs dir: %q", servoLogsDir)
	output, err := run(ctx, time.Minute, "ls", servoLogsDir)
	if err != nil {
		return errors.Annotate(err, "collect servod logs").Err()
	}
	startTime, err := getServosStartTime(ctx, logRoot, servod.Port(), run, log)
	if err != nil {
		log.Debugf("Fail to get start time of servod logs: %v", err)
	} else {
		log.Debugf("Planning to collect logs since %v", startTime)
	}
	for _, lf := range strings.Split(output, "\n") {
		t, err := extractTimeFromServoLog(lf, log)
		if err != nil {
			log.Debugf("Collect servod logs: %v", err)
			continue
		}
		if startTime != nil {
			if t.Before(*startTime) {
				log.Debugf("Collect servod logs: skip as created at %v before start time", t)
				continue
			}
		}
		srcFile := filepath.Join(servoLogsDir, lf)
		log.Infof("Try to collect servod log %q!", srcFile)
		if err := info.CopyFrom(ctx, resource, srcFile, servoLogDir); err != nil {
			log.Debugf("Collect servod logs: fail to copy file %q to logs! Error: %v", srcFile, err)
		}
	}
	return nil
}

func init() {
	execs.Register("cros_register_servod_logs_start", regServoLogsStartPointExec)
	execs.Register("cros_collect_servod_logs", collectServodLogsExec)
}
