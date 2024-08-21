// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gogo/protobuf/jsonpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/chromiumos/infra/proto/go/lab_platform"
	authclient "go.chromium.org/luci/auth"
	gitilesApi "go.chromium.org/luci/common/api/gitiles"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/server/auth"

	fleet "infra/appengine/crosskylabadmin/api/fleet/v1"
	"infra/appengine/crosskylabadmin/internal/app/config"
	dssv "infra/appengine/crosskylabadmin/internal/app/frontend/datastore/stableversion"
	"infra/appengine/crosskylabadmin/internal/app/frontend/datastore/stableversion/satlab"
	"infra/appengine/crosskylabadmin/internal/ufs"
	"infra/cros/stableversion"
	"infra/libs/git"
	"infra/libs/skylab/common/heuristics"
	"infra/libs/skylab/inventory"
)

// StableVersionGitClientFactory is a constructor for a git client pointed at the source of truth
// for the stable version information
type StableVersionGitClientFactory func(c context.Context) (git.ClientInterface, error)

// ServerImpl implements the fleet.InventoryServer interface.
type ServerImpl struct {
	// StableVersionGitClientFactory
	StableVersionGitClientFactory StableVersionGitClientFactory
}

type getStableVersionRecordsResult struct {
	cros     map[string]string
	faft     map[string]string
	firmware map[string]string
}

// DumpStableVersionToDatastore takes stable version info from the git repo where it lives
// and dumps it to datastore
func (is *ServerImpl) DumpStableVersionToDatastore(ctx context.Context, in *fleet.DumpStableVersionToDatastoreRequest) (*fleet.DumpStableVersionToDatastoreResponse, error) {
	client, err := is.newStableVersionGitClient(ctx)
	if err != nil {
		logging.Errorf(ctx, "get git client: %s", err)
		return nil, errors.Annotate(err, "get git client").Err()
	}
	return dumpStableVersionToDatastoreImpl(ctx, client.GetFile)
}

// GetStableVersion implements the method from fleet.InventoryServer interface
func (is *ServerImpl) GetStableVersion(ctx context.Context, req *fleet.GetStableVersionRequest) (resp *fleet.GetStableVersionResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()

	return getStableVersionImpl(ctx, req.GetBuildTarget(), req.GetModel(), req.GetHostname(), req.GetSatlabInformationalQuery())
}

// GetRecoveryVersion implements the method from fleet.InventoryServer interface
func (is *ServerImpl) GetRecoveryVersion(ctx context.Context, req *fleet.GetRecoveryVersionRequest) (resp *fleet.GetRecoveryVersionResponse, err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	v, err := getVersionImpl(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "get recovery version: %s", err)
	}
	return &fleet.GetRecoveryVersionResponse{Version: v}, nil
}

// getSatlabStableVersion gets a stable version for a satlab device.
//
// It returns a full response if there's no error, and a boolean ok which determines whether the error should cause
// the request to fail or not.
func getSatlabStableVersion(ctx context.Context, buildTarget string, model string, hostname string) (resp *fleet.GetStableVersionResponse, ok bool, e error) {
	logging.Infof(ctx, "using satlab flow board:%q model:%q host:%q", buildTarget, model, hostname)

	if hostnameID := satlab.MakeSatlabStableVersionID(hostname, "", ""); hostnameID != "" {
		entry, err := satlab.GetSatlabStableVersionEntryByRawID(ctx, hostnameID)
		switch {
		case err == nil:
			reason := fmt.Sprintf("looked up satlab device using id %q", hostnameID)
			resp := &fleet.GetStableVersionResponse{
				CrosVersion:     entry.OS,
				FirmwareVersion: entry.FW,
				FaftVersion:     entry.FWImage,
				Reason:          reason,
			}
			return resp, true, nil
		case datastore.IsErrNoSuchEntity(err):
			// Do nothing. If there is no override for the hostname, it is correct to
			// move on to the next case, checking by board & model.
		default:
			return nil, false, status.Errorf(codes.NotFound, "get satlab: %s", err)
		}
	}

	if hostname != "" && (buildTarget == "" || model == "") {
		logging.Infof(ctx, "looking up inventory info for DUT host:%q board:%q model:%q in order to get board and model info", hostname, buildTarget, model)
		dut, err := getDUT(ctx, hostname)
		if err != nil {
			return nil, false, status.Errorf(codes.NotFound, "get satlab: processing dut %q: %s", hostname, err)
		}

		buildTarget = dut.GetCommon().GetLabels().GetBoard()
		model = dut.GetCommon().GetLabels().GetModel()
	}

	if boardModelID := satlab.MakeSatlabStableVersionID("", buildTarget, model); boardModelID != "" {
		entry, err := satlab.GetSatlabStableVersionEntryByRawID(ctx, boardModelID)
		switch {
		case err == nil:
			reason := fmt.Sprintf("looked up satlab device using id %q", boardModelID)

			resp := &fleet.GetStableVersionResponse{
				CrosVersion:     entry.OS,
				FirmwareVersion: entry.FW,
				FaftVersion:     entry.FWImage,
				Reason:          reason,
			}

			return resp, true, nil
		case datastore.IsErrNoSuchEntity(err):
			// Do nothing.
		default:
			return nil, false, status.Errorf(codes.NotFound, "get satlab: lookup by board/model %q: %s", boardModelID, err)
		}
	}

	return nil, true, status.Error(codes.Aborted, "get satlab: falling back %s")
}

// getStableVersionImpl returns all the stable versions associated with a given buildTarget and model
// NOTE: hostname is explicitly allowed to be "". If hostname is "", then no hostname was provided in the GetStableVersion RPC call
// ALSO NOTE: If the hostname is "", then we assume that the device is not a satlab device and therefore we should not fall back to satlab.
func getStableVersionImpl(ctx context.Context, buildTarget string, model string, hostname string, satlabInformationalQuery bool) (*fleet.GetStableVersionResponse, error) {
	logging.Infof(ctx, "getting stable version for buildTarget: %s and model: %s", buildTarget, model)

	wantSatlab := heuristics.LooksLikeSatlabDevice(hostname) || satlabInformationalQuery

	if wantSatlab {
		resp, ok, err := getSatlabStableVersion(ctx, buildTarget, model, hostname)
		switch {
		case err == nil:
			return resp, nil
		case ok:
			// Do nothing and fall back
		default:
			return nil, err
		}
	}

	if hostname == "" {
		if buildTarget == "" || model == "" {
			return nil, status.Errorf(codes.FailedPrecondition, "search criteria must be provided.")
		}
		logging.Infof(ctx, "hostname not provided, using buildTarget (%s) and model (%s)", buildTarget, model)
		out, err := getStableVersionImplNoHostname(ctx, buildTarget, model)
		if err == nil {
			msg := "looked up board %q and model %q"
			if wantSatlab {
				msg = "wanted satlab, falling back to board %q and model %q"
			}
			maybeSetReason(out, fmt.Sprintf(msg, buildTarget, model))
			return out, nil
		}
		return out, status.Errorf(codes.NotFound, "get stable version impl: %s", err)
	}

	// Default case, not a satlab device.
	logging.Infof(ctx, "hostname (%s) provided, ignoring user-provided buildTarget (%s) and model (%s)", hostname, buildTarget, model)
	out, err := getStableVersionImplWithHostname(ctx, hostname)
	if err == nil {
		msg := "looked up non-satlab device hostname %q"
		if wantSatlab {
			msg = "falling back to non-satlab path for device %q"
		}
		maybeSetReason(out, fmt.Sprintf(msg, hostname))
		return out, nil
	}
	return out, status.Errorf(codes.NotFound, "get stable version impl: %s", err)
}

// deviceInfo read device-info from inventory.
func deviceInfo(ctx context.Context, hostname string) (*ufs.DeviceInfo, error) {
	cfg := config.Get(ctx)
	httpClient, err := ufs.NewHTTPClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "device info: fail create http client").Err()
	}
	client, err := ufs.NewClient(ctx, httpClient, cfg.GetUFS().GetHost())
	if err != nil {
		return nil, errors.Annotate(err, "device info: fail create ufs client").Err()
	}
	return ufs.GetDeviceInfo(ctx, client, hostname)
}

// getVersionImpl finds recovery version for request api.
func getVersionImpl(ctx context.Context, req *fleet.GetRecoveryVersionRequest) (*lab_platform.StableVersion, error) {
	hostname := req.GetDeviceName()
	board := req.GetBoard()
	model := req.GetModel()
	pools := req.GetPools()
	if hostname == "" && (board == "" || model == "") {
		return nil, errors.Reason("get version: search criteria not provided").Err()
	}
	// Satlab case supported only when hostname provided.
	if hostname != "" && heuristics.LooksLikeSatlabDevice(hostname) {
		// Satlab CLI allows to set versions per hostname only.
		// So we search only by a hostname, and if it is not found, we move on to other options.
		satlabKey := satlab.MakeSatlabStableVersionID(hostname, "", "")
		entry, err := satlab.GetSatlabStableVersionEntryByRawID(ctx, satlabKey)
		switch {
		case err == nil:
			logging.Infof(ctx, "Found version for Satlab device by hostname: %q", hostname)
			return &lab_platform.StableVersion{
				OsVersion:           entry.OS,
				OsImagePath:         fmt.Sprintf("%s-release/%s", board, entry.OS),
				FirmwareRoVersion:   entry.FW,
				FirmwareRoImagePath: entry.FWImage,
			}, nil
		case datastore.IsErrNoSuchEntity(err):
			// Do nothing. If there is no override for the hostname.
			// Proceed with next options.
		default:
			return nil, errors.Annotate(err, "get version: for satlab").Err()
		}
	}
	if hostname != "" && (board == "" || model == "") {
		// Only read data for internal usage, no partners at this point.
		di, err := deviceInfo(ctx, hostname)
		if err != nil {
			return nil, errors.Annotate(err, "get version").Err()
		}
		board = di.Board
		model = di.Model
		pools = di.Pools
		if board == "" || model == "" {
			return nil, errors.Reason("get version: board or model not found").Err()
		}
	}
	logging.Infof(ctx, "Finding a version for board:%q, model:%q, poools:%q", board, model, pools)
	return dssv.FindVersion(ctx, board, model, pools)
}

// getStableVersionImplNoHostname returns stableversion information given a buildTarget and model
// TODO(gregorynisbet): Consider under what circumstances an error leaving this function
// should be considered transient or non-transient.
// If the dut in question is a beaglebone servo, then failing to get the firmware version
// is non-fatal.
func getStableVersionImplNoHostname(ctx context.Context, buildTarget string, model string) (*fleet.GetStableVersionResponse, error) {
	logging.Infof(ctx, "getting stable version for buildTarget: (%s) and model: (%s)", buildTarget, model)

	buildTarget = strings.TrimSpace(buildTarget)
	model = strings.TrimSpace(model)
	if buildTarget == "" || model == "" {
		return nil, errors.Reason("get stable version: board and/or model is empty").Err()
	}
	var err error
	out := &fleet.GetStableVersionResponse{}
	out.CrosVersion, err = dssv.GetCrosStableVersion(ctx, buildTarget, model)
	if err != nil {
		return nil, errors.Annotate(err, "get stable version by board/model").Err()
	}
	logging.Infof(ctx, "Got cros version %q from datastore", out.CrosVersion)
	out.FaftVersion, err = dssv.GetFaftStableVersion(ctx, buildTarget, model)
	if err != nil {
		logging.Infof(ctx, "faft version not found: %w", err)
	} else {
		// Fw image path expected only if evrsion is present.
		logging.Infof(ctx, "Got faft version %q from datastore", out.FaftVersion)
		out.FirmwareVersion, err = dssv.GetFirmwareStableVersion(ctx, buildTarget, model)
		if err != nil {
			logging.Infof(ctx, "firmware version does not exist: %w", err)
		}
	}
	return out, nil
}

// getStableVersionImplWithHostname return stable version information given just a hostname
// TODO(gregorynisbet): Consider under what circumstances an error leaving this function
// should be considered transient or non-transient.
func getStableVersionImplWithHostname(ctx context.Context, hostname string) (*fleet.GetStableVersionResponse, error) {
	dut, err := getDUT(ctx, hostname)
	if err != nil {
		return nil, errors.Annotate(err, "get stable version per hostname").Err()
	}
	buildTarget := dut.GetCommon().GetLabels().GetBoard()
	model := dut.GetCommon().GetLabels().GetModel()
	out, err := getStableVersionImplNoHostname(ctx, buildTarget, model)
	return out, errors.Annotate(err, "get stable version per hostname").Err()
}

// getDUTOverrideForTests is an override for tests only.
//
// Do not set this variable for any other purpose.
var getDUTOverrideForTests func(context.Context, string) (*inventory.DeviceUnderTest, error) = nil

// getDUT returns the DUT associated with a particular hostname from datastore
func getDUT(ctx context.Context, hostname string) (*inventory.DeviceUnderTest, error) {
	if getDUTOverrideForTests != nil {
		return getDUTOverrideForTests(ctx, hostname)
	}
	// Call UFS directly to get DUT info, if fails, falling back to use the old workflow
	dutV1, err := ufs.GetDutV1(ctx, hostname)
	return dutV1, errors.Annotate(err, "get DUT from inventory by hostname: %q", hostname).Err()
}

// maybeSetReason sets the reason on a stable version response if the response is non-nil and the reason is "".
func maybeSetReason(resp *fleet.GetStableVersionResponse, msg string) {
	if resp != nil && resp.GetReason() == "" {
		resp.Reason = msg
	}
}

func (is *ServerImpl) newStableVersionGitClient(ctx context.Context) (git.ClientInterface, error) {
	if is.StableVersionGitClientFactory != nil {
		return is.StableVersionGitClientFactory(ctx)
	}
	hc, err := getAuthenticatedHTTPClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "newStableVersionGitClient").Err()
	}
	return getStableVersionGitClient(ctx, hc)
}

// dumpStableVersionToDatastoreImpl takes some way of getting a file and a context and writes to datastore
func dumpStableVersionToDatastoreImpl(ctx context.Context, getFile func(context.Context, string) (string, error)) (*fleet.DumpStableVersionToDatastoreResponse, error) {
	dataPath := config.Get(ctx).StableVersionConfig.StableVersionDataPath
	contents, err := getFile(ctx, dataPath)
	if err != nil {
		return nil, errors.Annotate(err, "fetch file").Err()
	}
	stableVersions, err := parseStableVersions(contents)
	if err != nil {
		return nil, errors.Annotate(err, "parse json").Err()
	}
	m := getStableVersionRecords(ctx, stableVersions)
	// TODO(gregorynisbet): Walk the board;models that exist and bring them up to date one-by-one
	// inside one transaction per key instead of discarding the result.
	keys, err := getAllBoardModels(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "dump stable version to datastore implementation").Err()
	}

	// allKeys are the keys from datastore and the incoming map. The incoming map is more
	// authoritative than the contents of datastore.
	allKeys := make(map[string]bool)
	for k := range keys {
		allKeys[k] = true
	}
	for _, versionMap := range []map[string]string{m.cros, m.firmware, m.faft} {
		for k := range versionMap {
			allKeys[k] = true
		}
	}

	// nil and "No Such Entity" are both acceptable get responses.
	// The latter unambiguously indicates that we succesfully determined that an item does not exist.
	isAcceptableGetResponse := func(e error) bool {
		return err == nil || datastore.IsErrNoSuchEntity(err)
	}

	for boardModel := range allKeys {
		if err := datastore.RunInTransaction(ctx, func(ctx context.Context) error {
			cros := &dssv.CrosStableVersionEntity{ID: boardModel}
			if err := datastore.Get(ctx, cros); !isAcceptableGetResponse(err) {
				return errors.Annotate(err, "dump stable version to datastore implementation: put cros stable version %q", boardModel).Err()
			}
			if err := cros.ImposeVersion(ctx, m.cros[boardModel]); err != nil {
				return errors.Annotate(err, "dump stable version to datastore implementation %q", boardModel).Err()
			}
			firmware := &dssv.FirmwareStableVersionEntity{ID: boardModel}
			if err := datastore.Get(ctx, firmware); !isAcceptableGetResponse(err) {
				return errors.Annotate(err, "dump stable version to datastore implementation: put firmware stable version %q", boardModel).Err()
			}
			if err := firmware.ImposeVersion(ctx, m.firmware[boardModel]); err != nil {
				return errors.Annotate(err, "dump stable version to datastore implementation %q", boardModel).Err()
			}
			faft := &dssv.FaftStableVersionEntity{ID: boardModel}
			if err := datastore.Get(ctx, faft); !isAcceptableGetResponse(err) {
				return errors.Annotate(err, "dump stable version to datastore implementation: put faft stable version for %q", boardModel).Err()
			}
			if err := faft.ImposeVersion(ctx, m.faft[boardModel]); err != nil {
				return errors.Annotate(err, "dump stable version to datastore implementation for %q", boardModel).Err()
			}
			return nil
		}, nil); err != nil {
			return nil, err
		}
	}
	logging.Infof(ctx, "successfully wrote legacy stable versions")
	if err := datastore.RunInTransaction(ctx, func(ctx context.Context) error {
		err := dssv.WriteVersions(ctx, stableVersions.GetVersions())
		return errors.Annotate(err, "dump stable version: fail update new versions").Err()
	}, nil); err != nil {
		return nil, err
	}
	logging.Infof(ctx, "successfully wrote new stable versions")
	return &fleet.DumpStableVersionToDatastoreResponse{}, nil
}

func parseStableVersions(contents string) (*lab_platform.StableVersions, error) {
	var stableVersions lab_platform.StableVersions
	if err := jsonpb.Unmarshal(strings.NewReader(contents), &stableVersions); err != nil {
		return nil, errors.Annotate(err, "unmarshal stableversions json").Err()
	}
	return &stableVersions, nil
}

// getStableVersionRecords takes a StableVersions proto and produces a structure containing maps from
// key names (buildTarget or buildTarget+model) to stable version strings
func getStableVersionRecords(ctx context.Context, stableVersions *lab_platform.StableVersions) getStableVersionRecordsResult {
	cros := make(map[string]string)
	faft := make(map[string]string)
	firmware := make(map[string]string)
	for _, item := range stableVersions.GetCros() {
		buildTarget := item.GetKey().GetBuildTarget().GetName()
		model := item.GetKey().GetModelId().GetValue()
		version := item.GetVersion()
		key, err := stableversion.JoinBuildTargetModel(buildTarget, model)
		if err != nil {
			logging.Debugf(ctx, "buildTarget and/or model contains invalid sequence: %s", err)
			continue
		}
		cros[key] = version
	}
	for _, item := range stableVersions.GetFirmware() {
		buildTarget := item.GetKey().GetBuildTarget().GetName()
		model := item.GetKey().GetModelId().GetValue()
		version := item.GetVersion()
		key, err := stableversion.JoinBuildTargetModel(buildTarget, model)
		if err != nil {
			logging.Debugf(ctx, "buildTarget and/or model contains invalid sequence: %s", err)
			continue
		}
		firmware[key] = version
	}
	for _, item := range stableVersions.GetFaft() {
		buildTarget := item.GetKey().GetBuildTarget().GetName()
		model := item.GetKey().GetModelId().GetValue()
		version := item.GetVersion()
		key, err := stableversion.JoinBuildTargetModel(buildTarget, model)
		if err != nil {
			logging.Debugf(ctx, "buildTarget and/or model contains invalid sequence: %s", err)
			continue
		}
		faft[key] = version
	}
	return getStableVersionRecordsResult{
		cros:     cros,
		faft:     faft,
		firmware: firmware,
	}
}

func getAuthenticatedHTTPClient(ctx context.Context) (*http.Client, error) {
	transport, err := auth.GetRPCTransport(ctx, auth.AsSelf, auth.WithScopes(authclient.OAuthScopeEmail, gitilesApi.OAuthScope))
	if err != nil {
		return nil, errors.Annotate(err, "new authenticated http client").Err()
	}
	return &http.Client{Transport: transport}, nil
}

func getStableVersionGitClient(ctx context.Context, hc *http.Client) (git.ClientInterface, error) {
	cfg := config.Get(ctx)
	s := cfg.StableVersionConfig
	if s == nil {
		return nil, fmt.Errorf("DumpStableVersionToDatastore: app config does not have StableVersionConfig")
	}
	client, err := git.NewClient(ctx, hc, s.GerritHost, s.GitilesHost, s.Project, s.Branch)
	if err != nil {
		return nil, errors.Annotate(err, "get git client").Err()
	}
	return client, nil
}

// getAllBoardModels gets all the keys of the form board;model that currently exist in datastore.
func getAllBoardModels(ctx context.Context) (map[string]bool, error) {
	out := make(map[string]bool)
	if err := datastore.Run(ctx, datastore.NewQuery(dssv.CrosStableVersionKind), func(ent *dssv.CrosStableVersionEntity) {
		out[ent.ID] = true
	}); err != nil {
		return nil, errors.Annotate(err, "get all board models").Err()
	}
	if err := datastore.Run(ctx, datastore.NewQuery(dssv.FirmwareStableVersionKind), func(ent *dssv.FirmwareStableVersionEntity) {
		out[ent.ID] = true
	}); err != nil {
		return nil, errors.Annotate(err, "get all board models").Err()
	}
	if err := datastore.Run(ctx, datastore.NewQuery(dssv.FaftStableVersionKind), func(ent *dssv.FaftStableVersionEntity) {
		out[ent.ID] = true
	}); err != nil {
		return nil, errors.Annotate(err, "get all board models").Err()
	}
	return out, nil
}
