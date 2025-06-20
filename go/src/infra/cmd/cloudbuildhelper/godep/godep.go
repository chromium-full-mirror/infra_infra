// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package godep contains description of external dependencies of a Go module.
package godep

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"

	"go.chromium.org/luci/common/errors"
)

// Deps describes external modules and packages the main module depends on.
//
// It describes a transitive set of external dependencies of a **subset** of
// the main module (i.e. only some of its packages, not all of them). It is
// a minimal description needed to construct `go.mod` and `vendor/...`
// sufficient for compiling this preselected subset of the main module in
// `-mod=vendor` mode (i.e. without downloading any modules).
//
// Note that the set of packages tracked by Deps is similar to what is produced
// by `go mod vendor`, except Go vendoring considers all packages in the main
// module, not a subset of them. This results in suboptimal dependency trees.
//
// Deps is additive: given a Deps representing dependencies of some set of
// packages in the main module it possible to add more entries to it to cover
// dependencies of some other package (i.e. we never need to remove anything
// and we don't need to know all "roots" in advance).
//
// In serialized state Deps is represented by generated `go.mod` and
// `vendor/modules.txt` files. `go.mod` contains module-level dependencies and
// `vendor/modules.txt` additionally contains package-level dependencies. They
// both are needed by `go build` in vendor mode.
//
// Note that the generated `go.mod` is pretty minimal and also not "tidy" at all
// in terms of `go mod tidy`. It is never going to be used for updating
// dependencies or even downloading them. It doesn't need extra structure that
// real tidy `go.mod` files have. Maintaining this structure is non-trivial.
// Since we aren't downloading anything nor contacting module registry at all,
// we don't need to worry about `go.sum` either.
type Deps struct {
	base     *modfile.File         // the original main module's `go.mod`
	modules  map[string]*moduleDep // dependency module name => moduleDep
	packages map[string]string     // dependency package  => module that has it
}

// SerializedState is a pair of generated `go.mod` and `modules.txt`.
type SerializedState struct {
	GoMod      []byte // generated `go.mod` body
	ModulesTxt []byte // generated `vendor/modules.txt` body
}

// moduleDep described a module the main module depends on.
//
// Version information is taken from the original `go.mod`.
type moduleDep struct {
	name         string   // module name e.g. "cloud.google.com/go"
	requiredVer  string   // version required by "require" statement
	replacedName string   // module name or path in "replace" directive
	replacedVer  string   // module version in "replace" directive
	goVer        string   // go version requested by this module, if known
	packages     []string // list of imported packages from this module
}

// summary is a summary of requested module name and its replacement.
//
// For `==` comparisons and error messages. Doesn't end up in any generated
// files.
func (m *moduleDep) summary() string {
	toks := make([]string, 0, 5)
	toks = append(toks, modfile.AutoQuote(m.name))
	toks = append(toks, modfile.AutoQuote(m.requiredVer))
	if m.replacedName != "" {
		toks = append(toks, "=>")
		toks = append(toks, modfile.AutoQuote(m.replacedName))
		if m.replacedVer != "" {
			toks = append(toks, modfile.AutoQuote(m.replacedVer))
		}
	}
	return strings.Join(toks, " ")
}

// NewDeps creates a new empty dependency set.
//
// `base` is the parsed `go.mod` of the main module. It will be used to look up
// versions of dependencies and other necessary information stored there. It is
// assumed to be valid.
func NewDeps(base *modfile.File) *Deps {
	return &Deps{
		base:     base,
		modules:  map[string]*moduleDep{},
		packages: map[string]string{},
	}
}

// Add records a dependency on a package in a non-main module.
//
// `goVer` is the Go version this module wants, if known. This can be obtained
// from this module's `go.mod`.
func (s *Deps) Add(pkg, mod, goVer string) error {
	if pkg != mod && !strings.HasPrefix(pkg, mod+"/") {
		return errors.Reason("package %q is not in module %q", pkg, mod).Err()
	}

	newPkg := true
	if prevMod := s.packages[pkg]; prevMod != "" {
		newPkg = false
		if prevMod != mod {
			return errors.Reason("conflicting modules for package %q: %q vs %q", pkg, mod, prevMod).Err()
		}
	}

	modDep := s.modules[mod]
	if modDep == nil {
		var err error
		if modDep, err = lookupMod(s.base, mod); err != nil {
			return err
		}
		modDep.goVer = goVer
		s.modules[mod] = modDep
	}

	if modDep.goVer != goVer {
		return errors.Reason("conflicting go version requirement for module %q: %q vs %q", mod, goVer, modDep.goVer).Err()
	}

	if newPkg {
		s.packages[pkg] = mod
		modDep.packages = append(modDep.packages, pkg)
	}

	return nil
}

// Save produces minimal `go.mod` and `modules.txt` with added dependencies.
func (s *Deps) Save() (SerializedState, error) {
	// Start with the minimal go.mod, parse and extend it. It appears parsing is
	// the only way to construct a valid *modfile.File instance.
	var lines []string
	emit := func(format string, args ...any) {
		lines = append(lines, fmt.Sprintf(format, args...))
	}
	if s.base.Module != nil {
		emit("module %s", modfile.AutoQuote(s.base.Module.Mod.Path))
	}
	if s.base.Go != nil {
		emit("go %s", modfile.AutoQuote(s.base.Go.Version))
	}
	if s.base.Toolchain != nil {
		emit("toolchain %s", modfile.AutoQuote(s.base.Toolchain.Name))
	}

	mod, err := modfile.Parse("go.mod", []byte(strings.Join(lines, "\n")), nil)
	if err != nil {
		return SerializedState{}, errors.Annotate(err, "bad go.mod").Err()
	}

	sortedModDeps := make([]*moduleDep, 0, len(s.modules))
	for _, modDep := range s.modules {
		sortedModDeps = append(sortedModDeps, modDep)
	}
	sort.Slice(sortedModDeps, func(i, j int) bool {
		return sortedModDeps[i].name < sortedModDeps[j].name
	})

	for _, modDep := range sortedModDeps {
		mod.AddNewRequire(modDep.name, modDep.requiredVer, false)
		if modDep.replacedName != "" {
			err := mod.AddReplace(modDep.name, "", modDep.replacedName, modDep.replacedVer)
			if err != nil {
				return SerializedState{}, errors.Annotate(err, "adding replacement for %q", modDep.name).Err()
			}
		}
	}
	mod.Cleanup()

	goModBlob, err := mod.Format()
	if err != nil {
		return SerializedState{}, errors.Annotate(err, "formating go.mod").Err()
	}

	// modules.txt looks like this:
	//
	//	# cloud.google.com/go v0.112.0
	//	## explicit; go 1.19
	//	cloud.google.com/go
	//	cloud.google.com/go/civil
	//	cloud.google.com/go/internal
	//	# github.com/davecgh/go-spew v1.1.1
	//	## explicit
	//	github.com/davecgh/go-spew/spew
	//	# go.chromium.org/luci v0.0.0-20230103053340-8a57daa72e32 => ../go.chromium.org/luci
	//	## explicit; go 1.21
	//	go.chromium.org/luci/appengine/bqlog
	//	# golang.org/x/mobile v0.0.0-20191031020345-0945064e013a => golang.org/x/mobile v0.0.0-20170111200746-6f0c9f6df9bb
	//	## explicit
	//	golang.org/x/mobile/bind
	//	# golang.org/x/mobile => golang.org/x/mobile v0.0.0-20170111200746-6f0c9f6df9bb
	//	# go.chromium.org/luci => ../go.chromium.org/luci

	lines = nil // reset the existing buffer to reuse emit(...)

	var replaceLines []string // will be appended at the end

	for _, modDep := range sortedModDeps {
		// E.g. "golang.org/x/mobile v0.0.0-20170111200746-6f0c9f6df9bb".
		replace := ""
		if modDep.replacedName != "" {
			replace = modDep.replacedName
			if modDep.replacedVer != "" {
				replace += " " + modDep.replacedVer
			}
		}

		if replace != "" {
			emit("# %s %s => %s", modDep.name, modDep.requiredVer, replace)
			replaceLines = append(replaceLines, fmt.Sprintf("# %s => %s", modDep.name, replace))
		} else {
			emit("# %s %s", modDep.name, modDep.requiredVer)
		}

		if modDep.goVer != "" {
			emit("## explicit; go %s", modDep.goVer)
		} else {
			emit("## explicit")
		}

		sort.Strings(modDep.packages)
		for _, pkg := range modDep.packages {
			emit("%s", pkg)
		}
	}
	for _, replaceLine := range replaceLines {
		emit("%s", replaceLine)
	}
	emit("")

	return SerializedState{
		GoMod:      goModBlob,
		ModulesTxt: []byte(strings.Join(lines, "\n")),
	}, nil
}

// Load loads the previously save state to allow extending it.
//
// It expects go.mod and modules.txt as produced by Save, based on the same
// original `go.mod`.
func (s *Deps) Load(blobs SerializedState) error {
	s.modules = map[string]*moduleDep{}
	s.packages = map[string]string{}

	mod, err := modfile.Parse("go.mod", blobs.GoMod, nil)
	if err != nil {
		return errors.Annotate(err, "when loading go.mod with bundle deps").Err()
	}

	// Loaded go.mod module name must match the original go.mod. This check should
	// be enough to catch a situation when the same Deps is attempted to bundle
	// multiple different main modules at once (this will not work).
	modName := func(m *modfile.File) string {
		if m.Module != nil {
			return m.Module.Mod.Path
		}
		return ""
	}
	if got, want := modName(s.base), modName(mod); got != want {
		return errors.Reason("a bundle for %q is reused for another module %q", want, got).Err()
	}

	// Information in modules.txt and in the original go.mod is enough to
	// reconstruct the full state. As a consistency check, verify modules.txt
	// entries are also in the loaded `mod`, at the same revisions as in the
	// original go.mod.
	var curMod *moduleDep
	for _, line := range strings.Split(string(blobs.ModulesTxt), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// "# <module name>[, <version>, =>, <replacement>]".
		if strings.HasPrefix(line, "# ") {
			// We care only about the module name here.
			modName := strings.SplitN(strings.TrimSpace(line[2:]), " ", 2)[0]

			// This module should be present in the go.mod we are loading as well as
			// in the original go.mod, and the entries must agree.
			loadMod, err := lookupMod(mod, modName)
			if err != nil {
				return errors.Annotate(err, "modules.txt doesn't match bundled go.mod").Err()
			}
			baseMod, err := lookupMod(s.base, modName)
			if err != nil {
				return errors.Annotate(err, "modules.txt doesn't match original go.mod").Err()
			}
			if loadVer, baseVer := loadMod.summary(), baseMod.summary(); loadVer != baseVer {
				return errors.Fmt("conflict between original and bundled go.mod: %s != %s", loadVer, baseVer)
			}

			// All good, start collecting packages for this module.
			curMod = loadMod
			continue
		}

		// "## explicit" or "## explicit; go 1.23".
		if strings.HasPrefix(line, "## explicit") {
			if curMod == nil {
				return errors.Reason("malformed modules.txt: explicit directive before the module name").Err()
			}
			if strings.HasPrefix(line, "## explicit; go ") {
				curMod.goVer = strings.TrimSpace(line[len("## explicit; go "):])
			} else {
				curMod.goVer = ""
			}
			continue
		}

		// Here we should see a list of package names, all for the current module.
		// This will be checked by Add.
		if curMod == nil {
			return errors.Reason("malformed modules.txt: no module directive before the list of packages").Err()
		}
		if err := s.Add(line, curMod.name, curMod.goVer); err != nil {
			return errors.Annotate(err, "malformed modules.txt").Err()
		}
	}

	return nil
}

// lookupMod looks up external module info in a go.mod file.
func lookupMod(f *modfile.File, mod string) (*moduleDep, error) {
	dep := &moduleDep{name: mod}

	// Find "require <mod> <ver>" line. There must be exactly one.
	var vers []module.Version
	for _, req := range f.Require {
		if req.Mod.Path == mod {
			vers = append(vers, req.Mod)
		}
	}
	switch {
	case len(vers) == 0:
		return nil, errors.Reason("module %q is not present in go.mod", mod).Err()
	case len(vers) > 1:
		return nil, errors.Reason("module %q is required more than once in go.mod", mod).Err()
	}
	dep.requiredVer = vers[0].Version

	// Find optional "replace" line. There can be at most one.
	var replaces []module.Version
	for _, rep := range f.Replace {
		if rep.Old.Path == mod {
			replaces = append(replaces, rep.New)
		}
	}
	switch {
	case len(replaces) > 1:
		return nil, errors.Reason("module %q is replaced more than once in go.mod", mod).Err()
	case len(replaces) == 1:
		dep.replacedName = replaces[0].Path
		dep.replacedVer = replaces[0].Version
	}

	// All other go.mod directives don't affect what we generate in Save.
	return dep, nil
}
