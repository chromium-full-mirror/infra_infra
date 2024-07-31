// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tls provides the canonical implementation of a common TLS server.
package tls

import (
	"fmt"
	"log"
	"path"

	"golang.org/x/crypto/ssh"

	"go.chromium.org/luci/common/errors"
)

type dlcSlot string

const (
	dlcSlotA dlcSlot = "dlc_a"
	dlcSlotB dlcSlot = "dlc_b"
)

const (
	dlcCacheDir    = "/var/cache/dlc"
	dlcImage       = "dlc.img"
	dlcLibDir      = "/var/lib/dlcservice/dlc"
	dlcPackage     = "package"
	dlcVerified    = "verified"
	dlcserviceUtil = "dlcservice_util"
)

func getActiveDLCSlot(r rootDev) dlcSlot {
	switch r.partNum {
	case partitionNumRootA:
		return dlcSlotA
	case partitionNumRootB:
		return dlcSlotB
	default:
		panic(fmt.Sprintf("Invalid partition number %s", r.partNum))
	}
}

func getInactiveDLCSlot(r rootDev) dlcSlot {
	switch slot := getActiveDLCSlot(r); slot {
	case dlcSlotA:
		return dlcSlotB
	case dlcSlotB:
		return dlcSlotA
	default:
		panic(fmt.Sprintf("Invalid DLC slot %s", slot))
	}
}

// clearInactiveDLCVerifiedMarks will clear the verified marks for all DLCs in the inactive slots.
func clearInactiveDLCVerifiedMarks(c *ssh.Client, r rootDev) error {
	// Stop dlcservice daemon in order to not interfere with clearing inactive verified DLCs.
	if err := runCmd(c, "stop dlcservice"); err != nil {
		log.Printf("clear inactive verified DLC marks: failed to stop dlcservice daemon, %s", err)
	}
	defer func() {
		if err := runCmd(c, "start dlcservice"); err != nil {
			log.Printf("clear inactive verified DLC marks: failed to start dlcservice daemon, %s", err)
		}
	}()

	inactiveSlot := getInactiveDLCSlot(r)
	err := runCmd(c, fmt.Sprintf("rm -f %s", path.Join(dlcCacheDir, "*", "*", string(inactiveSlot), dlcVerified)))
	return errors.Annotate(err, "clear inactive verified DLC marks: failed remove inactive verified DLCs").Err()
}
