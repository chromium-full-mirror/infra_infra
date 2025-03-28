// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dumper

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/sync/parallel"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/controller"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

const (
	// Nlyte "DUT STD" Material ID
	nlyteMaterialIDChromeOSAsset = 25144
)

var (
	//go:embed nlyte_bq_sync.sql
	nlyteQueryGetMountedDuts string
)

type assetResult struct {
	ID                 int
	Tag                string
	Row, Rack          string
	Host               int
	Model, Board, Zone bigquery.NullString
}

func (a assetResult) toAssetProto() (*ufspb.Asset, error) {
	var merr []error

	zoneString := strings.ToUpper(a.Zone.StringVal)
	if !strings.HasPrefix(zoneString, "ZONE_") {
		zoneString = fmt.Sprintf("ZONE_%s", zoneString)
	}
	zoneInt, ok := ufspb.Zone_value[zoneString]
	if !ok {
		merr = append(merr, errors.New(fmt.Sprintf("unknown zone %q", zoneString)))
	}

	return &ufspb.Asset{
		Name:  a.Tag,
		Type:  ufspb.AssetType_DUT,
		Model: a.Model.StringVal,
		Info: &ufspb.AssetInfo{
			AssetTag:    a.Tag,
			BuildTarget: a.Board.StringVal,
			Model:       a.Model.StringVal,
		},
		Location: &ufspb.Location{
			Zone:     ufspb.Zone(zoneInt),
			Row:      a.Row,
			Rack:     a.Rack,
			Position: strconv.Itoa(a.Host),
		},
	}, errors.Join(merr...)
}

func fetchNlyteBigQueryData(ctx context.Context) (rErr error) {
	logging.Debugf(ctx, "Entering Nlyte Sync")
	defer logging.Debugf(ctx, "Exiting Nlyte sync")
	defer func() {
		fetchNlyteBigQueryDataTick.Add(ctx, 1, rErr == nil)
	}()
	logging.Infof(ctx, "Setting namespce as OS")
	ctx, err := util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	client, err := bigquery.NewClient(ctx, "nlyte-tng-prod")
	if err != nil {
		return fmt.Errorf("bigquery.NewClient: %w", err)
	}
	defer client.Close()

	// Location must match that of the dataset(s) referenced in the query.
	client.Location = "US"
	q := client.Query(nlyteQueryGetMountedDuts)
	q.Parameters = []bigquery.QueryParameter{
		{
			Name:  "material_ids",
			Value: []int{nlyteMaterialIDChromeOSAsset},
		},
	}
	it, err := q.Read(ctx)
	if err != nil {
		return errors.Annotate(err, "executing nlyte query").Err()
	}

	return parallel.WorkPool(16, func(c chan<- func() error) {
		for {
			var row assetResult
			if err := it.Next(&row); err != nil {
				if !errors.Is(err, iterator.Done) {
					c <- func() error {
						return errors.Annotate(err, "reading nlyte query results").Err()
					}
				}
				break
			} else {
				logging.Debugf(ctx, "Nlyte sync attempt got data: %+v", row)
				c <- func() error {
					na, err := row.toAssetProto()
					if err != nil {
						return err
					}
					return nlyteUpcertAsset(ctx, na)
				}
			}
		}
	})
}

// Create a new nlyte asset Kind if one does not exist, otherwise update an existing nlyte asset Kind.
func nlyteUpcertAsset(ctx context.Context, nlyteAsset *ufspb.Asset) error {
	ufsAsset, err := controller.GetNlyteAsset(ctx, nlyteAsset.Name)
	if err == nil {
		logging.Debugf(ctx, "Nlyte kind asset %s found", nlyteAsset.Name)
		if proto.Equal(ufsAsset, nlyteAsset) {
			logging.Debugf(ctx, "Nlyte kind asset %s is up to date", nlyteAsset.Name)
		} else {
			logging.Debugf(ctx, "Nlyte kind asset %s is out of date, updating", nlyteAsset.Name)
			_, err = controller.UpdateNlyteAsset(ctx, nlyteAsset, nil)
		}
	} else if util.IsNotFoundError(err) {
		logging.Debugf(ctx, "Unable to find nlye kind asset %s, creating", nlyteAsset.Name)
		_, err = controller.NlyteAssetRegistration(ctx, nlyteAsset)
	}
	return err
}
