// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"

	"github.com/golang/protobuf/proto"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
)

// CreateDefaultWifi creates a default wifi in the datastore.
func CreateDefaultWifi(ctx context.Context, wifi *ufspb.DefaultWifi) (*ufspb.DefaultWifi, error) {
	f := func(ctx context.Context) error {
		if err := validateCreateDefaultWifi(ctx, wifi); err != nil {
			return errors.Annotate(err, "CreateDefaultWifi - validation failed").Err()
		}
		if _, err := registration.NonAtomicBatchCreateDefaultWifis(ctx, []*ufspb.DefaultWifi{wifi}); err != nil {
			return errors.Annotate(err, "Unable to create wifi %s", wifi).Err()
		}
		hc := getDefaultWifiHistoryClient()
		hc.logDefaultWifiChanges(nil, wifi)
		return hc.SaveChangeEvents(ctx)
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		return nil, errors.Annotate(err, "CreateDefaultWifi for %s", wifi).Err()
	}
	return wifi, nil
}

func GetDefaultWifi(ctx context.Context, name string) (*ufspb.DefaultWifi, error) {
	return registration.GetDefaultWifi(ctx, name)
}

func ListDefaultWifis(ctx context.Context, pageSize int32, pageToken, filter string, keysOnly bool) (res []*ufspb.DefaultWifi, nextPageToken string, err error) {
	// DefaultWifi has no filters.
	filterMap := map[string][]any{}
	q, err := ufsds.ListQuery(ctx, registration.DefaultWifiKind, pageSize, pageToken, filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}
	var nextCur datastore.Cursor
	err = datastore.Run(ctx, q, func(ent *registration.DefaultWifiEntry, cb datastore.CursorCB) error {
		if keysOnly {
			wifi := &ufspb.DefaultWifi{
				Name: ent.ID,
			}
			res = append(res, wifi)
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
				return nil
			}
			res = append(res, pm.(*ufspb.DefaultWifi))
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
		logging.Errorf(ctx, "Failed to list DefaultWifi: %s", err)
		return nil, "", status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

func DeleteDefaultWifi(ctx context.Context, id string) error {
	f := func(ctx context.Context) error {
		// Get the DefaultWifi for logging.
		wifi, err := GetDefaultWifi(ctx, id)
		if err != nil {
			return errors.Annotate(err, "DeleteDefaultWifi - get DefaultWifi %s failed", id).Err()
		}
		if err := registration.DeleteDefaultWifi(ctx, id); err != nil {
			return errors.Annotate(err, "DeleteDefaultWifi - unable to delete DefaultWifi %s", id).Err()
		}
		hc := getDefaultWifiHistoryClient()
		hc.logDefaultWifiChanges(wifi, nil)
		return hc.SaveChangeEvents(ctx)
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		return errors.Annotate(err, "DeleteDefaultWifi - failed to delete DefaultWifi %s in datastore", id).Err()
	}
	return nil
}

// UpdateDefaultWifi updates existing DefaultWifi in datastore.
func UpdateDefaultWifi(ctx context.Context, wifi *ufspb.DefaultWifi, mask *field_mask.FieldMask) (*ufspb.DefaultWifi, error) {
	f := func(ctx context.Context) error {
		// Get old/existing DefaultWifi for logging and partial update.
		oldWifi, err := registration.GetDefaultWifi(ctx, wifi.GetName())
		if err != nil {
			return errors.Annotate(err, "UpdateDefaultWifi - get DefaultWifi %q failed", wifi.GetName()).Err()
		}
		// Validate the input.
		if err := validateUpdateDefaultWifi(ctx, oldWifi, wifi, mask); err != nil {
			return errors.Annotate(err, "UpdateDefaultWifi - validation failed").Err()
		}
		// Copy for logging.
		oldWifiCopy := oldWifi
		// Partial update by field mask.
		if mask != nil && len(mask.Paths) > 0 {
			// Validate partial update field mask.
			if err := validateDefaultWifiUpdateMask(ctx, wifi, mask); err != nil {
				return err
			}
			// Clone oldWifi for logging as the oldWifi will be updated with new values.
			oldWifiCopy = proto.Clone(oldWifi).(*ufspb.DefaultWifi)
			// Process the field mask to get updated values.
			wifi, err = processDefaultWifiUpdateMask(ctx, oldWifi, wifi, mask)
			if err != nil {
				return errors.Annotate(err, "UpdateDefaultWifi - processing update mask failed").Err()
			}
		}
		if _, err := registration.NonAtomicBatchUpdateDefaultWifis(ctx, []*ufspb.DefaultWifi{wifi}); err != nil {
			return errors.Annotate(err, "UpdateDefaultWifi - unable to batch update DefaultWifi %s", wifi.Name).Err()
		}
		hc := getDefaultWifiHistoryClient()
		hc.logDefaultWifiChanges(oldWifiCopy, wifi)
		return hc.SaveChangeEvents(ctx)
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		return nil, errors.Annotate(err, "UpdateDefaultWifi - failed to update DefaultWifi %s in datastore", wifi.Name).Err()
	}
	return wifi, nil
}

func getDefaultWifiHistoryClient() *HistoryClient {
	return &HistoryClient{}
}

// validateCreateDefaultWifi validates if a DefaultWifi can be created.
//
// checks if the DefaultWifi already exists.
func validateCreateDefaultWifi(ctx context.Context, cs *ufspb.DefaultWifi) error {
	// Check if DefaultWifi already exists.
	return resourceAlreadyExists(ctx, []*Resource{GetDefaultWifiResource(cs.Name)}, nil)
}

// validateUpdateDefaultWifi validates if an existing DefaultWifi can be updated.
func validateUpdateDefaultWifi(ctx context.Context, oldWifi *ufspb.DefaultWifi, wifi *ufspb.DefaultWifi, mask *field_mask.FieldMask) error {
	// Check if resources does not exist.
	return ResourceExist(ctx, []*Resource{GetDefaultWifiResource(wifi.Name)}, nil)
}

// validateDefaultWifiUpdateMask validates the update mask for DefaultWifi partial update.
func validateDefaultWifiUpdateMask(ctx context.Context, wifi *ufspb.DefaultWifi, mask *field_mask.FieldMask) error {
	if mask != nil {
		// Validate the give field mask.
		for _, path := range mask.Paths {
			switch path {
			case "name":
				return status.Error(codes.InvalidArgument, "validateDefaultWifiUpdateMask - name cannot be updated, delete and create a DefaultWifi instead")
			case "wifi_secret.project_id":
			case "wifi_secret.secret_name":
				// Valid fields, nothing to validate.
			default:
				return status.Errorf(codes.InvalidArgument, "validateDefaultWifiUpdateMask - unsupported update mask path %q", path)
			}
		}
	}
	return nil
}

// processDefaultWifiUpdateMask processes update field mask to get only specific
// update fields and return a complete DefaultWifi object with updated and
// existing fields.
func processDefaultWifiUpdateMask(ctx context.Context, oldWifi *ufspb.DefaultWifi, wifi *ufspb.DefaultWifi, mask *field_mask.FieldMask) (*ufspb.DefaultWifi, error) {
	// Update the fields in the existing/old DefaultWifi.
	for _, path := range mask.Paths {
		switch path {
		case "wifi_secret.project_id":
			oldWifi.GetWifiSecret().ProjectId = wifi.GetWifiSecret().GetProjectId()
		case "wifi_secret.secret_name":
			oldWifi.GetWifiSecret().SecretName = wifi.GetWifiSecret().GetSecretName()
		}
	}
	// Return existing/old DefaultWifi with new updated values.
	return oldWifi, nil
}
