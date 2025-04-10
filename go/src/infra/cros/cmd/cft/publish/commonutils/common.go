// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commonutils

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// GetValueBoolMap creates a reverse map of inputMap. Any value in inputMap will be a key in the returned map.
func GetValueBoolMap(inputMap map[string]string) map[string]bool {
	retMap := map[string]bool{}
	for _, v := range inputMap {
		retMap[v] = true
	}
	return retMap
}

// IsSubsetOf returns if smallerMap is a subset of biggerMap.
func IsSubsetOf(smallerMap map[string]bool, biggerMap map[string]bool) bool {
	if len(smallerMap) > len(biggerMap) {
		return false
	}

	for k := range smallerMap {
		if _, ok := biggerMap[k]; !ok {
			return false
		}
	}

	return true
}

// MakeTempDir creates a temp directory at parentDirPath
func MakeTempDir(ctx context.Context, parentDirPath string, newDirName string) (string, error) {
	newDirPath, err := os.MkdirTemp(parentDirPath, newDirName)
	if err != nil {
		return "", fmt.Errorf("error during creating temp dir %q: %s", newDirName, err.Error())
	}
	return newDirPath, nil
}

// WriteProtoToJsonFile writes the provided proto to a json file
func WriteProtoJsonFile(ctx context.Context, filePath string, inputProto proto.Message) error {
	_, fileName := path.Split(filePath)

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("creating file %q: %w", fileName, err)
	}
	defer f.Close()

	bytes, err := protojson.Marshal(inputProto)
	if err != nil {
		return fmt.Errorf("marshalling proto for %q: %w", fileName, err)
	}

	_, err = f.Write(bytes)
	if err != nil {
		return fmt.Errorf("writing proto to file %q: %w", fileName, err)
	}

	log.Printf("proto successfully written to file: %s", string(bytes))

	err = f.Close()
	if err != nil {
		return fmt.Errorf("closing file %q: %w", fileName, err)
	}

	return nil
}
