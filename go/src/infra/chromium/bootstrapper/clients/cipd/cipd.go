// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cipd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"go.chromium.org/luci/common/errors"
)

// Package is the details of a package to ensure
type Package struct {
	// Name is the name of the package to ensure
	Name string
	// Version is the version of the package to ensure
	Version string
}

type ResolvedPackage struct {
	// Name is the name of the package that was downloaded
	Name string
	// RequestedVersion is the version of the package that was requested
	RequestedVersion string
	// ActualVersion is the resolved version of the package that was downloaded
	ActualVersion string
}

// Client provides operations for interacting with CIPD.
type Client interface {
	// Ensure downloads packages from a given CIPD service to the given CIPD root.
	//
	// The packages download are given as a map of subdirectories to the package to download to
	// the subdirectory.
	//
	// If the operation is successful a map containing resolved packages and a nil error will be
	// returned. The map of resolved packages maps the subdirectory to a Package instance where
	// the version has been resolved to an instance ID. In the case of an error, a nil map and
	// non-nil error will be returned.
	Ensure(ctx context.Context, serviceUrl, cipdRoot string, packages map[string]*Package) (map[string]string, error)
}

// ClientFactory creates the client for accessing CIPD.
type ClientFactory func(ctx context.Context) Client

var ctxKey = "infra/chromium/bootstrapper/recipe.CipdClientFactory"

// UseClientFactory returns a context that causes new Client instances to be created using the given
// factory.
func UseClientFactory(ctx context.Context, factory ClientFactory) context.Context {
	return context.WithValue(ctx, &ctxKey, factory)
}

func Ensure(ctx context.Context, serviceUrl, cipdRoot string, packages map[string]*Package) (map[string]*ResolvedPackage, error) {
	if serviceUrl == "" {
		return nil, errors.New("empty serviceUrl")
	}
	if cipdRoot == "" {
		return nil, errors.New("empty cipdRoot")
	}
	if len(packages) == 0 {
		return nil, errors.New("empty packages")
	}
	for subdir, pkg := range packages {
		if subdir == "" {
			return nil, errors.New("empty subdir in packages")
		}
		if pkg == nil {
			return nil, errors.Reason("nil package for subdir %#v", subdir).Err()
		}
		if pkg.Name == "" {
			return nil, errors.Reason("empty package name for subdir %#v", subdir).Err()
		}
		if pkg.Version == "" {
			return nil, errors.Reason("empty package version for subdir %#v", subdir).Err()
		}
	}
	factory, _ := ctx.Value(&ctxKey).(ClientFactory)
	var client Client
	if factory != nil {
		client = factory(ctx)
	} else {
		client = defaultClient{}
	}
	packageVersions, err := client.Ensure(ctx, serviceUrl, cipdRoot, packages)
	if err != nil {
		return nil, err
	}
	resolvedPackages := make(map[string]*ResolvedPackage, len(packages))
	for subdir, pkg := range packages {
		resolvedPackages[subdir] = &ResolvedPackage{
			Name:             pkg.Name,
			RequestedVersion: pkg.Version,
			ActualVersion:    packageVersions[subdir],
		}
	}
	return resolvedPackages, nil
}

type defaultClient struct{}

type jsonPackage struct {
	Package    string `json:"package"`
	InstanceId string `json:"instance_id"`
}

type jsonOut struct {
	Result map[string][]jsonPackage `json:"result"`
}

func (c defaultClient) Ensure(ctx context.Context, serviceUrl, cipdRoot string, packages map[string]*Package) (map[string]string, error) {
	var ensureContents strings.Builder
	ensureContents.WriteString("$OverrideInstallMode copy\n")
	for subdir, pkg := range packages {
		ensureContents.WriteString(fmt.Sprintf("@Subdir %s\n", subdir))
		ensureContents.WriteString(fmt.Sprintf("%s %s\n", pkg.Name, pkg.Version))
	}
	ensureFile := "cipd.ensure"
	if err := ioutil.WriteFile(ensureFile, []byte(ensureContents.String()), 0440); err != nil {
		return nil, errors.Annotate(err, "failed to write out CIPD ensure file").Err()
	}

	jsonOutFile := "cipd.json.out"
	cmdCtx := exec.CommandContext(ctx, "cipd", "ensure", "-service-url", serviceUrl, "-root", cipdRoot, "-ensure-file", ensureFile, "-json-output", jsonOutFile)
	cmdCtx.Stdout = os.Stdout
	cmdCtx.Stderr = os.Stderr
	err := cmdCtx.Run()
	if err != nil {
		return nil, errors.Annotate(err, "cipd ensure failed").Err()
	}

	jsonOutContents, err := ioutil.ReadFile(jsonOutFile)
	if err != nil {
		return nil, errors.Annotate(err, "failed to read json output for cipd ensure").Err()
	}

	out, err := unmarshalEnsureJsonOut(jsonOutContents)
	if err != nil {
		return nil, errors.Annotate(err, "failed to unmarshal json output for cipd ensure, contents: %s", jsonOutContents).Err()
	}

	resolvedVersions := make(map[string]string, len(packages))
	for subdir, pkgs := range out.Result {
		pkg := pkgs[0]
		resolvedVersions[subdir] = pkg.InstanceId
	}
	return resolvedVersions, nil
}

func unmarshalEnsureJsonOut(jsonOutContents []byte) (*jsonOut, error) {
	out := &jsonOut{}
	if err := json.Unmarshal(jsonOutContents, out); err != nil {
		return nil, err
	}
	return out, nil
}
