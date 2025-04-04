// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main is the entry point for the adb-logcat binary.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"go.chromium.org/chromiumos/test/util/adb"
)

type cliArgs struct {
	// logsDir is currently not used but will contain the directory mounted to
	// the container in which we'll place the log.txt file.
	logsDir string

	// adbAddress is the host address required by the ADB keep alive command.
	// This will be in the format of <dutName>:<portNumber>.
	// NOTE: Once this is fully containerized then this will be fetched via the
	// DUT topology interface rather than CLI args.
	adbAddress string
}

var (
	globArgs = &cliArgs{}
)

// readArgs uses the flag std library to parse os.args[1:] sent to the
// executable via CLI.
func readArgs() error {
	flag.StringVar(&globArgs.adbAddress, "adb-address", "", "Network address of the DUT. Used to initiate and maintain the ADB connection.")
	flag.StringVar(&globArgs.logsDir, "log-dir", "", "Mounted directory for storing logs.")
	flag.Parse()

	return nil
}

// ParseInputs is a helper method which parses input arguments.
func ParseInputs() error {
	if len(os.Args) < 1 {
		return fmt.Errorf("CLI arguments must be specified")
	}

	err := readArgs()
	if err != nil {
		return err
	}

	return nil
}

// SetUpLog creates a new logger.
// NOTE: This currently is sending the buffer to STDOUT because of how this
// binary is called in foil-provision. Once this becomes fully containerized and
// integrated into the CFT ecosystem this should instead write to a log.txt
// file.
func SetUpLog() (*log.Logger, error) {
	newLog := log.New(io.MultiWriter(os.Stdout, os.Stderr), "<adb-logcat>", log.LstdFlags|log.LUTC)
	newLog.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmsgprefix)

	return newLog, nil
}

func innerRun() error {
	// Fetch the args given to the CLI.
	err := ParseInputs()
	if err != nil {
		return err
	}

	// Create the logger to write output to the logs.txt file.
	logger, err := SetUpLog()
	if err != nil {
		return err
	}

	// Start up the adb auto connect service. This backgrounds a goroutine which
	// automatically reconnects adb when the connection is dropped. This will
	// keep logcat logs flowing in the following loop.
	exit := make(chan struct{})
	adb.KeepAdbAlive(logger, []string{globArgs.adbAddress}, exit)

	// Call adb logcat and redirect the STDOUT to append (>>) to the logcat
	// file. Also, redirect the STDERR to STDOUT (2>&1).
	actualCMD := fmt.Sprintf("adb logcat >>%s%s 2>&1", globArgs.logsDir, "logcat.txt")

	// Endlessly gather logs from `adb logcat`. If the command stops gracefully
	// because of an adb disconnect, there will be no error and the loop will go
	// to the next iteration.
	for {
		// Run the command using bash directly. We need to do this because
		// os/exec does not expand linux globs nor does it handle redirects.
		cmd := exec.Command("bash", "-c", actualCMD)

		// Run the above command and wait for an exit code. Run() is a blocking
		// call and we are using this rather than Start() since we do not want
		// to kick off endless numbers of the `adb logcat` command.
		err := cmd.Run()
		if err != nil {
			return err
		}
	}
}

func main() {
	// This binary is admittedly doing funky stuff by directly kicking off
	// commands via "CLI". This worked very well during the prototype stage so
	// it's being continued on here in the gen1 of the service. If possible I
	// will migrate this to using the ADB API that we have made in dev-utils.
	// For now capture any panic which may or may not occur.
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(os.Stderr, "panic: %s\n", err)
			os.Exit(1)
		}
	}()

	// Call the actual main and output any errors to STDERR.
	if err := innerRun(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
