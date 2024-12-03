// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Command drone-agent is the client that talks to the drone queen
// service to provide Swarming bots for running tasks against test
// devices.  See the README.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/metadata"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"infra/appengine/drone-queen/api"
	"infra/cmd/drone-agent/internal/agent"
	"infra/cmd/drone-agent/internal/bot"
	"infra/cmd/drone-agent/internal/draining"
	"infra/cmd/drone-agent/internal/megadrone"
	"infra/cmd/drone-agent/internal/metrics"
	"infra/cmd/drone-agent/internal/tokman"
	"infra/cmd/drone-agent/internal/tracing"
	"infra/libs/otil"
)

const (
	drainingFile   = "drone-agent.drain"
	oauthTokenPath = "/var/lib/swarming/oauth_bot_token.json"
)

// Derived from standard environment variables.
// Don't add new settings as environment variables; use the config file.
var (
	authOptions = auth.Options{
		Method:                 auth.ServiceAccountMethod,
		ServiceAccountJSONPath: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
	}
	hostname         = os.Getenv("HOSTNAME")
	physicalHostname = os.Getenv("DOCKER_DRONE_SERVER_NAME")
)

// Deprecated configuration environment variables for backward compatibility.
// Add new settings to the config file.
// TODO(ayatane): Remove these.
var (
	queenService = os.Getenv("DRONE_AGENT_QUEEN_SERVICE")
	// DRONE_AGENT_SWARMING_URL is the URL of the Swarming
	// instance.  Should be a full URL without the path,
	// e.g. https://host.example.com
	swarmingURL       = os.Getenv("DRONE_AGENT_SWARMING_URL")
	dutCapacity       = getIntEnv("DRONE_AGENT_DUT_CAPACITY", 10)
	reportingInterval = time.Duration(getIntEnv("DRONE_AGENT_REPORTING_INTERVAL_MINS", 1)) * time.Minute

	// hive value of the drone agent.  This is used for DUT/drone affinity.
	// A drone is assigned DUTs with same hive value.
	hive = initializeHive(os.Getenv("DRONE_AGENT_HIVE"), physicalHostname)

	// tsmonEndpoint is the URL (including file://, https://,
	// pubsub://project/topic) to post monitoring metrics to.
	// If empty, we will try to load configuration from LUCI tsmon default
	// configuration file, i.e. /etc/chrome-infra/ts-mon.json.
	tsmonEndpoint       = os.Getenv("DRONE_AGENT_TSMON_ENDPOINT")
	tsmonCredentialPath = os.Getenv("DRONE_AGENT_TSMON_CREDENTIAL_PATH")

	// botPrefix is used as the prefix for the bot ID.
	// If DRONE_AGENT_BOT_PREFIX env is not set, then 'crossk-' will be used as default
	botPrefix = getEnv("DRONE_AGENT_BOT_PREFIX", "crossk-")

	// Bot compute resources settings.
	// Block IO throttle settings. 0 means no throttling. Only /dev/sda (device
	// number 8:0) is supported.
	botBlkIOReadBPS  = getIntEnv("DRONE_AGENT_BOT_BLKIO_READ_BPS", 0)
	botBlkIOWriteBPS = getIntEnv("DRONE_AGENT_BOT_BLKIO_WRITE_BPS", 0)
)

// Flag options.
// Only add flags for settings that would be hard coded into the drone image.
// Other settings should go into the config file.
var (
	// configPath is the path to the drone-agent config file.
	configPath = flag.String("config-path", "", "Path for config file.")
	// versionFilePath is the path to a drone-agent version file.
	// This file should only contain the version i.e. 12345.
	versionFilePath = flag.String("version-file", "", "Path for drone-agent version file."+
		" This is reported to drone queen for analytics.")
)

// Deprecated flag options for backward compatibility.
// TODO(ayatane): Remove these.
var (
	// traceBackend denotes the backend used for OTel traces.
	traceBackend string
	// traceTarget is the destination for traces.
	traceTarget = flag.String("trace-target", "", "Traces destination. "+
		"See \"trace-backend\" description for usage.")
)

func init() {
	const desc = `Exporter for OTel traces. Valid options are "grpc" and "none".
For values other than "none", -trace-target must be set.
For "grpc", the format is "host:port" for an OTel collector service.
(default "none")`
	flag.Func("trace-backend", desc, func(s string) error {
		switch s {
		case "grpc", "none":
			traceBackend = s
			return nil
		default:
			return errors.Reason("invalid value %s. Allowed values are: %s", s, "grpc, none").Err()
		}
	})
}

func main() {
	flag.Parse()
	if err := innerMain(); err != nil {
		log.Fatal(err)
	}
}

func innerMain() error {
	cfg := parseConfigFile(*configPath)
	// Set up and defer the WaitGroup before the context because
	// the context cancellation needs to happen first to signal
	// things to stop.  Otherwise we deadlock waiting for things
	// to stop before signaling them to stop.
	var wg sync.WaitGroup
	defer wg.Wait()

	version := readVersionFile(*versionFilePath)
	log.Printf("drone-agent-version from file: %v", version)

	ctx, cf, err := setupContext(version, cfg)
	defer cf()
	if err != nil {
		return err
	}

	if err := metrics.Setup(ctx, cfg.TSMonEndpoint, cfg.TSMonCredentialPath); err != nil {
		log.Printf("Skipping metrics setup: %s", err)
	}
	defer metrics.Shutdown(ctx)

	tp, cf := setupTracing(version)
	defer cf()
	if cfg.OTLPExporterAddr != "" {
		exp, err := tracing.NewGRPCExporter(ctx, cfg.OTLPExporterAddr)
		if err != nil {
			return err
		}
		tp.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(exp))
	}

	h, err := setupAuthClient(ctx, &wg)
	if err != nil {
		return err
	}

	if cfg.EnableMegadrone {
		a := megadrone.Agent{
			WorkingDir:   cfg.WorkingDirPath,
			StartBotFunc: bot.NewStarter(h, cfg.SwarmingURL).Start,
			BotPrefix:    megadronePrefix(cfg, hostname),
			NumBots:      cfg.NumBots,
		}
		a.Run(ctx)
	} else {
		a := agent.Agent{
			Client: api.NewDronePRPCClient(&prpc.Client{
				C:    h,
				Host: cfg.QueenService,
			}),
			WorkingDir:        cfg.WorkingDirPath,
			ReportingInterval: cfg.ReportingInterval(),
			DUTCapacity:       cfg.DUTCapacity,
			StartBotFunc:      bot.NewStarter(h, cfg.SwarmingURL).Start,
			Hive:              cfg.Hive,
			BotPrefix:         cfg.BotPrefix,
			BotResources:      makeBotResources(cfg),
			PythonVersion:     cfg.PythonVersion,
		}
		a.Run(ctx)
	}
	return nil
}

// setupContext sets up global context for main.
//
// The caller must defer/call the cleanup even if an error is returned.
func setupContext(version string, cfg *config) (_ context.Context, cleanup func(), _ error) {
	var ds deferStack

	// Set up top level context and cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	ds.add(cancel)
	ctx, cancel = notifySIGTERM(ctx)
	ds.add(cancel)
	ctx = notifyDraining(ctx, filepath.Join(cfg.WorkingDirPath, drainingFile))
	if err := os.MkdirAll(cfg.WorkingDirPath, 0777); err != nil {
		return ctx, ds.run, err
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "drone-agent-version", version)
	return ctx, ds.run, nil
}

func setupTracing(version string) (_ *sdktrace.TracerProvider, cleanup func()) {
	p := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{})
	otel.SetTextMapPropagator(p)
	tp := tracing.NewTracerProvider(version)
	otel.SetTracerProvider(tp)
	return tp, func() {
		// Use a separate context as the main context would
		// already be canceled to trigger drone-agent
		// shutdown.
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Failed to shutdown tracer provider: %v", err)
		}
	}
}

func setupAuthClient(ctx context.Context, wg *sync.WaitGroup) (*http.Client, error) {
	authn := auth.NewAuthenticator(ctx, auth.SilentLogin, authOptions)
	r, err := tokman.Make(authn, oauthTokenPath, time.Minute)
	if err != nil {
		return nil, err
	}
	wg.Add(1)
	go func() {
		r.KeepNew(ctx)
		wg.Done()
	}()
	h, err := authn.Client()
	if err != nil {
		return nil, err
	}
	otil.AddHTTP(h)
	return h, nil
}

// readVersionFile reads drone agent version from a given version file.
func readVersionFile(versionFilePath string) string {
	const fallback = "unknown"
	if versionFilePath == "" {
		log.Println("no path to version file provided")
		return fallback
	}
	fileContent, err := os.ReadFile(versionFilePath)
	if err != nil {
		log.Printf("cannot read version file: %v", err)
		return fallback
	}
	version := string(fileContent)
	// Simple validation for now, to check that the version string only contains numbers.
	if _, err := strconv.Atoi(version); err != nil {
		log.Printf("illegal version string passed, version should only contain numbers")
		return fallback
	}
	return version
}

const checkDrainingInterval = time.Minute

// notifyDraining returns a context that is marked as draining when a
// file exists at the given path.
func notifyDraining(ctx context.Context, path string) context.Context {
	ctx, drain := draining.WithDraining(ctx)
	_, err := os.Stat(path)
	if err == nil {
		drain()
		return ctx
	}
	go func() {
		for {
			time.Sleep(checkDrainingInterval)
			_, err := os.Stat(path)
			if err == nil {
				drain()
				return
			}
		}
	}()
	return ctx
}

// getIntEnv gets an int value from an environment variable.  If the
// environment variable is not valid or is not set, use the default value.
func getIntEnv(key string, defaultValue int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("Invalid %s, using default value (error: %v)", key, err)
		return defaultValue
	}
	return n
}

// getEnv gets a string value from an environment variable.  If the
// environment variable not set, use the default value.
// If the environment variable is set with enplty value, the function
// will return empty.
func getEnv(key string, defaultValue string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	return v
}

// dcLabRegex is the regular expression to identify the Drone server is in a
// data center like lab, e.g. SFO36, in which the server name is like
// 'kube<N>-<SITE>'. If matched, we use the part of '<SITE>' as the hive.
var dcLabRegex = regexp.MustCompile(`^kube[0-9]+-([a-z]+)`)

// initializeHive returns the hive for the agent.
// If hive is not specified, we try to guess it from the hostname.
// The input args are from some envvars, but we don't get them from inside
// the function, so we can keep all code using envvars in a single code block at
// the head of this file for better readability.
func initializeHive(explicitHive, hostname string) string {
	if explicitHive != "" {
		return explicitHive
	}
	log.Printf("Hive not explicitly specified, now guess it by hostname %q", hostname)
	if m := dcLabRegex.FindStringSubmatch(hostname); m != nil {
		return m[1]
	}
	return ""
}

// makeBotResources returns a struct which defines the resources assigned to
// each bot.
func makeBotResources(cfg *config) *specs.LinuxResources {
	// 8 and 0 is major/minor device number of /dev/sda mounted to
	// drone containers. So far I don't see any other number than it.
	var diskMajor int64 = 8
	var diskMinor int64 = 0

	var spec specs.LinuxBlockIO
	if rate := uint64(cfg.BotBlockIOReadBPS); rate > 0 {
		spec.ThrottleReadBpsDevice = []specs.LinuxThrottleDevice{*newThrottleDevice(diskMajor, diskMinor, rate)}
	}
	if rate := uint64(cfg.BotBlockIOWriteBPS); rate > 0 {
		spec.ThrottleWriteBpsDevice = []specs.LinuxThrottleDevice{*newThrottleDevice(diskMajor, diskMinor, rate)}
	}
	return &specs.LinuxResources{
		BlockIO: &spec,
	}
}

// newThrottleDevice returns a new instance of LinuxThrottleDevice.
func newThrottleDevice(major, minor int64, rate uint64) *specs.LinuxThrottleDevice {
	// We cannot use struct literals to initialize this struct because "Major"
	// and "Minor" belong to a nested unexported struct. It has been fixed in
	// the upstream repo
	// https://github.com/opencontainers/runtime-spec/commit/84251a48404b19a99cc1b4a8f00c5b523e0d22d0
	// but is not included in the latest release (v1.0.2) yet.
	// TODO(guocb): initialize with struct literals when a newer release is
	// available.
	dev := specs.LinuxThrottleDevice{Rate: rate}
	dev.Major = major
	dev.Minor = minor
	return &dev
}

func megadronePrefix(cfg *config, hostname string) string {
	return "crossk-megadrone-" + hostname + "-"
}

// A deferStack reifies a stack of defers to pass around.
// Not concurrency safe (intended for a single goroutine).
type deferStack struct {
	// Stack of deferred funcs.
	// Newer funcs are added to the end.
	f []func()
}

func (s *deferStack) add(f func()) {
	s.f = append(s.f, f)
}

func (s *deferStack) run() {
	for i := len(s.f) - 1; i >= 0; i-- {
		s.f[i]()
	}
}
