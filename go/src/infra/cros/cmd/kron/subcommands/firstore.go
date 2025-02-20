// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subcommands

import (
	"context"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/encoding/protojson"

	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"

	infra_common "go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/kron/common"
	"go.chromium.org/infra/cros/cmd/kron/configparser"
)

// firestoreCommand is the interface for the firestore-sync subcommand
type firestoreCommand struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	authOpts  *auth.Options
	isProd    bool
}

// setFlags creates the flags that the user can set.
func (c *firestoreCommand) setFlags() {
	c.Flags.BoolVar(&c.isProd, "prod", false, "If provided the production project environment will be used.")
}

// GetFirestoreCommand creates the subcommand with CLI flags.
func GetFirestoreCommand(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "firestore-sync <options>",
		LongDesc:  "Sync the firestore DB with the current ToT SuiteScheduler.cfg.",
		ShortDesc: "Sync the firestore DB with the current ToT SuiteScheduler.cfg.",
		CommandRun: func() subcommands.CommandRun {
			cmd := &firestoreCommand{}
			cmd.authFlags = authcli.Flags{}
			cmd.authFlags.Register(cmd.GetFlags(), authOpts)
			cmd.setFlags()
			cmd.authOpts = &authOpts
			return cmd
		},
	}
}

// fetchToTConfigs fetches the ToT configs and returns the config list.
func fetchToTConfigs(authOpts *auth.Options) ([]*suschpb.SchedulerConfig, error) {
	// Ingest the ToT Configs
	configBytes, err := common.FetchFileFromInternalURL(common.SuiteSchedulerCfgURL, authOpts)
	if err != nil {
		return nil, err
	}

	// Convert the bytes into a proto object
	configs, err := configparser.BytesToSchedulerProto(configBytes)
	if err != nil {
		return nil, err
	}
	return configs.Configs, nil
}

// generateFirestoreItemList returns a properly formatted Firestore item for us
// to insert into the database.
func generateFirestoreItemList(configs []*suschpb.SchedulerConfig) ([]*infra_common.FirestoreItem, error) {
	insertItems := []*infra_common.FirestoreItem{}
	for _, config := range configs {
		jsonData, err := protojson.Marshal(config)
		if err != nil {
			return nil, err
		}

		datum := map[string]string{
			"configJSON": string(jsonData),
		}

		firestoreItem := infra_common.FirestoreItem{
			DocName: config.Name,
			Datum:   datum,
		}
		insertItems = append(insertItems, &firestoreItem)
	}
	return insertItems, nil
}

// Run is the "main()" of the firestore sync command.
func (c *firestoreCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	common.Stdout.Println("Starting kron firestore-sync run")
	ctx := context.Background()

	// Get the target ProjectID.
	projectID := common.StagingProjectID
	if c.isProd {
		projectID = common.ProdProjectID
	}
	common.Stdout.Printf("Connecting to project %s\n", projectID)

	// Initialize the client at the target projectID.
	common.Stdout.Printf("Initializing firestore client with db name %s\n", common.FirestoreDatabaseName)
	firestoreClient, err := infra_common.InitClient(ctx, projectID, common.FirestoreDatabaseName)
	if err != nil {
		common.Stderr.Println(err)
		return 1
	}
	common.Stdout.Printf("Received connection to %s\n", common.FirestoreDatabaseName)

	// Form the collectionRef for the configs Collection in Firestore.
	//
	// NOTE: If the name or structure changes this will ned to be updated.
	common.Stdout.Printf("Initializing connection to collection %s\n", common.FirestoreConfigCollectionName)
	configCollection := firestoreClient.Collection(common.FirestoreConfigCollectionName)
	common.Stdout.Printf("Received connection to %s\n", common.FirestoreConfigCollectionName)

	common.Stdout.Println("Fetching ToT suite scheduler configs.")
	configs, err := fetchToTConfigs(c.authOpts)
	if err != nil {
		common.Stderr.Println(err)
		return 1
	}
	common.Stdout.Println("Received configs.")

	// Generate the list of items to send to the firestore client.
	common.Stdout.Println("Converting configs to firestore items.")
	insertItems, err := generateFirestoreItemList(configs)
	if err != nil {
		common.Stderr.Println(err)
		return 1
	}
	common.Stdout.Printf("Generated %d Firestore items for bulk upsert.\n", len(insertItems))

	// Batch write the config items to firestore.
	common.Stdout.Println("Sending batch request to Firestore.")
	writeJobResults, err := infra_common.BatchSet(ctx, configCollection, firestoreClient, insertItems)
	if err != nil {
		common.Stderr.Println(err)
		return 1
	}
	common.Stdout.Println("Received batch results from Firestore.")

	// Handle result errors
	common.Stdout.Println("Checking results for errors.")
	for _, job := range writeJobResults {
		if _, err := job.Results(); err != nil {
			common.Stderr.Println(err)
			return 1

		}
	}
	common.Stdout.Println("Run completed successfully.")

	return 0
}
