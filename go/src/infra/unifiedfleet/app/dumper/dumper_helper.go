// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/storage"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	bqlib "infra/cros/lab_inventory/bq"
	ufspb "infra/unifiedfleet/api/v1/models"
	apibq "infra/unifiedfleet/api/v1/models/bigquery"
	chromeoslab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	"infra/unifiedfleet/app/config"
	"infra/unifiedfleet/app/model/configuration"
	"infra/unifiedfleet/app/model/history"
	"infra/unifiedfleet/app/util"
)

const pageSize = 500
const batchSize = 500

type dumperFrequency int32

const (
	dumperFrequencyUnspecified dumperFrequency = iota
	dumperFrequencyDaily
	dumperFrequencyHourly
)

func dumpChangeEventHelper(ctx context.Context, bqClient *bigquery.Client) error {
	ns := util.GetNamespaceFromCtx(ctx)
	dataset := DatastoreNamespaceToBigQueryDataset[ns]
	uploader := bqlib.InitBQUploaderWithClient(ctx, bqClient, dataset, "change_events")
	changes, err := history.GetAllChangeEventEntities(ctx)
	if err != nil {
		return errors.Annotate(err, "get all change events' entities").Err()
	}
	msgs := make([]proto.Message, 0)
	for _, p := range changes {
		data, err := p.GetProto()
		if err != nil {
			continue
		}
		msg := &apibq.ChangeEventRow{
			ChangeEvent: data.(*ufspb.ChangeEvent),
		}
		msgs = append(msgs, msg)
	}
	logging.Debugf(ctx, "Dumping %d change events to BigQuery", len(msgs))
	if err := uploader.Put(ctx, msgs...); err != nil {
		logging.Debugf(ctx, "fail to upload: %s", err.Error())
		return err
	}
	logging.Debugf(ctx, "Finish uploading change events successfully")
	logging.Debugf(ctx, "Deleting uploaded entities")
	if err := history.DeleteChangeEventEntities(ctx, changes); err != nil {
		logging.Debugf(ctx, "fail to delete entities: %s", err.Error())
		return err
	}
	logging.Debugf(ctx, "Finish deleting successfully")
	return nil
}

func dumpChangeSnapshotHelperBatched(ctx context.Context, bqClient *bigquery.Client) error {
	var curTimeStr string
	var currCursor datastore.Cursor
	proConfig, err := configuration.GetProjectConfig(ctx, getProject(ctx))
	if err != nil {
		curTimeStr = bqlib.GetPSTTimeStamp(time.Now())
	} else {
		curTimeStr = proConfig.DailyDumpTimeStr
	}

	for {
		q := datastore.NewQuery(history.SnapshotMsgKind).Limit(batchSize).FirestoreMode(true)
		if currCursor != nil {
			q.Start(currCursor)
		}
		msgs := make(map[string][]proto.Message, 0)
		count := 0
		var snapshots []*history.SnapshotMsgEntity
		f := func(s history.SnapshotMsgEntity, cb datastore.CursorCB) error {
			snapshots = append(snapshots, &s)
			resourceType := util.GetPrefix(s.ResourceName)
			logging.Debugf(ctx, "handling %s", s.ResourceName)
			count += 1
			switch resourceType {
			case util.MachineCollection:
				var data ufspb.Machine
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["machines"] = append(msgs["machines"], &apibq.MachineRow{
					Machine: &data,
					Delete:  s.Delete,
				})
			case util.NicCollection:
				var data ufspb.Nic
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["nics"] = append(msgs["nics"], &apibq.NicRow{
					Nic:    &data,
					Delete: s.Delete,
				})
			case util.DracCollection:
				var data ufspb.Drac
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["dracs"] = append(msgs["dracs"], &apibq.DracRow{
					Drac:   &data,
					Delete: s.Delete,
				})
			case util.RackCollection:
				var data ufspb.Rack
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["racks"] = append(msgs["racks"], &apibq.RackRow{
					Rack:   &data,
					Delete: s.Delete,
				})
			case util.KVMCollection:
				var data ufspb.KVM
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["kvms"] = append(msgs["kvms"], &apibq.KVMRow{
					Kvm:    &data,
					Delete: s.Delete,
				})
			case util.SwitchCollection:
				var data ufspb.Switch
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["switches"] = append(msgs["switches"], &apibq.SwitchRow{
					Switch: &data,
					Delete: s.Delete,
				})
			case util.HostCollection:
				var data ufspb.MachineLSE
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["machine_lses"] = append(msgs["machine_lses"], &apibq.MachineLSERow{
					MachineLse: &data,
					Delete:     s.Delete,
				})
			case util.VMCollection:
				var data ufspb.VM
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["vms"] = append(msgs["vms"], &apibq.VMRow{
					Vm:     &data,
					Delete: s.Delete,
				})
			case util.DHCPCollection:
				var data ufspb.DHCPConfig
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["dhcps"] = append(msgs["dhcps"], &apibq.DHCPConfigRow{
					DhcpConfig: &data,
					Delete:     s.Delete,
				})
			case util.StateCollection:
				var data ufspb.StateRecord
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["state_records"] = append(msgs["state_records"], &apibq.StateRecordRow{
					StateRecord: &data,
					Delete:      s.Delete,
				})
			case util.DutStateCollection:
				var data chromeoslab.DutState
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["dutstates"] = append(msgs["dutstates"], &apibq.DUTStateRecordRow{
					State: &data,
				})
			case util.CachingServiceCollection:
				var data ufspb.CachingService
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["caching_services"] = append(msgs["caching_services"], &apibq.CachingServiceRow{
					CachingService: &data,
					Delete:         s.Delete,
				})
			case util.MachineLSEDeploymentCollection:
				var data ufspb.MachineLSEDeployment
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["machine_lse_deployments"] = append(msgs["machine_lse_deployments"], &apibq.MachineLSEDeploymentRow{
					MachineLseDeployment: &data,
					Delete:               s.Delete,
				})
			case util.SchedulingUnitCollection:
				var data ufspb.SchedulingUnit
				if err := s.GetProto(&data); err != nil {
					logging.Debugf(ctx, "Wrong data type error: %s", err)
					return nil
				}
				msgs["scheduling_units"] = append(msgs["scheduling_units"], &apibq.SchedulingUnitRow{
					SchedulingUnit: &data,
					Delete:         s.Delete,
				})
			}
			if count >= batchSize {
				if currCursor, err = cb(); err != nil {
					return err
				}
				return datastore.Stop
			}
			return nil
		}
		if err := datastore.Run(ctx, q, f); err != nil {
			logging.Errorf(ctx, "Failed to get snapshots from BQ. %s", err)
			return err
		}
		for tableName, ms := range msgs {
			logging.Debugf(ctx, "Uploading %d entries to %v", len(ms), tableName)
			table := fmt.Sprintf("%s$%s", tableName, curTimeStr)
			if err := uploadDumpToBQ(ctx, bqClient, ms, table); err != nil {
				return err
			}
		}
		logging.Debugf(ctx, "Finish uploading the snapshots successfully")
		logging.Debugf(ctx, "Deleting the %d uploaded snapshots", len(snapshots))
		if err := history.DeleteSnapshotMsgEntities(ctx, snapshots); err != nil {
			logging.Debugf(ctx, "fail to delete snapshot msg entities: %s", err.Error())
			return err
		}
		if currCursor == nil || count < batchSize {
			break
		}
	}
	return nil
}

func dumpChangeSnapshotHelper(ctx context.Context, bqClient *bigquery.Client) error {
	snapshots, err := history.GetAllSnapshotMsg(ctx)
	if err != nil {
		return errors.Annotate(err, "get all snapshot msg entities").Err()
	}

	var curTimeStr string
	proConfig, err := configuration.GetProjectConfig(ctx, getProject(ctx))
	if err != nil {
		curTimeStr = bqlib.GetPSTTimeStamp(time.Now())
	} else {
		curTimeStr = proConfig.DailyDumpTimeStr
	}

	msgs := make(map[string][]proto.Message, 0)
	for _, s := range snapshots {
		resourceType := util.GetPrefix(s.ResourceName)
		logging.Debugf(ctx, "handling %s", s.ResourceName)
		switch resourceType {
		case util.MachineCollection:
			var data ufspb.Machine
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["machines"] = append(msgs["machines"], &apibq.MachineRow{
				Machine: &data,
				Delete:  s.Delete,
			})
		case util.NicCollection:
			var data ufspb.Nic
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["nics"] = append(msgs["nics"], &apibq.NicRow{
				Nic:    &data,
				Delete: s.Delete,
			})
		case util.DracCollection:
			var data ufspb.Drac
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["dracs"] = append(msgs["dracs"], &apibq.DracRow{
				Drac:   &data,
				Delete: s.Delete,
			})
		case util.RackCollection:
			var data ufspb.Rack
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["racks"] = append(msgs["racks"], &apibq.RackRow{
				Rack:   &data,
				Delete: s.Delete,
			})
		case util.KVMCollection:
			var data ufspb.KVM
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["kvms"] = append(msgs["kvms"], &apibq.KVMRow{
				Kvm:    &data,
				Delete: s.Delete,
			})
		case util.SwitchCollection:
			var data ufspb.Switch
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["switches"] = append(msgs["switches"], &apibq.SwitchRow{
				Switch: &data,
				Delete: s.Delete,
			})
		case util.HostCollection:
			var data ufspb.MachineLSE
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["machine_lses"] = append(msgs["machine_lses"], &apibq.MachineLSERow{
				MachineLse: &data,
				Delete:     s.Delete,
			})
		case util.VMCollection:
			var data ufspb.VM
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["vms"] = append(msgs["vms"], &apibq.VMRow{
				Vm:     &data,
				Delete: s.Delete,
			})
		case util.DHCPCollection:
			var data ufspb.DHCPConfig
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["dhcps"] = append(msgs["dhcps"], &apibq.DHCPConfigRow{
				DhcpConfig: &data,
				Delete:     s.Delete,
			})
		case util.StateCollection:
			var data ufspb.StateRecord
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["state_records"] = append(msgs["state_records"], &apibq.StateRecordRow{
				StateRecord: &data,
				Delete:      s.Delete,
			})
		case util.DutStateCollection:
			var data chromeoslab.DutState
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["dutstates"] = append(msgs["dutstates"], &apibq.DUTStateRecordRow{
				State: &data,
			})
		case util.CachingServiceCollection:
			var data ufspb.CachingService
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["caching_services"] = append(msgs["caching_services"], &apibq.CachingServiceRow{
				CachingService: &data,
				Delete:         s.Delete,
			})
		case util.MachineLSEDeploymentCollection:
			var data ufspb.MachineLSEDeployment
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["machine_lse_deployments"] = append(msgs["machine_lse_deployments"], &apibq.MachineLSEDeploymentRow{
				MachineLseDeployment: &data,
				Delete:               s.Delete,
			})
		case util.SchedulingUnitCollection:
			var data ufspb.SchedulingUnit
			if err := s.GetProto(&data); err != nil {
				continue
			}
			msgs["scheduling_units"] = append(msgs["scheduling_units"], &apibq.SchedulingUnitRow{
				SchedulingUnit: &data,
				Delete:         s.Delete,
			})
		}
	}
	logging.Debugf(ctx, "Uploading all %d snapshots...", len(snapshots))
	for tableName, ms := range msgs {
		table := fmt.Sprintf("%s$%s", tableName, curTimeStr)
		if err := uploadDumpToBQ(ctx, bqClient, ms, table); err != nil {
			return err
		}
	}
	logging.Debugf(ctx, "Finish uploading the snapshots successfully")
	logging.Debugf(ctx, "Deleting the uploaded snapshots")
	if err := history.DeleteSnapshotMsgEntities(ctx, snapshots); err != nil {
		logging.Debugf(ctx, "fail to delete snapshot msg entities: %s", err.Error())
		return err
	}
	logging.Debugf(ctx, "Finish deleting the snapshots successfully")
	return nil
}

func dumpConfigurations(ctx context.Context, bqClient *bigquery.Client, curTimeStr string, frequency dumperFrequency) error {
	return dumpTables(ctx, bqClient, curTimeStr, frequency, configurationDumpToolkit)
}

func dumpRegistration(ctx context.Context, bqClient *bigquery.Client, curTimeStr string, frequency dumperFrequency) error {
	return dumpTables(ctx, bqClient, curTimeStr, frequency, registrationDumpToolkit)
}

func dumpInventory(ctx context.Context, bqClient *bigquery.Client, curTimeStr string, frequency dumperFrequency) error {
	return dumpTables(ctx, bqClient, curTimeStr, frequency, inventoryDumpToolkit)
}

func dumpState(ctx context.Context, bqClient *bigquery.Client, curTimeStr string, frequency dumperFrequency) error {
	return dumpTables(ctx, bqClient, curTimeStr, frequency, stateDumpToolkit)
}

func dumpTables(ctx context.Context, bqClient *bigquery.Client, curTimeStr string, frequency dumperFrequency, funcs map[string]getAllFunc) error {
	var errs []error
	for k, f := range funcs {
		logging.Infof(ctx, "dumping %s", k)
		msgs, err := f(ctx)
		if err != nil {
			errs = append(errs, err)
		}
		name := k
		if len(msgs) == 0 {
			logging.Infof(ctx, "0 records found for %s table", name)
			continue
		}
		switch frequency {
		case dumperFrequencyDaily:
			name = fmt.Sprintf("%s$%s", k, curTimeStr)
		case dumperFrequencyHourly:
			name = fmt.Sprintf("%s_hourly", k)
		default:
			return errors.Reason("Dumper frequency %v is invalid", frequency).Err()
		}
		if err := uploadDumpToBQ(ctx, bqClient, msgs, name); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// getCloudStorageWriter creates a storage writer that uses the UFS bucket
func getCloudStorageWriter(ctx context.Context, filename string) (*storage.Writer, error) {
	bucketName := config.Get(ctx).SelfStorageBucket
	if bucketName == "" {
		bucketName = "unified-fleet-system.appspot.com"
	}
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		logging.Warningf(ctx, "failed to create cloud storage client")
		return nil, err
	}
	bucket := storageClient.Bucket(bucketName)
	logging.Infof(ctx, "The resulting file will be written to https://storage.cloud.google.com/%s/%s", bucketName, filename)
	return bucket.Object(filename).NewWriter(ctx), nil
}
