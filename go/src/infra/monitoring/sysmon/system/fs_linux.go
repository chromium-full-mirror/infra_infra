// Copyright (c) 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package system

import (
	"os"
	"strings"
	"sync"
	"unicode"
)

const (
	nodevPrefix = "nodev\t"
)

var (
	ignoredFstypes   map[string]struct{}
	ignoredFstypesMu sync.Mutex
)

func shouldIgnoreFstype(fstype string) bool {
	if strings.HasPrefix(fstype, "fuse.") || fstype == "none" {
		return true
	}

	ignoredFstypesMu.Lock()
	defer ignoredFstypesMu.Unlock()

	if ignoredFstypes == nil {
		contents, err := os.ReadFile("/proc/filesystems")
		if err != nil {
			return false
		}

		lines := strings.Split(string(contents), "\n")
		ignoredFstypes = make(map[string]struct{}, len(lines))
		for _, line := range lines {
			if strings.HasPrefix(line, nodevPrefix) {
				ignoredFstypes[line[len(nodevPrefix):]] = struct{}{}
			}
		}
	}

	_, ok := ignoredFstypes[fstype]
	return ok
}

func shouldIgnoreMountpoint(mountpoint string) bool {
	// This is standard location for Docker containers on Linux (and we only
	// support them on Linux atm), which are mounted to a separate partition when
	// the container is running. The disk stats reported for them are the same as
	// for the parent partition on which they are located, which causes disk space
	// alert to be fired twice for the same physical paritition.
	return strings.HasPrefix(mountpoint, "/var/lib/docker")
}

func removeDiskDevices(names []string) []string {
	disksWithPartitions := map[string]struct{}{}
	for _, name := range names {
		if len(name) > 0 && unicode.IsDigit(rune(name[len(name)-1])) {
			disksWithPartitions[name[:len(name)-1]] = struct{}{}
		}
	}

	var ret []string
	for _, name := range names {
		_, diskHasPartitions := disksWithPartitions[name]
		if len(name) > 0 && unicode.IsDigit(rune(name[len(name)-1])) || !diskHasPartitions {
			ret = append(ret, name)
		}
	}
	return ret
}
