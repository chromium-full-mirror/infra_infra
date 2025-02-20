// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rpc contains the top level RPC handlers for Sheriff-O-Matic.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"go.chromium.org/luci/common/errors"

	pb "go.chromium.org/infra/appengine/sheriff-o-matic/proto/v1"
	"go.chromium.org/infra/appengine/sheriff-o-matic/som/handler"
)

type alertsServer struct {
}

func NewAlertsServer() *pb.DecoratedAlerts {
	return &pb.DecoratedAlerts{
		Prelude:  checkAllowedPrelude,
		Service:  &alertsServer{},
		Postlude: gRPCifyAndLogPostlude,
	}
}

// ListAlerts lists the unresolved alerts for a given tree.
func (*alertsServer) ListAlerts(ctx context.Context, req *pb.ListAlertsRequest) (*pb.ListAlertsResponse, error) {
	if req.PageToken != "" {
		return nil, invalidArgumentError(errors.New("page_token not currently supported"))
	}
	tree, err := parseParentTree(req.Parent)
	if err != nil {
		return nil, invalidArgumentError(err)
	}
	summary, err := handler.GetAlerts(ctx, tree, true, false)
	if err != nil {
		return nil, errors.Annotate(err, "getting alerts").Err()
	}
	result := &pb.ListAlertsResponse{}
	for _, alert := range summary.Alerts {
		if req.PageSize > 0 && len(result.Alerts) >= int(req.PageSize) {
			break
		}
		bytes, err := json.Marshal(alert)
		if err != nil {
			return nil, errors.Annotate(err, "json encoding extension").Err()
		}
		a := &pb.Alert{
			Key:       alert.Key,
			AlertJson: string(bytes),
		}
		result.Alerts = append(result.Alerts, a)
	}
	return result, nil
}

var parentTreeExp = "^trees/([a-z_.]+)$"
var parentTreeExpCompiled = regexp.MustCompile(parentTreeExp)

// parseParentTree extracts the tree name from a ListAlerts parent string.
func parseParentTree(parent string) (string, error) {
	if parent == "" {
		return "", fmt.Errorf("parent must be provided")
	}
	matches := parentTreeExpCompiled.FindStringSubmatch(parent)
	if len(matches) < 2 {
		return "", fmt.Errorf("parent must be in the format %q", parentTreeExp)
	}
	return matches[1], nil
}
