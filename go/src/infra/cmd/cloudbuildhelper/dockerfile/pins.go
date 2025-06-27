// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dockerfile

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/errors/errtag"
	"go.chromium.org/luci/common/sync/parallel"
)

// Pins is a mapping (image name, tag) => digest.
type Pins struct {
	Pins []Pin `yaml:"pins"`
}

// Pin is a single "(image name, tag) => digest" pin.
type Pin struct {
	Comment string `yaml:"comment,omitempty"` // arbitrary string, for humans
	Image   string `yaml:"image"`             // required
	Tag     string `yaml:"tag,omitempty"`     // default is "latest"
	Digest  string `yaml:"digest"`            // required
	Freeze  string `yaml:"freeze,omitempty"`  // if set, don't update in pins-update
}

// ImageRef returns "<image>:<tag>" string.
func (p *Pin) ImageRef() string {
	return p.Image + ":" + p.Tag
}

// ReadPins loads and validates YAML file with pins.
func ReadPins(r io.Reader) (*Pins, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.Fmt("failed to read the pins file: %w", err)
	}
	out := Pins{}
	if err = yaml.Unmarshal(body, &out); err != nil {
		return nil, errors.Fmt("failed to parse the pins file: %w", err)
	}
	seen := stringset.New(len(out.Pins))
	for idx, p := range out.Pins {
		if p, err = NormalizePin(p, true); err != nil {
			return nil, errors.Fmt("pin #%d: %w", idx+1, err)
		}
		if !seen.Add(p.ImageRef()) {
			return nil, errors.Fmt("pin #%d: duplicate entry for %q", idx+1, p.ImageRef())
		}
		out.Pins[idx] = p
	}
	return &out, nil
}

// WritePins generates YAML with pins.
//
// Entries are normalized and sorted.
func WritePins(w io.Writer, p *Pins) error {
	pins := Pins{Pins: append([]Pin(nil), p.Pins...)}
	sort.Slice(pins.Pins, func(i, j int) bool {
		l, r := pins.Pins[i], pins.Pins[j]
		if l.Image == r.Image {
			return l.Tag < r.Tag
		}
		return l.Image < r.Image
	})

	blob, err := yaml.Marshal(pins)
	if err != nil {
		return errors.Fmt("failed to serialize pins: %w", err)
	}

	_, err = fmt.Fprintf(w, `# Managed by cloudbuildhelper.
#
# All comments or unrecognized fields will be overwritten. To comment an entry
# use "comment" field.
#
# To update digests of all entries:
#   $ cloudbuildhelper pins-update <path-to-this-file>
#
# To add an entry (or update an existing one):
#   $ cloudbuildhelper pins-add <path-to-this-file> <image>[:<tag>]
#
# To remove an entry just delete it from the file.
#
# To prevent an entry from being updated by pins-update, add "freeze" field with
# an explanation why it is frozen.

%s`, blob)

	return errors.WrapIf(err, "failed to write")
}

// Add adds or updates a pin (which should already be resolved).
func (p *Pins) Add(pin Pin) error {
	pin, err := NormalizePin(pin, true)
	if err != nil {
		return err
	}

	key := pin.ImageRef()
	for i := range p.Pins {
		if p.Pins[i].ImageRef() == key {
			p.Pins[i] = pin
			return nil
		}
	}

	p.Pins = append(p.Pins, pin)
	return nil
}

// Visit calls 'cb' concurrently for all pins.
//
// The callback can mutate any pin fields except Image and Tag (doing so will
// panic).
//
// Calls the callback even for pins that are marked as frozen. The callback
// should handle this itself (e.g. by logging and skipping them).
//
// Returns a multi-error with all errors that happened.
func (p *Pins) Visit(cb func(p *Pin) error) error {
	return parallel.WorkPool(16, func(tasks chan<- func() error) {
		for i := range p.Pins {
			tasks <- func() error {
				pin := p.Pins[i]
				key := pin.ImageRef()
				if err := cb(&pin); err != nil {
					return errors.Fmt("visiting %q: %w", key, err)
				}
				pin, err := NormalizePin(pin, true)
				if err != nil {
					return errors.Fmt("visiting %q: %w", key, err)
				}
				if pin.ImageRef() != key {
					panic(fmt.Sprintf("the callback changed the pin key from %q to %q", key, pin.ImageRef()))
				}
				p.Pins[i] = pin
				return nil
			}
		}
	})
}

// PinFromString takes <image>[:<tag>] reference and converts it to Pin struct.
func PinFromString(image string) (Pin, error) {
	var pin Pin
	switch chunks := strings.Split(image, ":"); {
	case len(chunks) == 1:
		pin.Image, pin.Tag = chunks[0], "latest"
	case len(chunks) == 2:
		pin.Image, pin.Tag = chunks[0], chunks[1]
	default:
		return pin, errors.Fmt("bad image reference %q, should have form <image>[:<tag>]", image)
	}
	return NormalizePin(pin, false)
}

// NormalizePin returns a copy of 'p' with defaults populated.
//
// Expands abbreviated image names into full references,
func NormalizePin(p Pin, requireDigest bool) (Pin, error) {
	switch {
	case p.Image == "":
		return p, errors.New("'image' field is required")
	case requireDigest && p.Digest == "":
		return p, errors.New("'digest' field is required")
	}

	// See https://github.com/docker/distribution/blob/master/reference/normalize.go
	// for defaults.
	switch strings.Count(p.Image, "/") {
	case 0: // e.g. "ubuntu"
		p.Image = "docker.io/library/" + p.Image
	case 1: // e.g. "library/ubuntu"
		p.Image = "docker.io/" + p.Image
	default: // e.g. "gcr.io/something/something", do nothing, good enough
	}
	if p.Tag == "" {
		p.Tag = "latest"
	}
	return p, nil
}

// Resolver returns a Resolver that uses a snapshot of Pins as a source.
func (p *Pins) Resolver() Resolver {
	m := make(pinsResolver, len(p.Pins))
	for _, pin := range p.Pins {
		m[pin.ImageRef()] = pin.Digest
	}
	return m
}

var missingPinTag = errtag.Make("dockerfile.MissingPin", (*Pin)(nil))

type pinsResolver map[string]string

func (p pinsResolver) ResolveTag(image, tag string) (digest string, err error) {
	if len(p) == 0 {
		return "", errors.New("not using pins YAML, the Dockerfile must use @<digest> refs")
	}

	pin, err := NormalizePin(Pin{Image: image, Tag: tag}, false)
	if err != nil {
		return "", errors.Fmt("bad image:tag combination: %w", err)
	}

	d, ok := p[pin.ImageRef()]
	if !ok {
		// Note: the outer error wrapper usually has enough context already, adding
		// 'image' and 'tag' values here causes duplication.
		return "", missingPinTag.ApplyValue(errors.New("no such pinned <image>:<tag> combination in pins YAML"), &pin)
	}
	return d, nil
}

// IsMissingPinErr returns true if 'err' is an error produced by ResolveTag.
//
// It may be wrapped. Returns a pin that ResolveTag was unable to resolve.
func IsMissingPinErr(err error) *Pin {
	if pin, ok := missingPinTag.Value(err); ok {
		return pin
	}
	return nil
}
