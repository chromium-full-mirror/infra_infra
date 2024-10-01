// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"go.chromium.org/luci/common/errors"
)

// CheckIfFieldDefinitionExists checks if a field definition exist in the provided msg type
func CheckIfFieldDefinitionExists(msg proto.Message, fieldName string) (bool, error) {
	dynMsg := dynamicpb.NewMessage(msg.ProtoReflect().Descriptor())
	msg1, err := proto.Marshal(msg)
	if err != nil {
		return false, errors.Annotate(err, "failed while marshaling provided proto").Err()
	}
	err = proto.Unmarshal(msg1, dynMsg)
	if err != nil {
		return false, errors.Annotate(err, "failed while unmarshalling dynamic proto").Err()
	}

	return dynMsg.Descriptor().Fields().ByName(protoreflect.Name(fieldName)) != nil, nil
}

// EncodeAnyObj encodes any type object using URL-safe base64 json encoding
func EncodeAnyObj(anyObj any) (string, error) {
	// Ensure the input is not empty
	if anyObj == nil {
		return "", fmt.Errorf("cannot encode nil message")
	}

	// Encode the message using json
	jsonData, err := json.Marshal(anyObj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}

	// Ensure the JSON data is in binary format (byte array)
	binaryData := []byte(jsonData)

	// Encode the binary data using URL-safe base64 encoding
	encodedData := base64.URLEncoding.EncodeToString(binaryData)

	// Ensure the encoded data is a string
	encodedString := string(encodedData)

	return encodedString, nil
}
