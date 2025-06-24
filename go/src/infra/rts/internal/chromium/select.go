// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chromium

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/rts/filegraph/git"
	"go.chromium.org/infra/rts/internal/gitutil"
	evalpb "go.chromium.org/infra/rts/presubmit/eval/proto"
)

type BaseSelectRun struct {
	BaseCommandRun

	// Direct input.

	Checkout           string
	ModelDir           string
	Out                string
	TargetChangeRecall float64
	IgnoreExceptions   bool
	ChangeRef          string
	GenerateInverse    bool

	// Indirect input.

	ChangedFiles stringset.Set        // files different between origin/main and the working tree
	TestFiles    map[string]*TestFile // indexed by source-absolute test file name
	Strategy     git.SelectionStrategy
}

func (r *BaseSelectRun) ValidateFlags() error {
	switch {
	case r.Checkout == "":
		return errors.New("-checkout is required")
	case r.ModelDir == "":
		return errors.New("-model-dir is required")
	case r.Out == "":
		return errors.New("-out is required")
	case !(r.TargetChangeRecall > 0 && r.TargetChangeRecall < 1):
		return errors.New("-target-change-recall must be in (0.0, 1.0) range")
	default:
		return nil
	}
}

// loadStrategy initializes r.strategy fields, except r.strategy.Graph.
func (r *BaseSelectRun) LoadStrategy(cfgFileName string) error {
	cfgBytes, err := os.ReadFile(cfgFileName)
	if err != nil {
		return err
	}
	cfg := &GitBasedStrategyConfig{}
	if err := protojson.Unmarshal(cfgBytes, cfg); err != nil {
		return err
	}

	r.Strategy.EdgeReader = &git.EdgeReader{
		ChangeLogDistanceFactor:     float64(cfg.ChangeLogDistanceFactor),
		FileStructureDistanceFactor: float64(cfg.FileStructureDistanceFactor),
	}
	threshold := chooseThreshold(cfg.Thresholds, r.TargetChangeRecall)
	if threshold == nil {
		return errors.Reason("no threshold for target change recall %.4f", r.TargetChangeRecall).Err()
	}
	r.Strategy.MaxDistance = float64(threshold.MaxDistance)
	return nil
}

func (r *BaseSelectRun) LogChangedFiles(ctx context.Context) {
	msg := &strings.Builder{}
	msg.WriteString("detected changed files:\n")
	for f := range r.ChangedFiles {
		fmt.Fprintf(msg, "  %s\n", f)
	}
	logging.Infof(ctx, "%s", msg)
}

// testNameReplacer is used to prepare a test name to be used in a .filter file.
var testNameReplacer = strings.NewReplacer(
	// Escape stars, since filter file lines are actually globs.
	"*", "\\*",

	// Java test names use "#" as a separator of class name and method name,
	// but the filter files accept "." instead (probably because comments start
	// with "#"). Thus replace "#" with ".".
	// Note: only Java tests use "#" in their test names.
	"#", ".",
)

func WriteFilterFile(fileName string, toSkip []string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, name := range toSkip {
		name = testNameReplacer.Replace(name)
		if _, err := fmt.Fprintf(f, "-%s\n", name); err != nil {
			return err
		}
	}
	return f.Close()
}

func WriteInvertedFilterFile(fileName string, toSkip []string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, name := range toSkip {
		name = testNameReplacer.Replace(name)
		if _, err := fmt.Fprintf(f, "%s\n", name); err != nil {
			return err
		}
	}
	return f.Close()
}

// LoadGraph loads r.strategy.Graph from the model.
func (r *BaseSelectRun) LoadGraph(fileName string) error {
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	// Note: it might be dangerous to sync with the current checkout.
	// There might have been such change in the repo that the chosen threshold,
	// the model or both are no longer good. Thus, do not sync.
	r.Strategy.Graph = &git.Graph{}
	return r.Strategy.Graph.Read(bufio.NewReader(f))
}

// LoadTestFileSet loads r.testFiles.
func (r *BaseSelectRun) LoadTestFileSet(fileName string) error {
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	r.TestFiles = map[string]*TestFile{}
	return ReadTestFiles(bufio.NewReader(f), func(file *TestFile) error {
		r.TestFiles[file.Path] = file
		return nil
	})
}

// LoadChangedFiles initializes r.changedFiles.
func (r *BaseSelectRun) LoadChangedFiles() error {
	changedFiles, err := gitutil.ChangedFiles(r.Checkout, r.ChangeRef)
	if err != nil {
		return err
	}

	r.ChangedFiles = stringset.New(len(changedFiles))
	for _, f := range changedFiles {
		r.ChangedFiles.Add("//" + f)
	}
	return nil
}

// chooseThreshold returns the distance threshold based on
// r.targetChangeRecall and r.evalResult.
func chooseThreshold(thresholds []*evalpb.Threshold, targetChangeRecall float64) *evalpb.Threshold {
	var ret *evalpb.Threshold
	for _, t := range thresholds {
		if t.ChangeRecall < float32(targetChangeRecall) {
			continue
		}
		if ret == nil || ret.ChangeRecall > t.ChangeRecall {
			ret = t
		}
	}
	return ret
}
