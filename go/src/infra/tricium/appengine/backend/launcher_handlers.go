// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"io/ioutil"
	"net/http"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server/router"

	admin "go.chromium.org/infra/tricium/api/admin/v1"
)

var launcher = &launcherServer{}

func launchHandler(ctx *router.Context) {
	c, r, w := ctx.Request.Context(), ctx.Request, ctx.Writer
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logging.WithError(err).Errorf(c, "Failed to read request body.")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	lr := &admin.LaunchRequest{}
	if err := proto.Unmarshal(body, lr); err != nil {
		logging.WithError(err).Errorf(c, "Failed to unmarshal launch request.")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := launcher.Launch(c, lr); err != nil {
		logging.WithError(err).Errorf(c, "Launch failed.")
		switch grpc.Code(err) {
		case codes.InvalidArgument:
			w.WriteHeader(http.StatusBadRequest)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}
