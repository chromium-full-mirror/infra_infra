// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package amt

import (
	"fmt"
	"regexp"
	"strings"
)

type digestResponse struct {
	realm    string
	domain   string
	nonce    string
	opaque   string
	stale    bool
	alg      string
	algSess  bool
	qop      []string
	charset  string
	userhash bool
}

// authParamExpr is used to match one key/value pair in a WWW-Authenticate
// header. For example, it matches:
//
//	foo="quoted value",
//	bar=unquoted-value,
//	baz="quoted value"<end-of-string>
//
// The key is substring #1 and the value is substring #3 if it's not empty or
// substring #4.
var authParamExpr = regexp.MustCompile(`^ *([A-Za-z0-9\-]+)=("([^"]*)"|([^" ,]*))(,|$)`)

// parseDigestResponse parses the digest auth challenge in the WWW-Authenticate
// header of a "401 Unauthorized" server response. See RFC 7616.
func parseDigestResponse(wwwAuthHdr string) (*digestResponse, error) {
	if !strings.HasPrefix(wwwAuthHdr, "Digest ") {
		return nil, fmt.Errorf("www-authenticate header does not specify digest: %q", wwwAuthHdr)
	}
	var r digestResponse
	for hdr := strings.TrimPrefix(wwwAuthHdr, "Digest "); hdr != ""; {
		sm := authParamExpr.FindStringSubmatch(hdr)
		if sm == nil {
			return nil, fmt.Errorf("invalid www-authenticate header: %q", wwwAuthHdr)
		}
		key := strings.ToLower(sm[1])
		value := sm[3]
		if sm[4] != "" {
			value = sm[4]
		}
		hdr = hdr[len(sm[0]):]

		switch key {
		case "realm":
			r.realm = value
		case "domain":
			r.domain = value
		case "nonce":
			r.nonce = value
		case "opaque":
			r.opaque = value
		case "stale":
			r.stale = strings.ToLower(value) == "true"
		case "algorithm":
			value = strings.ToLower(value)
			if strings.HasSuffix(value, "-sess") {
				value = strings.TrimSuffix(value, "-sess")
				r.algSess = true
			}
			r.alg = value
		case "qop":
			r.qop = strings.Split(strings.ToLower(value), ",")
		case "charset":
			r.charset = value
		case "userhash":
			r.userhash = strings.ToLower(value) == "true"
		default:
			return nil, fmt.Errorf("unknown www-authenticate header field: %q", key)
		}
	}
	return &r, nil
}

// digestFormatBool returns "true" or an empty string because false values are
// excluded from header output.
func digestFormatBool(v bool) string {
	if v {
		return "true"
	}
	return ""
}

func (r *digestResponse) String() string {
	alg := r.alg
	if r.algSess {
		alg += "-sess"
	}
	params := []struct {
		key   string
		value string
	}{
		{"realm", r.realm},
		{"domain", r.domain},
		{"nonce", r.nonce},
		{"opaque", r.opaque},
		{"stale", digestFormatBool(r.stale)},
		{"algorithm", alg},
		{"qop", strings.Join(r.qop, ",")},
		{"charset", r.charset},
		{"userhash", digestFormatBool(r.userhash)},
	}
	var parts []string
	for _, p := range params {
		if p.value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf(`%s="%s"`, p.key, p.value))
	}
	return fmt.Sprintf("Digest %s", strings.Join(parts, ", "))
}
