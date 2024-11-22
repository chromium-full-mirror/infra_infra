// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package amt

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDigestFormatBool(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		provided bool
		expected string
	}{
		{
			true,
			"true",
		},
		{
			false,
			"",
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(strconv.FormatBool(tt.provided), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, digestFormatBool(tt.provided))
		})
	}
}

// getTestResponse returns a mock WWW-Authenticate header to pass to parseDigestResponse.
func getTestResponse(algorithm string, qop string, userhash bool) string {
	fmtStr := `Digest realm="Digest:342EEFEAA60BACE47C5AEFD8A471EB4C", nonce="wk3WAUoBAAAAAAAAICeUA5ghsVCkDhfE", algorithm="%s", qop="%s", userhash="%t", stale="false"`
	return fmt.Sprintf(fmtStr, algorithm, qop, userhash)
}

func TestParseDigestResponseSuccess(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		testName         string
		providedAlg      string
		providedAuth     string
		providedUserhash bool
		expectedString   string
	}{
		{
			"Default values",
			"",
			"",
			false,
			"Digest realm=\"Digest:342EEFEAA60BACE47C5AEFD8A471EB4C\", nonce=\"wk3WAUoBAAAAAAAAICeUA5ghsVCkDhfE\"",
		},
		{
			"Authentication mode no userhash (-sess algoritm)",
			"md5-sess",
			"auth",
			false,
			"Digest realm=\"Digest:342EEFEAA60BACE47C5AEFD8A471EB4C\", nonce=\"wk3WAUoBAAAAAAAAICeUA5ghsVCkDhfE\", algorithm=\"md5-sess\", qop=\"auth\"",
		},
		{
			"Integrity mode with userhash",
			"sha-265",
			"auth-int",
			true,
			"Digest realm=\"Digest:342EEFEAA60BACE47C5AEFD8A471EB4C\", nonce=\"wk3WAUoBAAAAAAAAICeUA5ghsVCkDhfE\", algorithm=\"sha-265\", qop=\"auth-int\", userhash=\"true\"",
		},
	}
	for _, tt := range testCases {
		tt := tt
		got, err := parseDigestResponse(getTestResponse(tt.providedAlg, tt.providedAuth, tt.providedUserhash))
		assert.Nil(t, err, fmt.Sprintf("error calling parseDigestResponse: %q", err))
		assert.Equal(t, tt.expectedString, got.String())
	}

}

func TestParseDigestResponseFailure(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		testName     string
		specifiedHdr string
		expectedErr  string
	}{
		{
			"Invalid prefix",
			"Abc123",
			"www-authenticate header does not specify digest: \"Abc123\"",
		},
		{
			"Missing fields",
			"Digest abc123",
			"invalid www-authenticate header: \"Digest abc123\"",
		},
		{
			"Invalid fields",
			"Digest abc=123",
			"unknown www-authenticate header field: \"abc\"",
		},
	}
	for _, tt := range testCases {
		tt := tt
		_, err := parseDigestResponse(tt.specifiedHdr)
		assert.ErrorContains(t, err, tt.expectedErr)
	}
}
