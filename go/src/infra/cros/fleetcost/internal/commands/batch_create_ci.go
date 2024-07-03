// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/maruel/subcommands"
	"google.golang.org/genproto/googleapis/type/money"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	prpc "go.chromium.org/luci/grpc/prpc"

	"infra/cmdsupport/cmdlib"
	fleetcostpb "infra/cros/fleetcost/api/models"
	fleetcostAPI "infra/cros/fleetcost/api/rpc"
	"infra/cros/fleetcost/internal/site"
	"infra/cros/fleetcost/internal/validation"
)

var BatchCreateCICommand *subcommands.Command = &subcommands.Command{
	UsageLine: "batch-create-ci -csv ...",
	ShortDesc: "batch create models from a CSV",
	LongDesc:  "batch create models from a CSV",
	CommandRun: func() subcommands.CommandRun {
		c := &batchCreateCICommand{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.authFlags.RegisterIDTokenFlags(&c.Flags)
		c.commonFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.path, "path", "", "path to cost indicators")
		return c
	},
}

type batchCreateCICommand struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	commonFlags site.CommonFlags
	path        string
}

func (c *batchCreateCICommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *batchCreateCICommand) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	if c.path == "" {
		return errors.New("argument -path is mandatory")
	}
	host, err := c.commonFlags.Host()
	if err != nil {
		return errors.Annotate(err, "upload model v2 command").Err()
	}
	fh, err := os.Open(c.path)
	if err != nil {
		return fmt.Errorf("cannot open %q: %w", c.path, err)
	}
	csvReader := csv.NewReader(fh)
	records, err := csvReader.ReadAll()
	if err != nil {
		return fmt.Errorf("cannot read records from csv %q: %w", c.path, err)
	}
	if len(records) == 0 {
		return fmt.Errorf("rejecting file with no content %q", c.path)
	}
	var httpClient *http.Client
	if !c.commonFlags.HTTP() {
		var err error
		httpClient, err = getSecureClient(ctx, host, c.authFlags)
		if err != nil {
			return errors.Annotate(err, "create cost indicator command").Err()
		}
	}
	prpcClient := &prpc.Client{
		C:    httpClient,
		Host: host,
		Options: &prpc.Options{
			Insecure:      c.commonFlags.HTTP(),
			PerRPCTimeout: 30 * time.Second,
		},
	}
	fleetCostClient := fleetcostAPI.NewFleetCostPRPCClient(prpcClient)
	header := records[0]
	for i := range header {
		header[i] = strings.TrimSpace(header[i])
	}
	if err := validateHeader(header); err != nil {
		return errors.Annotate(err, "header is invalid").Err()
	}
	for _, line := range records[1:] {
		m, err := rowToMap(header, line)
		if err != nil {
			return err
		}
		if err := uploadRow(ctx, fleetCostClient, m); err != nil {
			return fmt.Errorf("error while uploading row: %w", err)
		}
	}
	return nil
}

// rowToMap takes a header and a row and produces a map from string to string consisting
// of the values of the row. It will trim whitespace from the ends only, but otherwise
// has no semantic awareness.
//
// rowToMap assumes that the header is already valid and doesn't contain any duplicates.
func rowToMap(header []string, row []string) (map[string]string, error) {
	if len(header) != len(row) {
		return nil, fmt.Errorf("length of header %d does not match length of row %d", len(header), len(row))
	}
	out := make(map[string]string, len(header))
	for i, key := range header {
		value := row[i]
		out[key] = strings.TrimSpace(value)
	}
	return out, nil
}

func uploadRow(ctx context.Context, fleetCostClient fleetcostAPI.FleetCostClient, m map[string]string) error {
	var typ fleetcostpb.IndicatorType
	if err := makeTypeRecorder(&typ)(m["type"]); err != nil {
		return err
	}
	board := m["board"]
	if replacement := site.ReservedCostIndicatorNames[board]; replacement != "" {
		return fmt.Errorf("board %q cannot be used in an indicator, use %q instead", board, replacement)
	}
	model := m["model"]
	var cost *money.Money
	if err := makeMoneyRecorder(&cost)(m["cost"]); err != nil {
		return errors.Annotate(err, "mistake in cost field %q", m["cost"]).Err()
	}
	var costCadence fleetcostpb.CostCadence
	if err := makeCostCadenceRecorder(&costCadence)(m["cadence"]); err != nil {
		return errors.Annotate(err, "mistake in cadence field %q", m["cadence"]).Err()
	}
	var burnoutRate float64
	if _, err := fmt.Sscan(m["burnout-rate"], &burnoutRate); err != nil {
		return errors.Annotate(err, "mistake in burnout rate %q", m["burnout-rate"]).Err()
	}
	var location fleetcostpb.Location
	if err := makeLocationRecorder(&location)(m["location"]); err != nil {
		return errors.Annotate(err, "mistake in location %q", m["location"]).Err()
	}
	description := m["decription"]
	var amortization float64
	if _, err := fmt.Sscan(m["amortization"], &amortization); err != nil {
		return errors.Annotate(err, "mistake in amortization %q", m["amortization"]).Err()
	}
	request := &fleetcostAPI.CreateCostIndicatorRequest{
		CostIndicator: &fleetcostpb.CostIndicator{
			// TODO(gregorynisbet): Remove name field.
			Name:                "",
			Type:                typ,
			Board:               board,
			Model:               model,
			Cost:                cost,
			CostCadence:         costCadence,
			BurnoutRate:         burnoutRate,
			Location:            location,
			Description:         description,
			AmortizationInYears: amortization,
		},
	}
	if err := validation.ValidateCreateCostIndicatorRequest(request); err != nil {
		return errors.Annotate(err, "create cost indicator command").Err()
	}
	_, err := fleetCostClient.CreateCostIndicator(ctx, request)
	if err != nil {
		return errors.Annotate(err, "create cost indicator command").Err()
	}
	return nil
}

func validateHeader(header []string) error {
	seen := make(map[string]bool, len(header))
	for _, key := range header {
		if _, didSee := seen[key]; didSee {
			return fmt.Errorf("column name %q appears at least twice in header", key)
		}
	}
	return nil
}
