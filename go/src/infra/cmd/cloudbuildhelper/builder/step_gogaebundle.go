// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package builder

import (
	"context"
	"fmt"
	"go/build"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cmd/cloudbuildhelper/bundledesc"
	"go.chromium.org/infra/cmd/cloudbuildhelper/fileset"
	"go.chromium.org/infra/cmd/cloudbuildhelper/gaeapp"
	"go.chromium.org/infra/cmd/cloudbuildhelper/gitignore"
	"go.chromium.org/infra/cmd/cloudbuildhelper/godep"
)

// Names of Go sources roots in the bundle for GOPATH and modules mode.
const (
	goPathRoot = "_gopath"
	goModRoot  = "_gomod"
)

// Locations of files used to track dependencies in modules mode.
const (
	bundledGoModPath      = goModRoot + "/go.mod"
	bundledModulesTxtPath = goModRoot + "/vendor/modules.txt"
)

// What go dependency mechanisms the bundle should use.
type bundleMode string

const (
	bundleUnknown bundleMode = "unknown"
	bundleGoPath  bundleMode = "GOPATH"
	bundleModules bundleMode = "modules"
)

// runGoGAEBundleBuildStep executes manifest.GoGAEBundleBuildStep.
func runGoGAEBundleBuildStep(ctx context.Context, inv *stepRunnerInv) error {
	mode := bundleGoPath
	if inv.BuildStep.ModulesMode {
		mode = bundleModules
	}

	logging.Infof(ctx, "Bundling %q in %s mode", inv.BuildStep.GoGAEBundle, mode)

	// Hybrid bundles aren't allowed.
	if cur := currentMode(inv.Output); cur != bundleUnknown && cur != mode {
		return errors.Fmt("the bundle is already in %s mode, but being extended using %s mode", cur, mode)
	}

	yamlPath, err := filepath.Abs(inv.BuildStep.GoGAEBundle)
	if err != nil {
		return errors.Fmt("failed to convert the path %q to absolute: %w", inv.BuildStep.GoGAEBundle, err)
	}
	yamlBlob, err := os.ReadFile(yamlPath)
	if err != nil {
		return errors.Fmt("failed to read %q: %w", yamlPath, err)
	}
	appYaml, err := gaeapp.LoadAppYAML(yamlBlob)
	if err != nil {
		return errors.Fmt("failed to parse %q: %w", yamlPath, err)
	}

	// Read go runtime version from the YAML to know what Go build flags to use.
	//
	// It is either e.g. "go113" for GAE Standard or "go1.13" or just "go" for
	// GAE Flex.
	runtime := appYaml.Runtime
	logging.Infof(ctx, "Runtime is %q", runtime)
	if runtime != "go" && !strings.HasPrefix(runtime, "go1") {
		return errors.Fmt("%q is not a supported go runtime", runtime)
	}
	var goMinorVer int64
	if strings.HasPrefix(runtime, "go1") {
		runtime = strings.ReplaceAll(runtime, ".", "")
		if goMinorVer, err = strconv.ParseInt(runtime[3:], 10, 32); err != nil {
			return errors.Fmt("can't parse %q: %w", runtime, err)
		}
	}

	// The directory with `main` package.
	mainDir := filepath.Dir(yamlPath)

	// Get a build.Context as if we are building for linux amd64. We primarily use
	// it to call its MatchFile method to check build tags.
	bc := buildContext(mainDir, int(goMinorVer))

	// Load the main package and all its transitive dependencies (they are stored
	// as a graph of packages.Package that can be accessed via pointer chasing
	// from the loaded root packages.Package).
	mainPkg, err := loadPackageTree(ctx, bc)
	if err != nil {
		return err
	}
	if mode == bundleModules && (mainPkg.Module == nil || !mainPkg.Module.Main) {
		return errors.New("the main package is not a main module")
	}

	// In modules mode we should keep track of visited dependencies to build the
	// `go.mod` and `vendors/modules.txt` files describing packages from non-main
	// modules.
	//
	// If we are bundling multiple GAE apps via multiple GoGAEBundleBuildSteps, we
	// should keep adding dependencies additively. prepareModDeps(...) loads the
	// existing godep.Deps state (if any) from inv.Output to keep appending to it.
	var modDeps *godep.Deps
	if mode == bundleModules {
		modDeps, err = prepareModDeps(mainPkg.Module, inv.Output)
		if err != nil {
			return errors.Fmt("preparing dependency tracker: %w", err)
		}
	}

	// In modules mode the main module goes into "_gomod" and all other modules
	// go under "_gomod/vendor" (where Go wants them). In GOPATH mode all packages
	// should be under a single GOPATH root "_gopath/src".
	var packageDest func(pkg *packages.Package) (string, error)
	if mode == bundleModules {
		packageDest = func(pkg *packages.Package) (string, error) {
			switch {
			case pkg.Module == nil:
				return "", errors.New("not in a module")
			case pkg.Module.Main:
				var relToMod string
				switch {
				case pkg.PkgPath == pkg.Module.Path:
					relToMod = "."
				case !strings.HasPrefix(pkg.PkgPath, pkg.Module.Path+"/"):
					return "", errors.Fmt("module %q doesn't match the package import path", pkg.Module.Path)
				default:
					relToMod = pkg.PkgPath[len(pkg.Module.Path)+1:]
				}
				return filepath.Join(goModRoot, filepath.FromSlash(relToMod)), nil
			default:
				return filepath.Join(goModRoot, "vendor", filepath.FromSlash(pkg.PkgPath)), nil
			}
		}
	} else if mode == bundleGoPath {
		packageDest = func(pkg *packages.Package) (string, error) {
			return filepath.Join(goPathRoot, "src", filepath.FromSlash(pkg.PkgPath)), nil
		}
	} else {
		panic("impossible")
	}

	// Respect .gcloudignore files when traversing the GAE app directory to avoid
	// uploading unnecessary files as "static files". We don't care about any
	// other directories, since we pick only *.go files from them.
	excludedByIgnoreFile, err := gitignore.NewExcluder(mainDir, ".gcloudignore")
	if err != nil {
		return errors.Fmt("when loading .gcloudignore files: %w", err)
	}

	// The directory inside the bundle that should contain the `main` package.
	mainPkgDestRel, err := packageDest(mainPkg)
	if err != nil {
		return errors.Fmt("when finding where to put the main package: %w", err)
	}
	// Absolute path to it in the staging directory.
	mainPkgDestAbs := filepath.Join(inv.Manifest.ContextDir, mainPkgDestRel)

	// Record where we are going to put the app YAML inside the tarball.
	appYamlBundlePath := fmt.Sprintf("%s/%s",
		filepath.ToSlash(mainPkgDestRel),
		filepath.Base(yamlPath),
	)
	err = bundledesc.Modify(inv.Output, func(desc *bundledesc.Description) error {
		desc.GoGAEBundles = append(desc.GoGAEBundles, bundledesc.GoGAEBundle{
			AppYAML: appYamlBundlePath,
		})
		return nil
	})
	if err != nil {
		return errors.Fmt("updating bundle description: %w", err)
	}

	// Copy all files that make up "main" package (they can be only at the root
	// of `mainDir`), and copy all non-go files recursively (they can potentially
	// be referenced by static_files in app.yaml). We'll deal with Go dependencies
	// separately.
	err = inv.addFilesToOutput(ctx, mainDir, mainPkgDestAbs, func(absPath string, isDir bool) bool {
		switch {
		case excludedByIgnoreFile(absPath, isDir):
			return true // respect .gcloudignore exclusions
		case isDir:
			return false // do not exclude directories, they may contain static files
		}
		rel, err := relPath(mainDir, absPath)
		if err != nil {
			panic(fmt.Sprintf("impossible: %s", err))
		}
		switch {
		// Do not exclude non-code files regardless of where they are.
		case !isGoSourceFile(rel):
			return false
		// Exclude code files not in the mainDir. If they are needed, they'll be
		// discovered by the next step that traverses Go dependencies.
		case rel != filepath.Base(rel):
			return true
		// For code files in the mainDir, pick up only ones matching the build
		// context (linux amd64).
		default:
			matches, err := bc.MatchFile(mainDir, rel)
			if err != nil {
				logging.Warningf(ctx, "Failed to check whether %q matches the build context, skipping it: %s", absPath, err)
				return true
			}
			return !matches
		}
	})
	if err != nil {
		return err
	}

	// Drop empty .gcloudignore in the main directory. We already skipped ignored
	// files, but gcloud wants some .gcloudignore anyway, creating the default one
	// otherwise.
	if err := inv.Output.Overlay().AddFromMemory(filepath.Join(mainPkgDestRel, ".gcloudignore"), nil, nil); err != nil {
		return errors.Fmt("failed to create .gcloudignore: %w", err)
	}

	// We moved the main package to be somewhere under "_gomod" or "_gopath" to
	// make the bundle be a self-contained Go tree. But the authors of the
	// manifest expect main package files (in particular various GAE YAMLs) be
	// reachable under their original names. They don't really "know" or care
	// about "_gomod" and "_gopath". Make a symlink that puts the main directory
	// to where it is really expected to make YAMLs addressable.
	linkName, err := relPath(inv.Manifest.ContextDir, inv.BuildStep.Dest)
	if err != nil {
		return err
	}
	linkTarget, err := relPath(filepath.Dir(inv.BuildStep.Dest), mainPkgDestAbs)
	if err != nil {
		return err
	}
	if err := inv.Output.AddSymlink(linkName, linkTarget); err != nil {
		return errors.Fmt("failed to setup a symlink to the main package: %w", err)
	}

	// Packages for different go versions may have different files in them due to
	// filtering based on build tags. For each Go runtime we keep a separate map
	// of visited packages in this runtime. In practice it means if the GAE app
	// uses more than one runtime, all packages will be visited more than once.
	// Each separate visit may add more files to the output (or just revisit
	// already added ones, which is a noop).
	goDeps := inv.State.goDeps(runtime)

	errs := 0    // number of errors in packages.Visit below
	visited := 0 // number of packages actually visited
	copied := 0  // number of files copied

	reportErr := func(format string, args ...any) {
		logging.Errorf(ctx, format, args...)
		errs++
	}

	// Copy all transitive dependencies into the bundle.
	logging.Infof(ctx, "Copying transitive dependencies...")
	packages.Visit([]*packages.Package{mainPkg}, nil, func(pkg *packages.Package) {
		switch {
		case errs != 0:
			return // failing already
		case !goDeps.Add(pkg.ID):
			return // added it already in some previous build step
		case isStdlib(bc, pkg):
			return // we are not bundling stdlib packages
		default:
			visited++
		}

		// List of absolute file paths to copy into the output. They all must be in
		// the same directory (the package directory). At least one *.go file is
		// expected there.
		var filesToAdd []string

		// We visit GoFiles and IgnoredFiles because we want to recheck the build
		// tags using bc.MatchFile: packages.Load *always* uses the current Go
		// version tags, but we want to apply bc.ReleaseTags instead. It means we
		// may need to pick up some files rejected by packages.Load (they end up in
		// IgnoredFiles list), or reject some files from GoFiles.
		addGoFiles := func(paths []string) {
			for _, p := range paths {
				switch match, err := bc.MatchFile(filepath.Split(p)); {
				case err != nil:
					reportErr("Failed to check build tags of %q: %s", p, err)
				case match:
					filesToAdd = append(filesToAdd, p)
				}
			}
		}
		addGoFiles(pkg.GoFiles)
		addGoFiles(pkg.IgnoredFiles)

		if errs != 0 {
			return
		}
		if len(filesToAdd) == 0 {
			logging.Warningf(ctx, "Skipping package %s: no relevant *.go files", pkg.PkgPath)
			return
		}

		// packages.Package doesn't tell the package directory path. Verify all *.go
		// files we discovered come from the same directory. It is the package
		// directory we are after.
		srcDir := filepath.Dir(filesToAdd[0])
		for _, path := range filesToAdd {
			if filepath.Dir(path) != srcDir {
				reportErr("Expected %q to be under %q", path, srcDir)
			}
		}
		if errs != 0 {
			return
		}

		// Add non-go files, like *.c or files embedded via "go:embed". They must
		// be under the package directory, but may be in a subdirectory (in case of
		// "go:embed").
		addNonGoFile := func(path string) {
			rel, err := filepath.Rel(srcDir, path)
			if err != nil {
				reportErr("Filed to get relative path of %q", path)
				return
			}
			if rel == "." || !filepath.IsLocal(rel) {
				reportErr("Expected %q to be under %q", path, srcDir)
				return
			}
			filesToAdd = append(filesToAdd, path)
		}
		for _, path := range pkg.OtherFiles {
			addNonGoFile(path)
		}
		for _, path := range pkg.EmbedFiles {
			addNonGoFile(path)
		}
		if errs != 0 {
			return
		}

		// Decide the destination directory in the bundle based on the module.
		dstDir, err := packageDest(pkg)
		if err != nil {
			reportErr("Cant decide where to put %v: %s", pkg.GoFiles, err)
			return
		}

		// Add all discovered files to the tarball.
		for _, path := range filesToAdd {
			name, err := filepath.Rel(srcDir, path)
			if err != nil {
				// We verified paths already above.
				panic(fmt.Sprintf("impossible filepath.Rel error: %s", err))
			}
			err = inv.Output.AddFromDisk(path, filepath.Join(dstDir, name), nil)
			if err != nil {
				reportErr("Failed to copy %q to the tarball: %s", path, err)
			} else {
				copied++
			}
		}
		if errs != 0 {
			return
		}

		// In modules mode record this package as a dependency of the main module
		// to make it show up in the generated go.mod. We don't need to do anything
		// for packages from the main module: they aren't tracked in go.mod.
		if mode == bundleModules && !pkg.Module.Main {
			if err := modDeps.Add(pkg.PkgPath, pkg.Module.Path, pkg.Module.GoVersion); err != nil {
				reportErr("Error adding %q as a module dependency: %s", pkg.PkgPath, err)
			}
		}
	})
	if errs != 0 {
		return errors.New("failed to add Go files to the tarball, see the log")
	}
	logging.Infof(ctx, "Visited %d packages and copied %d files", visited, copied)

	// Generate go.mod and modules.txt describing bundled dependencies.
	if mode == bundleModules {
		logging.Infof(ctx, "Writing %s and %s", bundledGoModPath, bundledModulesTxtPath)
		state, err := modDeps.Save()
		if err != nil {
			return errors.Fmt("generating bundled go.mod: %w", err)
		}
		err = inv.Output.AddFromMemory(bundledGoModPath, state.GoMod, nil)
		if err != nil {
			return errors.Fmt("adding bundled go.mod: %w", err)
		}
		err = inv.Output.AddFromMemory(bundledModulesTxtPath, state.ModulesTxt, nil)
		if err != nil {
			return errors.Fmt("adding bundled modules.txt: %w", err)
		}
	}

	// In modules mode GAE considers the root of the **go module** to be the
	// application root. This is different from GOPATH mode, where the main
	// package directory (the one with app.yaml) is the application root.
	//
	// The application root is significant in two ways:
	//   1. It is cwd of the server process. This is important if the server loads
	//      any files at runtime (e.g. HTML templates) from cwd.
	//   2. It is the root path used when evaluating static files patterns in
	//      app.yaml.
	//
	// All our existing app YAMLs are written for GOPATH mode. And arguably this
	// is their truly correct form (they look just like one would expect, e.g.
	// like Python GAE YAMLs, with no spooky action at a distance). So here we
	// rewrite paths in them to be relatively to the module root to make them
	// compatible with modules mode.
	//
	// Note the GAE team considers this discrepancy between modes "Working as
	// Intended": b/323980048.
	if mode == bundleModules && mainPkg.PkgPath != mainPkg.Module.Path {
		if !strings.HasPrefix(mainPkg.PkgPath, mainPkg.Module.Path+"/") {
			panic("impossible at this point, already checked")
		}
		relToMod := mainPkg.PkgPath[len(mainPkg.Module.Path)+1:]
		if err := adjustYAMLPaths(ctx, appYaml, relToMod); err != nil {
			return errors.Fmt("adjusting %s: %w", appYamlBundlePath, err)
		}
		blob, err := appYaml.Save()
		if err != nil {
			return errors.Fmt("formatting %s: %w", appYamlBundlePath, err)
		}
		logging.Infof(ctx, "Adjusted %s to use correct paths:\n%s", appYamlBundlePath, blob)
		// Add the fixed YAML to the output's overlay set. That way if this YAML is
		// readded to the main output set again by **another** bundle step (as a
		// static file this time), it still will end up being correct in the final
		// staged output.
		if err := inv.Output.Overlay().AddFromMemory(appYamlBundlePath, blob, nil); err != nil {
			return errors.Fmt("rewriting %s: %w", appYamlBundlePath, err)
		}
	}

	// Drop a script that can be used to manually test correctness of this bundle:
	//
	// $ cd _gomod
	// $ eval `./goenv`
	// $ go build -v ./...
	//
	// This script isn't supposed to be used for anything important though.
	var scriptPath string
	var scriptBody string
	switch mode {
	case bundleModules:
		scriptPath = filepath.Join(goModRoot, "goenv")
		scriptBody = envScriptModules
	case bundleGoPath:
		scriptPath = filepath.Join(goPathRoot, "goenv")
		scriptBody = envScriptGoPath
	default:
		panic("impossible")
	}
	return inv.Output.AddFromMemory(scriptPath, []byte(scriptBody), &fileset.File{
		Executable: true,
	})
}

// buildContext returns a build.Context targeting linux-amd64.
//
// If goMinorVer is not 0, sets ReleaseTags to pick the specific go release.
func buildContext(mainDir string, goMinorVer int) *build.Context {
	bc := build.Default
	bc.GOARCH = "amd64"
	bc.GOOS = "linux"
	bc.Dir = mainDir
	if goMinorVer != 0 {
		bc.ReleaseTags = nil
		for i := 1; i <= goMinorVer; i++ {
			bc.ReleaseTags = append(bc.ReleaseTags, fmt.Sprintf("go1.%d", i))
		}
	}
	return &bc
}

// loadPackageTree loads the main package with its dependencies.
func loadPackageTree(ctx context.Context, bc *build.Context) (*packages.Package, error) {
	logging.Infof(ctx, "Loading the package tree...")

	// Note: this can actually download files into the modules cache when running
	// in module mode and thus can be quite slow.
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedModule |
			packages.NeedEmbedFiles,
		Context: ctx,
		Logf:    func(format string, args ...any) { logging.Debugf(ctx, format, args...) },
		Dir:     bc.Dir,
		Env:     append(os.Environ(), "GOOS="+bc.GOOS, "GOARCH="+bc.GOARCH),
	}, ".")
	if err != nil {
		return nil, errors.Fmt("failed to load the main package: %w", err)
	}

	// `packages.Load` records some errors inside packages.Package.
	errs := 0
	visited := 0
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		visited++
		for _, err := range pkg.Errors {
			logging.Errorf(ctx, "Error loading package %q: %s", pkg.ID, err)
			errs++
		}
	})
	if errs != 0 {
		return nil, errors.New("failed to load the package tree")
	}

	// We expect only one package to match our load query.
	if len(pkgs) != 1 {
		return nil, errors.Fmt("expected to load 1 package, but got %d", len(pkgs))
	}

	// Make sure it is indeed `main` and log its path in the package tree.
	mainPkg := pkgs[0]
	if mainPkg.PkgPath == "" {
		return nil, errors.New("could not figure out import path of the main package")
	}
	logging.Infof(ctx, "Import path is %q", mainPkg.PkgPath)
	if mainPkg.Name != "main" {
		return nil, errors.Fmt("only \"main\" package can be bundled, got %q", mainPkg.Name)
	}
	if mainPkg.Module != nil {
		logging.Infof(ctx, "Module is %q at %q", mainPkg.Module.Path, mainPkg.Module.Dir)
	}

	logging.Infof(ctx, "Transitively depends on %d packages (including stdlib)", visited-1)
	return mainPkg, nil
}

// relPath calls filepath.Rel and annotates the error.
func relPath(base, path string) (string, error) {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return "", errors.Fmt("failed to calculate rel(%q, %q): %w", base, path, err)
	}
	return rel, nil
}

// isGoSourceFile returns true if rel may be read by Go compiler.
//
// See https://golang.org/src/go/build/build.go.
func isGoSourceFile(rel string) bool {
	switch filepath.Ext(rel) {
	case ".go", ".c", ".cc", ".cxx", ".cpp", ".m", ".s", ".h", ".hh", ".hpp", ".hxx", ".f", ".F", ".f90", ".S", ".sx", ".swig", ".swigcxx":
		return true
	default:
		return false
	}
}

// isStdlib returns true if the package has its *.go files under GOROOT.
func isStdlib(bc *build.Context, pkg *packages.Package) bool {
	switch {
	case pkg.Name == "unsafe":
		return true // this package is a magical indicator and has no Go files
	case len(pkg.GoFiles) == 0:
		return false // assume other stdlib packages have Go files
	default:
		root := filepath.Clean(bc.GOROOT) + string(filepath.Separator)
		return strings.HasPrefix(pkg.GoFiles[0], root)
	}
}

// prepareModDeps loads godep.Deps based on the state in the output.
func prepareModDeps(main *packages.Module, out *fileset.Set) (*godep.Deps, error) {
	// Existing go.mod with all dependencies of the main module.
	mainModPath := filepath.Join(main.Dir, "go.mod")
	mainModBlob, err := os.ReadFile(mainModPath)
	if err != nil {
		return nil, errors.Fmt("reading main module's go.mod: %w", err)
	}
	mainMod, err := modfile.Parse(mainModPath, mainModBlob, nil)
	if err != nil {
		return nil, errors.Fmt("parsing main module's go.mod: %w", err)
	}

	// GAE Cloud Build deployer uses a concrete toolchain version hardcoded in its
	// configuration. Attempting to ask for another toolchain via go.mod directive
	// results in a deployment error. Remove references to custom toolchains and
	// Go patch versions from go.mod to avoid that.
	mainMod.Toolchain = nil
	if mainMod.Go != nil {
		if chunks := strings.Split(mainMod.Go.Version, "."); len(chunks) > 2 {
			mainMod.Go.Version = fmt.Sprintf("%s.%s", chunks[0], chunks[1])
		}
	}

	deps := godep.NewDeps(mainMod)

	// Load the existing state in the bundle, if any, to append to it.
	if bundleMod, ok := out.File(bundledGoModPath); ok {
		state := godep.SerializedState{}
		if state.GoMod, err = bundleMod.ReadAll(); err != nil {
			return nil, errors.Fmt("reading %q: %w", bundledGoModPath, err)
		}
		modulesTxt, ok := out.File(bundledModulesTxtPath)
		if !ok {
			return nil, errors.Fmt("unexpectedly missing %q", bundledModulesTxtPath)
		}
		if state.ModulesTxt, err = modulesTxt.ReadAll(); err != nil {
			return nil, errors.Fmt("reading %q: %w", bundledModulesTxtPath, err)
		}
		if err := deps.Load(state); err != nil {
			return nil, errors.Fmt("loading bundle deps: %w", err)
		}
	}

	return deps, nil
}

// currentMode returns the bundling mode of the output.
//
// It looks at existing files in the output to decide.
func currentMode(out *fileset.Set) bundleMode {
	if _, ok := out.File(goPathRoot); ok {
		return bundleGoPath
	}
	if _, ok := out.File(goModRoot); ok {
		return bundleModules
	}
	return bundleUnknown
}

var pathElemReg = regexp.MustCompile(`^[a-zA-Z0-9\-._]*$`)

// adjustYAMLPaths prepends the given path to all static paths in the YAML.
func adjustYAMLPaths(ctx context.Context, app *gaeapp.AppYAML, p string) error {
	// Here `p` is some relative Go package name. It should be a simple
	// slash-separated clean path without any funny shell escape symbols
	// (including spaces). Check this to be sure.
	if p != path.Clean(p) {
		return errors.Fmt("unexpected format of the app path %q", p)
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." || !pathElemReg.MatchString(part) {
			return errors.Fmt("unexpected format of the app path %q", p)
		}
	}

	// The default server binary inside the GAE container is named "main" and
	// it is in PATH. If the service just launches this binary, we can change
	// the current directory it runs from (`entrypoint` is passed to a shell, so
	// we can just use "cd <dir> && main ...").
	//
	// If the service is doing something else (this is very rate) don't mess with
	// it, we don't know if it can run from a different directory.
	if app.Entrypoint == "" {
		app.Entrypoint = "main"
	}
	if app.Entrypoint == "main" || strings.HasPrefix(app.Entrypoint, "main ") {
		app.Entrypoint = fmt.Sprintf("cd %s && %s", p, app.Entrypoint)
	} else {
		logging.Warningf(ctx, "Unrecognized \"entrypoint\", leaving it as is: %s", app.Entrypoint)
	}

	// Attempt to "relocate" all static paths. Unfortunately some of them are
	// regexps, and "relocating" them 100% correctly is hard (if they use some
	// weird expressions involving "^"). Let's just hope there are no non-trivial
	// regexps there...
	prefix := p + "/"
	for _, h := range app.Handlers {
		if h.StaticDir != "" {
			h.StaticDir = prefix + h.StaticDir
		}
		if h.StaticFiles != "" {
			h.StaticFiles = prefix + h.StaticFiles
		}
		if h.Upload != "" {
			h.Upload = regexp.QuoteMeta(prefix) + strings.TrimPrefix(h.Upload, "^")
		}
	}

	return nil
}

// envScriptGoPath is a script that modifies Go env vars to point to files
// in the tarball built for GOPATH mode. Can be used to manually test the
// tarball's soundness.
const envScriptGoPath = `#!/usr/bin/env bash
cd "$(dirname "${BASH_SOURCE[0]}")"

echo "export GOARCH=amd64"
echo "export GOOS=linux"
echo "export GO111MODULE=off"
echo "export GOPATH=$(pwd)"
`

// envScriptModules is a script that modifies Go env vars to point to files
// in the tarball built for modules mode. Can be used to manually test the
// tarball's soundness.
const envScriptModules = `#!/usr/bin/env bash
echo "export GOARCH=amd64"
echo "export GOOS=linux"
echo "export GO111MODULE=on"
echo "unset GOPATH"
`
