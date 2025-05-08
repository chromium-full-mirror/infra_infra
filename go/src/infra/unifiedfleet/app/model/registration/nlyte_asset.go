// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package registration

import (
	"context"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// NlyteAssetKind is a datastore entity identifier for NlyteAsset
const NlyteAssetKind string = "NlyteAsset"

// NlyteAssetEntity is a datastore entity that tracks NlyteAssets
type NlyteAssetEntity struct {
	_kind       string                `gae:"$kind,NlyteAsset"`
	Extra       datastore.PropertyMap `gae:",extra"`
	Name        string                `gae:"$id"`
	Zone        string                `gae:"zone"`
	Type        string                `gae:"type"`
	Model       string                `gae:"model"`
	Rack        string                `gae:"rack"`
	BuildTarget string                `gae:"build_target"`
	Phase       string                `gae:"phase"`
	Tags        []string              `gae:"tags"`
	Realm       string                `gae:"realm"`
	Asset       []byte                `gae:",noindex"` // Marshalled Asset proto
}

// GetProto returns unmarshalled Asset.
func (a *NlyteAssetEntity) GetProto() (proto.Message, error) {
	var p ufspb.Asset
	if err := proto.Unmarshal(a.Asset, &p); err != nil {
		return nil, err
	}
	// Assign the realm and return the proto
	p.Realm = a.Realm
	return &p, nil
}

// Validate returns whether an NlyteAssetEntity is valid.
func (a *NlyteAssetEntity) Validate() error {
	return nil
}

func (a *NlyteAssetEntity) GetRealm() string {
	return a.Realm
}

// newNlyteAssetRealmEntity creates a new Realm entity object from proto message.
func newNlyteAssetRealmEntity(ctx context.Context, pm proto.Message) (ufsds.RealmEntity, error) {
	asset, err := newNlyteAssetEntity(ctx, pm)
	if err != nil {
		return nil, err
	}
	return asset.(*NlyteAssetEntity), nil
}

// newNlyteAssetEntity creates a new asset entity object from proto message.
func newNlyteAssetEntity(ctx context.Context, pm proto.Message) (ufsds.FleetEntity, error) {
	a, ok := pm.(*ufspb.Asset)
	if !ok {
		return nil, errors.Reason("Invalid asset (proto message is probably not asset or nil)").Err()
	}
	if a.GetName() == "" {
		return nil, errors.Reason("Empty Asset ID").Err()
	}
	// Assign realm to the proto. This will allow us to use BQ data with realms
	a.Realm = util.ToUFSRealm(a.GetLocation().GetZone().String())
	asset, err := proto.Marshal(a)
	if err != nil {
		return nil, errors.Annotate(err, "Failed to marshal asset %s", a).Err()
	}
	return &NlyteAssetEntity{
		Name:        a.GetName(),
		Zone:        a.GetLocation().GetZone().String(),
		Type:        a.GetType().String(),
		Model:       a.GetModel(),
		Rack:        a.GetLocation().GetRack(),
		BuildTarget: a.GetInfo().GetBuildTarget(),
		Phase:       a.GetInfo().GetPhase(),
		Tags:        a.GetTags(),
		Realm:       a.GetRealm(),
		Asset:       asset,
	}, nil
}

// GetNlyteAsset returns asset corresponding to the name.
func GetNlyteAsset(ctx context.Context, name string) (*ufspb.Asset, error) {
	pm, err := ufsds.Get(ctx, &ufspb.Asset{Name: name}, newNlyteAssetEntity)
	if err != nil {
		return nil, err
	}
	return pm.(*ufspb.Asset), err
}

// GetNlyteAssetACL routes the request to either the ACLed or
// unACLed method depending on the rollout status.
func GetNlyteAssetACL(ctx context.Context, id string) (*ufspb.Asset, error) {
	// Mlyte: No equivalent ACL method, so just call standard method.

	return GetNlyteAsset(ctx, id)
}

// getNlyteAssetACL returns a machine for the given ID after verifying the user
// has permission.
func getNlyteAssetACL(ctx context.Context, id string) (*ufspb.Asset, error) {
	pm, err := ufsds.GetACL(ctx, &ufspb.Asset{Name: id}, newAssetRealmEntity, util.RegistrationsGet)
	if err == nil {
		return pm.(*ufspb.Asset), err
	}
	return nil, err
}

// CreateNlyteAsset creates an asset record in the datastore using the given asset proto.
func CreateNlyteAsset(ctx context.Context, asset *ufspb.Asset) (*ufspb.Asset, error) {
	if asset == nil || asset.Name == "" || asset.Type == ufspb.AssetType_UNDEFINED || asset.Location == nil {
		return nil, errors.Reason("Invalid Asset [Asset is empty or one or more required fields are missing]").Err()
	}
	asset.UpdateTime = timestamppb.Now()
	pm, err := ufsds.Put(ctx, asset, newNlyteAssetEntity, false)
	if err != nil {
		return nil, err
	}
	return pm.(*ufspb.Asset), nil
}

// UpdateAsset updates the asset to the given asset proto.
func UpdateNlyteAsset(ctx context.Context, asset *ufspb.Asset) (*ufspb.Asset, error) {
	asset.UpdateTime = timestamppb.Now()
	pm, err := ufsds.Put(ctx, asset, newNlyteAssetEntity, true)
	if err != nil {
		return nil, err
	}
	return pm.(*ufspb.Asset), nil
}

// BatchUpdateNlyteAssets updates the assets to the datastore
func BatchUpdateNlyteAssets(ctx context.Context, assets []*ufspb.Asset) ([]*ufspb.Asset, error) {
	protos := make([]proto.Message, len(assets))
	updateTime := timestamppb.Now()
	for i, asset := range assets {
		if asset != nil {
			asset.UpdateTime = updateTime
			protos[i] = asset
		}
	}
	_, err := ufsds.PutAll(ctx, protos, newNlyteAssetEntity, false)
	if err == nil {
		return assets, nil
	}
	return nil, err
}

// ListNlyteAssets lists the nlyte assets
// Does a query over asset entities. Returns pageSize number of entities and a
// non-nil cursor if there are more results. pageSize must be positive
func ListNlyteAssets(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]any, keysOnly bool) (res []*ufspb.Asset, nextPageToken string, err error) {
	q, err := ufsds.ListQuery(ctx, NlyteAssetKind, pageSize, pageToken, filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}

	var nextCur datastore.Cursor
	err = datastore.Run(ctx, q, func(ent *NlyteAssetEntity, cb datastore.CursorCB) error {
		if keysOnly {
			asset := &ufspb.Asset{
				Name: ent.Name,
			}
			res = append(res, asset)
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to unmarshall nlyte asset: %s", err)
				return nil
			}
			res = append(res, pm.(*ufspb.Asset))
		}
		if len(res) >= int(pageSize) {
			if nextCur, err = cb(); err != nil {
				return err
			}
			return datastore.Stop
		}
		return nil
	})
	if err != nil {
		logging.Errorf(ctx, "Failed to list nlyte assets %s", err)
		return nil, "", status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}
