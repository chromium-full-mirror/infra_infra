// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bot wraps managing Swarming bots.
package bot

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"

	"go.chromium.org/luci/common/errors"
)

// GraceInterval is the amount of time to provide for bots to
// terminate gracefully.
const GraceInterval = 3 * time.Minute

// Bot is the interface for interacting with a started Swarming bot.
// Wait must be called to ensure the process is waited for.
type Bot interface {
	// Wait waits for the bot process to exit.  The return value
	// on subsequent calls is undefined.
	Wait() error
	// Drain signals for the bot to drain.  Note that this requires
	// support from the bot script.  This should be handled by Swarming
	// bots by waiting for the currently running task to finish before
	// exiting.
	Drain() error
	// TerminateOrKill terminates the bot with SIGTERM, then force kills it
	// after a grace period.
	// Swarming bots handle SIGTERM by aborting the currently running task and
	// exiting, but it can be hours long.
	// It relies on Wait() to be called to notify if the bot process has been
	// terminated.
	TerminateOrKill() error
}

type realBot struct {
	config     Config
	cgroup     Cgroup
	cmd        *exec.Cmd
	logFile    *os.File
	terminated chan struct{}
}

// Wait implements Bot.
func (b realBot) Wait() error {
	err := b.cmd.Wait()
	close(b.terminated)
	_ = b.logFile.Close()
	log.Printf("Done waiting for bot %s", b.config.BotID)
	return err
}

// Drain implements Bot.
func (b realBot) Drain() error {
	log.Printf("Draining bot %s", b.config.BotID)
	f, err := os.Create(b.config.drainFilePath())
	if err != nil {
		return errors.Annotate(err, "drain bot %s", b.config.BotID).Err()
	}
	if err := f.Close(); err != nil {
		return errors.Annotate(err, "drain bot %s", b.config.BotID).Err()
	}
	return nil
}

// Starter has a Start method for starting Swarming bots.
type Starter struct {
	client *http.Client
	// swarmingURL is the URL of the Swarming instance.  Should be
	// a full URL without the path, e.g. https://host.example.com
	swarmingURL string
}

// NewStarter returns a new Starter.
// swarmingURL is the URL of the Swarming instance.  Should be
// a full URL without the path, e.g. https://host.example.com
func NewStarter(c *http.Client, swarmingURL string) Starter {
	return Starter{
		client:      c,
		swarmingURL: swarmingURL,
	}
}

// Start starts a Swarming bot.  The returned Bot object can be used
// to interact with the bot.
func (s Starter) Start(c Config) (b Bot, err error) {
	if err := s.downloadBotCode(c); err != nil {
		return nil, errors.Annotate(err, "start bot with %+v", c).Err()
	}
	f, err := os.Create(c.logFilePath())
	if err != nil {
		return nil, errors.Annotate(err, "start bot with %+v", c).Err()
	}
	defer func() {
		if err != nil {
			_ = f.Close()
		}
	}()
	cmd := exec.Command(c.PythonVersion, c.botZipPath(), "start_bot")
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Env = append(c.env(), os.Environ()...)
	if err := cmd.Start(); err != nil {
		return nil, errors.Annotate(err, "start bot with %+v", c).Err()
	}
	bot := realBot{
		config:     c,
		cmd:        cmd,
		logFile:    f,
		terminated: make(chan struct{}),
	}
	control, err := addToCgroup(c.BotID, uint64(cmd.Process.Pid), c.Resources)
	if err != nil {
		bot.TerminateOrKill()
		return nil, errors.Annotate(err, "start bot with %+v", c).Err()
	}
	bot.cgroup = control
	log.Printf("Bot for %s started", c.BotID)
	return bot, nil
}

func (s Starter) downloadBotCode(c Config) error {
	f, err := os.Create(c.botZipPath())
	if err != nil {
		return errors.Annotate(err, "download bot code for %+v", c).Err()
	}
	defer f.Close()

	resp, err := s.client.Get(s.botCodeURL(c.BotID))
	if err != nil {
		return errors.Annotate(err, "download bot code for %+v", c).Err()
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.Reason("download bot code for %+v: unexpected status %v", c, resp.StatusCode).Err()
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		return errors.Annotate(err, "download bot code for %+v", c).Err()
	}
	if err := f.Close(); err != nil {
		return errors.Annotate(err, "download bot code for %+v", c).Err()
	}
	return nil
}

func (s Starter) botCodeURL(botID string) string {
	return fmt.Sprintf("%s/bot_code?bot_id=%s", s.swarmingURL, botID)
}

// Config is the configuration needed for starting a generic Swarming bot.
type Config struct {
	BotID string
	// WorkDirectory is the Swarming bot's work directory.
	// The caller should create this.
	// The parent directory should be writable to allow creation
	// of the drain file.
	WorkDirectory string
	// The Linux compute resources (CPU, RAM, I/O, etc.) assigned to this bot.
	Resources     *specs.LinuxResources
	PythonVersion string
}

func (c Config) drainFilePath() string {
	return c.WorkDirectory + ".drain"
}

func (c Config) logFilePath() string {
	return c.WorkDirectory + ".log"
}

func (c Config) botZipPath() string {
	return filepath.Join(c.WorkDirectory, "swarming_bot.zip")
}

func (c Config) env() []string {
	return []string{
		"SWARMING_BOT_ID=" + c.BotID,
	}
}
