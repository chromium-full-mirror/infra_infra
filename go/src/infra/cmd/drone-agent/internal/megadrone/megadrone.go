// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package megadrone implements a megadrone agent, which manages a
// static set of Swarming bots.
// Unlike the agent package, it does not talk to drone queen to get
// DUT assignments.
package megadrone

import (
	"context"
	"fmt"
	"log"
	"sync"

	"go.chromium.org/infra/cmd/drone-agent/internal/bot"
	"go.chromium.org/infra/cmd/drone-agent/internal/botman"
	"go.chromium.org/infra/cmd/drone-agent/internal/draining"
)

// An Agent manages a static number of Swarming bots.
type Agent struct {
	// WorkingDir is used for Swarming bot working dirs.  It is
	// the caller's responsibility to create this.
	WorkingDir string
	// StartBotFunc is used to start Swarming bots.
	// This must be set.
	StartBotFunc func(bot.Config) (bot.Bot, error)
	// BotPrefix is used to prefix IDs for bots.
	// This must be unique for the Swarming instance, as megadrone
	// does not use unique DUT hostnames.
	BotPrefix string
	// NumBots is the number of bots to run.
	NumBots int

	// logger is used for agent logging.  If nil, use the log package.
	logger logger
}

// A logger represents the logging interface used by the package.
type logger interface {
	Printf(string, ...interface{})
}

// Run runs the agent until it is canceled via the context.
func (a *Agent) Run(ctx context.Context) {
	a.log("Agent starting")
	b := botman.NewBotman(hook{a.droneStarter()})
	for i := range a.NumBots {
		id := a.botIDForIndex(i)
		b.AddBot(id)
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	stop := make(chan struct{})
	defer close(stop)
	wg.Add(2)
	go func() {
		defer wg.Done()
		select {
		case <-draining.C(ctx):
			b.BlockBots()
			b.DrainAll()
		case <-stop:
		}
	}()
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			b.BlockBots()
			b.TerminateAll()
		case <-stop:
		}
	}()
	b.Wait()
}

// botIDForIndex returns the bot ID for the given index.
// This is the ID that is passed into the bot starter and thus not the
// actual bot ID, as the bot starter will add some unique prefix.
func (a *Agent) botIDForIndex(i int) string {
	return fmt.Sprintf("bot%d", i)
}

// botConfig returns a bot config for starting a Swarming bot.
func (a *Agent) botConfig(botID string, workDir string) bot.Config {
	return bot.Config{
		BotID:         a.BotPrefix + botID,
		WorkDirectory: workDir,
	}
}

func (a *Agent) droneStarter() bot.DroneStarter {
	return bot.DroneStarter{
		WorkingDir:    a.WorkingDir,
		StartBotFunc:  a.StartBotFunc,
		BotConfigFunc: a.botConfig,
		LogFunc:       a.log,
	}
}

func (a *Agent) log(format string, args ...interface{}) {
	if v := a.logger; v != nil {
		v.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// A hook implements botman.WorldHook.
type hook struct {
	s bot.DroneStarter
}

// StartBot implements botman.WorldHook.
func (h hook) StartBot(id string) (bot.Bot, error) {
	return h.s.Start(id)
}

// ReleaseResources implements botman.WorldHook.
func (h hook) ReleaseResources(id string) {
	// TODO(ayatane): We should ideally clean up bot working dir,
	// but the regular agent implementation doesn't either.
}
