// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"io"
	"net/http"

	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/errors"

	fleetcostpb "go.chromium.org/infra/cros/fleetcost/api/models"
	"go.chromium.org/infra/cros/fleetcost/internal/utils"
)

// getSecureClient gets a secure http.Client pointed at a specific host.
//
// TODO(gregorynisbet): Remove this function as well as the dependency on authcli.Flags.
//
//	We should be able to manually construct an auth.Options object with the settings that we want.
//	However, I know for a fact that using authFlags to produce an authFlags.Options() object
//	produces a usable client. Sometime in the future, I will opportunistically replace this
//	function with something more reasonable.
func getSecureClient(ctx context.Context, host string, authFlags authcli.Flags) (*http.Client, error) {
	authOptions, err := authFlags.Options()
	if err != nil {
		return nil, errors.Annotate(err, "creating secure client").Err()
	}
	authOptions.UseIDTokens = true
	if authOptions.Audience == "" {
		authOptions.Audience = "https://" + host
	}
	authenticator := auth.NewAuthenticator(ctx, auth.InteractiveLogin, authOptions)
	httpClient, err := authenticator.Client()
	if err != nil {
		return nil, errors.Annotate(err, "creating secure client").Err()
	}
	return httpClient, nil
}

// Message showProto writes a proto message as an indentend object. Always adds a newline.
func showProto(dst io.Writer, message proto.Message) (int, error) {
	if dst == nil {
		return 0, errors.New("dest cannot be nil")
	}
	bytes, err := (&protojson.MarshalOptions{
		Indent: "  ",
	}).Marshal(message)
	if err != nil {
		return 0, errors.Annotate(err, "show proto").Err()
	}
	return dst.Write(append(bytes, byte('\n')))
}

// Function makeLocationRecorder makes a func(string) error that writes the location
// to somewhere on a command object.
//
// Sample usage:
//
//	c.Flags.Func("location", "where the device is located", makeLocationRecorder(&c.location))
func makeLocationRecorder(dest *fleetcostpb.Location) func(string) error {
	return func(value string) error {
		location, err := utils.ToLocation(value)
		if err != nil {
			return err
		}
		*dest = location
		return nil
	}
}

// Function makeTypeRecorder records the location of a type.
func makeTypeRecorder(dest *fleetcostpb.IndicatorType) func(string) error {
	return func(value string) error {
		typ, err := utils.ToIndicatorType(value)
		if err != nil {
			return err
		}
		*dest = typ
		return nil
	}
}

// Function makeCostCadenceRecorder records the cost cadence of a record.
func makeCostCadenceRecorder(dest *fleetcostpb.CostCadence) func(string) error {
	return func(value string) error {
		cadence, err := utils.ToCostCadence(value)
		if err != nil {
			return err
		}
		*dest = cadence
		return nil
	}
}

// Function makeMoneyRecorder records an argument in a *money.Money.
func makeMoneyRecorder(dest **money.Money) func(string) error {
	return func(value string) error {
		usd, err := utils.ToUSD(value)
		if err != nil {
			return err
		}
		*dest = usd
		return nil
	}
}
