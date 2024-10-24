// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// We only test the builder on a subset of platforms we support.
// Other platforms should be cross-compiled.
//go:build amd64 || (arm64 && darwin)

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"go.chromium.org/luci/cipkg/base/actions"
	"go.chromium.org/luci/cipkg/core"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/luciexe/build"
)

func TestRootStep(t *testing.T) {
	ftt.Run("root step", t, func(t *ftt.Test) {
		ctx := context.Background()
		s := NewRootStep(ctx, "root", "id")

		t.Run("ok", func(t *ftt.Test) {
			assert.Loosely(t, s.ID(), should.Equal("id"))

			executed := false
			err := s.RunSubstep(ctx, func(ctx context.Context, root *build.Step) error {
				executed = true
				return nil
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, executed, should.BeTrue)

			s.End()
			s.End() // idempotent
			assert.Loosely(t, s.IsEnded(), should.BeTrue)
		})

		t.Run("err", func(t *ftt.Test) {
			failed1 := fmt.Errorf("failed1")
			err := s.RunSubstep(ctx, func(ctx context.Context, root *build.Step) error {
				return failed1
			})
			assert.Loosely(t, fmt.Sprintf("%s", err), should.ContainSubstring("failed1"))

			err = s.RunSubstep(ctx, func(ctx context.Context, root *build.Step) error {
				return fmt.Errorf("failed2")
			})
			assert.Loosely(t, fmt.Sprintf("%s", err), should.ContainSubstring("failed2"))

			err = s.RunSubstep(ctx, func(ctx context.Context, root *build.Step) error {
				return nil
			})
			assert.Loosely(t, err, should.BeNil)

			assert.Loosely(t, fmt.Sprintf("%s", s.Err()), should.ContainSubstring("failed1"))
			assert.Loosely(t, fmt.Sprintf("%s", s.Err()), should.ContainSubstring("failed2"))

			postCheck := func() {
				err = s.RunSubstep(ctx, func(ctx context.Context, root *build.Step) error {
					return nil
				})
				assert.Loosely(t, err, should.ErrLike("ended"))
				assert.Loosely(t, s.IsEnded(), should.BeTrue)
			}

			t.Run("end", func(t *ftt.Test) {
				s.End()
				postCheck()
			})

			t.Run("end with err", func(t *ftt.Test) {
				s.EndWith(fmt.Errorf("failed end"))
				assert.Loosely(t, fmt.Sprintf("%s", s.Err()), should.ContainSubstring("failed end"))
				postCheck()
			})

			t.Run("end with err duplicate", func(t *ftt.Test) {
				assert.Loosely(t, s.Err().Error(), should.Equal("failed1\nfailed2"))

				s.EndWith(failed1)
				assert.Loosely(t, s.Err().Error(), should.Equal("failed1\nfailed2"))
				postCheck()
			})
		})

		t.Run("cancel", func(t *ftt.Test) {
			assert.Loosely(t, s.ID(), should.Equal("id"))

			ctx, cancel := context.WithCancel(ctx)
			done := make(chan struct{})

			var err error
			go func() {
				err = s.RunSubstep(ctx, func(ctx context.Context, root *build.Step) error {
					<-ctx.Done()
					return nil
				})
				close(done)
			}()
			cancel()
			<-done
			assert.Loosely(t, fmt.Sprintf("%s", err), should.ContainSubstring("cancled"))
		})
	})
}

func TestRootSteps(t *testing.T) {
	ftt.Run("root steps", t, func(t *ftt.Test) {
		ctx := context.Background()
		s := NewRootSteps()

		t.Run("ok", func(t *ftt.Test) {
			r, err := s.UpdateRoot(ctx, actions.Package{
				Action: &core.Action{Name: "first"}, ActionID: "first-xxxx",
				BuildDependencies: []actions.Package{
					{Action: &core.Action{Name: "second"}, ActionID: "second-xxxx"},
					{
						Action: &core.Action{
							Name:     "third",
							Metadata: &core.Action_Metadata{Luciexe: &core.Action_Metadata_LUCIExe{StepName: "third-step"}},
						},
						BuildDependencies: []actions.Package{
							{Action: &core.Action{Name: "fourth"}, ActionID: "fourth-xxxx"},
						},
						ActionID: "third-xxxx",
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, r.ID(), should.Equal("first-xxxx"))

			assert.Loosely(t, s.GetRoot("first-xxxx").ID(), should.Equal("first-xxxx"))
			assert.Loosely(t, s.GetRoot("second-xxxx").ID(), should.Equal("first-xxxx"))
			assert.Loosely(t, s.GetRoot("third-xxxx").ID(), should.Equal("third-xxxx"))
			assert.Loosely(t, s.GetRoot("fourth-xxxx").ID(), should.Equal("third-xxxx"))

			r, err = s.UpdateRoot(ctx, actions.Package{
				Action: &core.Action{Name: "third"}, ActionID: "third-xxxx",
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, r.ID(), should.Equal("third-xxxx"))
		})

		t.Run("conflict", func(t *ftt.Test) {
			r, err := s.UpdateRoot(ctx, actions.Package{
				Action: &core.Action{Name: "first-1"}, ActionID: "first-1-xxxx",
				BuildDependencies: []actions.Package{
					{Action: &core.Action{Name: "second"}, ActionID: "second-xxxx"},
					{Action: &core.Action{
						Name:     "third",
						Metadata: &core.Action_Metadata{Luciexe: &core.Action_Metadata_LUCIExe{StepName: "third-step"}},
					}, ActionID: "third-xxxx"},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, r.ID(), should.Equal("first-1-xxxx"))

			r, err = s.UpdateRoot(ctx, actions.Package{
				Action: &core.Action{Name: "first-2"}, ActionID: "first-2-xxxx",
				BuildDependencies: []actions.Package{
					{Action: &core.Action{
						Name:     "third",
						Metadata: &core.Action_Metadata{Luciexe: &core.Action_Metadata_LUCIExe{StepName: "third-step"}},
					}, ActionID: "third-xxxx"},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, r.ID(), should.Equal("first-2-xxxx"))

			assert.Loosely(t, s.GetRoot("second-xxxx").ID(), should.Equal("first-1-xxxx"))
			assert.Loosely(t, s.GetRoot("third-xxxx").ID(), should.Equal("third-xxxx"))

			r, err = s.UpdateRoot(ctx, actions.Package{
				Action: &core.Action{Name: "first-3"}, ActionID: "first-3-xxxx",
				BuildDependencies: []actions.Package{
					{Action: &core.Action{Name: "second"}, ActionID: "second-xxxx"},
				},
			})
			assert.Loosely(t, r, should.BeNil)
			assert.Loosely(t, fmt.Sprintf("%s", err), should.ContainSubstring("must only belong to one root"))

			r, err = s.UpdateRoot(ctx, actions.Package{
				Action: &core.Action{Name: "first-4"}, ActionID: "first-4-xxxx",
				RuntimeDependencies: []actions.Package{
					{Action: &core.Action{Name: "second"}, ActionID: "second-xxxx"},
				},
			})
			assert.Loosely(t, r, should.BeNil)
			assert.Loosely(t, fmt.Sprintf("%s", err), should.ContainSubstring("must only belong to one root"))

			r, err = s.UpdateRoot(ctx, actions.Package{
				Action: &core.Action{Name: "second"}, ActionID: "second-xxxx",
			})
			assert.Loosely(t, r, should.BeNil)
			assert.Loosely(t, fmt.Sprintf("%s", err), should.ContainSubstring("top level package"))
		})
	})
}

func TestStepUtilities(t *testing.T) {
	ftt.Run("step utilities", t, func(t *ftt.Test) {
		ctx := context.Background()

		t.Run("runStepCommand", func(t *ftt.Test) {
			self, err := os.Executable()
			assert.Loosely(t, err, should.BeNil)

			t.Run("without iostream", func(t *ftt.Test) {
				err := runStepCommand(ctx, &exec.Cmd{
					Path: self,
					Args: []string{self, "-help"},
				})
				assert.Loosely(t, err, should.BeNil)
			})
			t.Run("with iostream", func(t *ftt.Test) {
				b := bytes.NewBuffer(nil)
				err := runStepCommand(ctx, &exec.Cmd{
					Path:   self,
					Args:   []string{self, "-help"},
					Stdout: b,
					Stderr: b,
				})
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, b.String(), should.ContainSubstring("test"))
			})
		})
	})
}
