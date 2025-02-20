// Copyright (c) 2023 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"

	"go.chromium.org/luci/cipd/client/cipd/ensure"
	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/exec"
	"go.chromium.org/luci/common/flag/stringmapflag"

	"go.chromium.org/infra/experimental/crderiveinputs/inputpb"
)

//go:embed all:embed
var embedded embed.FS

type EmbedTools string

func ExtractEmbed(args *Args) (EmbedTools, error) {
	if err := os.RemoveAll(filepath.Join(args.CacheDirectory, "embed")); err != nil {
		return "", nil
	}

	// NOTE - this is dumb, but I didn't find a routine to recursively one FS to
	// a location on disk...
	err := fs.WalkDir(embedded, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(args.CacheDirectory, path)
		if d.IsDir() {
			err := os.Mkdir(target, 0700)
			if os.IsExist(err) {
				return nil
			}
			return err
		}

		infile, err := embedded.Open(path)
		if err != nil {
			return err
		}
		outfile, err := os.Create(target)
		if err != nil {
			return err
		}
		defer func() {
			if err := outfile.Close(); err != nil {
				panic(err)
			}
		}()
		_, err = io.Copy(outfile, infile)
		return err
	})
	if err != nil {
		return "", err
	}
	return EmbedTools(filepath.Join(args.CacheDirectory, "embed")), nil
}

type GclientGitDep struct {
	Repo string `json:"repo"`
	Rev  string `json:"rev"`
}

type GclientCipdPackage struct {
	Package string `json:"package"`
	Version string `json:"version"`
}

type GclientCipdDep struct {
	Packages []GclientCipdPackage
}

type GclientHook struct {
	Name      string   `json:"name"`
	Pattern   string   `json:"pattern"`
	Action    []string `json:"action"`
	Condition string   `json:"condition"`
}

type GclientDEPS struct {
	UseRelativePaths  bool                      `json:"use_relative_paths"`
	GitDeps           map[string]GclientGitDep  `json:"git_deps"`
	CipdDeps          map[string]GclientCipdDep `json:"cipd_deps"`
	Vars              GClientVars               `json:"vars"`
	RecurseDeps       []string                  `json:"recursedeps"`
	Hooks             []GclientHook             `json:"hooks"`
	GclientGNArgsFile string                    `json:"gclient_gn_args_file"`
	GclientGNArgs     map[string]any            `json:"gclient_gn_args"`
	GitDependencies   string                    `json:"git_dependencies"`
}

type GClientVars struct {
	HostOS    string              `json:"host_os"`
	HostCPU   string              `json:"host_cpu"`
	TargetOS  string              `json:"target_os"`
	TargetCPU string              `json:"target_cpu"`
	StrVars   stringmapflag.Value `json:"str_vars"`
	BoolVars  boolmapflag         `json:"bool_vars"`
}

func (v *GClientVars) AddToFlagset(fs *flag.FlagSet) {
	fs.StringVar(&v.HostOS, "gclient-host-os", "linux",
		"Use to set gclient host_os argument (e.g. linux).")

	fs.StringVar(&v.HostCPU, "gclient-host-cpu", "x64",
		"Use to set gclient host_os argument (e.g. x64).")

	fs.StringVar(&v.TargetOS, "gclient-target-os", "linux",
		"Use to set gclient target_os argument (e.g. linux).")

	fs.StringVar(&v.TargetCPU, "gclient-target-cpu", "x64",
		"Use to set gclient target_cpu argument (e.g. x64).")

	fs.Var(&v.StrVars, "gclient-str",
		"Use to set gclient string argument (e.g. varname=value).")

	fs.Var(&v.BoolVars, "gclient-bool",
		"Use to set gclient bool arguments (e.g. varname=true, varname=false, varname (implies true)).")
}

func (v GClientVars) MakeArgs() []string {
	ret := make([]string, 0, 4+len(v.StrVars)+len(v.BoolVars))
	ret = append(ret, "--target-os", v.TargetOS)
	ret = append(ret, "--target-cpu", v.TargetCPU)
	ret = append(ret, "--host-os", v.HostOS)
	ret = append(ret, "--host-cpu", v.HostCPU)
	for varname, value := range v.StrVars {
		ret = append(ret, "--str-var", fmt.Sprintf("%s=%s", varname, value))
	}
	for varname, value := range v.BoolVars {
		ret = append(ret, "--bool-var", fmt.Sprintf("%s=%t", varname, value))
	}
	return ret
}

func (v GClientVars) ToGclientInputs() map[string]*inputpb.Manifest_GclientVal {
	ret := make(map[string]*inputpb.Manifest_GclientVal, len(v.BoolVars)+len(v.StrVars)+4)
	mkStr := func(s string) *inputpb.Manifest_GclientVal {
		ret := &inputpb.Manifest_GclientVal{}
		ret.Value = &inputpb.Manifest_GclientVal_StrVal{StrVal: s}
		return ret
	}
	mkBool := func(b bool) *inputpb.Manifest_GclientVal {
		ret := &inputpb.Manifest_GclientVal{}
		ret.Value = &inputpb.Manifest_GclientVal_BoolVal{BoolVal: b}
		return ret
	}

	for k, v := range v.StrVars {
		ret[k] = mkStr(v)
	}
	for k, v := range v.BoolVars {
		ret[k] = mkBool(v)
	}
	ret["target_os"] = mkStr(v.TargetOS)
	ret["target_cpu"] = mkStr(v.TargetCPU)
	ret["host_os"] = mkStr(v.HostOS)
	ret["host_cpu"] = mkStr(v.HostCPU)

	return ret
}

func (e EmbedTools) ParseDEPS(ctx context.Context, oracle *Oracle, solutionRoot, repoRoot string, vars GClientVars, root bool, hookImpls []HookImpl) error {
	PIN("using embedded gclient to resolve DEPS files - could use resolved depot_tools instead.")

	eg := &errgroup.Group{}

	// TODO: pass in full path to DEPS instead
	depsPath := path.Join(repoRoot, "DEPS")
	Logger.Infof("Processing %s", depsPath)
	SBOM("%q - assuming all dependencies selected by current conditions are necessary for build", depsPath)

	DEPSContent, err := oracle.ReadFullString(depsPath)
	if err != nil {
		return err
	}

	cmd := exec.Command(ctx, "vpython3", append([]string{
		filepath.Join(string(e), "scripts", "gclient_deps_parser.py"),
		"--DEPS-filename", depsPath,
	}, vars.MakeArgs()...)...)
	cmd.Stdin = strings.NewReader(DEPSContent)
	cmd.Stderr = os.Stderr
	outJSON, err := cmd.Output()
	if err != nil {
		return err
	}

	parsed := &GclientDEPS{}
	if err := json.Unmarshal(outJSON, parsed); err != nil {
		return err
	}

	if parsed.GitDependencies == "SUBMODULES" {
		TODO("%q - git_dependencies: SUBMODULES", depsPath)
	}

	usedGitDeps := stringset.New(len(parsed.GitDeps))

	for subdir, gitdep := range parsed.GitDeps {
		subdir, gitdep := subdir, gitdep
		usedGitDeps.Add(subdir)

		eg.Go(func() error {
			target := subdir
			if parsed.UseRelativePaths {
				target = path.Join(repoRoot, subdir)
			}
			return oracle.PinGit(target, gitdep.Repo, gitdep.Rev)
		})
	}

	for subdir, cipddeps := range parsed.CipdDeps {
		subdir, cipddeps := subdir, cipddeps
		eg.Go(func() error {
			target := subdir
			if parsed.UseRelativePaths {
				target = path.Join(repoRoot, subdir)
			}
			for _, pkg := range cipddeps.Packages {
				if err := oracle.PinCipd(target, ensure.PackageDef{
					PackageTemplate:   pkg.Package,
					UnresolvedVersion: pkg.Version,
				}, nil, ""); err != nil {
					return err
				}
			}
			return nil
		})
	}

	for _, subdir := range parsed.RecurseDeps {
		if !usedGitDeps.Has(subdir) {
			continue
		}

		subdir := subdir
		eg.Go(func() error {
			target := subdir
			if parsed.UseRelativePaths {
				target = path.Join(repoRoot, subdir)
			}
			return e.ParseDEPS(ctx, oracle, solutionRoot, target, parsed.Vars, false, hookImpls)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	hookCwd := repoRoot
	if !parsed.UseRelativePaths {
		hookCwd = solutionRoot
	}
	for _, hook := range parsed.Hooks {
		hook := hook

		eg.Go(func() error {
			for _, hookImpl := range hookImpls {
				handled, err := hookImpl.HandleHook(oracle, hookCwd, &hook)
				if err != nil {
					return err
				}
				if handled {
					return nil
				}
			}
			TODO("%q - resolve %q hook {%v}", depsPath, hook.Name, hook.Action)
			return nil
		})
	}

	if root && parsed.GclientGNArgsFile != "" {
		LEAKY("Currently only generating GNArgsFile for %q - this empirically matches gclient behavior, but difficult to determine in gclient.py code", repoRoot)
		var argsFile string
		if parsed.UseRelativePaths {
			argsFile = path.Join(repoRoot, parsed.GclientGNArgsFile)
		} else {
			argsFile = parsed.GclientGNArgsFile
		}
		content := strings.Builder{}
		fmt.Fprintln(&content, "# Generated from 'DEPS'")

		keys := make([]string, 0, len(parsed.GclientGNArgs))
		for key := range parsed.GclientGNArgs {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, varname := range keys {
			switch x := parsed.GclientGNArgs[varname].(type) {
			case bool:
				fmt.Fprintf(&content, "%s = %t\n", varname, x)
			case string:
				fmt.Fprintf(&content, "%s = %q\n", varname, x)
			default:
				panic(fmt.Sprintf("impossible GclientGNArgs type %T", x))
			}
		}

		oracle.PinRawFile(argsFile, content.String(), "ParseDEPS")
	}

	return eg.Wait()
}
