package main

import (
	"context"
	"os/exec"

	"go.chromium.org/luci/luciexe/build"
)

// Entry-point for compiled build-wheels binary.
func main() {
	// this is done in main() because *InputProps is defined in the same package
	// as this, meaning that the order between init-time actions is undefined.
	props := build.RegisterInputProperty[*InputProps]("")

	build.Main(func(ctx context.Context, userArgs []string, state *build.State) error {
		executor := (func(*exec.Cmd) error)(nil)
		if props.GetInput(ctx).ExperimentalDryrun {
			executor = CreateDryRunExecutor(true)
		}
		return RunDockerBuild(ctx, userArgs, state, executor)
	})
}
