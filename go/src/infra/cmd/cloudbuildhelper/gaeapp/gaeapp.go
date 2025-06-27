// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gaeapp contains helpers for working with GAE's app.yaml.
package gaeapp

import (
	"gopkg.in/yaml.v3"

	"go.chromium.org/luci/common/errors"
)

// AppYAML is a loaded app.yaml.
//
// See https://cloud.google.com/appengine/docs/standard/reference/app-yaml
// Only fields needed by cloudbuildhepler are exposed.
//
// Note that loading and saving an app.yaml via this struct will drop all
// comments and custom formatting. Only significant YAML data will be preserved.
type AppYAML struct {
	// Runtime defines what GAE runtime to use e.g. "go121".
	Runtime string
	// Entrypoint is a shell command to run to start the app.
	Entrypoint string
	// Handlers is a list of URL handlers.
	Handlers []*HandlerYAML

	all map[string]any // this is the only way to preserve "unrecognized" fields
}

// HandlerYAML is a handle to a single "handlers" entry in an App YAML.
//
// Do not construct it by hand, only take it from AppYAML struct.
type HandlerYAML struct {
	// StaticDir is a directory with static files to upload.
	StaticDir string
	// StaticFiles is a regexp used to map URLs to static files to serve.
	StaticFiles string
	// Upload is a rgexp used to define what static files to upload.
	Upload string

	all map[string]any
}

func take(m map[string]any, key string) string {
	val, _ := m[key].(string)
	return val
}

func put(m map[string]any, key, val string) {
	if val != "" {
		m[key] = val
	} else {
		delete(m, key)
	}
}

// LoadAppYAML parses app.yaml.
func LoadAppYAML(blob []byte) (*AppYAML, error) {
	all := map[string]any{}
	if err := yaml.Unmarshal(blob, &all); err != nil {
		return nil, err
	}

	handlersRaw, _ := all["handlers"].([]any)
	handlers := make([]*HandlerYAML, 0, len(handlersRaw))
	for _, handler := range handlersRaw {
		handler, ok := handler.(map[string]any)
		if !ok {
			return nil, errors.New("bad `handlers` structure")
		}
		handlers = append(handlers, &HandlerYAML{
			StaticDir:   take(handler, "static_dir"),
			StaticFiles: take(handler, "static_files"),
			Upload:      take(handler, "upload"),
			all:         handler,
		})
	}

	return &AppYAML{
		Runtime:    take(all, "runtime"),
		Entrypoint: take(all, "entrypoint"),
		Handlers:   handlers,
		all:        all,
	}, nil
}

// Save produces app.yaml in a serialized form.
func (a *AppYAML) Save() ([]byte, error) {
	put(a.all, "runtime", a.Runtime)
	put(a.all, "entrypoint", a.Entrypoint)

	handlersRaw := make([]any, 0, len(a.Handlers))
	for _, h := range a.Handlers {
		put(h.all, "static_dir", h.StaticDir)
		put(h.all, "static_files", h.StaticFiles)
		put(h.all, "upload", h.Upload)
		handlersRaw = append(handlersRaw, h.all)
	}

	if len(handlersRaw) != 0 {
		a.all["handlers"] = handlersRaw
	} else {
		delete(a.all, "handlers")
	}

	return yaml.Marshal(a.all)
}
