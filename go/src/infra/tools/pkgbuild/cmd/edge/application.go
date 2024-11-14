// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/cipd/client/cipd/platform"
	"go.chromium.org/luci/cipkg/base/actions"
	"go.chromium.org/luci/cipkg/base/generators"
	"go.chromium.org/luci/cipkg/base/workflow"
	"go.chromium.org/luci/cipkg/core"
	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/system/filesystem"
	"go.chromium.org/luci/luciexe/build"
	"go.chromium.org/luci/provenance/api/snooperpb/v1"
	"go.chromium.org/luci/provenance/client"

	"infra/tools/pkgbuild/pkg/spec/loader"
)

type Application struct {
	// Input properties defined in proto for pkgbuild recipe
	// This can be both passed from command-line flags or proto message if
	// luciexe is enabled.
	*Input

	// Optional buildbucket ID from build state.
	// This may be attached to the uploaded package indicating its source.
	BuildID int64

	// Logging level for pkgbuild
	LoggingLevel logging.Level

	// Display help message
	Help bool

	// List of packages to be built. If empty, build all packages in the pool.
	Packages []string

	// Local Package Manager
	PackageManager *workflow.LocalPackageManager
}

// Parse parses input arguments.
func (a *Application) Parse(args []string) error {
	fs := flag.NewFlagSet("pkgbuild", flag.ContinueOnError)

	fs.Var(&a.LoggingLevel, "logging-level", "Logging level for pkgbuild.")

	fs.StringVar(&a.TargetPlatform, "target-platform", a.TargetPlatform, "Target CIPD platform.")

	fs.StringVar(&a.StorageDir, "storage-dir", a.StorageDir, "Required; Local storage directory for build and cache packages.")
	fs.StringVar(&a.SpecPool, "spec-pool", a.SpecPool, "Required; Spec pool directory for finding 3pp specs.")

	fs.BoolVar(&a.Update, "update", a.Update, "If true, packages will be updated to latest version.")
	fs.BoolVar(&a.Build, "build", a.Build, "If true, packages will be built.")
	fs.BoolVar(&a.Upload, "upload", a.Upload, "If true, packages will be uploaded to CIPD.")

	fs.BoolVar(&a.UpdateSourceLock, "update-source-lock", a.UpdateSourceLock, "If true, source lock file under the package spec directory will be updated.")
	fs.StringVar(&a.CipdService, "cipd-service", a.CipdService, "CIPD service URL for downloading and uploading packages.")
	fs.StringVar(&a.CipdPackagePrefix, "cipd-package-prefix", a.CipdPackagePrefix, "Required; The prefix to use for uploading built packages.")

	fs.StringVar(&a.SnoopyService, "snoopy-service", a.SnoopyService, "Snoopy service URL for reporting artifact hash.")

	fs.BoolVar(&a.Help, "help", false, "Display help message.")

	fs.Usage = func() {
		fmt.Fprint(fs.Output(), "Usage: pkgbuild [OPTION]... [PKG_NAME]...\n")
		fmt.Fprint(fs.Output(), "Build listed packages. If no package name (e.g. tools/ninja) is provided, build all packages in the pool.\n\n")
		fmt.Fprint(fs.Output(), "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if a.Help {
		fs.Usage()
		return nil
	}

	if a.StorageDir == "" || a.SpecPool == "" {
		fs.Usage()
		return fmt.Errorf("storage-dir and spec-pool are required")
	}

	if !a.Update && a.UpdateSourceLock {
		return fmt.Errorf("-update=true is required for updating source lock file")
	}

	// Maybe separate build and upload?
	if !a.Build && a.Upload {
		return fmt.Errorf("-build=true is required for -upload")
	}

	if a.CipdPackagePrefix == "" {
		fs.Usage()
		return fmt.Errorf("cipd-package-prefix is required")
	}

	a.Packages = fs.Args()

	pm, err := workflow.NewLocalPackageManager(a.StorageDir)
	if err != nil {
		return err
	}
	a.PackageManager = pm

	return nil
}

// NewBuilder creates the PackageBuilder used for building packages based on
// the configuration and platform.
func (a *Application) NewBuilder(ctx context.Context) (*PackageBuilder, error) {
	specLoaderCfg := loader.DefaultSpecLoaderConfig(a.TargetPlatform)
	specLoaderCfg.CIPDPackagePrefix = a.CipdPackagePrefix
	loader, err := loader.NewSpecLoader(a.SpecPool, specLoaderCfg)
	if err != nil {
		return nil, errors.Annotate(err, "failed to load specs").Err()
	}

	target := generators.PlatformFromCIPD(a.TargetPlatform)

	return &PackageBuilder{
		Packages: a.PackageManager,
		Platforms: generators.Platforms{
			Build:  generators.CurrentPlatform(),
			Host:   target,
			Target: target,
		},

		CipdService: a.CipdService,
		CIPDHost:    platform.CurrentPlatform(),
		CIPDTarget:  a.TargetPlatform,
		SpecLoader:  loader,

		BuildTempDir: filepath.Join(a.StorageDir, "temp"),
	}, nil
}

// UploadCIPDAll build and register the cipd if Application.Upload set to true.
func (a *Application) UploadCIPDAll(ctx context.Context, pkgs []actions.Package) (err error) {
	step, ctx := build.StartStep(ctx, "upload packages")
	defer func() { step.End(err) }()

	clt, err := client.MakeProvenanceClient(ctx, a.SnoopyService)
	if err != nil {
		err = errors.Annotate(err, "failed to create provenance client").Err()
		return
	}

	tmp, err := os.MkdirTemp("", "pkgbuild-")
	if err != nil {
		err = errors.Annotate(err, "failed to create tmp dir").Err()
		return
	}
	defer filesystem.RemoveAll(tmp)

	// Ensure we only upload each package once.
	var errs []error
	toBeUpload := make(map[string]actions.Package)
	for _, pkg := range flattenPkgs(pkgs) {
		cipd := pkg.Action.Metadata.GetCipd()
		if cipd.GetName() == "" || cipd.DisableUpload {
			continue
		}

		id := cipd.Name + ":" + pkg.DerivationID
		if p, ok := toBeUpload[id]; ok {
			if !proto.Equal(cipd, p.Action.Metadata.Cipd) {
				logging.Errorf(ctx, "cipd metadata missmatch: %s %s", cipd, p.Action.Metadata.Cipd)
				errs = append(errs, fmt.Errorf("cipd metadata missmatch for %s", id))
			}
		} else {
			toBeUpload[id] = pkg
		}
	}

	for _, pkg := range toBeUpload {
		errs = append(errs, a.uploadCIPD(ctx, clt, tmp, pkg))
	}
	err = errors.Join(errs...)

	return
}

// provenanceClient interface for snoopy ProvenanceClient.
type provenanceClient interface {
	ReportCipd(context.Context, *snooperpb.ReportCipdRequest, ...grpc.CallOption) (*emptypb.Empty, error)
}

func flattenPkgs(pkgs []actions.Package) []actions.Package {
	var flatten []actions.Package
	for _, pkg := range pkgs {
		flatten = append(flatten, flattenPkg(pkg)...)
	}
	return flatten
}

func flattenPkg(pkg actions.Package) []actions.Package {
	pkgs := []actions.Package{pkg}
	for _, dep := range slices.Concat(pkg.BuildDependencies, pkg.RuntimeDependencies) {
		pkgs = append(pkgs, flattenPkg(dep)...)
	}
	return pkgs
}

// uploadCIPD uploads the package provided. If reporter function is not nil,
// it will be called after the cipd file generated in tmp, to report the
// cipd package to snoopy service.
func (a *Application) uploadCIPD(ctx context.Context, clt provenanceClient, tmp string, pkg actions.Package) (err error) {
	cipdPkg := toCIPDPackage(pkg)
	if cipdPkg == nil {
		return nil
	}

	// This helps avoiding attaching possible ambiguous tags from different
	// builders to same native package during cross-compiling.
	if pkg.Action.Metadata.Cipd.DisableUpload {
		return
	}

	step, ctx := build.StartStep(ctx, pkg.Action.Metadata.Cipd.String())
	defer func() { step.End(err) }()

	var tags, refs []string
	versionTag := fmt.Sprintf("version:%s", pkg.Action.Metadata.Cipd.Version)
	if cipdPkg.checkVersion(ctx, a.CipdService, versionTag) != nil {
		// Only append version tag or refs if it haven't been attached to another
		// package.
		// We may have packages with different derivation ID representing same
		// version. It's not ideal but it will persist until we move to a different
		// version schema.
		tags = append(tags, versionTag)
		tags = append(tags, pkg.Action.Metadata.Cipd.Tags...)
		refs = append(refs, pkg.Action.Metadata.Cipd.Refs...)
	}

	// Package is available in cipd
	if err = cipdPkg.check(ctx, a.CipdService); err == nil {
		err = errors.Join(
			cipdPkg.setTags(ctx, a.CipdService, tags),
			cipdPkg.setRefs(ctx, a.CipdService, refs),
		)
		return
	} else if !errors.Is(err, errPackgeNotExist) {
		return
	}

	// Skip if Package is not available locally.
	// Ignore error here.
	if err := cipdPkg.Handler.IncRef(); err != nil {
		return nil
	}
	defer cipdPkg.Handler.DecRef()

	// Only attach buildbucket id if we actually build it.
	if a.BuildID != 0 {
		tags = append(tags, fmt.Sprintf("build_id:%d", a.BuildID))
	}
	name, iid, err := cipdPkg.upload(ctx, tmp, a.CipdService, tags, refs)
	if err != nil {
		return
	}

	if clt != nil && iid != "" {
		// Report package info to server to trigger provenance generation.
		// Ignore error here.
		if _, err := clt.ReportCipd(ctx, &snooperpb.ReportCipdRequest{
			CipdReport: &snooperpb.CipdReport{
				PackageName: name,
				Iid:         iid,
				EventTs:     timestamppb.New(time.Now()),
			},
		}); err != nil {
			// Error during reporting won't block the package build.
			logging.Warningf(ctx, "report cipd package to snoopy failed: %s: %s", pkg.Action.Metadata.Cipd.Name, err)
		}
	}

	return
}

type PackageBuilder struct {
	Packages  core.PackageManager
	Platforms generators.Platforms

	CipdService string
	CIPDHost    string
	CIPDTarget  string
	SpecLoader  *loader.SpecLoader

	BuildTempDir string

	loaded []generators.Generator

	// For testing purpose
	packageExecutor *workflow.PackageExecutor
}

// Load 3pp spec by name and convert it into a cipkg.Package. If the 3pp spec
// depends on other specs, they will also be loaded and added. The package is
// added to the builder so its content will be available after BuildAll
// executed.
func (b *PackageBuilder) Load(ctx context.Context, name string) error {
	g, err := b.SpecLoader.FromSpec(name, b.CIPDHost, b.CIPDTarget)
	if err != nil {
		return err
	}
	b.loaded = append(b.loaded, g)
	return nil
}

// BuildAll builds loaded packages.
// - If skipUploaded is true and a package we loaded is available in cipd, we
// don't need to rebuild or make it available locally. Note this also mean if
// any extra tags supplied, they won't be attached to the package.
// - If a package we added depends on a package available in cipd, we can
// download the prebuilt package using PreExecuteHook.
// - Otherwise, we will build the package locally.
func (b *PackageBuilder) BuildAll(ctx context.Context, skipUploaded bool) ([]actions.Package, error) {
	// TODO(fancl): we should use a real temp directory.
	if err := filesystem.RemoveAll(b.BuildTempDir); err != nil {
		return nil, err
	}
	if err := os.Mkdir(b.BuildTempDir, os.ModePerm); err != nil {
		return nil, err
	}

	builder := workflow.NewBuilder(b.Platforms, b.Packages, actions.NewActionProcessor())

	pkgs, err := builder.GeneratePackages(ctx, b.loaded)
	if err != nil {
		return nil, err
	}

	newPkgs := slices.Clone(pkgs)
	if skipUploaded {
		if newPkgs, err = b.filterUnuploaded(ctx, newPkgs); err != nil {
			return nil, err
		}
	}

	executor := b.packageExecutor
	if executor == nil {
		if executor, err = b.defaultPackageExecutor(ctx, newPkgs); err != nil {
			return nil, err
		}
	}

	// Make packages available. We still return all packages regardless of the
	// error.
	if err := builder.BuildPackages(ctx, executor, newPkgs, true); err != nil {
		return pkgs, err
	}

	return pkgs, nil
}

// filterUnuploaded filters out package has been built and available in cipd.
func (b *PackageBuilder) filterUnuploaded(ctx context.Context, pkgs []actions.Package) (ret []actions.Package, err error) {
	step, ctx := build.StartStep(ctx, "compute packages to be built")
	defer func() { step.End(err) }()

	checked := stringset.New(len(pkgs))
	for _, pkg := range pkgs {
		if !checked.Add(pkg.ActionID) {
			continue
		}

		cipdPkg := toCIPDPackage(pkg)
		if cipdPkg == nil {
			continue
		}

		if err = cipdPkg.check(ctx, b.CipdService); err != nil {
			if errors.Is(err, errPackgeNotExist) {
				err = nil
				ret = append(ret, pkg)
			} else {
				return
			}
		}
	}

	return
}

func (b *PackageBuilder) defaultPackageExecutor(ctx context.Context, pkgs []actions.Package) (*workflow.PackageExecutor, error) {
	rootSteps := NewRootSteps()
	for _, pkg := range pkgs {
		if _, err := rootSteps.UpdateRoot(ctx, pkg); err != nil {
			return nil, err
		}
	}

	expanded := stringset.New(len(pkgs))
	preExpandFn := func(ctx context.Context, pkg actions.Package) error {
		if !expanded.Add(pkg.ActionID) {
			return nil
		}

		cipdPkg := toCIPDPackage(pkg)
		if cipdPkg == nil {
			return nil
		}

		r := rootSteps.GetRoot(pkg.ActionID)
		return r.RunSubstep(ctx, func(ctx context.Context, root *build.Step) error {
			return cipdPkg.download(ctx, b.CipdService, true)
		})
	}

	type rootContextType string
	var rootContext rootContextType = "rootContext"

	preExecFn := func(ctx context.Context, pkg actions.Package) (context.Context, error) {
		r := rootSteps.GetRoot(pkg.ActionID)
		return context.WithValue(ctx, rootContext, r), nil
	}

	execFn := func(ctx context.Context, cfg *workflow.ExecutionConfig, drv *core.Derivation) error {
		r := ctx.Value(rootContext).(*RootStep)

		err := r.RunSubstep(ctx, func(ctx context.Context, root *build.Step) (err error) {
			s, ctx := build.StartStep(ctx, fmt.Sprintf("build %s", drv.Name))
			defer func() { s.End(err) }()

			stepOutput := s.Log("stdout")
			cmd := exec.CommandContext(ctx, drv.Args[0], drv.Args[1:]...)
			cmd.Path = drv.Args[0]
			cmd.Dir = cfg.WorkingDir
			cmd.Stdin = cfg.Stdin
			cmd.Stdout = io.MultiWriter(stepOutput, cfg.Stdout)
			cmd.Stderr = io.MultiWriter(stepOutput, cfg.Stderr)
			cmd.Env = append(slices.Clone(drv.Env), "out="+cfg.OutputDir)

			fmt.Fprintf(s.Log("execution details"), "%#v\n", cmd)
			err = cmd.Run()

			return
		})

		return err
	}

	postExecFn := func(ctx context.Context, pkg actions.Package, execErr error) error {
		r := rootSteps.GetRoot(pkg.ActionID)
		if execErr != nil || r.ID() == pkg.ActionID {
			r.EndWith(execErr)
		}
		return nil
	}

	return workflow.NewPackageExecutor(b.BuildTempDir, preExpandFn, preExecFn, postExecFn, execFn), nil
}
