// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dut

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/paths"
	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	ufsModels "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsApi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

type DeleteClient interface {
	DeleteAsset(context.Context, *ufsApi.DeleteAssetRequest, ...grpc.CallOption) (*emptypb.Empty, error)
	DeleteMachineLSE(context.Context, *ufsApi.DeleteMachineLSERequest, ...grpc.CallOption) (*emptypb.Empty, error)
	DeleteRack(context.Context, *ufsApi.DeleteRackRequest, ...grpc.CallOption) (*emptypb.Empty, error)
	GetMachineLSE(ctx context.Context, in *ufsApi.GetMachineLSERequest, opts ...grpc.CallOption) (*ufsModels.MachineLSE, error)
}

type DeleteDUT struct {
	// SatlabID the Satlab ID from the Command Line
	// if it is empty it will get the ID from the enviornment
	SatlabID string

	// full if true, it deletes `DUT`, `rack`, and `Asset`. Otherwise,
	// it only deletes `DUT`
	Full bool

	// Names contain the DUT name that we want to delete
	Names []string
}

type Result struct {
	Pass []string
	Fail []string
}

type DeleteDUTResult struct {
	// MachineLSEs show which machineLSE we want to delete
	MachineLSEs []*ufsModels.MachineLSE

	// DutResults contain the name which has passed or failed after
	// deleting DUT.
	DutResults *Result

	// AssetResults contain the name which has passed or failed after
	// deleting Asset
	AssetResults *Result

	// RackResults contain the name which has passed or failed after
	// deleting Rack
	RackResults *Result
}

// Validate verfiy the input is valid.
func (d *DeleteDUT) Validate() error {
	if len(d.Names) == 0 {
		return errors.New("dut names are empty")
	}
	return nil
}

// TriggerRun deletes the DUTs by given names
//
// If we want to pass the `Namespace`, we can set up it in the context
// e.g.
// ```
// import "go.chromium.org/infra/cmd/shivas/utils"
//
// ctx = utils.SetupContext(ctx, c.envFlags.GetNamespace())
// ```
func (d *DeleteDUT) TriggerRun(ctx context.Context, executor executor.IExecCommander, ufs DeleteClient) (*DeleteDUTResult, error) {
	var err error
	res := DeleteDUTResult{
		MachineLSEs:  []*ufsModels.MachineLSE{},
		DutResults:   &Result{},
		AssetResults: &Result{},
		RackResults:  &Result{},
	}
	if d.SatlabID == "" {
		d.SatlabID, err = satlabcommands.GetDockerHostBoxIdentifier(ctx, executor)
		if err != nil {
			return nil, errors.Annotate(err, "delete dut").Err()
		}
	}

	for idx, name := range d.Names {
		d.Names[idx] = site.MaybePrepend(site.Satlab, d.SatlabID, name)
	}

	duts := getAllDuts(ctx, d.Names, ufs)
	res.MachineLSEs = duts

	pass, fail := deleteAllDuts(ctx, d.Names, ufs)
	res.DutResults = &Result{
		Pass: pass,
		Fail: fail,
	}

	if d.Full {
		// Delete all assets for DUTs. If the DUT still exists (due to a
		// failure when deleting), the DeleteAsset RPC will return an error,
		// so we can be relatively sloppy when finding which assets to delete.
		assetsToDelete := []string{}
		for _, dut := range duts {
			assetsToDelete = append(assetsToDelete, dut.Machines...)
		}

		pass, fail = deleteAllAssets(ctx, assetsToDelete, ufs)
		res.AssetResults = &Result{
			Pass: pass,
			Fail: fail,
		}

		// Delete all racks. Similarly, if a rack still has assets associated
		// with it, the RPC will fail, so we can give a best effort attempt and
		// tell the user the RPC failed if there is some issue.
		//
		// In theory this is just `satlab-<id>-rack`, but it's easy enough to
		// use the actual rack that `GetMachineLSE` reports.
		racksToDelete := []string{}
		for _, dut := range duts {
			racksToDelete = append(racksToDelete, dut.Rack)
		}
		pass, fail = deleteAllRacks(ctx, racksToDelete, ufs)
		res.RackResults = &Result{
			Pass: pass,
			Fail: fail,
		}
	}

	return &res, nil
}

// getAllDuts fetches all DUTs with name in names.
//
// Should eventually be replaced with BatchGet or ConcurrentGet methods but
// since the caller will only be using this with a low # of DUTs is acceptable
// for now.
func getAllDuts(ctx context.Context, names []string, ufs DeleteClient) []*ufsModels.MachineLSE {
	machineLSEs := []*ufsModels.MachineLSE{}
	for _, n := range names {
		m, err := ufs.GetMachineLSE(ctx, &ufsApi.GetMachineLSERequest{
			Name: ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, n),
		})
		if err != nil {
			// skip error here.
		} else {
			machineLSEs = append(machineLSEs, m)
		}
	}

	return machineLSEs
}

// deleteAllDuts deletes all DUTs with certain names. Returns an two arrays
// with the names that have been deleted successfully and unsuccessfully.
func deleteAllDuts(ctx context.Context, names []string, ufs DeleteClient) ([]string, []string) {
	success := []string{}
	fail := []string{}

	for _, dut := range names {
		_, err := ufs.DeleteMachineLSE(ctx, &ufsApi.DeleteMachineLSERequest{
			Name: ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, dut),
		})
		if err != nil {
			fail = append(fail, dut)
		} else {
			success = append(success, dut)
			fmt.Fprintf(os.Stderr, "The dut %s has been deleted successfully, removing the DNS entry\n", dut)
			if err := DeleteDNSEntry(dut); err != nil {
				fmt.Fprintf(os.Stderr, "Cannot remove the DNS entry for %s\n", dut)
			}
		}
	}
	return success, fail
}

// deleteAllAssets deletes all Assets with certain names. Returns an two arrays
// with the names that have been deleted successfully and unsuccessfully.
func deleteAllAssets(ctx context.Context, names []string, ufs DeleteClient) ([]string, []string) {
	success := []string{}
	fail := []string{}

	for _, assetName := range names {
		_, err := ufs.DeleteAsset(ctx, &ufsApi.DeleteAssetRequest{
			Name: ufsUtil.AddPrefix(ufsUtil.AssetCollection, assetName),
		})
		if err != nil {
			fail = append(fail, assetName)
		} else {
			success = append(success, assetName)
		}
	}

	return success, fail
}

// deleteAllRacks deletes all Racks with certain names. Returns an two arrays
// with the names that have been deleted successfully and unsuccessfully.
func deleteAllRacks(ctx context.Context, names []string, ufs DeleteClient) ([]string, []string) {
	success := []string{}
	fail := []string{}

	for _, rackName := range names {
		_, err := ufs.DeleteRack(ctx, &ufsApi.DeleteRackRequest{
			Name: ufsUtil.AddPrefix(ufsUtil.RackCollection, rackName),
		})
		if err != nil {
			fail = append(fail, rackName)
		} else {
			success = append(success, rackName)
		}
	}

	return success, fail
}

// DeleteDNSEntry removes the dns entry from /etc/dut_hosts/hosts.
func DeleteDNSEntry(hostname string) error {
	args := []string{
		paths.DockerPath,
		"exec",
		"dns",
		"sed",
		"-i",
		"/" + hostname + "$/d",
		"/etc/dut_hosts/hosts",
	}
	return exec.Command(args[0], args[1:]...).Run()
}
