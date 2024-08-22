// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/vpython/common"
)

const vpythonTestReexec = "_VPYTHON_TEST_REEXEC"

func cpythonEnsureFile() string {
	s := `
@Subdir ${prefix}3.8
infra/3pp/tools/cpython3/${platform} version:2@3.8.10.chromium.25
@Subdir ${prefix}3.11
infra/3pp/tools/cpython3/${platform} version:2@3.11.5.chromium.30
`
	var prefix string
	if runtime.GOOS == "darwin" {
		prefix = "Contents/Resources/"
	}
	return strings.ReplaceAll(s, "${prefix}", prefix)
}

func vpythonPath(root string) string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(root, "vpython.exe")
	case "darwin":
		return filepath.Join(root, "Contents", "MacOS", "vpython")
	default:
		return filepath.Join(root, "vpython")
	}
}

func setupExecutable(tb testing.TB, root string) {
	tb.Helper()

	cmd := common.CIPDCommand("export", "-root", root, "-ensure-file", "-")
	cmd.Stdin = strings.NewReader(cpythonEnsureFile())
	if err := cmd.Run(); err != nil {
		tb.Fatalf("failed to export cpython packages: %v", err)
	}

	self, err := os.Executable()
	if err != nil {
		tb.Fatalf("failed to get self: %v", err)
	}
	src, err := os.Open(self)
	if err != nil {
		tb.Fatalf("failed to open src: %v", err)
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(vpythonPath(root)), fs.ModePerm); err != nil {
		tb.Fatalf("failed to mkdir dst: %v", err)
	}
	dst, err := os.Create(vpythonPath(root))
	if err != nil {
		tb.Fatalf("failed to open dst: %v", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		tb.Fatalf("failed to copy self: %v", err)
	}
	srcInfo, err := src.Stat()
	if err != nil {
		tb.Fatalf("failed to stat src: %v", err)
	}
	if err := dst.Chmod(srcInfo.Mode()); err != nil {
		tb.Fatalf("failed to chmod dst: %v", err)
	}
}

func generateSpec(t *ftt.Test, dir, ver string) string {
	t.Helper()

	spec := filepath.Join(dir, "test.vpython")
	f, err := os.Create(spec)
	defer f.Close()
	assert.Loosely(t, err, should.BeNil)
	_, err = fmt.Fprintf(f, `python_version: "%s"`, ver)
	assert.Loosely(t, err, should.BeNil)
	return spec
}

func vpython(root string, arg ...string) *exec.Cmd {
	arg = append([]string{"-vpython-root", filepath.Join(root, "vpython-root")}, arg...)
	cmd := exec.Command(vpythonPath(root), arg...)
	cmd.Env = append(os.Environ(), vpythonTestReexec+"=1")
	return cmd
}

func TestMain(m *testing.M) {
	if os.Getenv(vpythonTestReexec) != "" || os.Getenv("_CIPKG_EXEC_CMD") != "" {
		if err := os.Unsetenv(vpythonTestReexec); err != nil {
			panic(err)
		}
		if err := Main(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	os.Exit(m.Run())
}

func TestPythonBasic(t *testing.T) {
	root := t.TempDir()
	setupExecutable(t, root)
	if err := os.Setenv(cipd.EnvCacheDir, filepath.Join(root, ".cipd")); err != nil {
		t.Fatalf("failed setenv %s", err)
	}

	ftt.Run("main", t, func(t *ftt.Test) {
		for _, ver := range []string{"3.8", "3.11"} {
			spec := generateSpec(t, t.TempDir(), ver)

			t.Run(ver, func(t *ftt.Test) {
				t.Run("ok", func(t *ftt.Test) {
					out, err := vpython(root, "-vpython-spec", spec, "-c", "print(123)").CombinedOutput()
					assert.Loosely(t, strings.TrimSpace(string(out)), should.Equal("123"))
					assert.Loosely(t, err, should.BeNil)
				})

				t.Run("exit code", func(t *ftt.Test) {
					out, err := vpython(root, "-vpython-spec", spec, "-c", "exit(42)").CombinedOutput()
					assert.Loosely(t, string(out), should.BeEmpty)
					var exitErr *exec.ExitError
					assert.Loosely(t, errors.As(err, &exitErr), should.BeTrue)
					assert.Loosely(t, exitErr.ExitCode(), should.Equal(42))
				})

				t.Run("preserve NoDefaultCurrentDirectoryInExePath state", func(t *ftt.Test) {
					cmd := vpython(root, "-vpython-spec", spec, "-c",
						"import os; print(os.environ['NoDefaultCurrentDirectoryInExePath'])",
					)
					cmd.Env = append(cmd.Env, "NoDefaultCurrentDirectoryInExePath=SOMETHING")
					out, err := cmd.CombinedOutput()
					assert.Loosely(t, strings.TrimSpace(string(out)), should.Equal("SOMETHING"))
					assert.Loosely(t, err, should.BeNil)
				})

				t.Run("help", func(t *ftt.Test) {
					out, err := vpython(root, "-vpython-spec", spec, "-help").CombinedOutput()
					assert.Loosely(t, string(out), should.ContainSubstring("Usage of vpython:"))
					assert.Loosely(t, err, should.BeNil)
				})
			})
		}
	})
}
