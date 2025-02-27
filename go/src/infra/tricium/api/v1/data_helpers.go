// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tricium

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	// GitFileDetailsPath stores the path to the GIT_FILE_DETAILS data type file.
	GitFileDetailsPath = "tricium/data/git_file_details.json"
	// FilesPath stores the path to the FILES data type file.
	FilesPath = "tricium/data/files.json"
	// ResultsPath stores the path to the RESULTS data type file.
	ResultsPath = "tricium/data/results.json"
)

// GetPathForDataType returns the file path to use for the provided Tricium
// data type.
func GetPathForDataType(t any) (string, error) {
	switch t := t.(type) {
	case *Data_GitFileDetails:
		return GitFileDetailsPath, nil
	case *Data_Files:
		return FilesPath, nil
	case *Data_Results:
		return ResultsPath, nil
	default:
		return "", fmt.Errorf("unknown path for data type, type: %T", t)
	}
}

// WriteDataType writes a Tricium data type to the file path assigned to the type.
func WriteDataType(prefix string, t proto.Message) (string, error) {
	// The jsonpb marshaler produces a different output than the standard
	// "encoding/json" package would.
	//
	// Specifically, the jsonpb marshaler uses camelCase field names for
	// proto structs, whereas the encoding/json marshaler uses lowercase
	// with underscores.
	//
	// In Tricium we try to use jsonpb whenever working with protos.

	json, err := protojson.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("failed to marshal: %w", err)
	}
	p, err := GetPathForDataType(t)
	if err != nil {
		return p, fmt.Errorf("failed to get path for type: %w", err)
	}
	path := path.Join(prefix, p)
	if err = os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return path, fmt.Errorf("failed to make directories for path: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return path, fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(json); err != nil {
		return path, fmt.Errorf("failed to write to file: %w", err)
	}
	return path, nil
}

// ReadDataType reads a Tricium data type to the provided type.
func ReadDataType(prefix string, t proto.Message) error {
	p, err := GetPathForDataType(t)
	if err != nil {
		return fmt.Errorf("failed to get path for type: %w", err)
	}
	path := path.Join(prefix, p)
	msg, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := protojson.Unmarshal(msg, t); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}
	return nil
}

// FilterFiles returns files whose basename matches any of the given patterns.
func FilterFiles(files []*Data_File, filters ...string) ([]*Data_File, error) {
	var filteredFiles []*Data_File
	for _, f := range files {
		for _, filter := range filters {
			matched, err := filepath.Match(filter, filepath.Base(f.Path))
			if err != nil {
				return nil, fmt.Errorf("bad path_filters pattern %q: %w", filter, err)
			}
			if matched {
				filteredFiles = append(filteredFiles, f)
				break
			}
		}
	}
	return filteredFiles, nil
}
