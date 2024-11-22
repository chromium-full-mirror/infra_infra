// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package amt

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"math/rand"
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

const (
	authIntMode = "auth-int"
	authMode    = "auth"
)

// strongestQOP returns the strongest supported quality-of-protection mode from the `qop`
// member of the digestResponse. It is expected to be quoted string of comma-seperated
// tokens indicating quality-of-protection modes supported by the server.
//
// Modes from strongest to weakest are:
//
//	auth-int:       integrity protection mode
//	auth:           authentication mode
//	<empty-string>: none
func (r *digestResponse) strongestQOP() string {
	var qop string
	for _, v := range r.qop {
		// auth-int: mode is supported in AMT versions >= 14.0.
		if v == authIntMode {
			return v
		} else if v == authMode {
			qop = v
		}
	}
	return qop
}

// newAuthorization creates a response to the server's digest challenge.
func (r *digestResponse) newAuthorization(method, uri, username, password, body string, nc int) (*digestAuthorization, error) {
	var newHash func() hash.Hash
	switch r.alg {
	case "md5", "":
		newHash = md5.New
	case "sha-256":
		newHash = sha256.New
	case "sha-512":
		return nil, fmt.Errorf("AMT does not support SHA-512")
	default:
		return nil, fmt.Errorf("unknown algorithm: %q", r.alg)
	}
	// sum returns the raw hash of one or more colon-separated strings.
	sum := func(parts ...string) []byte {
		var b []byte
		h := newHash()
		h.Write([]byte(strings.Join(parts, ":")))
		return h.Sum(b)
	}

	ha1 := sum(username, r.realm, password)
	cnonce := fmt.Sprintf("%08x", rand.Uint32())
	if r.algSess {
		ha1 = sum(string(ha1), r.nonce, cnonce)
	}
	ha1Hex := hex.EncodeToString(ha1)

	qop := r.strongestQOP()

	// b/357592780: auth-int is supported in AMT versions >= 14.0.
	var ha2 []byte
	switch qop {
	case authIntMode:
		h := newHash()
		h.Write([]byte(body))
		ha2 = sum(method, uri, hex.EncodeToString(h.Sum(nil)))
	default:
		ha2 = sum(method, uri)
	}
	ha2Hex := hex.EncodeToString(ha2)

	var response []byte
	switch qop {
	case authMode, authIntMode:
		response = sum(ha1Hex, r.nonce, fmt.Sprintf("%08x", nc), cnonce, qop, ha2Hex)
	default:
		response = sum(ha1Hex, r.nonce, ha2Hex)
	}

	var userhash bool
	if r.userhash {
		username = hex.EncodeToString(sum(username, r.realm))
		userhash = true
	}

	return &digestAuthorization{
		realm:  r.realm,
		domain: r.domain,
		nonce:  r.nonce,
		opaque: r.opaque,

		response: hex.EncodeToString(response),
		username: username,
		uri:      uri,
		qop:      qop,
		cnonce:   cnonce,
		nc:       nc,
		userhash: userhash,
	}, nil
}

// digestAuthorization is the response to the server challenge that's passed
// back to the server in the Authorization header of the next HTTP request.
type digestAuthorization struct {
	// Unchanged values from digestResponse.
	realm  string
	domain string
	nonce  string
	opaque string

	// New or changed values.
	response string
	username string
	uri      string
	qop      string
	cnonce   string
	nc       int
	userhash bool
}

func (a digestAuthorization) String() string {
	params := []struct {
		key   string
		value string
		quote bool
	}{
		// Must be quoted.
		{"username", a.username, true},
		{"realm", a.realm, true},
		{"nonce", a.nonce, true},
		{"uri", a.uri, true},
		{"response", a.response, true},
		{"cnonce", a.cnonce, true},
		{"opaque", a.opaque, true},

		// Must not be quoted.
		{"qop", a.qop, false},
		{"nc", fmt.Sprintf("%08x", a.nc), false},

		// Either is fine.
		{"domain", a.domain, true},
		{"userhash", digestFormatBool(a.userhash), true},
	}
	var parts []string
	for _, p := range params {
		if p.value == "" {
			continue
		}
		if p.quote {
			parts = append(parts, p.key+`="`+p.value+`"`)
		} else {
			parts = append(parts, p.key+`=`+p.value)
		}
	}
	return fmt.Sprintf("Digest %s", strings.Join(parts, ", "))
}
