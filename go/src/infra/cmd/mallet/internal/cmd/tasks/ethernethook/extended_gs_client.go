// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ethernethook

import (
	"context"
	"fmt"
	"regexp"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"go.chromium.org/luci/common/errors"
)

// IteratorStatus is an enum that makes it easy to distinguish between iterators that
// hit the end of their iteration successfully and genuine errors.
type IteratorStatus int32

const (
	invalid   = IteratorStatus(0)
	keepGoing = IteratorStatus(1)
	done      = IteratorStatus(2)
)

// extendedGSClient is an extended storage client.
type extendedGSClient struct {
	*storage.Client
}

// NewExtendedGSClient takes a Google Storage client and returns a wrapped version.
//
// A wrapped Google Storage client, much like the raw client it wraps, is intended to be
// a long-lived object. For that reason, we return an error value as well to make error handling
// more obvious at the call site.
func NewExtendedGSClient(client *storage.Client) (*extendedGSClient, error) {
	if client == nil {
		return nil, errors.New("new extended gs client: wrapped client cannot be nil")
	}
	return &extendedGSClient{client}, nil
}

// LsState stores the current iteration state as we're traversing the results of Ls.
type LsState struct {
	Attrs *storage.ObjectAttrs
	Err   error
}

// Reset resets the values of all the fields to zero.
func (s *LsState) Reset() {
	if s != nil {
		var zero LsState
		*s = zero
	}
}

// LsResult is an iterator over objects.
type LsResult func(state *LsState) bool

// Ls iterates over items in Google Storage beginning with a prefix.
func (e *extendedGSClient) Ls(ctx context.Context, bucket string, query *storage.Query) LsResult {
	objectIterator := e.Bucket(bucket).Objects(ctx, query)
	res := func(state *LsState) bool {
		state.Reset()
		objectAttrs, err := objectIterator.Next()
		if err != nil {
			state.Err = err
			return false
		}
		state.Attrs = objectAttrs
		return true
	}
	return res
}

// LsSmall synchronously grabs at most 10000 records.
//
// We fail and return no records if the limit is exceeded.
func (e *extendedGSClient) LsSmall(ctx context.Context, bucket string, query *storage.Query) ([]*storage.ObjectAttrs, error) {
	const smallQueryLimit = 10000
	var out []*storage.ObjectAttrs
	it := e.Ls(ctx, bucket, query)
	state := &LsState{}
	for it(state) {
		if len(out) >= smallQueryLimit {
			return nil, fmt.Errorf("ls small: limit %d on result set size exceeded", smallQueryLimit)
		}
		out = append(out, state.Attrs)
	}
	if state.Err == nil || errors.Is(state.Err, iterator.Done) {
		return out, nil
	}
	return nil, errors.Fmt("ls small: %w", state.Err)
}

// Expand name takes the name of a bucket and an object or prefix in that bucket and produces a GSUrl.
//
// If given inconsistent or invalid data, produce an empty string.
func (*extendedGSClient) ExpandName(bucket string, attrs *storage.ObjectAttrs) string {
	hasPrefix := attrs.Prefix != ""
	hasName := attrs.Name != ""
	if hasPrefix && hasName {
		// Return early. An ObjectAttrs value with both a prefix and a name is in an invalid state.
		return ""
	}
	if !hasPrefix && !hasName {
		return ""
	}
	if hasPrefix {
		return fmt.Sprintf("gs://%s/%s", bucket, attrs.Prefix)
	}
	return fmt.Sprintf("gs://%s/%s", bucket, attrs.Name)
}

// CountSections counts the number of sections excluding the protocol specifier in a Google Storage URL.
func (*extendedGSClient) CountSections(gsURL string) int {
	trimLead := regexp.MustCompile(`\Ags://`)
	trimTail := regexp.MustCompile(`/*\z`)
	gsURL = trimLead.ReplaceAllString(gsURL, "")
	gsURL = trimTail.ReplaceAllString(gsURL, "")
	if gsURL == "" {
		return 0
	}
	tally := 0
	for _, ch := range gsURL {
		if ch == '/' {
			tally++
		}
	}
	return 1 + tally
}

// EnsureTrailingSlash ensures exactly one trailing slash.
func (*extendedGSClient) EnsureTrailingSlash(s string) string {
	trimTail := regexp.MustCompile(`/*\z`)
	s = trimTail.ReplaceAllString(s, "")
	return fmt.Sprintf("%s/", s)
}
