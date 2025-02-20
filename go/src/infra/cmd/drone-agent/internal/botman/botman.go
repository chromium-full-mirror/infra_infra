// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package botman provides a bot manager that ensures that designated
// Swarming bots are running, restarting them if necessary.
package botman

import (
	"log"
	"sync"

	"go.chromium.org/infra/cmd/drone-agent/internal/bot"
)

// WorldHook defines the interface that a Botman uses to
// interact with the external world.
type WorldHook interface {
	// StartBot starts a bot process for the given ID.
	// This method should be safe to call concurrently.
	StartBot(id string) (bot.Bot, error)
	// ReleaseResources is called to release resources for a bot process
	// that has finished.  This method should be idempotent.
	ReleaseResources(id string)
}

// Botman manages running Swarming bots.  Callers tell Botman what
// bots to add, drain, or terminate using an ID, and Botman makes sure
// there are bots running or not running for those IDs.  IDs may refer
// to resources such as DUTs or some arbitrary index of bots to run.
type Botman struct {
	hook WorldHook
	wg   sync.WaitGroup

	// The following fields are covered by the mutex.
	m       sync.Mutex
	blocked bool
	bots    map[string]botSignals
}

// NewBotman creates a new Botman.
func NewBotman(h WorldHook) *Botman {
	b := &Botman{
		hook: h,
		bots: make(map[string]botSignals),
	}
	return b
}

// AddBot adds a bot to the Botman.
// The controller ensures that an instance Swarming bot is running for the given resource ID.
// If the bot was already added or if the controller is blocked, do nothing.
// This method is concurrency safe.
func (b *Botman) AddBot(id string) {
	b.m.Lock()
	defer b.m.Unlock()
	if b.blocked {
		return
	}
	if _, ok := b.bots[id]; ok {
		// ID already has bot running.
		return
	}
	log.Printf("Starting new bot for ID %v", id)
	s := newBotSignals()
	b.bots[id] = s
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		runBotForID(b.hook, id, s)
		b.m.Lock()
		delete(b.bots, id)
		b.m.Unlock()
	}()
}

// runBotForID keeps a Swarming bot running for the ID.
// Signals to drain or terminate should be sent using botSignals.
// This function otherwise runs forever.
func runBotForID(h WorldHook, id string, s botSignals) {
	defer h.ReleaseResources(id)
	for {
		select {
		case <-s.drain:
			return
		case <-s.terminate:
			return
		default:
		}
		b, err := h.StartBot(id)
		if err != nil {
			log.Printf("Fail to start bot %s %s", id, err)
			continue
		}
		wait := make(chan struct{})
		go func() {
			_ = b.Wait()
			close(wait)
		}()
		var stop bool
	listenForSignals:
		for {
			select {
			case <-s.drain:
				_ = b.Drain()
				stop = true
			case <-s.terminate:
				if err = b.TerminateOrKill(); err != nil {
					log.Printf("Failed to terminate or kill bot: %s", err)
				}
				stop = true
			case <-wait:
				break listenForSignals
			}
		}
		if stop {
			return
		}
	}
}

// DrainBot removes an ID to no longer have bots running for it and
// drains its current bot.
// This method can be called repeatedly.
// If the controller does not have the ID, just call ReleaseResources on
// the controller's hook.
// This method is concurrency safe.
func (b *Botman) DrainBot(id string) {
	b.m.Lock()
	s, ok := b.bots[id]
	b.m.Unlock()
	if ok {
		log.Printf("Draining Bot with ID %v", id)
		s.sendDrain()
	} else {
		b.hook.ReleaseResources(id)
	}
}

// TerminateBot removes an ID to no longer have bots running for it
// and terminates its current bot.
// This method can be called repeatedly.
// If the controller does not have the ID, just call ReleaseResources on
// the controller's hook.
// This method is concurrency safe.
func (b *Botman) TerminateBot(id string) {
	b.m.Lock()
	s, ok := b.bots[id]
	b.m.Unlock()
	if ok {
		log.Printf("Terminating Bot with ID %v", id)
		s.sendTerminate()
	} else {
		b.hook.ReleaseResources(id)
	}
}

// DrainAll drains all Bots.
// You almost certainly want to call BlockBots first to make sure Bots
// don't get added right after calling this.
func (b *Botman) DrainAll() {
	b.m.Lock()
	for _, s := range b.bots {
		s.sendDrain()
	}
	b.m.Unlock()
}

// TerminateAll terminates all Bots.
// You almost certainly want to call BlockBots first to make sure Bots
// don't get added right after calling this.
func (b *Botman) TerminateAll() {
	b.m.Lock()
	for _, s := range b.bots {
		s.sendTerminate()
	}
	b.m.Unlock()
}

// BlockBots marks the controller to not accept new Bots.
// This method is safe to call concurrently.
func (b *Botman) BlockBots() {
	b.m.Lock()
	b.blocked = true
	b.m.Unlock()
}

// ActiveBots returns a slice of all Bots the controller is keeping alive.
// This includes Bots that are draining or terminated but not exited yet.
// This method is safe to call concurrently.
func (b *Botman) ActiveBots() []string {
	var ds []string
	b.m.Lock()
	for d := range b.bots {
		ds = append(ds, d)
	}
	b.m.Unlock()
	return ds
}

// Wait for all Swarming bots to finish.  It is the caller's
// responsibility to make sure all bots are terminated or drained,
// else this call will hang.
func (b *Botman) Wait() {
	b.wg.Wait()
}

type botSignals struct {
	drain     chan struct{}
	terminate chan struct{}
}

func newBotSignals() botSignals {
	return botSignals{
		drain:     make(chan struct{}, 1),
		terminate: make(chan struct{}, 1),
	}
}

func (s botSignals) sendDrain() {
	select {
	case s.drain <- struct{}{}:
	default:
	}
}

func (s botSignals) sendTerminate() {
	select {
	case s.terminate <- struct{}{}:
	default:
	}
}
