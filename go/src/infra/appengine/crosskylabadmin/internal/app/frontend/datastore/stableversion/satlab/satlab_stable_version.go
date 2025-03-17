// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package satlab

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/gae/service/datastore"

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
)

const SatlabStableVersionKind = "satlab_stable_version"

// SatlabStableVersionEntry is a stable version entry.
type SatlabStableVersionEntry struct {
	_kind string `gae:"$kind,satlab_stable_version"`
	// ID has the following format. Earlier entries have precedence over later ones.
	//
	//   hostname     -- for a version scoped to a specific device.
	//   board|model  -- for a version scoped to just the board and the model.
	//
	ID string `gae:"$id"`
	// Base64Req stores a base64-encoded SetSatlabStableVersionRequest.
	// This request can be used to reconstitute this entire entry if required.
	Base64Req string `gae:"base64_req"`
	OS        string `gae:"os"`
	FW        string `gae:"fw"`
	FWImage   string `gae:"fw_image"`
}

// MakeSatlabStableVersionEntry creates a stable version entry from a stable version request.
func MakeSatlabStableVersionEntry(req *fleet.SetSatlabStableVersionRequest, normalizeCase bool) (*SatlabStableVersionEntry, error) {
	if req == nil {
		return nil, errors.Reason("make satlab stable version: request cannot be nil").Err()
	}
	var hostname string
	var board string
	var model string
	switch v := req.GetStrategy().(type) {
	case *fleet.SetSatlabStableVersionRequest_SatlabBoardAndModelStrategy:
		s := v.SatlabBoardAndModelStrategy
		board = s.GetBoard()
		model = s.GetModel()
		if normalizeCase {
			board = strings.ToLower(board)
			model = strings.ToLower(model)
		}
	case *fleet.SetSatlabStableVersionRequest_SatlabHostnameStrategy:
		hostname = v.SatlabHostnameStrategy.GetHostname()
		if normalizeCase {
			hostname = strings.ToLower(hostname)
		}
	}
	var base64Req string
	bytes, err := proto.Marshal(req)
	if err != nil {
		return nil, errors.Annotate(err, "make satlab stable version entry: marshalling proto failed").Err()
	}
	base64Req = base64.StdEncoding.EncodeToString(bytes)
	return &SatlabStableVersionEntry{
		ID:        MakeSatlabStableVersionID(hostname, board, model),
		Base64Req: base64Req,
		OS:        req.GetCrosVersion(),
		FW:        req.GetFirmwareVersion(),
		FWImage:   req.GetFirmwareImage(),
	}, nil
}

// PutSatlabStableVersionEntry puts a single SatlabStableVersionEntry in datastore.
func PutSatlabStableVersionEntry(ctx context.Context, entry *SatlabStableVersionEntry) error {
	if err := datastore.Put(ctx, entry); err != nil {
		return errors.Annotate(err, "put satlab stable version entry").Err()
	}
	return nil
}

// GetSatlabStableVersionEntryByRawID uses the ID to look up a satlab stable entry.
func GetSatlabStableVersionEntryByRawID(ctx context.Context, id string) (*SatlabStableVersionEntry, error) {
	entry := &SatlabStableVersionEntry{ID: id}
	if err := datastore.Get(ctx, entry); err != nil {
		return nil, errors.Annotate(err, "get satlab stable version entry").Err()
	}
	return entry, nil
}

// DeleteSatlabStableVersionEntryByRawID takes an ID and deletes the associated entry.
func DeleteSatlabStableVersionEntryByRawID(ctx context.Context, id string) error {
	entry := &SatlabStableVersionEntry{ID: id}
	err := datastore.Delete(ctx, entry)
	return errors.Annotate(err, "delete satlab stable version entry").Err()
}

// MakeSatlabStableVersionID takes a hostname, board, and model and combines them into an ID.
func MakeSatlabStableVersionID(hostname string, board string, model string) string {
	return makeSatlabStableVersionIDImpl(hostname, board, model, true)
}

// MakeSatlabStableVersionIDImpl takes a hostname, board, and model and combines them into an ID, possibly performing case normalization.
func makeSatlabStableVersionIDImpl(hostname string, board string, model string, normalizeCase bool) string {
	if hostname != "" {
		if normalizeCase {
			hostname = strings.TrimSpace(strings.ToLower(hostname))
		}
		return hostname
	}
	if model != "" && board != "" {
		if normalizeCase {
			board = strings.TrimSpace(strings.ToLower(board))
			model = strings.TrimSpace(strings.ToLower(model))
		}
		return fmt.Sprintf("%s|%s", board, model)
	}
	return ""
}
