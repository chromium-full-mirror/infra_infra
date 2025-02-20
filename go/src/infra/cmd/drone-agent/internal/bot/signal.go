// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package bot

import (
	"fmt"
	"log"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"
)

// TerminateOrKill implements Bot.
func (b realBot) TerminateOrKill() error {
	log.Printf("Terminating bot %s", b.config.BotID)
	defer func() {
		if b.cgroup == nil {
			return
		}
		if err := b.cgroup.Delete(); err != nil {
			log.Printf("Error deleting cgroup of %s: %s", b.config.BotID, err)
		}
	}()
	err := b.cmd.Process.Signal(unix.SIGTERM)
	if err != nil {
		return fmt.Errorf("SIGTERM bot %s: %w", b.config.BotID, err)
	}

	killTimer := time.NewTimer(GraceInterval)
	defer killTimer.Stop()

	select {
	case <-b.terminated:
		log.Printf("Terminated bot %s", b.config.BotID)
	case <-killTimer.C:
		// Kill the bot process tree, not the bot itself only.
		// The processes may in different process groups or sessions, so
		// we have to kill each of them recursively.
		log.Printf("Wait for SIGTERM expired so try killing bot %q and its descendants", b.config.BotID)
		p, err := process.NewProcess(int32(b.cmd.Process.Pid))
		if err != nil {
			return fmt.Errorf("Failed to create process object for %s: %w", b.config.BotID, err)
		}
		tryKillProcessTree(p)
	}
	return nil
}

// tryKillProcessTree kills the process tree led by specified process at the
// best effort.
// We ignore any errors happened during the operation and doesn't commit to
// kill all descendants successfully.
func tryKillProcessTree(p *process.Process) {
	// Stop the process first so it won't spawn new children during the killing.
	if err := p.Suspend(); err != nil {
		log.Printf("Suspend %q: %s", p, err)
		return
	}
	// Get children of current process and recursively kill them.
	// Don't use `process.Children` method of gopsutil since which depends on
	// `pgrep` command.
	allProc, err := process.Processes()
	if err != nil {
		log.Printf("Get all processes: %s", err)
		return
	}
	for _, proc := range allProc {
		ppid, err := proc.Ppid()
		if err != nil {
			continue
		}
		if ppid == p.Pid {
			tryKillProcessTree(proc)
		}
	}
	if err := p.Kill(); err != nil {
		log.Printf("Kill %q: %s", p, err)
	}
}
