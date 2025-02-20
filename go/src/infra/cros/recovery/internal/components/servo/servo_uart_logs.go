// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// List of UART streams consoles of CPU, EC, Cr50, etc.
var uartConsolesToCaptureList = []string{"cpu", "cr50", "ec", "servo_micro",
	"servo_v4", "usbpd", "servo_v4p1", "c2d2",
	"ccd_cr50.ec", "ccd_cr50.cpu", "ccd_cr50.cr50",
	"ccd_gsc.ec", "ccd_gsc.cpu", "ccd_gsc.cr50"}

const (
	uartCaptureContolrGlob = "%s_uart_capture"
	uartStreamContolrGlob  = "%s_uart_stream"
)

func tryStartStopUartCapture(ctx context.Context, servod components.Servod, newState string) []string {
	var validConsoles []string
	for _, uartConsole := range uartConsolesToCaptureList {
		uartControl := fmt.Sprintf(uartCaptureContolrGlob, uartConsole)
		if err := servod.Has(ctx, uartControl); err != nil {
			log.Debugf(ctx, "UART console control %q is not present on servod.", uartControl)
			continue
		}
		// Start enable the copturing
		if err := servod.Set(ctx, uartControl, newState); err != nil {
			log.Debugf(ctx, "Fail to enable UART console by control %q", uartControl)
			continue
		}
		state, err := GetString(ctx, servod, uartControl)
		if err != nil {
			log.Debugf(ctx, "Fail to enable UART console by control %q", uartControl)
			continue
		}
		if state != newState {
			log.Debugf(ctx, "Fail to enable UART console by control %q, as got %q", uartControl, state)
			continue
		}
		validConsoles = append(validConsoles, uartConsole)
		log.Debugf(ctx, "Console %q is enabled", uartConsole)
	}
	return validConsoles
}

// StartUartCapture sets all available UART capture to state 'on'.
func StartUartCapture(ctx context.Context, servod components.Servod) error {
	log.Debugf(ctx, "Start UART capture")
	validConsoles := tryStartStopUartCapture(ctx, servod, "on")
	log.Infof(ctx, "successful started consoles %v", validConsoles)
	return nil
}

// StopUartCapture sets all available UART capture to state 'off'.
func StopUartCapture(ctx context.Context, servod components.Servod) error {
	log.Debugf(ctx, "Stop UART capture")
	validConsoles := tryStartStopUartCapture(ctx, servod, "off")
	log.Infof(ctx, "successful stoped consoles %v", validConsoles)
	return nil
}

// SaveUartStreamToFiles saves all available UART streams to files.
func SaveUartStreamToFiles(ctx context.Context, servod components.Servod, dirPath string) error {
	log.Debugf(ctx, "Starting saving UART console contents...")
	if err := exec.CommandContext(ctx, "mkdir", "-p", dirPath).Run(); err != nil {
		return errors.Annotate(err, "collect servod logs").Err()
	}
	for _, uartConsole := range uartConsolesToCaptureList {
		uartControl := fmt.Sprintf(uartStreamContolrGlob, uartConsole)
		if err := servod.Has(ctx, uartControl); err != nil {
			log.Debugf(ctx, "UART stream control %q is not present on servod.", uartControl)
			continue
		}
		content, err := GetString(ctx, servod, uartControl)
		if err != nil {
			log.Debugf(ctx, "Fail to get context of UART console for %q", uartConsole)
			continue
		}
		if content == "not_applicable" {
			log.Debugf(ctx, "UART console %q is not applicable.", uartConsole)
			continue
		}
		contextFile := fmt.Sprintf("%s_uart.content", uartConsole)
		filePath := filepath.Join(dirPath, contextFile)
		log.Debugf(ctx, "Content of stream for %q", uartConsole)
		if err := saveUARTConsoleContext(ctx, content, filePath); err != nil {
			return errors.Annotate(err, "damp UART console for %q", uartConsole).Err()
		}
	}
	return nil
}

func saveUARTConsoleContext(ctx context.Context, content, filePath string) error {
	if len(content) <= 1 {
		return nil
	}
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, components.DefaultFilePermissions)
	if err != nil {
		return errors.Annotate(err, "save UART content").Err()
	}
	// Ignore close error as it is not critical.
	defer f.Close()
	for i, line := range parseUartStreamContent(content) {
		log.Debugf(ctx, "line %d:%s", i, line)
		if _, err := f.Write([]byte(line + "\n")); err != nil {

			return errors.Annotate(err, "save UART content").Err()
		}
	}
	return nil
}

// Parse UART console content logs as a list of strings before saving them.
// TODO: Replace by a standard library call to parse python based logs.
func parseUartStreamContent(content string) []string {
	content = strings.Trim(strings.Trim(content, "'"), "\"")
	if len(content) == 0 {
		return nil
	}
	return strings.Split(content, "\\r\\n")
}
