// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package agent implements an agent which talks to a drone queen
// service and manages Swarming bots.
package agent

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/opencontainers/runtime-spec/specs-go"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/appengine/drone-queen/api"
	"go.chromium.org/infra/cmd/drone-agent/internal/agent/state"
	"go.chromium.org/infra/cmd/drone-agent/internal/bot"
	"go.chromium.org/infra/cmd/drone-agent/internal/draining"
	"go.chromium.org/infra/libs/otil"
)

// Agent talks to a drone queen service and manages Swarming bots.
// This struct stores the static configuration for the agent.  The
// dynamic state is stored in state.State.
type Agent struct {
	Client api.DroneClient
	// WorkingDir is used for Swarming bot working dirs.  It is
	// the caller's responsibility to create this.
	WorkingDir        string
	ReportingInterval time.Duration
	DUTCapacity       int
	// StartBotFunc is used to start Swarming bot processes.
	// This must be set.
	StartBotFunc func(bot.Config) (bot.Bot, error)
	// Hive value of the drone agent.  This is used for DUT/drone affinity.
	// A drone is assigned DUTs with same hive value.
	Hive string
	// BotPrefix is used to prefix hostnames for bots.
	BotPrefix string
	// BotResources is the compute resources (CPU, RAM, disk I/O etc.) assigned
	// to each bot.
	BotResources *specs.LinuxResources

	// logger is used for Agent logging.  If nil, use the log package.
	logger logger
	// wrapStateFunc is called to wrap the agent state.  This is
	// used for instrumenting the state for testing.  If nil, this
	// is a no-op.
	wrapStateFunc func(*state.State) stateInterface
	// Python version used for swarming bot
	PythonVersion string
}

// logger defines the logging interface used by Agent.
type logger interface {
	Printf(string, ...any)
}

// stateInterface is the state interface used by the agent.  The usual
// implementation of the interface is in the state package.
type stateInterface interface {
	UUID() string
	WithExpire(ctx context.Context, t time.Time) context.Context
	SetExpiration(t time.Time)
	botmanInterface
}

// botmanInterface is the bot management interface used by the agent.
// The usual implementation is in the botman package.
// This is only used as embedded in stateInterface, but we separate it
// for ease of searching/reading.
type botmanInterface interface {
	AddBot(dutID string)
	DrainBot(dutID string)
	TerminateBot(dutID string)
	DrainAll()
	TerminateAll()
	Wait()
	BlockBots()
	ActiveBots() []string
}

// Run runs the agent until it is canceled via the context.
func (a *Agent) Run(ctx context.Context) {
	a.log("Agent starting")
	for {
		if draining.IsDraining(ctx) || ctx.Err() != nil {
			a.log("Agent exited")
			return
		}
		if err := a.runOnce(ctx); err != nil {
			a.log("Lost drone assignment: %v", err)
		}
	}
}

// runOnce runs one instance of registering and maintaining a drone
// assignment with the queen.
//
// If the context is canceled, this function terminates quickly and
// gracefully (e.g., like handling a SIGTERM or an abort).  If the
// context is drained, this function terminates slowly and gracefully.
// In either case, this function returns nil.
//
// If the assignment is lost or expired for whatever reason, this
// function returns an error.
func (a *Agent) runOnce(ctx context.Context) (err error) {
	ctx, s, err := a.registerWithQueen(ctx)
	if err != nil {
		return err
	}
	return a.reportLoop(ctx, s)
}

// register does the initial registration with the queen, before the
// core reporting loop.
func (a *Agent) registerWithQueen(ctx context.Context) (_ context.Context, _ stateInterface, err error) {
	// The tracing context is only used inside this function and
	// should not be returned to the caller.
	ctx2, span := otil.FuncSpan(ctx)
	defer func() { otil.EndSpan(span, err) }()
	a.log("Registering with queen")
	res, err := a.Client.ReportDrone(ctx2, a.reportRequest(ctx2, ""))
	if err != nil {
		return ctx, nil, errors.Annotate(err, "register with queen").Err()
	}
	if s := res.GetStatus(); s != api.ReportDroneResponse_OK {
		// TODO(ayatane): We should handle the potential unknown UUID error specially.
		return ctx, nil, errors.Reason("register with queen: got unexpected status %v", s).Err()
	}

	// Set up state.
	uuid := res.GetDroneUuid()
	if uuid == "" {
		return ctx, nil, errors.Reason("register with queen: got empty UUID").Err()
	}
	a.log("UUID assigned: %s", uuid)
	s := a.wrapState(state.New(uuid, hook{
		botStarter: a.droneStarter(),
		c:          a.Client,
		logFunc:    a.log,
		uuid:       uuid,
	}))

	// Set up expiration context.
	t, err := ptypes.Timestamp(res.GetExpirationTime())
	if err != nil {
		return ctx, nil, errors.Annotate(err, "register with queen: read expiration").Err()
	}
	ctx = s.WithExpire(ctx, t)

	// Do normal report update.
	if err := applyUpdateToState(res, s); err != nil {
		return ctx, nil, errors.Annotate(err, "register with queen").Err()
	}
	return ctx, s, nil
}

// reportLoop implements the core reporting loop of the agent.
// See also runOnce.
func (a *Agent) reportLoop(ctx context.Context, s stateInterface) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	defer wg.Wait()
	readyToExit := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			a.log("Terminating all DUTs due to expired context")
			s.BlockBots()
			s.TerminateAll()
		case <-readyToExit:
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-draining.C(ctx):
			a.log("Draining all DUTs")
			s.BlockBots()
			s.DrainAll()
		case <-readyToExit:
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-draining.C(ctx):
		case <-ctx.Done():
		}
		s.BlockBots()
		s.Wait()
		close(readyToExit)
	}()

	for {
		select {
		case <-time.After(a.ReportingInterval):
		case <-readyToExit:
			return nil
		}
		a.log("Reporting to queen")
		if err := a.reportDrone(ctx, s); err != nil {
			if _, ok := err.(fatalError); ok {
				a.log("Terminating due to fatal error: %s", err)
				cancel()
				return err
			}
			a.log("Error reporting to queen: %s", err)
		}
	}
}

// reportDrone does one cycle of calling the ReportDrone queen RPC and
// handling the response.
func (a *Agent) reportDrone(ctx context.Context, s stateInterface) (err error) {
	ctx, span := otil.FuncSpan(ctx)
	defer func() { otil.EndSpan(span, err) }()
	res, err := a.Client.ReportDrone(ctx, a.reportRequest(ctx, s.UUID()))
	if err != nil {
		return errors.Annotate(err, "report to queen").Err()
	}
	switch rs := res.GetStatus(); rs {
	case api.ReportDroneResponse_OK:
	case api.ReportDroneResponse_UNKNOWN_UUID:
		s.TerminateAll()
		return fatalError{reason: "queen returned UNKNOWN_UUID"}
	default:
		return errors.Reason("report to queen: got unexpected status %v", rs).Err()
	}
	if err := applyUpdateToState(res, s); err != nil {
		return errors.Annotate(err, "report to queen").Err()
	}
	return nil
}

// applyUpdateToState applies the response from a ReportDrone call to the agent state.
func applyUpdateToState(res *api.ReportDroneResponse, s stateInterface) error {
	t, err := ptypes.Timestamp(res.GetExpirationTime())
	if err != nil {
		return errors.Annotate(err, "apply update to state").Err()
	}
	s.SetExpiration(t)
	draining := make(map[string]bool)
	for _, d := range res.GetDrainingDuts() {
		s.DrainBot(d)
		draining[d] = true
	}
	assigned := make(map[string]bool)
	for _, d := range res.GetAssignedDuts() {
		assigned[d] = true
		if !draining[d] {
			s.AddBot(d)
		}
	}
	for _, d := range s.ActiveBots() {
		if !assigned[d] {
			s.TerminateBot(d)
		}
	}
	return nil
}

// reportRequest returns the api.ReportDroneRequest to use when
// reporting to the drone queen.
func (a *Agent) reportRequest(ctx context.Context, uuid string) *api.ReportDroneRequest {
	hostname, err := os.Hostname()
	if err != nil {
		a.log("Error getting drone hostname: %s", err)
	}

	req := api.ReportDroneRequest{
		DroneUuid: uuid,
		LoadIndicators: &api.ReportDroneRequest_LoadIndicators{
			DutCapacity: intToUint32(a.DUTCapacity),
		},
		DroneDescription: hostname,
		Hive:             a.Hive,
	}
	if shouldRefuseNewDUTs(ctx) {
		req.LoadIndicators.DutCapacity = 0
	}
	return &req
}

// shouldRefuseNewDUTs returns true if we should refuse new DUTs.
func shouldRefuseNewDUTs(ctx context.Context) bool {
	return draining.IsDraining(ctx) || ctx.Err() != nil
}

func (a *Agent) log(format string, args ...any) {
	if v := a.logger; v != nil {
		v.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func (a *Agent) wrapState(s *state.State) stateInterface {
	if a.wrapStateFunc == nil {
		return s
	}
	return a.wrapStateFunc(s)
}

// botConfig returns a bot config for starting a Swarming bot.
func (a *Agent) botConfig(botID string, workDir string) bot.Config {
	return bot.Config{
		BotID:         a.BotPrefix + botID,
		WorkDirectory: workDir,
		Resources:     a.BotResources,
		PythonVersion: a.PythonVersion,
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

// hook implements botman.WorldHook.
type hook struct {
	botStarter interface {
		Start(botID string) (bot.Bot, error)
	}
	c       api.DroneClient
	logFunc func(string, ...any)
	uuid    string
}

// StartBot implements state.ControllerHook.
func (h hook) StartBot(dutID string) (bot.Bot, error) {
	return h.botStarter.Start(dutID)
}

// ReleaseResources implements botman.WorldHook.
func (h hook) ReleaseResources(dutID string) {
	const releaseDUTsTimeout = time.Minute
	ctx := context.Background()
	ctx, f := context.WithTimeout(ctx, releaseDUTsTimeout)
	defer f()
	req := api.ReleaseDutsRequest{
		DroneUuid: h.uuid,
		Duts:      []string{dutID},
	}
	// Releasing DUTs is best-effort.  Ignore any errors since
	// there's no way to handle them.
	h.logFunc("Releasing %s", dutID)
	_, _ = h.c.ReleaseDuts(ctx, &req)
}

// fatalError indicates that the agent should terminate its current
// UUID assignment session and re-register with the queen.
type fatalError struct {
	reason string
}

func (e fatalError) Error() string {
	return fmt.Sprintf("agent fatal error: %s", e.reason)
}

// intToUint32 converts an int to a uint32.
// If the value is negative, return 0.
// If the value overflows, return the max value.
func intToUint32(a int) uint32 {
	if a < 0 {
		return 0
	}
	if a > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(a)
}
