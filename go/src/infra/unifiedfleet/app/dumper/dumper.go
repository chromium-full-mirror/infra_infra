// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"context"
	"time"

	"cloud.google.com/go/bigquery"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	bqlib "infra/cros/lab_inventory/bq"
	"infra/unifiedfleet/app/model/configuration"
	"infra/unifiedfleet/app/util"
)

// dumpDaily dumps a snapshot to BQ daily
func dumpDaily(ctx context.Context) error {
	ctx = logging.SetLevel(ctx, logging.Info)
	if err := exportToBQ(ctx, dumpToBQDaily); err != nil {
		return err
	}
	return nil
}

// dumpHourly Similar to dumpDaily, but hourly
func dumpHourly(ctx context.Context) error {
	ctx = logging.SetLevel(ctx, logging.Info)
	if err := exportToBQ(ctx, dumpToBQHourly); err != nil {
		return err
	}
	return nil
}

func dumpToBQDaily(ctx context.Context, bqClient *bigquery.Client) (err error) {
	defer func() {
		ns := util.GetNamespaceFromCtx(ctx)
		dumpToBQDailyTick.Add(ctx, 1, err == nil, ns)
	}()
	err = dumpToBQ(ctx, bqClient, dumperFrequencyDaily)
	return
}

func dumpToBQHourly(ctx context.Context, bqClient *bigquery.Client) (err error) {
	defer func() {
		ns := util.GetNamespaceFromCtx(ctx)
		dumpToBQHourlyTick.Add(ctx, 1, err == nil, ns)
	}()
	err = dumpToBQ(ctx, bqClient, dumperFrequencyHourly)
	return
}

// TODO(echoyang@): Parallelize
func dumpToBQ(ctx context.Context, bqClient *bigquery.Client, frequency dumperFrequency) error {
	logging.Infof(ctx, "Dumping to BQ")
	curTime := time.Now()
	curTimeStr := bqlib.GetPSTTimeStamp(curTime)
	if err := configuration.SaveProjectConfig(ctx, &configuration.ProjectConfigEntity{
		Name:             getProject(ctx),
		DailyDumpTimeStr: curTimeStr,
	}); err != nil {
		return err
	}
	var errs []error
	if err := dumpConfigurations(ctx, bqClient, curTimeStr, frequency); err != nil {
		errs = append(errs, errors.Annotate(err, "dump configurations").Err())
	}
	if err := dumpRegistration(ctx, bqClient, curTimeStr, frequency); err != nil {
		errs = append(errs, errors.Annotate(err, "dump registrations").Err())
	}
	if err := dumpInventory(ctx, bqClient, curTimeStr, frequency); err != nil {
		errs = append(errs, errors.Annotate(err, "dump inventories").Err())
	}
	if err := dumpState(ctx, bqClient, curTimeStr, frequency); err != nil {
		errs = append(errs, errors.Annotate(err, "dump states").Err())
	}
	logging.Debugf(ctx, "Dump successfully finished")
	return errors.Join(errs...)
}

func dumpChangeEvent(ctx context.Context) (err error) {
	defer func() {
		dumpChangeEventTick.Add(ctx, 1, err == nil)
	}()
	ctx = logging.SetLevel(ctx, logging.Info)
	logging.Debugf(ctx, "Dumping change event to BQ")
	return exportToBQ(ctx, dumpChangeEventHelper)
}

func dumpChangeSnapshots(ctx context.Context) (err error) {
	defer func() {
		dumpChangeSnapshotTick.Add(ctx, 1, err == nil)
	}()
	//TODO(b/368319251): Temp enable logs
	ctx = logging.SetLevel(ctx, logging.Debug)
	logging.Debugf(ctx, "Dumping change snapshots to BQ")
	return exportToBQ(ctx, dumpChangeSnapshotHelperBatched)
}

func dumpCrosNetwork(ctx context.Context) (err error) {
	defer func() {
		dumpCrosNetworkTick.Add(ctx, 1, err == nil)
	}()
	// In UFS write to 'os' namespace
	ctx, err = util.SetupDatastoreNamespace(ctx, util.OSNamespace)
	if err != nil {
		return err
	}
	return importCrosNetwork(ctx)
}

// unique key used to store and retrieve context.
var contextKey = util.Key("ufs bigquery-client key")
var projectKey = util.Key("ufs project key")

// Use installs bigquery client to context.
func Use(ctx context.Context, bqClient *bigquery.Client) context.Context {
	return context.WithValue(ctx, contextKey, bqClient)
}

func get(ctx context.Context) *bigquery.Client {
	return ctx.Value(contextKey).(*bigquery.Client)
}

// UseProject installs project name to context.
func UseProject(ctx context.Context, project string) context.Context {
	return context.WithValue(ctx, projectKey, project)
}

func getProject(ctx context.Context) string {
	return ctx.Value(projectKey).(string)
}

func exportToBQ(ctx context.Context, f func(ctx context.Context, bqClient *bigquery.Client) error) error {
	var mErr error
	for _, ns := range util.ClientToDatastoreNamespace {
		newCtx, err := util.SetupDatastoreNamespace(ctx, ns)
		if ns == "" {
			// This is only for printing error message for default namespace.
			ns = "default (chrome)"
		}
		logging.Infof(newCtx, "Exporting to BQ for namespace %q", ns)
		if err != nil {
			logging.Errorf(ctx, "Setting namespace %q failed, BQ export skipped: %s", ns, err.Error())
			mErr = errors.NewMultiError(mErr, err)
			continue
		}
		err = f(newCtx, get(newCtx))
		if err != nil {
			logging.Errorf(ctx, "BQ export failed for the namespace %q: %s", ns, err.Error())
			mErr = errors.NewMultiError(mErr, err)
		}
	}
	return mErr
}
