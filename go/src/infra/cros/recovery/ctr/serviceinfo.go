// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ctr

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/internal/env"
	"infra/cros/recovery/dev"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/namespace"
	"infra/cros/recovery/scopes"
)

const (
	cipdTag = "prod"
	// Directory when CTR service will create a file.
	metadateDirName  = "ctr_metadata"
	artifactsDirName = "ctr_artifacts"
)

// ServiceInfo describes abilities of CTR service.
type ServiceInfo interface {
	Stop(ctx context.Context) error
	GetNetwork(ctx context.Context, name string) (Network, error)
	CreateContainer(ctx context.Context, req *api.StartTemplatedContainerRequest) (BaseContainer, error)
	GetContainer(ctx context.Context, name string) (BaseContainer, error)
	StopContainer(ctx context.Context, name string) error
	GenerateContainerImagePath(ctx context.Context, imageName, tag string) (string, error)
	IsUp() bool
}

// Init initializes the CTR service and does all the preparation for its use.
// If it fails to start or authorize then it will be closed.
func Init(ctx context.Context, rootDir string) (ServiceInfo, error) {
	i := &serviceInfoImpl{
		ctr:                   nil,
		rootDir:               rootDir,
		containerCache:        make(map[string]BaseContainer),
		dockerKeyFileLocation: dockerKeyFileLocation(ctx),
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

// IsUp tells if CTR service is up or not.
func IsUp(ctx context.Context) bool {
	_, ok := Get(ctx)
	return ok
}

// Get returns ctr ServiceInfo from context.
func Get(ctx context.Context) (i ServiceInfo, ok bool) {
	if p, ok := scopes.GetParam(ctx, scopes.ParamKeyCTRClient); !ok {
		return nil, false
	} else if v, ok := p.(ServiceInfo); ok {
		return v, true
	} else {
		return nil, false
	}
}

type serviceInfoImpl struct {
	ctr           *crostoolrunner.CrosToolRunner
	serverAddress string

	// Directories used at run time.
	rootDir      string
	metadataDir  string
	artifactsDir string

	// Path to local docker file.
	dockerKeyFileLocation string

	// All container need to be listed here to be sure they closed or use started one if needed.
	containerCache map[string]BaseContainer
}

// GenerateContainerImagePath generate container image name with tag.
func (c *serviceInfoImpl) GenerateContainerImagePath(ctx context.Context, imageName, tag string) (string, error) {
	if c == nil || c.ctr == nil || c.ctr.Version == "" {
		return "", errors.Reason("generate container name: service is not started").Err()
	}
	if imageName == "" {
		return "", errors.Reason("generate container name: image name is not provided").Err()
	}
	if tag == "" {
		return "", errors.Reason("generate container name: tag is not provided").Err()
	}
	log.Infof(ctx, "Using DockerKeyFile: %s", c.dockerKeyFileLocation)
	datastoreName := common.TestPlatformFireStore
	if namespace.IsPartner(ctx) {
		datastoreName = common.PartnerTestPlatformFireStore
	}
	containerInfo, err := common.FetchContainerInfoFromFirestore(ctx, datastoreName, c.dockerKeyFileLocation, c.ctr.Version, imageName)
	if err != nil {
		return "", errors.Annotate(err, "generate container name: fail to fetch container info").Err()
	}
	if containerInfo == nil || containerInfo.GetContainer().GetRepository() == nil {
		return "", errors.Reason("generate container name: received empty container info").Err()
	}
	repoPath := filepath.Join(
		containerInfo.GetContainer().GetRepository().GetHostname(),
		containerInfo.GetContainer().GetRepository().GetProject(),
		imageName,
	)
	image := repoPath + ":" + tag
	log.Infof(ctx, "Generate Container image path: %s", image)
	return image, nil
}

// Stop stops CTR service.
func (c *serviceInfoImpl) Stop(ctx context.Context) error {
	if c.ctr == nil {
		return nil
	}
	errs := []error{}
	log.Infof(ctx, "Try to stop CTR service...")
	for _, v := range c.containerCache {
		if err := v.Close(ctx); err != nil {
			errs = append(errs, errors.Annotate(err, "stop").Err())
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
func (c *serviceInfoImpl) IsUp() bool {
	return c.ctr != nil && c.serverAddress != "" && c.ctr.CtrClient != nil
}

// GetNetwork find or create requested network.
func (c *serviceInfoImpl) GetNetwork(ctx context.Context, name string) (_ Network, rErr error) {
	if name == "" {
		return nil, errors.Reason("get network: invalid request").Err()
	} else if c.ctr.CtrClient == nil {
		return nil, errors.Reason("get network: ctr-client not found, probably server is not started").Err()
	}
	n := &networkImpl{name: name}

	// If network present in docker level then no need to create it.
	if res, err := c.ctr.CtrClient.GetNetwork(ctx, &api.GetNetworkRequest{Name: n.name}); err != nil {
		log.Debugf(ctx, "Network %q is not found in the docker level: %s", n.name, err)
	} else {
		log.Infof(ctx, "Network %q found: %v", n.name, res)
		return n, nil
	}
	// So network was not exist before. Let's create it.
	if res, err := c.ctr.CtrClient.CreateNetwork(ctx, &api.CreateNetworkRequest{Name: n.name}); err != nil {
		return nil, errors.Annotate(err, "get network %q", n.name).Err()
	} else {
		log.Infof(ctx, "Network %q created: %v", n.name, res)
		return n, nil
	}
}

// GetContainer create requested container.
func (c *serviceInfoImpl) GetContainer(ctx context.Context, name string) (BaseContainer, error) {
	if container, ok := c.containerCache[name]; ok {
		log.Infof(ctx, "Got container %q from cache!", name)
		return container, nil
	}
	// If container created outside the call then we can find it at docker level.
	if _, err := c.ctr.GetContainer(ctx, name); err != nil {
		return nil, errors.Annotate(err, "get container %q: probably not created yet", name).Err()
	}
	// If no errors then container is exist.
	container := &baseContainerImpl{
		name: name,
		ci:   c,
	}
	c.containerCache[name] = container
	return container, nil
}

// StopContainer stops a container.
func (c *serviceInfoImpl) StopContainer(ctx context.Context, name string) error {
	if name == "" {
		log.Infof(ctx, "Container name is empty. Skipping!")
		return nil
	}
	stopCmd := exec.CommandContext(ctx, "docker", "stop", name)
	if _, _, err := common.RunCommand(ctx, stopCmd, "docker-stop-container", nil, false); err != nil {
		return errors.Annotate(err, "stop container %q", name).Err()
	}
	// Clear the cache if it is listed there.
	delete(c.containerCache, name)
	log.Infof(ctx, "Container %q stopped!", name)
	return nil
}

func (c *serviceInfoImpl) printContainers(ctx context.Context) {
	printCmd := exec.CommandContext(ctx, "docker", "ps", "-a")
	if out, _, err := common.RunCommand(ctx, printCmd, "docker-ps-a", nil, false); err != nil {
		log.Infof(ctx, "Fail to print container lits: %s", err)
	} else {
		log.Infof(ctx, "Container lits:\n %s", out)
	}
}

// CreateContainer creates a requested container.
func (c *serviceInfoImpl) CreateContainer(ctx context.Context, req *api.StartTemplatedContainerRequest) (_ BaseContainer, rErr error) {
	if req.GetName() == "" {
		return nil, errors.Reason("create container: invalid request").Err()
	} else if c.ctr.CtrClient == nil {
		return nil, errors.Reason("create container %q: ctr-client not found, probably server is not started", req.GetName()).Err()
	}
	if container, ok := c.containerCache[req.GetName()]; ok {
		log.Infof(ctx, "Got container %q from cache!", req.GetName())
		return container, nil
	} else {
		log.Infof(ctx, "Container %q isn't exist yet!", req.GetName())
	}
	container := &baseContainerImpl{
		name: req.GetName(),
		ci:   c,
	}
	if aDir, err := c.createDir(req.GetName() + "-logs"); err != nil {
		return nil, errors.Reason("create container %q", req.GetName()).Err()
	} else {
		req.ArtifactDir = aDir
	}
	res, err := c.ctr.StartTemplatedContainer(ctx, req)
	if err != nil {
		// Just print containers for future debugging.
		c.printContainers(ctx)
		return nil, errors.Annotate(err, "get container %q", container.name).Err()
	}
	c.containerCache[container.name] = container
	log.Infof(ctx, "Container %q created: %v", container.name, res)
	return container, nil
}

// start pulls CIPD and start service from it.
func (c *serviceInfoImpl) start(ctx context.Context) error {
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

func (c *serviceInfoImpl) gcloudAuth(ctx context.Context) error {
	log.Infof(ctx, "Prepare to get Gcloud auth.")
	if c.ctr == nil || c.serverAddress == "" {
		return errors.Reason("gcloud auth: service is not started").Err()
	}
	useDockerKey := false
	log.Infof(ctx, "Using DockerKeyFile: %s", c.dockerKeyFileLocation)
	res, err := c.ctr.GcloudAuth(ctx, c.dockerKeyFileLocation, useDockerKey)
	if err != nil {
		return errors.Annotate(err, "gcloud auth").Err()
	}
	log.Infof(ctx, "GcloudAuth response %v", res)
	return nil
}

// createDit creates required directory in the rootDir.
func (c *serviceInfoImpl) createDir(name string) (string, error) {
	newDir := filepath.Join(c.rootDir, name)
	// Always try to clean up directory first to avoid data pollution.
	_ = os.RemoveAll(newDir)
	if err := os.MkdirAll(newDir, 0755); err != nil {
		return "", errors.Annotate(err, "create directory %q", name).Err()
	}
	return newDir, nil
}

func dockerKeyFileLocation(ctx context.Context) string {
	if dev.IsActive(ctx) {
		return ""
	}
	if env.IsCloudBot() {
		return common.VmLabDockerKeyFileLocation
	}
	return common.LabDockerKeyFileLocation
}
