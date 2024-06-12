// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.chromium.org/luci/cipkg/base/actions"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/system/filesystem"
	"go.chromium.org/luci/luciexe/build"
)

type cipdPackage actions.Package

var (
	errPackgeNotExist     = errors.New("no such derivation tag")
	errAmbiguousPackgeTag = errors.New("ambiguity when resolving the derivation tag")
)

func toCIPDPackage(pkg actions.Package) *cipdPackage {
	if pkg.Action.Metadata.GetCipd().GetName() == "" {
		return nil
	}
	ret := cipdPackage(pkg)
	return &ret
}

func (pkg *cipdPackage) check(ctx context.Context, cipdService string) error {
	return pkg.checkVersion(ctx, cipdService, pkg.derivationTag())
}

func (pkg *cipdPackage) checkVersion(ctx context.Context, cipdService, version string) error {
	cipd := pkg.Action.Metadata.GetCipd()

	var b bytes.Buffer
	cmd := cipdCommand("describe", cipd.Name,
		"-service-url", cipdService,
		"-version", version,
	)
	cmd.Stderr = &b

	if err := runStepCommand(ctx, cmd); err != nil {
		out := b.String()
		if strings.Contains(out, "no such tag") || strings.Contains(out, "no such package") {
			return errPackgeNotExist
		} else if strings.Contains(out, "ambiguity when resolving the tag") {
			return errAmbiguousPackgeTag
		}
		return err
	}

	return nil
}

func (pkg *cipdPackage) upload(ctx context.Context, workdir, cipdService string, tags, refs []string) (name string, iid string, err error) {
	cipd := pkg.Action.Metadata.GetCipd()
	name = cipd.Name

	step, ctx := build.StartStep(ctx, fmt.Sprintf("creating cipd package %s:%s with %s", name, pkg.derivationTag(), cipd.Version))
	defer func() { step.End(err) }()

	// If .cipd file already exists, assume it has been uploaded.
	out := filepath.Join(workdir, pkg.DerivationID+".cipd")
	if _, err = os.Stat(out); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return
	}

	iid, err = buildCIPD(ctx, name, pkg.Handler.OutputDirectory(), out)
	if err != nil {
		_ = filesystem.RemoveAll(out)
		err = errors.Annotate(err, "failed to build cipd package").Err()
		return
	}

	if err = registerCIPD(ctx, cipdService, out, append([]string{pkg.derivationTag()}, tags...), refs); err != nil {
		err = errors.Annotate(err, "failed to register cipd package").Err()
		return
	}

	return
}

func (pkg *cipdPackage) download(ctx context.Context, cipdService string, ignoreErr bool) (err error) {
	cipd := pkg.Action.Metadata.GetCipd()

	step, ctx := build.StartStep(ctx, fmt.Sprintf("downloading cipd package %s:%s", cipd.Name, pkg.derivationTag()))
	defer func() { step.End(err) }()

	errOut := bytes.NewBuffer(nil)
	err = pkg.Handler.Build(func() error {
		cmd := cipdCommand("export",
			"-service-url", cipdService,
			"-root", pkg.Handler.OutputDirectory(),
			"-ensure-file", "-",
		)
		cmd.Stdin = strings.NewReader(fmt.Sprintf("%s %s", cipd.Name, pkg.derivationTag()))
		cmd.Stderr = errOut

		return runStepCommand(ctx, cmd)
	})

	if ignoreErr && err != nil {
		step.SetSummaryMarkdown(errOut.String())
		err = nil
	}

	return
}

func (pkg *cipdPackage) setTags(ctx context.Context, cipdService string, tags []string) error {
	cipd := pkg.Action.Metadata.GetCipd()

	if len(tags) == 0 {
		return nil
	}

	cmd := cipdCommand("set-tag", cipd.Name,
		"-service-url", cipdService,
		"-version", pkg.derivationTag(),
	)

	for _, tag := range tags {
		cmd.Args = append(cmd.Args, "-tag", tag)
	}

	return runStepCommand(ctx, cmd)
}

func (pkg *cipdPackage) setRefs(ctx context.Context, cipdService string, refs []string) error {
	cipd := pkg.Action.Metadata.GetCipd()

	if len(refs) == 0 {
		return nil
	}

	cmd := cipdCommand("set-ref", cipd.Name,
		"-service-url", cipdService,
		"-version", pkg.derivationTag(),
	)

	for _, ref := range refs {
		cmd.Args = append(cmd.Args, "-ref", ref)
	}

	return runStepCommand(ctx, cmd)
}

func (pkg *cipdPackage) derivationTag() string {
	return "derivation:" + pkg.DerivationID
}

// TODO(fancl): to support version_file we need to generate package definition
// instead of passing all arguments through cli.
func buildCIPD(ctx context.Context, name, src, dst string) (Iid string, err error) {
	resultFile := dst + ".json"
	cmd := cipdCommand("pkg-build",
		"-name", name,
		"-in", src,
		"-out", dst,
		"-json-output", resultFile,
	)

	if err := runStepCommand(ctx, cmd); err != nil {
		return "", err
	}

	f, err := os.Open(resultFile)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var result struct {
		Result struct {
			Package    string
			InstanceID string `json:"instance_id"`
		}
	}
	if err := json.NewDecoder(f).Decode(&result); err != nil {
		return "", err
	}

	return result.Result.InstanceID, nil
}

func registerCIPD(ctx context.Context, cipdService, pkg string, tags, refs []string) error {
	cmd := cipdCommand("pkg-register", pkg,
		"-service-url", cipdService,
	)

	for _, tag := range tags {
		cmd.Args = append(cmd.Args, "-tag", tag)
	}
	for _, ref := range refs {
		cmd.Args = append(cmd.Args, "-ref", ref)
	}

	return runStepCommand(ctx, cmd)
}

func cipdCommand(arg ...string) *exec.Cmd {
	cipd, err := exec.LookPath("cipd")
	if err != nil {
		cipd = "cipd"
	}

	var cmd *exec.Cmd
	// Use cmd to execute batch file on windows.
	if filepath.Ext(cipd) == ".bat" {
		cmd = exec.Command("cmd.exe", append([]string{"/C", cipd}, arg...)...)
	} else {
		cmd = exec.Command(cipd, arg...)
	}
	return cmd
}
