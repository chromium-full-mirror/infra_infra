// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// WriteProtoToStepLog writes provided proto to build step.
func WriteProtoToStepLog(ctx context.Context, step *build.Step, proto proto.Message, logText string) {
	if step == nil || proto == nil {
		return
	}

	outputLog := step.Log(logText)
	marshaller := protojson.MarshalOptions{Multiline: true, Indent: "  "}
	bytes, err := marshaller.Marshal(proto)
	if err != nil {
		logging.Infof(ctx, "%s: %q", fmt.Sprintf("Marshalling %q failed:", logText), err.Error())
		return
	}
	_, err = outputLog.Write(bytes)
	if err != nil {
		logging.Infof(ctx, "%s: %q", fmt.Sprintf("Writing %q failed:", logText), err.Error())
		return
	}
}

// WriteAnyObjectToStepLog writes provided obj to build step.
func WriteAnyObjectToStepLog(ctx context.Context, step *build.Step, obj any, logText string) {
	if step == nil || obj == nil {
		return
	}

	jsonBytes, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		logging.Infof(ctx, "%s: %q", fmt.Sprintf("Marshalling %q failed:", logText), err.Error())
		return
	}

	outputLog := step.Log(logText)
	_, err = outputLog.Write(jsonBytes)
	if err != nil {
		logging.Infof(ctx, "%s: %q", fmt.Sprintf("Writing %q failed:", logText), err.Error())
		return
	}
}

// WriteStringToStepLog writes provided string to build step.
func WriteStringToStepLog(ctx context.Context, step *build.Step, data, logText string) {
	outputLog := step.Log(logText)
	_, err := outputLog.Write([]byte(data))
	if err != nil {
		logging.Infof(ctx, "%s: %q", fmt.Sprintf("Writing %q failed:", logText), err.Error())
		return
	}
}

// GetFileContents finds the file and return the file contents.
func GetFileContents(ctx context.Context, fileName string, rootDir string) ([]byte, error) {
	filePath, err := FindFile(ctx, fileName, rootDir)
	if err != nil {
		logging.Infof(ctx, "finding file '%s' at '%s' failed: %s", fileName, rootDir, err)
		return nil, err
	}
	fileContents, err := os.ReadFile(filePath)
	if err != nil {
		logging.Infof(ctx, "reading file '%s' at '%s' failed: %s", fileName, filePath, err)
		return nil, err
	}

	return fileContents, nil
}

// WriteFileContentsToStepLog writes provided fileName contents at rootDir to
// build step log.
func WriteFileContentsToStepLog(ctx context.Context, step *build.Step, fileName string, rootDir string, logText string) error {
	if step == nil {
		return nil
	}

	fileContents, err := GetFileContents(ctx, fileName, rootDir)
	if err != nil {
		logging.Infof(ctx, "getting file contents for '%s' failed: %s", fileName, err)
		return err
	}

	outputLog := step.Log(logText)
	_, err = outputLog.Write(fileContents)
	if err != nil {
		logging.Infof(ctx, "%s: %q", fmt.Sprintf("writing %q failed:", logText), err.Error())
		return err
	}

	return nil
}

// WriteContainerLogToStepLog writes container log contents to step log.
func WriteContainerLogToStepLog(ctx context.Context, container interfaces.ContainerInterface, step *build.Step, logTitle string) error {
	logsLoc, err := container.GetLogsLocation()
	if err != nil {
		return errors.Annotate(err, "error during getting container log location: ").Err()
	}

	err = WriteFileContentsToStepLog(ctx, step, "log.txt", logsLoc, logTitle)
	if err != nil {
		return errors.Annotate(err, "error during writing container log contents: ").Err()
	}

	return nil
}

// WriteProtoToJSONFile writes provided proto to a json file.
func WriteProtoToJSONFile(
	ctx context.Context,
	dirPath string,
	fileName string,
	inputProto proto.Message) (string, error) {

	protoFilePath := path.Join(dirPath, fileName)
	f, err := os.Create(protoFilePath)
	if err != nil {
		return "", fmt.Errorf("error during creating file %q: %s", fileName, err.Error())
	}
	defer f.Close()

	bytes, err := protojson.Marshal(inputProto)
	if err != nil {
		return "", fmt.Errorf("error during marshalling proto for %q: %s", fileName, err.Error())
	}

	_, err = f.Write(bytes)
	if err != nil {
		return "", fmt.Errorf("error during writing proto to file %q: %s", fileName, err.Error())
	}

	logging.Infof(ctx, "proto successfully written to file: %s", bytes)

	err = f.Close()
	if err != nil {
		return "", fmt.Errorf("error during closing file %q: %s", fileName, err.Error())
	}

	return protoFilePath, nil
}

// LogExecutionDetails logs provided cmds to build step.
func LogExecutionDetails(ctx context.Context, step *build.Step, cmds []string) {
	if step == nil {
		return
	}

	outputLog := step.Log("Execution Details")
	logCmdsStr := fmt.Sprintf("%+q", cmds)
	_, err := outputLog.Write([]byte(logCmdsStr))
	if err != nil {
		logging.Infof(ctx, "Failed to create execution details for cmd: %q", cmds)
	}
}

func LogWarningIfErr(ctx context.Context, err error) {
	if err != nil {
		logging.Infof(ctx, fmt.Sprintf("Warning: %s", err))
	}
}

// CreateLogFile creates a file and its parent directory for logging purpose.
func CreateLogFile(fullPath string) (*os.File, error) {
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %v: %w", fullPath, err)
	}

	logFullPathName := filepath.Join(fullPath, "log.txt")

	// Log the full output of the command to disk.
	logFile, err := os.Create(logFullPathName)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %v: %w", fullPath, err)
	}
	return logFile, nil
}

// NewLogger creates a logger. Using go default logger for now.
func NewLogger(logFile *os.File) *log.Logger {
	mw := io.MultiWriter(logFile, os.Stderr)
	return log.New(mw, "", log.LstdFlags|log.LUTC)
}
