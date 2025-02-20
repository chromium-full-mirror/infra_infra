// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// Permissions is the default file permissions for log files.
// Currently, we allow everyone to read and write and nobody to execute.
const defaultFilePermissions fs.FileMode = 0666

// DmesgExec grabs dmesg and persists the file into the log directory.
// DmesgExec fails if and only if the dmesg executable doesn't exist or returns nonzero.
//
// This exec function accepts the following parameters from the action:
// human_readable: whether the dmesg output is expected to be in human-readable form.
// device_type: specifies device which used as source of data.
// create_crashinfo_dir: whether the subdirectory created for crashinfo needs to be created.
// use_host_dir: whether the subdirectory named like device_type name.
func dmesgExec(ctx context.Context, info *execs.ExecInfo) error {
	argMap := info.GetActionArgs(ctx)
	logRoot := info.GetLogRoot()
	resource, err := info.GetDeviceName(argMap.AsString(ctx, "device_type", "active"))
	if err != nil {
		return errors.Annotate(err, "dmesg exec").Err()
	}
	if argMap.AsBool(ctx, "create_crashinfo_dir", false) {
		logRoot = filepath.Join(logRoot, fmt.Sprintf("crashinfo.%s", resource))
	} else if argMap.AsBool(ctx, "use_host_dir", false) {
		logRoot = filepath.Join(logRoot, resource)
	} else if path := argMap.AsString(ctx, "custom_dir", ""); path != "" {
		logRoot = filepath.Join(logRoot, path)
	}
	run := info.NewRunner(resource)
	var output string
	if argMap.AsBool(ctx, "human_readable", true) {
		output, err = run(ctx, time.Minute, "dmesg", "-H")
	} else {
		output, err = run(ctx, time.Minute, "dmesg")
	}
	if err != nil {
		return errors.Annotate(err, "dmesg exec").Err()
	}
	// Output is non-empty and dmesg ran successfully.

	// Attempting to create a directory that already exists will not
	// result in an error, hence we can just create this new directory
	// without checking whether it already exists.
	if err = os.MkdirAll(logRoot, os.ModePerm); err != nil {
		return errors.Annotate(err, "dmesg exec").Err()
	}
	f := filepath.Join(logRoot, "dmesg")
	log.Debugf(ctx, "dmesg path to safe: %s", f)
	err = os.WriteFile(f, []byte(output), defaultFilePermissions)
	return errors.Annotate(err, "dmesg exec").Err()
}

// copyToLogsExec grabs a file or directory from the host and copy to
// the log directory.
//
// This exec function accepts the following parameters from the action:
// src_host_type: specifies the type of source, options are "dut" and "servo_host"
// src_path: specifies the source for copy operation.
// src_type: specifies whether the source is a file or a directory, options are "file" and "dir".
// use_host_dir: specifies whether the a subdirectory with the resource-name needs to be created.
// dest_suffix: specifies any subdirectory that needs to be created within the source path.
func copyToLogsExec(ctx context.Context, info *execs.ExecInfo) error {
	argMap := info.GetActionArgs(ctx)
	srcPath := argMap.AsString(ctx, "src_path", "")
	if srcPath == "" {
		return errors.Reason("copy to logs: src_path is empty or not provided").Err()
	}
	if newName := filepath.Base(srcPath); newName == "" || newName == "." || newName == ".." || newName == "/" {
		return errors.Reason("copy to logs: could not extracted filaname from src_path").Err()
	}
	resource, err := info.GetDeviceName(argMap.AsString(ctx, "src_host_type", ""))
	if err != nil {
		return errors.Annotate(err, "copy to logs").Err()
	}
	logRoot := info.GetLogRoot()
	// Logs will be saved to the resource folder.
	if argMap.AsBool(ctx, "use_host_dir", false) {
		logRoot = filepath.Join(logRoot, resource)
	} else if path := argMap.AsString(ctx, "custom_dir", ""); path != "" {
		logRoot = filepath.Join(logRoot, path)
	}
	// If a suffix for destination path has been specified, include it
	// in the path for storing files on destination side.
	if destSuffix := argMap.AsString(ctx, "dest_suffix", ""); destSuffix != "" {
		logRoot = filepath.Join(logRoot, destSuffix)
	}
	if err := exec.CommandContext(ctx, "mkdir", "-p", logRoot).Run(); err != nil {
		return errors.Annotate(err, "copy to logs").Err()
	}
	isDir := argMap.AsString(ctx, "src_type", "file") == "dir"
	testCmdFlag := "-f"
	if isDir {
		testCmdFlag = "-d"
	}
	run := info.NewRunner(resource)
	if _, err := run(ctx, time.Minute, "test", testCmdFlag, srcPath); err != nil {
		return errors.Annotate(err, "copy to logs: the src_file:%q does not exist", srcPath).Err()
	}
	if isDir {
		log.Debugf(ctx, "Copy to Logs: Attempting to collect the logs from %q to %q", srcPath, logRoot)
		if err := info.CopyDirectoryFrom(ctx, resource, srcPath, logRoot); err != nil {
			return errors.Annotate(err, "copy to logs").Err()
		}
	} else {
		destDir := logRoot
		log.Debugf(ctx, "Copy to Logs: Attempting to collect the logs from %q to %q!", srcPath, destDir)
		if err := info.CopyFrom(ctx, resource, srcPath, destDir); err != nil {
			return errors.Annotate(err, "copy to logs").Err()
		}
	}
	return nil
}

// collectCrashDumpsExec fetches the crash dumps from the DUT.
//
// This exec function accepts the following parameters from the action:
// clean: remove the source file on the DUT, even if the copy attempt did not succeed.
// cleanup_timeout: the amount of time within which file cleanup should happen.
func collectCrashDumpsExec(ctx context.Context, info *execs.ExecInfo) error {
	logRoot := info.GetLogRoot()
	resource := info.GetActiveResource()
	infoDir := filepath.Join(logRoot, fmt.Sprintf("crashinfo.%s", resource))
	if err := os.MkdirAll(infoDir, os.ModePerm); err != nil {
		return errors.Annotate(err, "collect crash dumps execs").Err()
	}
	// Note: at this time we are only interested in the collection of
	// logs. The legacy logic to additionally create the stacktrace
	// for the crashdumps does not work correctly. We will eventually
	// include a correct implementation of the same in Paris. Bug
	// http://b/262346604 has been created to keep track of this.

	// The location of crash dumps on the DUT.
	const crashDir = "/var/spool/crash/"
	const crashFiles = crashDir + "*"
	run := info.NewRunner(resource)
	output, err := run(ctx, time.Minute, "ls", "-1", crashFiles)
	if err != nil {
		return errors.Annotate(err, "collect crash dumps exec").Err()
	}
	if output != "" {
		orphans := strings.Split(output, "\n")
		argMap := info.GetActionArgs(ctx)
		cleanUp := argMap.AsBool(ctx, "clean", true)
		timeout := argMap.AsDuration(ctx, "cleanup_timeout", 10, time.Second)
		for _, f := range orphans {
			log.Debugf(ctx, "Collect Crash Dumps Exec: Attempting to collect orphan file %q", f)
			if err := info.CopyFrom(ctx, info.GetDut().Name, f, infoDir); err != nil {
				log.Debugf(ctx, "Collect Crash Dumps Exec: (non-critical) error %s while copying %q to %q", err.Error(), f, infoDir)
			}
			if cleanUp {
				fPath := crashDir + f
				if _, err := run(ctx, timeout, "rm", "-f", fPath); err != nil {
					log.Debugf(ctx, "Collect Crash Dumps Exec: (non-critical) error %s while removing file %q on the DUT", err.Error(), fPath)
				}
			}
		}
		if len(orphans) == 0 {
			log.Debugf(ctx, "Collect Crash Dumps Exec: There are no orphaned crashdump files on the source-side, hence deleting the destination folder %q", infoDir)
			if err := os.RemoveAll(infoDir); err != nil {
				log.Debugf(ctx, "Collect Crash Dumps Exec: (non-critical) error %s while removing the source directory %q", err.Error(), infoDir)
			}
		}
	}
	return nil
}

// createLogCollectionInfoExec creates a marker file that indicates
// that the log files have been collected.
//
// This exec accepts the following parameters from the action:
// info_file: the name of the info file that will be created.
func createLogCollectionInfoExec(ctx context.Context, info *execs.ExecInfo) error {
	argMap := info.GetActionArgs(ctx)
	logRoot := info.GetLogRoot()
	infoFilePath := filepath.Join(logRoot, argMap.AsString(ctx, "info_file", "log_collection_info"))
	infoFile, err := os.Create(infoFilePath)
	if err != nil {
		return errors.Annotate(err, "create log collection info").Err()
	}
	_, err = infoFile.WriteString(fmt.Sprintf("Retrieved the prior logs at %q\n", time.Now()))
	return errors.Annotate(err, "create log collection info").Err()
}

// confirmFileNotExistsExec confirms that the file mentioned in the
// parameters does not exist on the actual file system.
//
// This exec accepts the following parameters from the action:
// target_file: complete path of the file that needs to be checked.
func confirmFileNotExistsExec(ctx context.Context, info *execs.ExecInfo) error {
	argMap := info.GetActionArgs(ctx)
	logRoot := info.GetLogRoot()
	infoFilePath := filepath.Join(logRoot, argMap.AsString(ctx, "target_file", "log_collection_info"))
	_, err := os.Stat(infoFilePath)
	if err == nil {
		return errors.Reason("confirm file not exists: the file %q already exists", infoFilePath).Err()
	}
	log := info.NewLogger()
	if errors.Is(err, os.ErrNotExist) {
		log.Debugf("Confirm File Not Exists: the file %q does not exist beforehand", infoFilePath)
		return nil
	}
	log.Debugf("Confirm File Not Exists: cannot determine whether the file %q exists or not.", infoFilePath)
	return errors.Annotate(err, "confirm file not exists").Err()
}

// verboseShillLogsExec enables/disables verbose shill logs.
func verboseShillLogsExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	enabled := argsMap.AsBool(ctx, "is_enabled", true)

	var cmd string
	if enabled {
		cmd = "ff_debug cellular+modem+device+dbus+manager --level -3"
	} else {
		cmd = "ff_debug reset --level 0"
	}

	runner := info.DefaultRunner()
	if _, err := runner(ctx, info.GetExecTimeout(), cmd); err != nil {
		return errors.Annotate(err, "verbose network logs: set shill logging level").Err()
	}
	return nil
}

// verboseModemManagerLogsExec enables/disables verbose ModemManager logs.
func verboseModemManagerLogsExec(ctx context.Context, info *execs.ExecInfo) error {
	argsMap := info.GetActionArgs(ctx)
	enabled := argsMap.AsBool(ctx, "is_enabled", true)

	var cmd string
	if enabled {
		cmd = "modem set-logging debug"
	} else {
		cmd = "modem set-logging info"
	}

	runner := info.DefaultRunner()
	if _, err := runner(ctx, info.GetExecTimeout(), cmd); err != nil {
		return errors.Annotate(err, "verbose network logs: set shill logging level").Err()
	}
	return nil
}

func init() {
	execs.Register("cros_dmesg", dmesgExec)
	execs.Register("cros_copy_to_logs", copyToLogsExec)
	execs.Register("cros_collect_crash_dumps", collectCrashDumpsExec)
	execs.Register("cros_create_log_collection_info", createLogCollectionInfoExec)
	execs.Register("cros_confirm_file_not_exists", confirmFileNotExistsExec)
	execs.Register("cros_set_verbose_shill_logs", verboseShillLogsExec)
	execs.Register("cros_set_verbose_mm_logs", verboseModemManagerLogsExec)
}
