// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servodserver implements servod_service.proto (see proto for details)
package servodserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"infra/cros/servo"
	"infra/cros/servo/errors"
	"infra/cros/servo/logging"
	"infra/cros/servo/testing"
)

type logger struct {
	impl *log.Logger
}

// Log gets called for a log entry.
func (l *logger) Log(level logging.Level, ts time.Time, msg string) {
	l.impl.Println(msg)
}

// SaveServoLog save logs from servo and servo host.
// TODO: Change the interface to get connection directly instead of passing the servoHost.
func SaveServoLog(ctx context.Context, servoHost, dockerContainer string, servoPorts []int32,
	sshClient *ssh.Client, dst string, l *log.Logger) error {
	ctx = logging.AttachLogger(ctx, &logger{impl: l})
	for i, servoPort := range servoPorts {

		hostnameForProxy, err := composeHostnameForProxy(servoHost, dockerContainer, servoPort)
		if err != nil {
			return errors.Wrapf(err, "failed to parse host nameto save servo logs")
		}
		proxy, err := servo.NewProxy(ctx, hostnameForProxy, "", "", sshClient)
		if err != nil {
			return errors.Wrapf(err, "failed to create new proxy to %s", hostnameForProxy)
		}
		defer proxy.Close(ctx)
		if i == 0 {
			// collect system logs once.
			if err := collectSystemLogs(ctx, proxy, dst); err != nil {
				return errors.Wrapf(err, "failed to collect servo host logs for %s", hostnameForProxy)
			}
		}
		if err := collectServoLogs(ctx, proxy, servoPort, dst); err != nil {
			return errors.Wrapf(err, "failed to collect servo logs for %s", hostnameForProxy)
		}
	}
	return nil
}

// composeHostnameForProxy composes a hostname that following convension used by Tast servo library.
// TODO: Remove this function after we refactor NewProxy. We only use this function temporary to
// reuse the Tast servo library.
func composeHostnameForProxy(servoHost, dockerContainer string, servoPort int32) (string, error) {
	if servoPort == 0 {
		servoPort = 9999
	}
	if dockerContainer != "" {
		return fmt.Sprintf("%s:%d", dockerContainer, servoPort), nil
	}
	if servoHost == "localhost" || servoHost == "127.0.0.1" || servoHost == "::1" {
		// format: localhost:<servo_port>
		return fmt.Sprintf("localhost:%d", servoPort), nil
	}
	host, sshPort, err := splitHostPort(servoHost)
	if err != nil {
		return "", errors.Wrap(err, "failed parse servo host")
	}
	if sshPort == "" {
		// format: host:port (ssh port defaults to 22)
		return fmt.Sprintf("%s:%d", host, servoPort), nil
	}
	// format: host:port:ssh:sshport.
	return fmt.Sprintf("%s:%d:ssh:%s", host, servoPort, sshPort), nil
}

// splitHostPort splits an address to host and port.
// Example 1: "127.0.0.1" -> "127.0.0.1" "".
// Example 2: "[0:0:0:0:0:ffff:7f00:1]:2" -> "0:0:0:0:0:ffff:7f00:1" "2".
func splitHostPort(hostAndPort string) (host, port string, err error) {
	numColons := strings.Count(hostAndPort, ":")
	// For case like 127.0.0.1.
	host = hostAndPort
	port = ""
	if numColons == 0 {
		// For case like 127.0.0.1.
		return hostAndPort, "", nil
	}
	if strings.HasPrefix(hostAndPort, "[") && strings.HasSuffix(hostAndPort, "]") {
		// Example: "[0:0:0:0:0:ffff:7f00:1]".
		// SSH config does not support ipv6 names with [].
		return hostAndPort[1 : len(hostAndPort)-1], "", nil
	}
	if numColons == 1 || strings.HasPrefix(hostAndPort, "[") {
		// Example: "[0:0:0:0:0:ffff:7f00:1]:10" or "test:1".
		if host, port, err = net.SplitHostPort(hostAndPort); err != nil {
			return "", "", err
		}
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		// Since SSH config does not support ipv6 names with [], strip them.
		return host[1 : len(host)-1], port, nil
	}
	return host, port, nil
}

// collectServoLogs downloads servo logs to the dest directory.
func collectServoLogs(ctx context.Context, proxy *servo.Proxy, servoPort int32, dst string) (retErr error) {
	servodLogDir := fmt.Sprintf("/var/log/servod_%d", servoPort)

	testing.ContextLog(ctx, "Collecting servo logs from ", servodLogDir)
	defer testing.ContextLog(ctx, "Done collecting servo logs from ", servodLogDir)

	destDir := filepath.Join(dst, fmt.Sprintf("servod_%d", servoPort))
	if err := os.MkdirAll(destDir, 0755); err != nil {
		testing.ContextLogf(ctx, "Failed to create dir %s for downloading servo logs: %v", destDir, err)
	}

	// Getting servod logs.
	downloadServodLogs(ctx, proxy, servodLogDir, destDir)

	// Extract MCU logs from latest.DEBUG.
	extractServodMCULogs(ctx, destDir)

	writeServoInfoLog(ctx, proxy.Servo(), destDir)

	return nil
}

// collectSystemLogs downloads servo host system logs.
func collectSystemLogs(ctx context.Context, proxy *servo.Proxy, dst string) (retErr error) {
	if err := os.MkdirAll(dst, 0755); err != nil {
		testing.ContextLogf(ctx, "Failed to create dir %s for downloading servo logs: %v", dst, err)
	}

	// Getting dmesg.
	downloadMessagesLog(ctx, proxy, dst)

	// Getting dmesg.
	downloadServodDMesgLogs(ctx, proxy, dst)

	return nil
}

func downloadMessagesLog(ctx context.Context, proxy *servo.Proxy, dst string) {
	// Getting servo start log from servo host.
	fn := filepath.Join(dst, "messages")
	testing.ContextLogf(ctx, "Saving /var/log/messages log %s", fn)
	if err := proxy.GetFile(ctx, false, "/var/log/messages", fn); err != nil {
		testing.ContextLogf(ctx, "Failed to get /var/log/messages: %v", err)
	}
}

func downloadServodLogs(ctx context.Context, proxy *servo.Proxy, servodLogDir, destDir string) {
	fn := "latest.DEBUG"
	src := filepath.Join(servodLogDir, fn)
	dst := filepath.Join(destDir, fn)

	out, err := proxy.OutputCommand(ctx, false, "realpath", src)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to get real servo log path %s: %v", fn, err)
		return
	}
	realpath := strings.TrimSpace(string(out))
	testing.ContextLog(ctx, "Saving servo log ", realpath)
	if err := proxy.GetFile(ctx, false, realpath, dst); err != nil {
		testing.ContextLogf(ctx, "Failed to get servod log %s: %v", src, err)
	}
}

func downloadServodDMesgLogs(ctx context.Context, proxy *servo.Proxy, servoHostDestDir string) {
	testing.ContextLog(ctx, "Saving dmesg log")
	out, err := proxy.OutputCommand(ctx, false, "dmesg", "-H")
	if err != nil {
		testing.ContextLogf(ctx, "Failed to run dmesg: %v", err)
		return
	}
	dmesgFile := filepath.Join(servoHostDestDir, "dmesg")
	dmesgOut, err := os.Create(dmesgFile)
	if err != nil {
		testing.ContextLog(ctx, "Failed to create servo log dmesg: ", err)
		return
	}
	defer dmesgOut.Close()
	if _, err := dmesgOut.Write(out); err != nil {
		testing.ContextLog(ctx, "Failed to write dmesg to file: ", err)
	}
}

// extractServodMCULogs extract MCU logs from latest.DEBUG.
func extractServodMCULogs(ctx context.Context, destDir string) {
	testing.ContextLog(ctx, "Extracing servod MCU logs")
	mcuFiles := make(map[string]*os.File)
	src := filepath.Join(destDir, "latest.DEBUG")
	f, err := os.Open(src)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to open %s: %v", src, err)
		return
	}
	defer f.Close()

	regExpr := `(?P<time>[\d\-]+ [\d:,]+ )` +
		`- (?P<mcu>[\w/]+) - ` +
		`EC3PO\.Console[\s\-\w\d:.]+LogConsoleOutput - /dev/pts/\d+ - ` +
		`(?P<line>.+$)`

	re, err := regexp.Compile(regExpr)
	if err != nil {
		fmt.Printf("Fail in compiling expression %v\n", err)
		return
	}

	sc := bufio.NewScanner(f)
	sc.Split(bufio.ScanLines)
	for sc.Scan() {
		text := sc.Text()
		matches := re.FindStringSubmatch(text)
		timeIndex := re.SubexpIndex("time")
		if timeIndex < 0 || timeIndex >= len(matches) {
			continue
		}
		mcuIndex := re.SubexpIndex("mcu")
		if mcuIndex < 0 || mcuIndex >= len(matches) {
			continue
		}
		lineIndex := re.SubexpIndex("line")
		if lineIndex < 0 || lineIndex >= len(matches) {
			continue
		}
		timestamp := matches[timeIndex]
		mcu := strings.ToLower(matches[mcuIndex])
		line := matches[lineIndex]
		mcuFile, ok := mcuFiles[mcu]
		if !ok {
			mcuFile, err = os.Create(filepath.Join(destDir, fmt.Sprintf("%s.txt", mcu)))
			if err != nil {
				testing.ContextLogf(ctx, "Failed to create servo log %s.txt: %v", mcu, err)
				mcuFiles[mcu] = nil
				continue
			}
			mcuFiles[mcu] = mcuFile
			defer mcuFile.Close()
		}
		if mcuFile == nil {
			continue
		}
		_, _ = fmt.Fprintln(mcuFile, timestamp, "- ", line)
	}
}

type servoInfo struct {
	ServodVersion string `json:"servod_version"`
	ServoType     string `json:"servo_type"`
}

func writeServoInfoLog(ctx context.Context, servo *servo.Servo, outDir string) {
	testing.ContextLog(ctx, "Writing servo_info.json")

	if err := os.MkdirAll(outDir, 0755); err != nil {
		testing.ContextLog(ctx, "Failed to create dir: ", err)
	}

	servodVersion, err := servo.GetServodVersion(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get servod version: ", err)
	}
	// The servod version output is multiple lines:
	//   v1.0.2382+643d2b40
	//   Date: 2024-10-02 11:54:12
	//   Builder: 613dcd9ba833
	//   Hash: +643d2b40
	//   Branch: hdctools-release-1024.1
	// Reduce this to just version and date: "v1.0.2382+643d2b40 2024-10-02 11:54:12"
	verRE := regexp.MustCompile(`^(v\S+)\s+(.*)`)
	matches := verRE.FindStringSubmatch(servodVersion)
	if len(matches) == 3 {
		ver := matches[1]
		date := strings.ReplaceAll(matches[2], "Date: ", "")
		servodVersion = ver + " " + date
	}

	// Servo type is a string like "servo_v4_with_c2d2_and_ccd_gsc".
	servoType, err := servo.GetServoType(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get servo type: ", err)
	}

	var si servoInfo
	si.ServodVersion = servodVersion
	si.ServoType = servoType
	jsonData, err := json.Marshal(si)
	if err != nil {
		testing.ContextLog(ctx, "Failed to marshal json: ", err)
	}
	err = os.WriteFile(filepath.Join(outDir, "servo_info.json"), jsonData, 0666)
	if err != nil {
		testing.ContextLog(ctx, "Failed to write file: ", err)
	}
}
