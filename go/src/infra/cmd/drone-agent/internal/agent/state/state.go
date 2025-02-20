// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package state implements the agent state.  This is a separate
// package to enforce access to a limited public API.
package state

import (
	"context"
	"time"

	"go.chromium.org/infra/cmd/drone-agent/internal/bot"
	"go.chromium.org/infra/cmd/drone-agent/internal/botman"
	"go.chromium.org/infra/cmd/drone-agent/internal/delay"
)

// State contains the agent state for the lifetime of one drone UUID
// assignment.
type State struct {
	uuid string
	*botman.Botman
	expireTimer *delay.Timer
}

// New creates a new instance of agent state.
func New(uuid string, h botman.WorldHook) *State {
	return &State{
		uuid:   uuid,
		Botman: botman.NewBotman(h),
	}
}

// UUID returns the drone UUID.
func (s *State) UUID() string {
	return s.uuid
}

// WithExpire sets up the delayable expiration context.
func (s *State) WithExpire(ctx context.Context, t time.Time) context.Context {
	t = t.Add(-bot.GraceInterval)
	ctx, s.expireTimer = delay.WithTimer(ctx, t)
	return ctx
}

// SetExpiration sets a new expiration time.  Note that if the
// expiration already fired, this does nothing.
func (s *State) SetExpiration(t time.Time) {
	s.expireTimer.Set(t)
}
