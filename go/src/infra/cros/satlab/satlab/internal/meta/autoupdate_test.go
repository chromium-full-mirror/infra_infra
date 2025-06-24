// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"go.chromium.org/luci/common/cli"
)

func md5Sum(s string) string {
	h := md5.New()
	io.WriteString(h, s)
	return hex.EncodeToString(h.Sum(nil))
}

// fakeDeps implements the Deps interface for testing.
type fakeDeps struct {
	envGetterFunc  func(key string) string
	runCommandFunc func(app *cli.Application, args []string) int
	openFileFunc   func(name string) (*os.File, error)
	reExecFuncFunc func(args []string) error
}

func (f *fakeDeps) RunCommand(app *cli.Application, args []string) int {
	return f.runCommandFunc(app, args)
}

func (f *fakeDeps) OpenFile(name string) (*os.File, error) {
	return f.openFileFunc(name)
}

func (f *fakeDeps) EnvGetter(key string) string {
	return f.envGetterFunc(key)
}

func (f *fakeDeps) ReExecFunc(args []string) error {
	return f.reExecFuncFunc(args)
}

func TestShouldUpdate(t *testing.T) {
	fd := &fakeDeps{
		envGetterFunc: func(key string) string {
			if key == SkipAutoUpdateEnvVar {
				return "true"
			}
			return ""
		},
	}
	updater := NewUpdaterWithDeps(&cli.Application{}, fd, "")
	if updater.shouldUpdate() {
		t.Error("shouldUpdate returned true when SKIP_AUTO_UPDATE is true")
	}

	fd.envGetterFunc = func(key string) string {
		if key == SkipAutoUpdateEnvVar {
			return "false"
		}
		return ""
	}
	if !updater.shouldUpdate() {
		t.Error("shouldUpdate returned false when SKIP_AUTO_UPDATE is false")
	}
}

func TestCalculateFileHash(t *testing.T) {
	content := "Hello, World!"
	tmpFile, err := os.CreateTemp("", "testhash")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	fd := &fakeDeps{
		openFileFunc: os.Open,
	}
	updater := NewUpdaterWithDeps(&cli.Application{}, fd, tmpFile.Name())
	hash, err := updater.calculateFileHash(tmpFile.Name())
	if err != nil {
		t.Fatalf("calculateFileHash returned error: %v", err)
	}
	expected := md5Sum(content)
	if hash != expected {
		t.Errorf("Expected hash %s, got %s", expected, hash)
	}
}

func TestAttemptUpdate(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "binary")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	initialContent := "version1"
	if _, err := tmpFile.Write([]byte(initialContent)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	fd := &fakeDeps{
		runCommandFunc: func(app *cli.Application, args []string) int {
			os.WriteFile(tmpFile.Name(), []byte("version2"), 0644)
			return 0
		},
		openFileFunc: os.Open,
	}
	updater := NewUpdaterWithDeps(&cli.Application{}, fd, tmpFile.Name())
	updated, err := updater.attemptUpdate()
	if err != nil {
		t.Fatalf("attemptUpdate returned error: %v", err)
	}
	if !updated {
		t.Error("attemptUpdate did not detect update")
	}

	os.WriteFile(tmpFile.Name(), []byte("version1"), 0644)
	fd.runCommandFunc = func(app *cli.Application, args []string) int {
		os.WriteFile(tmpFile.Name(), []byte("version1"), 0644)
		return 0
	}
	updated, err = updater.attemptUpdate()
	if err != nil {
		t.Fatalf("attemptUpdate returned error: %v", err)
	}
	if updated {
		t.Error("attemptUpdate detected update when there was none")
	}
}

func TestReExecuteBinary(t *testing.T) {
	originalArgs := []string{"arg1", "arg2"}
	origOsArgs := os.Args
	os.Args = append([]string{origOsArgs[0]}, originalArgs...)
	defer func() { os.Args = origOsArgs }()

	fd := &fakeDeps{
		reExecFuncFunc: func(args []string) error {
			if strings.Join(args, " ") != strings.Join(originalArgs, " ") {
				return fmt.Errorf("unexpected args: got %v, expected %v", args, originalArgs)
			}
			return nil
		},
	}
	updater := NewUpdaterWithDeps(&cli.Application{}, fd, "")
	if err := updater.reExecuteBinary(); err != nil {
		t.Errorf("reExecuteBinary returned error: %v", err)
	}
}
