// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"regexp"
	"strings"

	"go.chromium.org/luci/common/errors"
)

var (
	AnyRegex = regexp.MustCompile(`^ANY\((?<type>.+)\)=`)
)

func boolHandler(value string) bool {
	switch strings.ToLower(value) {
	case "true":
		return true
	default:
		return false
	}
}

type InjectablePlaceholderLookup struct {
	storage *InjectableStorage
}

func (lookup *InjectablePlaceholderLookup) Get(key string) (val string, ok bool) {
	valUntyped, err := lookup.storage.Get(key)
	if err != nil {
		return "", false
	}
	val = fmt.Sprint(valUntyped)
	ok = true
	return
}

func fmtHandler(storage *InjectableStorage, value string) string {
	lookup := &InjectablePlaceholderLookup{
		storage: storage,
	}
	return ResolvePlaceholders(value, lookup)
}

// anyHandler attaches the @type field to the found object, setting it to
// the provided type, essentially converting the object into a valid AnyProto.
func anyHandler(storage *InjectableStorage, value string) (obj interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	matches := AnyRegex.FindStringSubmatch(value)
	typeIndex := AnyRegex.SubexpIndex("type")
	objType := matches[typeIndex]
	objKey := strings.TrimPrefix(value, matches[0])
	obj, err = storage.Get(objKey)
	if err != nil {
		err = errors.Annotate(err, "failed to get obj %s", objKey).Err()
		return
	}

	obj.(map[string]interface{})["@type"] = objType
	return
}
