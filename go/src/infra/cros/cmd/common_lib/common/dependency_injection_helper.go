// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"strings"
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
