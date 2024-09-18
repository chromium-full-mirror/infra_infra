// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestVerifyByteUnit(t *testing.T) {
	ftt.Run("Test Valid Byte Unit String", t, func(t *ftt.Test) {
		byteUnits := []string{
			"B",
			"KiB",
		}
		for _, bu := range byteUnits {
			assert.Loosely(t, VerifyByteUnit(bu), should.BeNil)
		}
	})
	ftt.Run("Test Invalid Byte Unit String", t, func(t *ftt.Test) {
		byteUnits := []string{
			"",
			"mb",
			"chrome",
		}
		for _, bu := range byteUnits {
			assert.Loosely(t, VerifyByteUnit(bu), should.NotBeNil)
		}
	})
}

func TestTrimByteString(t *testing.T) {
	ftt.Run("Test Trimming", t, func(t *ftt.Test) {
		byteStrings := map[string]string{
			"   5,000 MB   ": "5000MB",
			"6.000 GB   ":    "6000GB",
			"   70 00":       "7000",
			"a\\&*c":         "a\\&*c",
		}
		for k, v := range byteStrings {
			assert.Loosely(t, TrimByteString(k), should.Equal(v))
		}
	})
}

func TestConvertToBytes(t *testing.T) {
	ftt.Run("Test Byte String Conversion", t, func(t *ftt.Test) {
		t.Run("Empty String", func(t *ftt.Test) {
			bytes, err := ConvertToBytes("0")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, bytes, should.BeZero)
		})
		t.Run("Valid String with Unit Suffix", func(t *ftt.Test) {
			bytes, err := ConvertToBytes("500KB")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, bytes, should.Equal(500000))
		})
		t.Run("Valid String No Unit Suffix", func(t *ftt.Test) {
			bytes, err := ConvertToBytes("1000")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, bytes, should.Equal(1000))
		})
		t.Run("Invalid Numeric", func(t *ftt.Test) {
			bytes, err := ConvertToBytes("E39")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, bytes, should.BeZero)
		})
		t.Run("Invalid Unit Suffix", func(t *ftt.Test) {
			bytes, err := ConvertToBytes("100Giraffe")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, bytes, should.BeZero)
		})
	})
}

func TestGetMultipleForByteUnit(t *testing.T) {
	ftt.Run("Test Fetching Multiple for Valid Byte Unit", t, func(t *ftt.Test) {
		byteUnits := map[string]int64{
			"B":   1,
			"KiB": 1024,
			"MiB": 1024 * 1024,
		}
		for k, v := range byteUnits {
			multiple, err := GetMultipleForByteUnit(k)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, multiple, should.Equal(v))
		}
	})
	ftt.Run("Test Fetching Multiple for Invalid Byte Unit", t, func(t *ftt.Test) {
		byteUnits := []string{
			"",
			"mb",
			"chrome",
		}
		for _, byteUnit := range byteUnits {
			multiple, err := GetMultipleForByteUnit(byteUnit)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, multiple, should.BeZero)
		}
	})
}
