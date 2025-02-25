// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"encoding/base64"
	"hash/fnv"
	"strconv"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/fleetconsole/internal/internalproto"
)

func OffsetToPageToken(offset int, hashComponents map[string]string) (string, error) {
	nextPageToken, err := proto.Marshal(&internalproto.PaginationToken{
		Offset:     int32(offset),
		ParamsHash: hashRequest(hashComponents),
	})

	if err != nil {
		return "", errors.Annotate(err, "failed to encrypt page token").Err()
	}

	return base64.RawURLEncoding.EncodeToString(nextPageToken), nil
}

func PageTokenToOffset(pageToken string, hashComponents map[string]string) (int, error) {
	encodedProto, err := base64.RawURLEncoding.DecodeString(pageToken)
	if err != nil {
		return 0, InvalidTokenError(err)
	}

	var tokenProto internalproto.PaginationToken
	if err := proto.Unmarshal(encodedProto, &tokenProto); err != nil {
		return 0, InvalidTokenError(err)
	}

	// Only compare request hashes when a `offset` is provided.
	offset := tokenProto.GetOffset()
	if offset != 0 && tokenProto.GetParamsHash() != hashRequest(hashComponents) {
		return 0, InvalidTokenError(errors.New("request message fields do not match fields for the current page"))
	}

	return int(offset), nil
}

func hashRequest(hashComponents map[string]string) string {
	hash := fnv.New64a()
	for key, value := range hashComponents {
		hash.Write([]byte(key))
		hash.Write([]byte(value))
	}
	return strconv.FormatUint(hash.Sum64(), 36)
}
