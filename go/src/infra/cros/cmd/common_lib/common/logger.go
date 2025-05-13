// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	gol "github.com/op/go-logging"
	"golang.org/x/term"

	"go.chromium.org/luci/common/logging"
)

const (
	StepKey = "build.step"
)

type localLoggerWrapper struct {
	sync.Mutex
	l              *gol.Logger
	prevStepHeader string
}

type localLoggerImpl struct {
	*localLoggerWrapper

	level  logging.Level
	fields string
	steps  map[string]*Step
}

// Debugf performs a Debug level log call.
func (li *localLoggerImpl) Debugf(format string, args ...any) {
	li.LogCall(logging.Debug, 1, format, args)
}

// Infof performs a Info level log call.
func (li *localLoggerImpl) Infof(format string, args ...any) {
	li.LogCall(logging.Info, 1, format, args)
}

// Warningf performs a Warning level log call.
func (li *localLoggerImpl) Warningf(format string, args ...any) {
	li.LogCall(logging.Warning, 1, format, args)
}

// Errorf performs a Error level log call.
func (li *localLoggerImpl) Errorf(format string, args ...any) {
	li.LogCall(logging.Error, 1, format, args)
}

// LogCall intercepts logging calls with fields attached to format their output and capture important step related content.
func (li *localLoggerImpl) LogCall(l logging.Level, calldepth int, format string, args []any) {
	if l < li.level {
		return
	}
	li.Lock()
	defer li.Unlock()

	// Try to parse the fileds string to map.
	fields, _ := toFields(li.fields)
	// If we can not get the StepKey from fields. it causes `updateStep` fatal error.
	//
	// updateStep will access the Step pointer but it is nil
	if _, ok := fields[StepKey]; ok {
		text := li.formatWithStepHeaders(format, args)
		format = strings.Replace(text, "%", "%%", -1)
		args = nil
	}
	li.l.ExtraCalldepth = (calldepth + 1)
	switch l {
	case logging.Debug:
		li.l.Debugf(format, args...)
	case logging.Info:
		li.l.Infof(format, args...)
	case logging.Warning:
		li.l.Warningf(format, args...)
	case logging.Error:
		li.l.Errorf(format, args...)
	}
}

// toFields parse the fields string.
func toFields(s string) (map[string]any, error) {
	var fields map[string]any
	if err := json.Unmarshal([]byte(s), &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// updateStep traverses through the steps tree to add the message to the step referenced in the logging fields.
func (li *localLoggerImpl) updateStep(message string) (*Step, string) {
	// Parse out the fields into something accessible
	fields, err := toFields(li.fields)
	if err != nil {
		panic(err)
	}

	var stepPtr *Step = nil
	if val, ok := fields[StepKey]; ok {
		steps := strings.Split(val.(string), "|")
		stepPtr = getSubstep(li.steps, steps[0], 0, nil)
		for _, stepName := range steps[1:] {
			stepPtr = traverseStep(stepPtr, stepName)
		}
	}

	logname := "Log"
	if val, ok := fields["build.logname"]; ok {
		logname = val.(string)
	}

	return stepPtr, updateStep(stepPtr, logname, message)
}

// formatWithStepHeaders updates step data and ensures that logging output is sectioned off by the step they are called under.
func (li *localLoggerImpl) formatWithStepHeaders(format string, args []any) string {
	buf := new(bytes.Buffer)
	text := fmt.Sprintf(format, args...)
	step, text := li.updateStep(text)
	stepHeader := step.Name
	stepPtr := step.Parent
	for stepPtr != nil {
		stepHeader = stepPtr.Name + " | " + stepHeader
		stepPtr = stepPtr.Parent
	}
	if li.prevStepHeader != stepHeader {
		li.prevStepHeader = stepHeader
		li.l.Infof("\033[32m" + stepHeader + "\033[0m")
	}
	buf.WriteString(text)
	return buf.String()
}

// updateStep determines what happens with a message and modifies the step object accordingly.
func updateStep(step *Step, logname string, message string) string {
	statusPrefix := "set status"
	statusSplitter := ": "
	splitMessage := strings.Split(message, statusSplitter)
	if strings.Contains(message, statusPrefix) {
		step.Status = strings.Split(splitMessage[1], "\n")[0]
	} else {
		log, ok := step.Logs[logname]
		if !ok {
			log = &StepLog{
				Name: logname,
				Log:  new(bytes.Buffer),
			}
			step.Logs[logname] = log
		}
		log.Log.WriteString(message)
		if !strings.HasSuffix(message, "\n") {
			log.Log.WriteString("\n")
		}
		message = fmt.Sprintf("%s: %s", logname, message)
	}

	return message
}

// getSubstep uses the stepName to obtain the substep from a step and creates a new step if it does not exist.
func getSubstep(steps map[string]*Step, stepName string, depth int, parent *Step) *Step {
	substep, ok := steps[stepName]
	if !ok {
		substep = &Step{
			Name:     stepName,
			Order:    len(steps) + 1,
			Depth:    depth + 1,
			Status:   "SCHEDULED",
			Logs:     make(map[string]*StepLog),
			SubSteps: make(map[string]*Step),
			Parent:   parent,
		}
		steps[stepName] = substep
	}
	return substep
}

// traverseStep provides an entry point for traversing the steps tree.
func traverseStep(step *Step, stepName string) *Step {
	return getSubstep(step.SubSteps, stepName, step.Depth, step)
}

type StepLog struct {
	Name string
	Log  *bytes.Buffer
}

type Step struct {
	Name     string
	Order    int
	Depth    int
	Status   string
	Logs     map[string]*StepLog
	SubSteps map[string]*Step
	Parent   *Step
}

type LoggerConfig struct {
	Out      io.Writer   // where to write the log to, required
	Format   string      // how to format the log, default is PickStdFormat(Out)
	Logger   *gol.Logger // if set, will be used as is, overrides everything else
	initOnce sync.Once
	w        *localLoggerWrapper
	steps    map[string]*Step
}

// DumpStepsToFolder cleans and writes the step information gathered during execution to a designated path.
func (lc *LoggerConfig) DumpStepsToFolder(basePath string) {
	if basePath == "" {
		// Use temporary folder
		tempPath, err := os.MkdirTemp("/tmp", "cros_test_runner*")
		if err != nil {
			panic(err)
		}
		basePath = tempPath
	}
	layout := "20060102-150405"
	t := time.Now()
	root := path.Join(basePath, "test_runner_steps"+t.Format(layout))

	if err := os.RemoveAll(root); err != nil {
		panic(err)
	}

	if err := os.MkdirAll(root, DirPermission); err != nil {
		panic(err)
	}

	for _, step := range lc.steps {
		dumpStepToFolder(step, root)
	}
}

// dumpStepToFolder formats the step information into discoverable content.
func dumpStepToFolder(step *Step, parentFolder string) {
	stepFolderName := fmt.Sprintf("%d. [%s] %s", step.Order, step.Status, step.Name)
	stepFolder := path.Join(parentFolder, stepFolderName)
	if err := os.MkdirAll(stepFolder, DirPermission); err != nil {
		panic(err)
	}

	for _, log := range step.Logs {
		logNamePath := path.Join(stepFolder, log.Name+".txt")
		if err := os.WriteFile(logNamePath, log.Log.Bytes(), FilePermission); err != nil {
			panic(err)
		}
	}

	for _, subStep := range step.SubSteps {
		dumpStepToFolder(subStep, stepFolder)
	}
}

// NewLogger returns new go-logging based logger bound to the given context.
func (lc *LoggerConfig) NewLogger(c context.Context, lctx *logging.LogContext) logging.Logger {
	lc.initOnce.Do(func() {
		logger := lc.Logger
		if logger == nil {
			fmt := lc.Format
			if fmt == "" {
				fmt = PickStdFormat(lc.Out)
			}
			// Leveled formatted file backend.
			backend := gol.AddModuleLevel(
				gol.NewBackendFormatter(
					gol.NewLogBackend(lc.Out, "", 0),
					gol.MustStringFormatter(fmt)))
			backend.SetLevel(gol.DEBUG, "")
			logger = gol.MustGetLogger("")
			logger.SetBackend(backend)
		}
		lc.steps = make(map[string]*Step)
		lc.w = &localLoggerWrapper{l: logger}
	})
	ret := &localLoggerImpl{localLoggerWrapper: lc.w, steps: lc.steps}
	ret.level = lctx.Level
	if fields := lctx.Fields; len(fields) > 0 {
		ret.fields = lctx.Fields.String()
	}
	return ret
}

// Use registers go-logging based logger as default logger of the context.
func (lc *LoggerConfig) Use(c context.Context) context.Context {
	return logging.SetFactory(c, lc.NewLogger)
}

// StdFormat is a preferred logging format to use.
//
// It is compatible with logging format used by luci-py. The zero after %{pid}
// is "thread ID" which is unavailable in go.
const StdFormat = `[%{level:.1s}%{time:2006-01-02T15:04:05.000000Z07:00} ` +
	`%{pid} 0 %{shortfile}] %{message}`

// StdFormatWithColor is same as StdFormat, except with fancy colors.
//
// Use it when logging to terminal. Note that StdConfig will pick it
// automatically if it detects that given io.Writer is an os.File and it
// is a terminal. See PickStdFormat().
const StdFormatWithColor = `%{color}[%{level:.1s}%{time:2006-01-02T15:04:05.000000Z07:00} ` +
	`%{pid} 0 %{shortfile}]%{color:reset} %{message}`

// PickStdFormat returns StdFormat for non terminal-backed files or
// StdFormatWithColor for io.Writers that are io.Files backed by a terminal.
//
// Used by default StdConfig.
func PickStdFormat(w io.Writer) string {
	if file, _ := w.(*os.File); file != nil {
		if term.IsTerminal(int(file.Fd())) {
			return StdFormatWithColor
		}
	}
	return StdFormat
}
