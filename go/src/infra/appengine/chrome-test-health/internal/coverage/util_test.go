// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package coverage

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	structpb "github.com/golang/protobuf/ptypes/struct"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func getFakeCompressedString() (string, []byte) {
	str := `{"name":"foo","type":"foo1"}`
	strBytes, _ := compressString(str)
	return str, strBytes
}

func getFakeCompressedStringWithInvalidJson() (string, []byte) {
	str := `{"name":"foo","type":"foo1"`
	strBytes, _ := compressString(str)
	return str, strBytes
}

func TestGetStructFromCompressedData(t *testing.T) {
	t.Parallel()

	ftt.Run(`Should be able to return decompressed data`, t, func(t *ftt.Test) {
		t.Run(`Not compressed in correct format`, func(t *ftt.Test) {
			compressedMalformedData := []byte("malformed data")
			st := structpb.Struct{}
			err := getStructFromCompressedData(compressedMalformedData, &st)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.Resemble(errors.New("zlib: invalid header")))
		})
		t.Run(`Compressed in correct format but invalid json`, func(t *ftt.Test) {
			_, compressedInvalidJsonData := getFakeCompressedStringWithInvalidJson()
			st := structpb.Struct{}
			err := getStructFromCompressedData(compressedInvalidJsonData, &st)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, fmt.Sprintf("%s", err), should.ContainSubstring("unexpected end of JSON input"))
		})
		t.Run(`Well formed data`, func(t *ftt.Test) {
			str, compressedWellFormedData := getFakeCompressedString()
			st := structpb.Struct{}
			err := getStructFromCompressedData(compressedWellFormedData, &st)
			jsonStr, _ := json.Marshal(st.Fields)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, string(jsonStr), should.Resemble(str))
		})
	})
}
