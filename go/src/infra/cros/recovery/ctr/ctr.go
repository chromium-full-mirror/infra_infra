// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ctr

import (
	"context"
	"os"
	"path/filepath"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/recovery/dev"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/scopes"
)

// Info describes abilities of CTR service.
type Info interface {
	Stop(ctx context.Context) error
	GetContainer(ctx context.Context, req *testapi.StartTemplatedContainerRequest) (BaseContainer, error)
	IsUp() bool
}

const (
	// TODO(otabek): Switch to prod when finish testing of ADB.
	// cipdTag = "prod"
	cipdTag = "latest"
	// Directory when CTR service will create a file.
	metadateDirName  = "ctr_metadata"
	artifactsDirName = "ctr_artifacts"
)

// Init initializes the CTR service and does all the preparation for its use.
// If it fails to start or authorize then it will be closed.
func Init(ctx context.Context, rootDir string) (Info, error) {
	i := &infoImpl{
		ctr:            nil,
		rootDir:        rootDir,
		containerCache: make(map[string]BaseContainer),
	}
	if metadataDir, err := i.createDir(metadateDirName); err != nil {
		return nil, errors.Annotate(err, "new CTR").Err()
	} else {
		i.metadataDir = metadataDir
	}
	if aDir, err := i.createDir(artifactsDirName); err != nil {
		return nil, errors.Annotate(err, "new CTR").Err()
	} else {
		i.artifactsDir = aDir
	}
	close := func(reason string) {
		if err := i.Stop(ctx); err != nil {
			log.Debugf(ctx, "Stop service after failed at %q: %s", reason, err)
		}
	}
	if err := i.start(ctx); err != nil {
		defer close("start")
		return nil, errors.Annotate(err, "new CTR").Err()
	}
	if err := i.gcloudAuth(ctx); err != nil {
		defer close("gcloud auth")
		return nil, errors.Annotate(err, "new CTR").Err()
	}
	return i, nil
}

// Get returns ctr Info from context.
func Get(ctx context.Context) (i Info, ok bool) {
	if p, ok := scopes.GetParam(ctx, scopes.ParamKeyCTRClient); !ok {
		return nil, false
	} else if v, ok := p.(Info); ok {
		return v, true
	} else {
		return nil, false
	}
}

type infoImpl struct {
	ctr           *crostoolrunner.CrosToolRunner
	serverAddress string

	// Directories used at run time.
	rootDir      string
	metadataDir  string
	artifactsDir string

	// All container need to be listed here to be sure they closed or use started one if needed.
	containerCache map[string]BaseContainer
}

// Stop stops CTR service.
func (c *infoImpl) Stop(ctx context.Context) error {
	if c.ctr == nil {
		return nil
	}
	errs := []error{}
	log.Infof(ctx, "Try to stop CTR service...")
	for _, v := range c.containerCache {
		if err := v.Stop(ctx, true); err != nil {
			errs = append(errs, errors.Annotate(err, "stop container").Err())
		}
	}
	if len(errs) != 0 {
		return errors.NewMultiError(errs...)
	}
	c.containerCache = nil

	if err := c.ctr.StopCTRServer(ctx); err != nil {
		return errors.Annotate(err, "stop CTR").Err()
	}
	c.serverAddress = ""
	c.ctr = nil
	log.Infof(ctx, "CTR stopped!")
	return nil
}

// IsUp tells if CTR service is up or not.
func (c *infoImpl) IsUp() bool {
	return c.ctr != nil && c.serverAddress != "" && c.ctr.CtrClient != nil
}

// start pulls CIPD and start service from it.
func (c *infoImpl) start(ctx context.Context) error {
	log.Infof(ctx, "Prepare start cros-tool-runner as service.")
	ctr := &crostoolrunner.CrosToolRunner{
		CtrCipdInfo: crostoolrunner.CtrCipdInfo{
			Version:        cipdTag,
			CtrCipdPackage: common.CtrCipdPackage,
			CtrTempDirLoc:  c.metadataDir,
		},
		EnvVarsToPreserve: common.DockerEnvVarsToPreserve(),
		// Do not use sudo when run on localhost.
		NoSudo: dev.IsActive(ctx),
	}
	if err := ctr.StartCTRServerAsync(ctx); err != nil {
		return errors.Annotate(err, "start CTR").Err()
	}
	log.Debugf(ctx, "CTR downloaded to: %s", ctr.CtrCipdInfo.CtrPath)
	c.ctr = ctr
	select {
	case <-ctx.Done():
		log.Debugf(ctx, "Start CTR: context canceled")
		return errors.Reason("start CTR: context canceled").Err()
	default:
	}
	// Retrieve server address from metadata
	serverAddress, err := ctr.GetServerAddressFromServiceMetadata(ctx)
	if err != nil {
		return errors.Annotate(err, "start CTR").Err()
	}
	select {
	case <-ctx.Done():
		log.Debugf(ctx, "Start CTR: context canceled")
		return errors.Reason("start CTR: context canceled").Err()
	default:
	}
	c.serverAddress = serverAddress
	// Connect to server
	if _, err = ctr.ConnectToCTRServer(ctx, serverAddress); err != nil {
		return errors.Annotate(err, "start CTR").Err()
	}
	log.Infof(ctx, "CTR started on the addr: %q", c.serverAddress)
	select {
	case <-ctx.Done():
		log.Debugf(ctx, "Start CTR: context canceled")
		return errors.Reason("start CTR: context canceled").Err()
	default:
	}
	log.Infof(ctx, "CTR started and ready!")
	return nil
}

func (c *infoImpl) gcloudAuth(ctx context.Context) error {
	log.Infof(ctx, "Prepare to get Gcloud auth.")
	if c.ctr == nil || c.serverAddress == "" {
		return errors.Reason("gcloud auth: service is not started").Err()
	}
	// File is not available at workstation, so we do not use it for local run.
	dockerFileLocation := "/creds/service_accounts/skylab-drone.json"
	useDockerKey := !dev.IsActive(ctx)
	if !useDockerKey {
		dockerFileLocation = ""
	}
	res, err := c.ctr.GcloudAuth(ctx, dockerFileLocation, useDockerKey)
	if err != nil {
		return errors.Annotate(err, "gcloud auth").Err()
	}
	log.Infof(ctx, "GcloudAuth response %v", res)
	return nil
}

// createDit creates required directory in the rootDir.
func (c *infoImpl) createDir(name string) (string, error) {
	newDir := filepath.Join(c.rootDir, name)
	// Always try to clean up directory first to avoid data pollution.
	_ = os.RemoveAll(newDir)
	if err := os.MkdirAll(newDir, 0755); err != nil {
		return "", errors.Annotate(err, "create directory %q", name).Err()
	}
	return newDir, nil
}
