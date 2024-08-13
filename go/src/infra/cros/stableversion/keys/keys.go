// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package keys provides builder to generate keys for recovery-versions.
package keys

import (
	"fmt"
	"strings"

	"go.chromium.org/luci/common/errors"
)

const (
	separator = ";"
)

type Builder interface {
	Add(name, value string) error
	String() string
}

// New creates new Builder with provided values.
func New(board, model, pool string) Builder {
	kb := NewBuilder()
	if err := kb.Add("board", board); err != nil {
		panic(err)
	}
	if err := kb.Add("model", model); err != nil {
		panic(err)
	}
	if err := kb.Add("pool", pool); err != nil {
		panic(err)
	}
	return kb
}

// NewBuilder created new Builder.
func NewBuilder() Builder {
	return &keyBuilder{
		parts: nil,
	}
}

// keyBuilder is implementation of NewBuilder interface.
type keyBuilder struct {
	parts []string
}

// Add add new key=value to the builder.
// Name is required, value can be empty.
func (k *keyBuilder) Add(name, value string) error {
	n := strings.TrimSpace(strings.ToLower(name))
	if n == "" {
		return errors.Reason("add: name is empty").Err()
	}
	v := strings.TrimSpace(strings.ToLower(value))
	k.parts = append(k.parts, fmt.Sprintf("%s=%s", n, v))
	return nil
}

// String retuend combination of all keys added to the builder.
func (k *keyBuilder) String() string {
	return strings.Join(k.parts, separator)
}
