// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cipd

import (
	"context"
	"fmt"
	"path"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/testfs"

	real "go.chromium.org/infra/chromium/bootstrapper/clients/cipd"
	"go.chromium.org/infra/chromium/util"
)

// PackageInstance is the fake data for an instance of a package.
type PackageInstance struct {
	// Contents maps file paths to the contents.
	Contents map[string]string
}

// Package is the fake data for a package including its refs and all its
// instances.
type Package struct {
	// Refs maps refs to their instance IDs.
	//
	// Missing keys will have a default instance ID computed. An empty
	// string value indicates that the ref does not exist.
	Refs map[string]string

	// Instances maps instance IDs to the instances.
	//
	// Missing keys will have a default instance. A nil value indicates that
	// the instance does not exist.
	Instances map[string]*PackageInstance
}

// Client is the client that will serve fake data for a given host.
type Client struct {
	packages map[string]*Package
}

// Factory creates a factory that returns CIPD clients that use fake data to
// respond to requests.
//
// The fake data is taken from the packages argument, which is a map from
// package names to the Package instances containing the fake data for the
// package. Missing keys will have a default Package. A nil value indicates that
// the given package is not the name of a package.
func Factory(packages map[string]*Package) real.ClientFactory {
	return func(ctx context.Context) real.Client {
		return &Client{packages: packages}
	}
}

func (c *Client) Ensure(ctx context.Context, serviceUrl, cipdRoot string, packages map[string]*real.Package) (map[string]string, error) {
	packageVersions := make(map[string]string, len(packages))
	layout := map[string]string{}

	for subdir, pin := range packages {
		pkg, ok := c.packages[pin.Name]
		if !ok {
			pkg = &Package{}
		} else if pkg == nil {
			return nil, errors.Reason("unknown package %#v", pin.Name).Err()
		}
		instanceId := pin.Version
		if _, ok := pkg.Instances[pin.Version]; !ok {
			var ok bool
			if instanceId, ok = pkg.Refs[instanceId]; !ok {
				instanceId = fmt.Sprintf("fake-instance-id|%s|%s", pin.Name, pin.Version)
			}
		}
		var instance *PackageInstance
		if instanceId != "" {
			var ok bool
			if instance, ok = pkg.Instances[instanceId]; !ok {
				instance = &PackageInstance{}
			}
		}
		if instance == nil {
			return nil, errors.Reason("unknown version %#v of package %#v", pin.Version, pin.Name).Err()
		}
		packageVersions[subdir] = instanceId

		layout[subdir+"/"] = ""
		for file, contents := range instance.Contents {
			layout[path.Join(subdir, file)] = contents
		}
	}
	err := testfs.Build(cipdRoot, layout)
	util.PanicOnError(err)
	return packageVersions, nil
}
