// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bootstrap

import (
	"context"
	"path/filepath"

	"golang.org/x/sync/errgroup"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"

	"go.chromium.org/infra/chromium/bootstrapper/clients/cas"
	"go.chromium.org/infra/chromium/bootstrapper/clients/cipd"
)

// ID values for referring to the packages to be downloaded during bootstrapping
const (
	ExeId        = "exe"
	DepotToolsId = "depot-tools"
)

const (
	depotToolsPackage        = "go.chromium.org/infra/recipe_bundles/chromium.googlesource.com/chromium/tools/depot_tools"
	depotToolsPackageVersion = "refs/heads/main"
)

// DownloadPackages downloads the software packages necessary for bootstrapping the build.
//
// The package for the bootstrapped executable will be downloaded from CIPD, unless the input
// indicates that there is a CAS bundle. If the bootstrapped build uses a configuration for a
// dependent project, the depot_tools package will additionally be downloaded from CIPD to provide
// access to gclient. All packages will be downloaded to directories located under packagesRoot. The
// packageChannels argument allows for the caller to be notified when packages besides the
// executable have been downloaded. It is a map of a package ID to a buffered channel. When the
// corresponding package has been downloaded, the channel will receive the path to the package.
//
// If there is no error, a protobuf message indicating the source of the bootstrapped executable and
// the command used to execute the executable will be returned with a nil error. In the case of an
// error, the protobuf message and command will both be nil and the error will be non-nil.
func DownloadPackages(ctx context.Context, input *Input, packagesRoot string, packageChannels map[string]chan<- string) (*BootstrappedExe, []string, error) {
	if input == nil {
		return nil, nil, errors.Reason("nil input provided").Err()
	}
	if packagesRoot == "" {
		return nil, nil, errors.Reason("empty packagesRoot provided").Err()
	}
	for id, ch := range packageChannels {
		switch id {
		case DepotToolsId:
		case ExeId:
			return nil, nil, errors.Reason("channel provided for ExeId").Err()
		default:
			return nil, nil, errors.Reason("channel provided for unknown package ID %s", id).Err()
		}
		if cap(ch) == 0 {
			return nil, nil, errors.Reason("channel for package ID %s is unbuffered", id).Err()
		}
	}

	cipdPackages := make(map[string]*cipd.Package, 2)
	// This could be nil in the case of properties optional bootstrapping
	if input.propsProperties != nil {
		switch x := input.propsProperties.ConfigProject.(type) {
		case *BootstrapPropertiesProperties_TopLevelProject_:
			// Do nothing

		case *BootstrapPropertiesProperties_DependencyProject_:
			cipdPackages[string(DepotToolsId)] = &cipd.Package{
				Name:    depotToolsPackage,
				Version: depotToolsPackageVersion,
			}

		default:
			return nil, nil, errors.Reason("package handling for type %T is not implemented", x).Err()
		}
	}

	group, ctx := errgroup.WithContext(ctx)

	var exeSource isBootstrappedExe_Source
	var exePackagePath string

	if cas := input.casRecipeBundle; cas != nil {
		group.Go(func() error {
			var err error
			exeSource, exePackagePath, err = downloadExeFromCas(ctx, filepath.Join(packagesRoot, "cas"), cas)
			return err
		})
	} else {
		cipdPackages[string(ExeId)] = &cipd.Package{
			Name:    input.exeProperties.Exe.CipdPackage,
			Version: input.exeProperties.Exe.CipdVersion,
		}
	}

	if len(cipdPackages) != 0 {
		group.Go(func() error {
			source, packagePath, err := downloadPackagesFromCipd(ctx, filepath.Join(packagesRoot, "cipd"), cipdPackages, packageChannels)
			if err != nil {
				return err
			}
			if source != nil {
				exeSource = source
				exePackagePath = packagePath
			}
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, nil, err
	}

	exe := &BootstrappedExe{
		Source: exeSource,
		Cmd:    input.exeProperties.Exe.Cmd,
	}
	cmd := append([]string(nil), input.exeProperties.Exe.Cmd...)
	cmd[0] = filepath.Join(exePackagePath, cmd[0])
	return exe, cmd, nil
}

func downloadExeFromCas(ctx context.Context, outDir string, casRef *apipb.CASReference) (*BootstrappedExe_Cas, string, error) {
	casClient := cas.NewClient(ctx)

	logging.Infof(ctx, "downloading CAS isolated %s/%d", casRef.Digest.Hash, casRef.Digest.SizeBytes)
	if err := casClient.Download(ctx, outDir, casRef.CasInstance, casRef.Digest); err != nil {
		return nil, "", err
	}

	return &BootstrappedExe_Cas{Cas: casRef}, outDir, nil
}

func downloadPackagesFromCipd(ctx context.Context, cipdRoot string, packages map[string]*cipd.Package, packageChannels map[string]chan<- string) (*BootstrappedExe_Cipd, string, error) {
	logging.Infof(ctx, "downloading packages from CIPD")
	resolvedPackages, err := cipd.Ensure(ctx, chromeinfra.CIPDServiceURL, cipdRoot, packages)
	if err != nil {
		return nil, "", err
	}

	for id, ch := range packageChannels {
		ch <- filepath.Join(cipdRoot, id)
	}

	var exeSource *BootstrappedExe_Cipd
	var exePackagePath string
	if pkg, ok := resolvedPackages[string(ExeId)]; ok {
		exeSource = &BootstrappedExe_Cipd{
			Cipd: &Cipd{
				Server:           chromeinfra.CIPDServiceURL,
				Package:          pkg.Name,
				RequestedVersion: pkg.RequestedVersion,
				ActualVersion:    pkg.ActualVersion,
			},
		}
		exePackagePath = filepath.Join(cipdRoot, string(ExeId))
	}

	return exeSource, exePackagePath, nil
}
