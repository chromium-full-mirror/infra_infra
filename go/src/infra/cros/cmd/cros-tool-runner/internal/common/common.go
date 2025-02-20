// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/protobuf/jsonpb"

	"go.chromium.org/luci/common/errors"
)

func IsCriticalPullCrash(code int) bool {
	criticalCodes := map[int]bool{125: true} // Critical crash codes which should stop forward progress. Expand as needed.
	_, ok := criticalCodes[code]
	return ok
}

// RunWithTimeout runs command with timeout limit.
func RunWithTimeout(ctx context.Context, cmd *exec.Cmd, timeout time.Duration, block bool) (stdout string, stderr string, err error) {
	log.Printf("Run cmd: %q", cmd)
	return runWithTimeout(ctx, cmd, timeout, block)
}

func runWithTimeout(ctx context.Context, cmd *exec.Cmd, timeout time.Duration, block bool) (stdout string, stderr string, err error) {
	// TODO(b/219094608): Implement timeout usage.
	var se, so bytes.Buffer
	cmd.Stderr = &se
	cmd.Stdout = &so
	defer func() {
		stdout = so.String()
		stderr = se.String()
	}()

	if block {
		err = cmd.Run()
	} else {
		err = cmd.Start()
	}

	if err != nil {
		log.Printf("error found with cmd: %s", err)
	}
	return
}

// RunWithTimeoutRedacted runs command with timeout limit, however only will log whats provided in the logstr input. Useful when running cmds with private info, like oath tokens.
func RunWithTimeoutSpecialLog(ctx context.Context, cmd *exec.Cmd, timeout time.Duration, block bool, logstr string) (stdout string, stderr string, err error) {
	log.Printf("Run cmd: %q", logstr)
	return runWithTimeout(ctx, cmd, timeout, block)
}

// PrintToLog prints cmd, stdout, stderr to log
func PrintToLog(cmd string, stdout string, stderr string) {
	if cmd != "" {
		log.Printf("%q command execution.", cmd)
	}
	if stdout != "" {
		log.Printf("stdout: %s", stdout)
	}
	if stderr != "" {
		log.Printf("stderr: %s", stderr)
	}
}

// SetUpLog sets up the logging for CTR within /var/tmp/bbid/ctrlog.txt
func SetUpLog() error {
	bbid := BuildBucketID()
	if bbid == "" {
		// Not found? Use random time.
		bbid = fmt.Sprint(rand.New(rand.NewSource(time.Now().UnixNano())).Int())
	}

	basedir := "/var/tmp/"
	logPath := path.Join(basedir, bbid)
	if err := os.MkdirAll(logPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %v: %w", basedir, err)
	}
	lfp := filepath.Join(logPath, "ctrlog.txt")
	lf, err := os.Create(lfp)
	if err != nil {
		return fmt.Errorf("failed to create file %v: %w", lfp, err)
	}
	log.SetOutput(io.MultiWriter(lf, os.Stderr))
	log.SetPrefix("<cros-tool-runner> ")
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmsgprefix)
	log.Printf("Made logger @ %s", lfp)
	return nil
}

// AddContentsToLog adds contents of the file of fileName to log
func AddContentsToLog(fileName string, rootDir string, msgToAdd string) error {
	filePath, err := FindFile(fileName, rootDir)
	if err != nil {
		log.Printf("%s finding file '%s' at '%s' failed:%s", msgToAdd, fileName, filePath, err)
		return err
	}
	fileContents, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("%s reading file '%s' at '%s' failed:%s", msgToAdd, fileName, filePath, err)
		return err
	}
	log.Printf("%s file '%s' info at '%s':\n\n%s\n", msgToAdd, fileName, filePath, string(fileContents))
	return nil
}

// FindFile finds file path in rootDir of fileName
func FindFile(fileName string, rootDir string) (string, error) {
	filePath := ""
	filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == fileName {
			filePath = path
		}
		return nil
	})

	if filePath != "" {
		return filePath, nil
	}

	return "", errors.Reason(fmt.Sprintf("file '%s' not found!", fileName)).Err()
}

// JsonPbUnMarshaler returns the unmarshaler which should be used across CTR.
func JsonPbUnmarshaler() jsonpb.Unmarshaler {
	return jsonpb.Unmarshaler{AllowUnknownFields: true}
}

// StreamScanner makes a scanner to read from test streams.
func StreamScanner(stream io.Reader, logtag string) {
	const maxCapacity = 4096 * 1024
	scanner := bufio.NewScanner(stream)
	// Expand the buffer size to avoid deadlocks on heavy logs
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		log.Printf("[%v] %v", logtag, scanner.Text())
	}
	if scanner.Err() != nil {
		log.Println("Failed to read pipe: ", scanner.Err())
	}
}

func BuildBucketID() string {
	bbidArr := strings.Split(os.Getenv("LOGDOG_STREAM_PREFIX"), "/")
	return bbidArr[len(bbidArr)-1]
}
