// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package source

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/common/exec"
	"go.chromium.org/luci/common/logging"

	"infra/tools/pkgbuild/pkg/spec"
)

// Resolver can resolve the latest valid version from source definition.
type Resolver interface {
	Resolve(ctx context.Context, plat, dir string, src *spec.Spec_Create_Source) (info *SourceInfo, err error)
}

type resolver struct{}

func NewResolver() Resolver {
	return &resolver{}
}

//go:embed resolve_git.py
var resolveGitScript string

func (r *resolver) Resolve(ctx context.Context, plat, dir string, s *spec.Spec_Create_Source) (*SourceInfo, error) {
	switch s.Method.(type) {
	case *spec.Spec_Create_Source_Git:
		return r.resolveGitSource(ctx, dir, s.GetGit())
	case *spec.Spec_Create_Source_Script:
		return r.resolveScriptSource(ctx, plat, dir, s.GetScript())
	case *spec.Spec_Create_Source_Url:
		u := s.GetUrl()
		ext := u.Extension
		if ext == "" {
			ext = ".tar.gz"
		}
		return &SourceInfo{
			Version: u.Version,
			Source: &SourceInfo_Http{
				Http: &SourceInfo_HTTP{
					Url: []string{u.DownloadUrl},
					Ext: ext,
				},
			},
		}, nil
	case *spec.Spec_Create_Source_Cipd:
		return nil, fmt.Errorf("cipd source method not implemented")
	default:
		return nil, fmt.Errorf("unknown source method")
	}
}

func (r *resolver) resolveGitSource(ctx context.Context, dir string, git *spec.GitSource) (*SourceInfo, error) {
	cmd := r.command(ctx, "resolve_git.py", resolveGitScript, dir)

	in, err := json.Marshal(git)
	if err != nil {
		return nil, err
	}
	cmd.Args = append(cmd.Args, string(in))

	out, err := output(ctx, cmd)
	if err != nil {
		return nil, err
	}
	info := &SourceInfo{}
	if err := protojson.Unmarshal([]byte(out), info); err != nil {
		return nil, err
	}

	return info, nil
}

func (r *resolver) resolveScriptSource(ctx context.Context, plat, dir string, script *spec.ScriptSource) (*SourceInfo, error) {
	// Get version
	cmd := r.command(ctx, script.GetName()[0], "", dir)
	cmd.Args = append(cmd.Args, script.GetName()[1:]...)
	cmd.Args = append(cmd.Args, "latest")
	cmd.Env = append(cmd.Env, fmt.Sprintf("_3PP_PLATFORM=%s", plat))

	out, err := output(ctx, cmd)
	if err != nil {
		return nil, err
	}
	version := strings.TrimSpace(string(out))

	if script.UseFetchCheckoutWorkflow {
		// Fetch script will be invoked in the preUnpack hook.
		return &SourceInfo{Version: version}, nil
	}

	// Get download urls
	cmd = r.command(ctx, script.GetName()[0], "", dir)
	cmd.Args = append(cmd.Args, script.Name[1:]...)
	cmd.Args = append(cmd.Args, "get_url")
	cmd.Env = append(cmd.Env, fmt.Sprintf("_3PP_VERSION=%s", version))
	cmd.Env = append(cmd.Env, fmt.Sprintf("_3PP_PLATFORM=%s", plat))

	out, err = output(ctx, cmd)
	if err != nil {
		return nil, err
	}
	http := &SourceInfo_HTTP{}
	if err = protojson.Unmarshal(out, http); err != nil {
		return nil, err
	}

	return &SourceInfo{
		Version: version,
		Source: &SourceInfo_Http{
			Http: http,
		},
	}, nil
}

func output(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	out, err := cmd.Output()
	if err != nil {
		err = fmt.Errorf("executing %s failed: %w", cmd, err)

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			logging.Errorf(ctx, "%s:\n %s", err, exitErr.Stderr)
		}
	}
	return out, err
}

func (r *resolver) command(ctx context.Context, name, script, cwd string) *exec.Cmd {
	var cmd *exec.Cmd
	switch filepath.Ext(name) {
	case ".py":
		cmd = exec.Command(ctx, "vpython3")
		if script != "" {
			// script should only be used with embedded python scripts.
			cmd.Args = append(cmd.Args, "-")
			cmd.Stdin = strings.NewReader(script)
		} else {
			cmd.Args = append(cmd.Args, name)
		}
	case ".sh":
		cmd = exec.Command(ctx, "bash", name)
		if script != "" {
			panic("invalid embedded script: " + name)
		}
	default:
		panic("unknown script: " + name)
	}
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	return cmd
}
