// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package jsonutil

import (
	"encoding/json"
	"fmt"

	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/errors"
)

// JSONPBMarshaller marshals protobufs as JSON.
var JSONPBMarshaller = &jsonpb.Marshaler{
	EmitDefaults: true,
}

// JSONPBUnmarshaller unmarshals JSON and creates corresponding protobufs.
var JSONPBUnmarshaller = jsonpb.Unmarshaler{
	AllowUnknownFields: true,
}

// ParseJSONProto takes an input string and a proto message indicating the type of message
// and parses either a single object or an array of objects. It always returns an array, which
// will be a singleton array if exactly one item was passed.
//
// The parameter msg is treated as a template only. We *always* clone it defensively before
// reading into it.
func ParseJSONProto(in []byte, msg proto.Message) ([]proto.Message, error) {
	if msg == nil {
		return nil, fmt.Errorf("parse json proto: message cannot be nil")
	}

	rawItems, err := segmentJSONArray(in)
	if err != nil {
		return nil, errors.Annotate(err, "parse json proto").Err()
	}
	out := []proto.Message{}
	for _, item := range rawItems {
		m := proto.Clone(msg)
		err := protojson.Unmarshal(item, m)
		if err != nil {
			panic(fmt.Sprintf("parse json proto: %q should have round-tripped!", item))
		}
		out = append(out, m)
	}
	return out, nil
}

// segmentJSONArray takes a string containing a JSON array and turns it into an array of its elements.
// The elements are re-serialized as JSON.
//
// The intent of this function is to enable users to take an array containing structured objects, break it up,
// and then parse the items manually.
func segmentJSONArray(in []byte) ([][]byte, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("segment json array: input cannot be empty")
	}

	hopper, err := parseToArray(in)
	if err != nil {
		return nil, err
	}

	out := [][]byte{}
	for _, item := range hopper {
		b, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

// parseToArray takes a string containing JSON and parses it.
//
// If the toplevel item is an array, we return our argument parsed.
// If the toplevel item is anything other than an array, we pack it into an a singleton array.
//
// parseToArray returns a non-nil error if and only if the item does not parse as JSON.
func parseToArray(msg []byte) ([]any, error) {
	var out any
	if err := json.Unmarshal(msg, &out); err != nil {
		return nil, errors.Annotate(err, "parse to array").Err()
	}
	switch v := out.(type) {
	case []any:
		return v, nil
	default:
		return []any{out}, nil
	}
}
