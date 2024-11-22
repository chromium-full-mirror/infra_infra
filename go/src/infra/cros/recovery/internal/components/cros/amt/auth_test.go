// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package amt

import (
	"fmt"
	"net/http"
	"regexp"
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

func TestStrongestQOP(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		testName    string
		providedQOP string
		expectedQOP string
	}{
		{
			"Default values",
			"",
			"",
		},
		{
			"Supports auth mode",
			"auth",
			"auth",
		},
		{
			"Supports integrity mode",
			"auth,auth-int",
			"auth-int",
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			dr, err := parseDigestResponse(getTestResponse("md2", tt.providedQOP, false))
			assert.Nil(t, err, fmt.Sprintf("error calling parseDigestResponse: %q", err))
			assert.Equal(t, tt.expectedQOP, dr.strongestQOP())
		})
	}
}

func TestNewAuthorizationSuccess(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		testName         string
		providedAlg      string
		providedQop      string
		providedUserhash bool
		expectedUsername string
		expectedRegexp   string
	}{
		{
			"Default values",
			"",
			"",
			false,
			"admin",
			"Digest username=\"admin\", realm=\"Digest:342EEFEAA60BACE47C5AEFD8A471EB4C\", nonce=\"wk3WAUoBAAAAAAAAICeUA5ghsVCkDhfE\", uri=\"https://192.168.231.123:16993\", response=\"[0-9a-fA-F]{32}\", cnonce=\"[0-9a-fA-F]{8}\", nc=00000001",
		},
		{
			"Authentication mode no userhash",
			"md5",
			"auth",
			false,
			"admin",
			"Digest username=\"admin\", realm=\"Digest:342EEFEAA60BACE47C5AEFD8A471EB4C\", nonce=\"wk3WAUoBAAAAAAAAICeUA5ghsVCkDhfE\", uri=\"https://192.168.231.123:16993\", response=\"[0-9a-fA-F]{32}\", cnonce=\"[0-9a-fA-F]{8}\", qop=auth, nc=00000001",
		},

		{
			"Integrity mode with userhash (-sess algorithm)",
			"sha-256-sess",
			"auth-int",
			true,
			// "admin" after hashing.
			"1cb8d2fbaba7e0655ac3b4c7c72f691108955db4b18398a1b8a3bae295edbf3e",
			"Digest username=\"1cb8d2fbaba7e0655ac3b4c7c72f691108955db4b18398a1b8a3bae295edbf3e\", realm=\"Digest:342EEFEAA60BACE47C5AEFD8A471EB4C\", nonce=\"wk3WAUoBAAAAAAAAICeUA5ghsVCkDhfE\", uri=\"https://192.168.231.123:16993\", response=\"[0-9a-fA-F]{64}\", cnonce=\"[0-9a-fA-F]{8}\", qop=auth-int, nc=00000001, userhash=\"true\"",
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			dr, err := parseDigestResponse(getTestResponse(tt.providedAlg, tt.providedQop, tt.providedUserhash))
			assert.Nil(t, err, fmt.Sprintf("error calling parseDigestResponse: %q", err))
			da, err := dr.newAuthorization(http.MethodPost, "https://192.168.231.123:16993", "admin", "P@ssword123", "\r\n\r\n", 00000001)
			assert.Nil(t, err, fmt.Sprintf("error calling newAuthorization: %q", err))
			assert.Regexp(t, regexp.MustCompile(tt.expectedRegexp), da.String())
			assert.Equal(t, "wk3WAUoBAAAAAAAAICeUA5ghsVCkDhfE", da.nonce)
			assert.Equal(t, tt.expectedUsername, da.username)
			assert.Equal(t, tt.providedQop, da.qop)
		})
	}
}

func TestNewAuthorizationFailure(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		testName    string
		providedAlg string
		expectedErr string
	}{
		{
			"Unsupported algorithm",
			"sha-512",
			"AMT does not support SHA-512",
		},
		{
			"Unknown algorithm",
			"abc-123",
			// the format string contains a %q.
			"unknown algorithm: \"abc-123\"",
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			dr, err := parseDigestResponse(getTestResponse(tt.providedAlg, "auth", false))
			assert.Nil(t, err, fmt.Sprintf("error calling parseDigestResponse: %q", err))
			_, err = dr.newAuthorization(http.MethodPost, "https://192.168.231.123:16993", "admin", "P@ssword123", "\r\n\r\n", 00000001)
			assert.ErrorContains(t, err, tt.expectedErr)
		})
	}
}
