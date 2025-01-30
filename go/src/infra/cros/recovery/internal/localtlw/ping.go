// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package localtlw

import (
	"bytes"
	"os/exec"
	"strconv"

	"go.chromium.org/luci/common/errors"
)

const cloudBotPingServer = "openssh-server"

func formatPingCommand(addr string, count int) ([]string, error) {
	if addr == "" {
		return nil, errors.Reason("formatPingCommand: addr is empty").Err()
	}
	return []string{"ping",
		addr,
		"-c",
		strconv.Itoa(count), // How many times will ping.
		"-W",
		"1", // How long wait for response.
	}, nil
}

// ping represent simple network verification by ping by hostname.
func ping(addr string, count int) error {
	pingCmd, err := formatPingCommand(addr, count)
	if err != nil {
		return errors.Annotate(err, "ping").Err()
	}
	cmd := exec.Command(pingCmd[0], pingCmd[1:]...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err = cmd.Run(); err != nil {
		return errors.Annotate(err, "%s", stderr.String()).Err()
	}
	return nil
}
