// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package experimental

import (
	"context"
	"fmt"
	"strconv"

	"github.com/maruel/subcommands"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// DumpNlyteCmd dumps updated entries from Nlyte to UFS.
var DumpNlyteCmd = &subcommands.Command{
	UsageLine: "nlyte ...",
	ShortDesc: "Dump nlyte updates",
	LongDesc: `Dump nlyte updates from the provided json file.
Example:
shivas nlyte -f testing.json`,
	CommandRun: func() subcommands.CommandRun {
		c := &dumpNlyte{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		c.Flags.StringVar(&c.updatedEntryFile, "f", "", "Path to a file containing AssetAndHosts specification in JSON format.")

		c.outputFlags.Register(&c.Flags)
		return c
	},
}

type dumpNlyte struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	updatedEntryFile string

	outputFlags site.OutputFlags
}

func (c *dumpNlyte) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *dumpNlyte) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	ctx = utils.SetupContext(ctx, ufsUtil.OSNamespace)
	hc, err := cmdlib.NewHTTPClient(ctx, &c.authFlags)
	if err != nil {
		return err
	}
	e := c.envFlags.Env()
	if c.commonFlags.Verbose() {
		fmt.Printf("Using UFS service %s\n", e.UnifiedFleetService)
	}
	ic := ufsAPI.NewFleetPRPCClient(&prpc.Client{
		C:       hc,
		Host:    e.UnifiedFleetService,
		Options: site.DefaultPRPCOptions(c.envFlags),
	})

	if c.updatedEntryFile == "" {
		return errors.New("Failed to dump asset from Nlyte: empty json filename")
	}

	var assetAndHosts ufspb.AssetAndHosts
	if err := utils.ParseJSONFile(c.updatedEntryFile, &assetAndHosts); err != nil {
		return fmt.Errorf("Failed to dump asset from Nlyte: %w", err)
	}

	failedAssets := []string{}

	for _, assetAndHostInfo := range assetAndHosts.GetRecords() {
		if assetAndHostInfo.GetAssetName() == "" {
			fmt.Printf("Failed to dump asset %s from Nlyte: empty AssetName\n", assetAndHostInfo.String())
			continue
		}

		currentAsset, err := ic.GetAsset(ctx, &ufsAPI.GetAssetRequest{
			Name: ufsUtil.AddPrefix(ufsUtil.AssetCollection, assetAndHostInfo.GetAssetName()),
		})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				newAsset := &ufspb.Asset{
					Name: assetAndHostInfo.GetAssetName(),
					Info: &ufspb.AssetInfo{
						AssetTag: assetAndHostInfo.GetAssetName(),
					},
					Location: &ufspb.Location{},
				}

				if err := c.parseRecord(ctx, assetAndHostInfo, newAsset); err != nil {
					failedAssets = append(failedAssets, assetAndHostInfo.GetAssetName())
					fmt.Printf("Failed to dump asset %s from Nlyte: %s\n", assetAndHostInfo.GetAssetName(), err)
					continue
				}
				req := &ufsAPI.CreateAssetRequest{
					Asset: newAsset,
				}
				req.Asset.Name = ufsUtil.AddPrefix(ufsUtil.AssetCollection, req.Asset.Name)

				res, err := c.addAssetToUFS(ctx, ic, req)
				if err != nil {
					failedAssets = append(failedAssets, newAsset.GetName())
					fmt.Printf("Failed to dump asset %s from Nlyte: %s\n", newAsset.String(), err)
					continue
				}
				utils.PrintProtoJSON(res, !utils.NoEmitMode(false))
				fmt.Println("Successfully added the asset: ", res.GetName())
			} else {
				failedAssets = append(failedAssets, assetAndHostInfo.GetAssetName())
				fmt.Printf("Failed to dump asset %s from Nlyte: %s\n", assetAndHostInfo.GetAssetName(), err)
			}
			continue
		}

		if _, err := utils.PrintExistingAsset(ctx, ic, assetAndHostInfo.GetAssetName()); err != nil {
			failedAssets = append(failedAssets, assetAndHostInfo.GetAssetName())
			fmt.Printf("Failed to dump asset %s from Nlyte: %s\n", assetAndHostInfo.GetAssetName(), err)
			continue
		}

		if err := c.parseRecord(ctx, assetAndHostInfo, currentAsset); err != nil {
			failedAssets = append(failedAssets, assetAndHostInfo.GetAssetName())
			fmt.Printf("Failed to dump asset %s from Nlyte: %s\n", assetAndHostInfo.GetAssetName(), err)
			continue
		}

		res, err := ic.UpdateAsset(ctx, &ufsAPI.UpdateAssetRequest{
			Asset: currentAsset,
		})
		if err != nil {
			failedAssets = append(failedAssets, currentAsset.GetName())
			fmt.Printf("Failed to dump asset %s from Nlyte: %s\n", currentAsset.String(), err)
			continue
		}
		res.Name = ufsUtil.RemovePrefix(res.GetName())
		fmt.Println("The asset after update:")
		utils.PrintProtoJSON(res, !utils.NoEmitMode(false))
		fmt.Printf("Successfully updated the asset %s\n", res.GetName())
	}

	if len(failedAssets) == 0 {
		return nil
	}
	return fmt.Errorf("Failed to dump the following assets from Nlyte: %v", failedAssets)
}

// parseRecord parses the input record and update the input Asset with parsed info.
func (c *dumpNlyte) parseRecord(ctx context.Context, record *ufspb.AssetAndHostInfo, asset *ufspb.Asset) error {
	if record.GetAssetName() == "" {
		return fmt.Errorf("Failed to parse record: Empty AssetName\n")
	}

	// TODO: (b/260134576) Use field masks to specify the fields to update as that is a better way to update.
	// Add more field masks as we increase the amount of tracked field in Nlyte.
	asset.Location.Position = strconv.Itoa(int(record.GetCabinetUNumber()))
	// TODO: (b/260134131) Rack implementation right now is just a place holder because we don't have row and col info in Nlyte yet.
	asset.Location.Rack = fmt.Sprintf("rack-%s", strconv.Itoa(int(record.GetCabinetAssetId())))
	asset.Location.RackId = record.GetCabinetAssetId()
	asset.Location.LabId = record.GetLocationGroup().GetLocationGroupId()
	asset.Location.FullLocationName = record.GetLocationGroup().GetFullLocationName()

	for _, customField := range record.GetAssetInfo().GetCustomFields() {
		if customField.GetFieldName() == "Asset Type" {
			if val, ok := ufspb.AssetType_value[customField.GetFieldStringValue()]; ok {
				asset.Type = ufspb.AssetType(val)
				continue
			}
			return fmt.Errorf("Failed to parse record: Invalid asset type: %s\n", customField.GetFieldStringValue())
		} else if customField.GetFieldName() == "Zone" {
			if val, ok := ufspb.Zone_value[customField.GetFieldStringValue()]; ok {
				asset.Location.Zone = ufspb.Zone(val)
				asset.Realm = ufsUtil.ToUFSRealm(ufspb.Zone(val).String())
				continue
			}
			return fmt.Errorf("Failed to parse record: Invalid zone type: %s\n", customField.GetFieldStringValue())
		}
	}

	return nil
}

// addAssetToUFS attempts to add given asset to UFS. Returns updated asset and error if any
func (c *dumpNlyte) addAssetToUFS(ctx context.Context, ic ufsAPI.FleetClient, req *ufsAPI.CreateAssetRequest) (*ufspb.Asset, error) {
	if req.Asset == nil {
		return nil, cmdlib.NewQuietUsageError(c.Flags, "Failed to add asset: Invalid input, Missing asset to add")
	}
	if req.Asset.Location == nil {
		return nil, cmdlib.NewQuietUsageError(c.Flags, "Failed to add asset %s: Invalid input, Missing any location information", req.Asset.GetName())
	}
	if req.Asset.Location.Rack == "" {
		return nil, cmdlib.NewQuietUsageError(c.Flags, "Failed to add asset %s: Invalid input, Missing rack", req.Asset.GetName())
	}
	if req.Asset.Location.Zone == ufspb.Zone_ZONE_UNSPECIFIED {
		return nil, cmdlib.NewQuietUsageError(c.Flags, "Failed to add asset %s: Invalid zone", req.Asset.GetName())
	}
	ufsAsset, err := ic.CreateAsset(ctx, req)
	if ufsAsset != nil {
		// Remove the prefix from the asset returned by UFS
		ufsAsset.Name = ufsUtil.RemovePrefix(ufsAsset.Name)
	}
	return ufsAsset, err
}
