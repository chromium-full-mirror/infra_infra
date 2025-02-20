// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package defaultwifi

import (
	"fmt"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/cmd/shivas/cmdhelp"
	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	"go.chromium.org/infra/libs/skylab/common/heuristics"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// AddDefaultWifiCmd add DefaultWifi to the system.
var AddDefaultWifiCmd = &subcommands.Command{
	UsageLine: "defaultwifi",
	ShortDesc: "Add wifi credential for a UFS zone or DUT pool",
	LongDesc:  cmdhelp.AddDefaultWifiLongDesc,
	CommandRun: func() subcommands.CommandRun {
		c := &addDefaultWifi{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.commonFlags.Register(&c.Flags)

		c.Flags.StringVar(&c.newSpecsFile, "f", "", cmdhelp.DefaultWifiFileText)

		c.Flags.StringVar(&c.name, "name", "", "name of UFS zone or DUT pool with the wifi (all in lower case, and zone name must prefixed with 'zone_')")
		c.Flags.StringVar(&c.projectID, "project-id ", "unified-fleet-system", "project ID of the GCP Secret Manager hosting the wifi secret")
		c.Flags.StringVar(&c.secretName, "secret-name", "", "the secret name in the GCP Secret Manager")
		return c
	},
}

type addDefaultWifi struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	envFlags    site.EnvFlags
	commonFlags site.CommonFlags

	newSpecsFile string

	name       string
	projectID  string
	secretName string
}

var mcsvFields = []string{
	"name",
	"project_id",
	"secret_name",
}

func (c *addDefaultWifi) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

func (c *addDefaultWifi) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if err := c.validateArgs(); err != nil {
		return err
	}
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
	var wifi ufspb.DefaultWifi
	var defaultwifis []*ufspb.DefaultWifi
	if c.newSpecsFile != "" {
		if utils.IsCSVFile(c.newSpecsFile) {
			defaultwifis, err = c.parseMCSV()
			if err != nil {
				return err
			}
		} else {
			if err = utils.ParseJSONFile(c.newSpecsFile, &wifi); err != nil {
				return err
			}
		}
	} else {
		c.parseArgs(&wifi)
	}
	if len(defaultwifis) == 0 {
		defaultwifis = append(defaultwifis, &wifi)
	}
	for _, r := range defaultwifis {
		res, err := ic.CreateDefaultWifi(ctx, &ufsAPI.CreateDefaultWifiRequest{
			DefaultWifi:   r,
			DefaultWifiId: r.GetName(),
		})
		if err != nil {
			fmt.Printf("Failed to add DefaultWifi %s. %s\n", r.GetName(), err)
			continue
		}
		res.Name = ufsUtil.RemovePrefix(res.Name)
		utils.PrintProtoJSON(res, !utils.NoEmitMode(false))
		fmt.Printf("Successfully added the DefaultWifi %s\n", res.Name)
	}
	return nil
}

func (c *addDefaultWifi) parseArgs(wifi *ufspb.DefaultWifi) {
	wifi.Name = c.name
	wifi.WifiSecret = &ufspb.Secret{
		ProjectId:  c.projectID,
		SecretName: c.secretName,
	}
}

func (c *addDefaultWifi) validateArgs() error {
	if c.newSpecsFile != "" {
		if c.name != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-name' cannot be specified at the same time.")
		}
		if c.projectID != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-project-id' cannot be specified at the same time.")
		}
		if c.secretName != "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\nThe file mode is specified. '-secret-name' cannot be specified at the same time.")
		}
	}
	if c.newSpecsFile == "" {
		if c.name == "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-name' is required, no mode ('-f') is specified.")
		}
		if err := validateName(c.name); err != nil {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-name' is invalid: %s", err)
		}
		if c.secretName == "" {
			return cmdlib.NewQuietUsageError(c.Flags, "Wrong usage!!\n'-secret-name' is required, no mode ('-f') is specified.")
		}
	}
	return nil
}

// parseMCSV parses the MCSV file and returns DefaultWifi requests.
func (c *addDefaultWifi) parseMCSV() ([]*ufspb.DefaultWifi, error) {
	records, err := utils.ParseMCSVFile(c.newSpecsFile)
	if err != nil {
		return nil, err
	}
	var defaultwifis []*ufspb.DefaultWifi
	for i, rec := range records {
		// if i is 0, determine whether this is a header.
		if i == 0 && heuristics.LooksLikeHeader(rec) {
			if err := utils.ValidateSameStringArray(mcsvFields, rec); err != nil {
				return nil, err
			}
			continue
		}
		wifi := &ufspb.DefaultWifi{WifiSecret: &ufspb.Secret{}}
		for i := range mcsvFields {
			name := mcsvFields[i]
			value := rec[i]
			switch name {
			case "name":
				if err := validateName(name); err != nil {
					return nil, err
				}
				wifi.Name = value
			case "project_id":
				wifi.WifiSecret.ProjectId = value
			case "secret_name":
				wifi.WifiSecret.SecretName = value
			default:
				return nil, fmt.Errorf("Error in line %d.\nUnknown field: %s", i, name)
			}
		}
		defaultwifis = append(defaultwifis, wifi)
	}
	return defaultwifis, nil
}

func validateName(name string) error {
	if strings.ToLower(name) != name {
		return fmt.Errorf("name %q not in lower case", name)
	}
	if !strings.HasPrefix(name, "zone_") {
		return nil
	}
	if ufsUtil.IsUFSZone(ufsUtil.RemoveZonePrefix(name)) {
		return nil
	}
	return fmt.Errorf("name %q starts with 'zone_' but isn't a real zone name", name)
}
