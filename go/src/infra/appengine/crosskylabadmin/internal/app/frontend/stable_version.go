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

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/config"
	dssv "go.chromium.org/infra/appengine/crosskylabadmin/internal/app/frontend/datastore/stableversion"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/frontend/datastore/stableversion/satlab"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/ufs"
	"go.chromium.org/infra/libs/git"
	"go.chromium.org/infra/libs/skylab/common/heuristics"
	"go.chromium.org/infra/libs/skylab/inventory"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// StableVersionGitClientFactory is a constructor for a git client pointed at the source of truth
// for the stable version information
type StableVersionGitClientFactory func(c context.Context) (git.ClientInterface, error)

// ServerImpl implements the fleet.InventoryServer interface.
type ServerImpl struct {
	// StableVersionGitClientFactory
	StableVersionGitClientFactory StableVersionGitClientFactory
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

// deviceInfo read device-info from inventory.
func deviceInfo(ctx context.Context, hostname string) (*ufs.DeviceInfo, error) {
	cfg := config.Get(ctx)
	httpClient, err := ufs.NewHTTPClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "device info: fail create http client").Err()
	}
	// We only support chromeos DUTs at this point.
	// TODO: Create RPC interpreters to read the namespace from the header.
	namespace := ufsUtil.OSNamespace
	ufsCtx := ufs.ContextWithNamespace(ctx, namespace)
	logging.Infof(ctx, "Set namespace %q for UFS client before getting device info: %q", namespace, hostname)
	client, err := ufs.NewClient(ufsCtx, httpClient, cfg.GetUFS().GetHost())
	if err != nil {
		return nil, errors.Annotate(err, "device info: fail create ufs client").Err()
	}
	return ufs.GetDeviceInfo(ufsCtx, client, hostname)
}

// getVersionImpl finds recovery version for request api.
func getVersionImpl(ctx context.Context, req *fleet.GetRecoveryVersionRequest) (*lab_platform.StableVersion, error) {
	hostname := req.GetDeviceName()
	deviceType := req.GetDeviceType()
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
		logging.Infof(ctx, "No board/model provided, so try to find device by hostname: %q", hostname)
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
	return dssv.FindVersion(ctx, deviceType, board, model, pools)
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
	if err := dssv.WriteVersions(ctx, stableVersions.GetVersions()); err != nil {
		return nil, errors.Annotate(err, "dump stable version: new versions").Err()
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
