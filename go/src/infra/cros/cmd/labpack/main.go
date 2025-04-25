// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// The labpack program allows to run repair tasks for ChromeOS devices in the lab.
// For more information please read go/paris-.
// Managed by Chrome Fleet Software (go/chrome-fleet-software).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"

	lab "go.chromium.org/chromiumos/infra/proto/go/lab"
	luciauth "go.chromium.org/luci/auth"
	"go.chromium.org/luci/buildbucket/protoutil"
	"go.chromium.org/luci/common/errors"
	lucigs "go.chromium.org/luci/common/gcloud/gs"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/labpack/internal/site"
	"go.chromium.org/infra/cros/cmd/labpack/internal/tlw"
	kclient "go.chromium.org/infra/cros/karte/client"
	"go.chromium.org/infra/cros/recovery"
	"go.chromium.org/infra/cros/recovery/karte"
	"go.chromium.org/infra/cros/recovery/logger"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	"go.chromium.org/infra/cros/recovery/upload"
	dm "go.chromium.org/infra/device_manager/client"
	"go.chromium.org/infra/libs/skylab/buildbucket"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// DescribeMyDirectoryAndEnvironment controls whether labpack should write information
// about where it was run (cwd), what files are near it, and the contents of the environment.
const DescribeMyDirectoryAndEnvironment = true

// DescriptionCommand describes the environment where labpack was run. It must write all of its output to stdout.
const DescriptionCommand = `( echo BEGIN; echo PWD; pwd ; echo FIND; find . ; echo ENV; env; echo END )`

var ioProps = build.RegisterSplitProperty[*lab.LabpackInput, *lab.LabpackResponse]("")

func main() {
	log.SetPrefix(fmt.Sprintf("%s: ", filepath.Base(os.Args[0])))
	log.Printf("Running version: %s", site.VersionNumber)
	log.Printf("Running in buildbucket mode")

	build.Main(
		func(ctx context.Context, args []string, state *build.State) error {
			// Right after instantiating the logger, but inside build.Main's callback,
			// make sure that we log what our environment looks like.
			if DescribeMyDirectoryAndEnvironment {
				describeEnvironment(os.Stderr)
				// Describe the contents of the directory once on the way out too.
				// We will use this information to decide what to persist.
				defer describeEnvironment(os.Stderr)
			}

			// Set the log (via the Go standard library's log package) to Stderr, since we know that stderr is collected
			// for the process as a whole.
			log.SetOutput(os.Stderr)

			// We need more logging in order to fix some data gaps (like the lack of a buildbucket ID).
			// Log the input string as JSON so we can see exactly which fields are populated with what in prod.
			b := protojson.MarshalOptions{
				Indent: "  ",
			}.Format(ioProps.GetInput(ctx))
			log.Printf("%s\n", string(b))

			// Set up logger.
			logRoot, err := getTaskDir()
			if err != nil {
				return errors.Annotate(err, "main run").Err()
			}
			// TODO: Change level to Info when all logging files will be upload to GC.
			ctx, lg, err := createLogger(ctx, logRoot, logging.Debug)
			if err != nil {
				return errors.Annotate(err, "main run").Err()
			}
			defer func() { lg.Close() }()

			// Synchronously try to reach out to device manager
			// to confirm that we still have a lease by extending it by zero seconds.
			//
			// If we have an invalid lease, then exit.
			// If no lease was specified at all, do nothing.
			//
			// See b/398915101 for more information.
			pool, ok := getPool(state)
			if !ok {
				return errors.New("early check: couldn't find the device's label-pool dimension")
			}
			leaseID := getLeaseID(state)
			if leaseID != "" {
				if err := checkLeaseAlreadyExpired(ctx, lg, leaseID, pool, nil); err != nil {
					return errors.Annotate(err, "main run").Err()
				}
			}

			ctx, cancel := context.WithCancel(ctx)
			eg, ctx := errgroup.WithContext(ctx)
			eg.Go(func() error {
				defer cancel()
				return mainRunInternal(ctx, logRoot, lg, ioProps.GetInput(ctx), state)
			})
			eg.Go(func() error {
				return watchDMLease(ctx, lg, leaseID, pool)
			})
			err = eg.Wait()
			return errors.Annotate(err, "main").Err()
		},
	)
	log.Printf("Labpack done!")
}

// mainRun runs function for BB and provide result.
func mainRunInternal(ctx context.Context, logRoot string, lg logger.Logger, input *lab.LabpackInput, state *build.State) error {
	// Result errors which specify the result of main run.
	var resultErrors []error

	// Run recovery lib and get response.
	// Set result as fail by default in case it fail to finish by some reason.
	res := &lab.LabpackResponse{
		Success:    false,
		FailReason: "Fail by unknown reason!",
	}
	defer func() {
		// Write result as last step.
		ioProps.SetOutput(ctx, res)
	}()
	lg.Infof("Prepare print input params...")
	if err := printInputs(ctx, input); err != nil {
		lg.Debugf("main run internal: failed to marshal proto. Error: %s", err)
		return err
	}
	ad := &tlw.AccessData{
		TaskTags: make(map[string]string),
	}

	// Update input with default values.
	// If any identifier is provided by the client, we use it as is.
	if input.GetSwarmingTaskId() == "" && input.GetBbid() == "" {
		input.SwarmingTaskId = state.Build().GetInfra().GetBackend().GetTask().GetId().GetId()
		if input.SwarmingTaskId == "" {
			// Fall back to build.Infra.Swarming.TaskId.
			// TODO(b/40949135): remove this after the swarming -> backend migration
			// completes.
			input.SwarmingTaskId = state.Build().GetInfra().GetSwarming().GetTaskId()
		}
		if bbid := state.Build().GetId(); bbid > int64(0) {
			input.Bbid = fmt.Sprintf("%d", bbid)
		}
		for _, p := range state.Build().GetTags() {
			lg.Debugf("Swarming tag: %q=%q found!", p.GetKey(), p.GetValue())
			if p.GetKey() != "" && p.GetValue() != "" {
				ad.TaskTags[p.GetKey()] = p.GetValue()
			}
		}
	}
	lg.Infof("Prepare inventory namespace...")
	if input.GetInventoryNamespace() == "" {
		input.InventoryNamespace = ufsUtil.OSNamespace
	}
	lg.Infof("Using inventory namespace: %q", input.GetInventoryNamespace())
	ctx = setupContextNamespace(ctx, input.GetInventoryNamespace())
	useMetrics := !input.GetNoMetrics()
	isOSPartnerNamespace := input.GetInventoryNamespace() == ufsUtil.OSPartnerNamespace
	if useMetrics && isOSPartnerNamespace {
		// Partners do not have access for metrics service.
		useMetrics = false
	}
	var metrics metrics.Metrics
	if useMetrics {
		lg.Infof("Prepare create Karte client...")
		var err error
		metrics, err = karte.NewMetrics(ctx, kclient.ProdConfig(luciauth.Options{}))
		if err == nil {
			lg.Infof("Karte client successfully created.")
		} else {
			lg.Errorf("Failed to instantiate Karte client: %s", err)
			resultErrors = append(resultErrors, err)
		}
	}
	lg.Infof("Starting task execution...")
	if err := internalRun(ctx, input, metrics, lg, ad, logRoot); err != nil {
		res.Success = false
		res.FailReason = err.Error()
		resultErrors = append(resultErrors, err)
	}
	lg.Infof("Finished task execution.")
	// Partners do not have access to karte service.
	if !isOSPartnerNamespace {
		lg.Infof("Starting uploading logs...")
		if err := uploadLogs(ctx, input, lg); err != nil {
			res.Success = false
			if len(resultErrors) == 0 {
				// We should not override runerror reason as it more important.
				// If upload logs error is only exits then set it as reason.
				res.FailReason = err.Error()
			}
			resultErrors = append(resultErrors, err)
		}
		lg.Infof("Finished uploading logs.")
	}
	// if err is nil then will marked as SUCCESS
	if len(resultErrors) == 0 {
		// Reset reason and state as no errors detected.
		res.Success = true
		res.FailReason = ""
		return nil
	}
	return errors.Annotate(errors.MultiError(resultErrors), "run recovery").Err()
}

// getLeaseID gets the ID of the current lease from the build state.
// Returns "" if and only if there is no lease, legitimately.
//
// If it looks like a lease should have been there, but is not, we will panic.
// This case will only happen if something is seriously wrong further up the stack.
func getLeaseID(state *build.State) string {
	leaseIDStructVal, ok := state.Build().GetInput().GetProperties().GetFields()["device_manager_lease_id"]
	if !ok {
		return ""
	}
	out := leaseIDStructVal.GetStringValue()
	if out == "" {
		panic("Lease is empty but device_manager_lease_id field is present. This can only happen if something is seriously wrong with how labpack was called.")
	}
	return out
}

// checkLeaseAlreadyExpired checks if the lease has already expired. It is synchronous and intended to be run
// before doing any significant repair actions.
//
// Only set the extenderOverride to override the default implementation during testing.
func checkLeaseAlreadyExpired(
	ctx context.Context,
	lg logger.Logger,
	leaseID string,
	pool string,
	extenderOverride func(context.Context, string, time.Duration) (time.Time, error),
) error {
	if leaseID == "" {
		return errors.New("lease id cannot be empty")
	}
	if pool == "" {
		return errors.New("pool cannot be empty")
	}
	var extender func(context.Context, string, time.Duration) (time.Time, error)
	switch {
	case extenderOverride != nil:
		extender = extenderOverride
	default:
		dmc, err := dm.NewClient(ctx, pool)
		if err != nil {
			err = errors.Annotate(err, "early check: connecting to Device Manager").Err()
			lg.Infof(err.Error())
			return err
		}
		extender = dmc.Extend
	}

	_, err := extender(ctx, leaseID, 0)
	return errors.Annotate(err, "early check").Err()
}

// watchDMLease watches the Device Manager lease associated with this build for
// the duration of the build (if a lease exists), and cancels it when the given
// context is cancelled or an error is encountered.
func watchDMLease(ctx context.Context, lg logger.Logger, leaseID string, pool string) error {
	if pool == "" {
		lg.Infof("no pool")
		return nil
	}
	if leaseID == "" {
		lg.Infof("no lease")
		return nil
	}
	// Only watch the DM lease if one has been created for this build.
	dmc, err := dm.NewClient(ctx, pool)
	if err != nil {
		err = errors.Annotate(err, "watching DM lease: connecting to Device Manager").Err()
		lg.Infof(err.Error())
		return err
	}

	// Only cancel the lease with a fresh context in case the existing context has
	// been cancelled.
	defer func() {
		err := releaseDMLease(context.Background(), lg, leaseID, pool)
		if err != nil {
			err = errors.Annotate(err, "attempting to release DM lease").Err()
			lg.Errorf(err.Error())
		}
	}()

	// Renew the lease every few minutes on a loop until ctx is cancelled.
	lastLeaseExtensionTime := time.Now()
	loopSleepInterval := 5 * time.Second
	for {
		if ctx.Err() != nil {
			break
		}

		if time.Since(lastLeaseExtensionTime) >= dm.LeaseExtensionInterval {
			_, err = dmc.Extend(ctx, leaseID, dm.LeaseExtensionAmount)
			if err != nil {
				err = errors.Annotate(err, "watching DM lease: sending lease extension request to DM").Err()
				lg.Infof(err.Error())

				// See b:396467394 for details. Control must leave watchDMLease if we make it here,
				// but the context expiring should be forgiven and shouldn't propagate up to a user-visible error.
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil
				} else {
					return err
				}
			}
			lastLeaseExtensionTime = time.Now()
		}

		time.Sleep(loopSleepInterval)
	}

	return nil
}

// releaseDMLease releases the given Device Manager lease.
func releaseDMLease(ctx context.Context, lg logger.Logger, leaseID, pool string) error {
	dmc, err := dm.NewClient(ctx, pool)
	if err != nil {
		err = errors.Annotate(err, "releasing DM lease: connecting to Device Manager").Err()
		lg.Infof(err.Error())
		return err
	}
	err = dmc.Release(ctx, leaseID)
	if err != nil {
		err = errors.Annotate(err, "releasing DM lease: sending lease cancellation request to DM").Err()
		lg.Infof(err.Error())
		return err
	}

	lg.Infof("released DM lease %s", leaseID)
	return nil
}

// getPool returns the pool for the DUT that the given build state is running
// on, or returns false if it was not found.
func getPool(s *build.State) (string, bool) {
	for _, d := range protoutil.MustBotDimensions(s.Build()) {
		if d.GetKey() == "label-pool" {
			v := d.GetValue()
			if v == "" {
				return "", false
			}
			return v, true
		}
	}
	return "", false
}

// getTag gets the value for the given Swarming tag

// Upload logs to google cloud.
func uploadLogs(ctx context.Context, input *lab.LabpackInput, lg logger.Logger) (rErr error) {
	step, ctx := build.StartStep(ctx, "Upload logs")
	lg.Infof("Beginning to upload logs")
	defer func() {
		if r := recover(); r != nil {
			lg.Debugf("Received panic: %v\n", r)
			rErr = errors.Reason("panic: %v", r).Err()
		}
		lg.Infof("Finished uploading logs: ok=%t.", rErr == nil)
		step.End(rErr)
	}()
	// Construct the client that we will need to push the logs first.
	authenticator := luciauth.NewAuthenticator(
		ctx,
		luciauth.SilentLogin,
		luciauth.Options{
			Scopes: []string{
				luciauth.OAuthScopeEmail,
				"https://www.googleapis.com/auth/devstorage.read_write",
			},
		},
	)
	if authenticator != nil {
		lg.Infof("NewAuthenticator(...): successfully authed!")
	} else {
		return errors.Reason("NewAuthenticator(...): did not successfully auth!").Err()
	}
	email, err := authenticator.GetEmail()
	if err != nil {
		return errors.Annotate(err, "upload logs").Err()
	}
	lg.Infof("Auth email is %q", email)

	rt, err := authenticator.Transport()
	if err != nil {
		return errors.Annotate(err, "authenticator.Transport(...): error").Err()
	}
	// The ProdClient will cache the context, which will be used later to upload files.
	uploadTimeout := 5 * time.Minute
	timeoutCtx, cancel := context.WithTimeout(ctx, uploadTimeout)
	lg.Infof("Set %v timeout for uploading files.", uploadTimeout)
	defer cancel()
	client, err := lucigs.NewProdClient(timeoutCtx, rt)
	if err != nil {
		return errors.Annotate(err, "failed to create client(...)").Err()
	}

	lg.Infof("Persist the swarming logs")
	// Actually persist the logs.
	gsURL, err := parallelUpload(timeoutCtx, lg, client, input.GetSwarmingTaskId())
	if err != nil {
		return errors.Annotate(err, "upload logs").Err()
	}
	// Set the summary markdown to something noticeable.
	// In the future, change this to be a link to the logs.
	step.Modify(func(sv *build.StepView) {
		u := strings.TrimPrefix(gsURL, "gs://")
		u = fmt.Sprintf("https://%s/%s", "stainless.corp.google.com/browse", u)
		sv.SummaryMarkdown = fmt.Sprintf("[GS logs](%s)", u)
	})
	return nil
}

// parallelUpload performs an upload in parallel to the google-storage bucket.
//
// parallelUpload will fail when given invalid arguments. However, it will not fail
// simply because the upload attempt was unsuccessful.
func parallelUpload(ctx context.Context, lg logger.Logger, client lucigs.Client, swarmingTaskID string) (string, error) {
	if lg == nil {
		return "", errors.Reason("parallel-upload: logger cannot be nil").Err()
	}
	if client == nil {
		return "", errors.Reason("paralel-upload: client cannot be nil").Err()
	}
	if swarmingTaskID == "" {
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		lg.Errorf("Swarming task is empty. Falling back to timestamp %q.", timestamp)
		swarmingTaskID = fmt.Sprintf("FAKE-ID-%s", timestamp)
	}
	// upload.Upload can potentially run for a long time. Set a timeout of 30s.
	//
	// upload.Upload does respond to cancellation (which callFuncWithTimeout uses internally), but
	// the correct of this code does not and should not depend on this fact.
	//
	// callFuncWithTimeout synchronously calls a function with a timeout and then unconditionally hands control
	// back to its caller. The goroutine that's created in the background will not by itself keep the process alive.
	// TODO(gregorynisbet): Allow this parameter to be overridden from outside.
	// TODO(crbug/1311842): Switch this bucket back to chromeos-autotest-results.
	gsURL := fmt.Sprintf("gs://chrome-fleet-karte-autotest-results/swarming-%s", swarmingTaskID)
	lg.Infof("Swarming task %q is non-empty. Uploading to %q", swarmingTaskID, gsURL)
	uploadParams := &upload.Params{
		// TODO(gregorynisbet): Change this to the log root.
		SourceDir:         ".",
		GSURL:             gsURL,
		MaxConcurrentJobs: 10,
	}
	if err := upload.Upload(ctx, client, uploadParams); err != nil {
		// TODO: Register error to Karte.
		lg.Errorf("Upload task error: %s", err)
	} else {
		lg.Infof("Upload task finished without erorrs.")
	}
	return gsURL, nil
}

// internalRun main entry point to execution received request.
func internalRun(ctx context.Context, in *lab.LabpackInput, metrics metrics.Metrics, lg logger.Logger, ad *tlw.AccessData, logRoot string) (err error) {
	defer func() {
		// Catching the panic here as luciexe just set a step as fail and but not exit execution.
		lg.Debugf("Checking if there is a panic!")
		if r := recover(); r != nil {
			lg.Debugf("Received panic: %v\n", r)
			err = errors.Reason("panic: %v", r).Err()
		}
	}()
	ctx, access, cftCloser, err := tlw.NewAccess(ctx, in, ad, logRoot, metrics, lg)
	if err != nil {
		return errors.Annotate(err, "internal run").Err()
	}
	defer func() {
		lg.Debugf("Stopping CTR service...")
		if cftCloser != nil {
			if err := cftCloser(ctx); err != nil {
				lg.Debugf("(Not critical) Fail to stop CTR service: %s", err)
			}
		}
		lg.Debugf("Close access point: starting...")
		access.Close(ctx)
		lg.Debugf("Close access point: finished!")
	}()

	// Recovery is the task that we want 90% of the time. However, silently making
	// recovery the default can cause us to silently fall back to performing a recovery task
	// when we did not intend to, which is hard to discover unless you carefully read the logs.
	//
	// To avoid this, I am making the logic here much stricter and ending the task early if
	// we use an unrecognized (or empty) task name.
	task, ok := supportedTasks[in.TaskName]
	if !ok {
		lg.Errorf("Unsupported task-name %q!", in.TaskName)
		return errors.Reason("task name %q is invalid", in.TaskName).Err()
	}
	runArgs := &recovery.RunArgs{
		UnitName:              in.GetUnitName(),
		TaskName:              task,
		Access:                access,
		Logger:                lg,
		ShowSteps:             !in.GetNoStepper(),
		Metrics:               metrics,
		EnableRecovery:        in.GetEnableRecovery(),
		EnableUpdateInventory: in.GetUpdateInventory(),
		SwarmingTaskID:        in.SwarmingTaskId,
		BuildbucketID:         in.Bbid,
		LogRoot:               logRoot,
	}
	if uErr := runArgs.UseConfigBase64(in.GetConfiguration()); uErr != nil {
		return uErr
	}

	lg.Debugf("Labpack: starting the task...")
	if err := recovery.Run(ctx, runArgs); err != nil {
		lg.Debugf("Labpack: finished task run with error: %v", err)
		return errors.Annotate(err, "internal run").Err()
	}
	lg.Debugf("Labpack: finished task successful!")
	return nil
}

// Mapping of all supported tasks.
var supportedTasks = map[string]buildbucket.TaskName{
	string(buildbucket.AuditRPM):     buildbucket.AuditRPM,
	string(buildbucket.AuditStorage): buildbucket.AuditStorage,
	string(buildbucket.AuditUSB):     buildbucket.AuditUSB,
	string(buildbucket.Custom):       buildbucket.Custom,
	string(buildbucket.Deploy):       buildbucket.Deploy,
	string(buildbucket.Recovery):     buildbucket.Recovery,
	string(buildbucket.MHRecovery):   buildbucket.MHRecovery,
	string(buildbucket.MHDeploy):     buildbucket.MHDeploy,
	string(buildbucket.DeepRecovery): buildbucket.DeepRecovery,
	string(buildbucket.DryRun):       buildbucket.DryRun,
	string(buildbucket.PostTest):     buildbucket.PostTest,
}
