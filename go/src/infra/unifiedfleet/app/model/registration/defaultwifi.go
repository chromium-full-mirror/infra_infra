// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package registration

import (
	"context"

	"github.com/golang/protobuf/proto"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "infra/unifiedfleet/api/v1/models"
	ufsds "infra/unifiedfleet/app/model/datastore"
)

// DefaultWifiKind is the datastore entity kind DefaultWifi.
const DefaultWifiKind string = "DefaultWifi"

// DefaultWifiEntry is a datastore entity that tracks DefaultWifi.
type DefaultWifiEntry struct {
	_kind string                `gae:"$kind,DefaultWifi"`
	Extra datastore.PropertyMap `gae:",extra"`
	ID    string                `gae:"$id"`
	// ufspb.DefaultWifi cannot be directly used as it contains pointer.
	DefaultWifi []byte `gae:",noindex"`
}

// GetProto returns the unmarshaled DefaultWifi.
func (e *DefaultWifiEntry) GetProto() (proto.Message, error) {
	var p ufspb.DefaultWifi
	if err := proto.Unmarshal(e.DefaultWifi, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate returns whether an DefaultWifiEntry is valid.
func (e *DefaultWifiEntry) Validate() error {
	return nil
}

func newDefaultWifiEntry(ctx context.Context, pm proto.Message) (ufsds.FleetEntity, error) {
	p := pm.(*ufspb.DefaultWifi)
	name := p.GetName()
	if name == "" {
		return nil, errors.Reason("Empty DefaultWifi name").Err()
	}
	wifi, err := proto.Marshal(p)
	if err != nil {
		return nil, errors.Annotate(err, "fail to marshal DefaultWifi %s", p).Err()
	}
	return &DefaultWifiEntry{
		ID:          name,
		DefaultWifi: wifi,
	}, nil
}

// NonAtomicBatchCreateDefaultWifis updates wifis in datastore in a non-atomic
// operation.
func NonAtomicBatchCreateDefaultWifis(ctx context.Context, wifis []*ufspb.DefaultWifi) ([]*ufspb.DefaultWifi, error) {
	return nonAtomicBatchCreateOrUpdateDefaultWifis(ctx, wifis, false /*create instead of update*/)
}

// NonAtomicBatchUpdateDefaultWifis updates DefaultWifis in datastore.
func NonAtomicBatchUpdateDefaultWifis(ctx context.Context, wifis []*ufspb.DefaultWifi) ([]*ufspb.DefaultWifi, error) {
	return nonAtomicBatchCreateOrUpdateDefaultWifis(ctx, wifis, true /*update instead of create*/)
}

func nonAtomicBatchCreateOrUpdateDefaultWifis(ctx context.Context, wifis []*ufspb.DefaultWifi, update bool) ([]*ufspb.DefaultWifi, error) {
	wifiProtos := make([]proto.Message, len(wifis))
	for i, w := range wifis {
		wifiProtos[i] = w
	}
	if _, err := ufsds.PutAll(ctx, wifiProtos, newDefaultWifiEntry, update); err != nil {
		return nil, err
	}
	return wifis, nil
}

// GetDefaultWifi returns the specified default wifi.
func GetDefaultWifi(ctx context.Context, name string) (*ufspb.DefaultWifi, error) {
	pm, err := ufsds.Get(ctx, &ufspb.DefaultWifi{Name: name}, newDefaultWifiEntry)
	if err == nil {
		return pm.(*ufspb.DefaultWifi), err
	}
	return nil, err
}

// DeleteDefaultWifi deletes the DefaultWifi in datastore.
func DeleteDefaultWifi(ctx context.Context, name string) error {
	return ufsds.Delete(ctx, &ufspb.DefaultWifi{Name: name}, newDefaultWifiEntry)
}
