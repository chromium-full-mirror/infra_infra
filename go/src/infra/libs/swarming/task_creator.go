// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package swarming

import (
	"context"

	"github.com/google/uuid"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cmdsupport/cmdlib"
	sw "go.chromium.org/infra/libs/skylab/swarming"
)

const (
	// luciProject is used to tag the chromeos tasks.
	luciProject = "chromeos"

	// defaultAdminTaskPriority is the priority used as default for the admin tasks
	// if other was not specified.
	defaultAdminTaskPriority = 25

	// dutIDDimensionKey is the dimension key for dut ID.
	dutIDDimensionKey = "dut_id"

	// dutNameDimensionKey is the dimension key for dut hostname.
	dutNameDimensionKey = "dut_name"

	// idDimensionKey is the dimension key for ID.
	idDimensionKey = "id"

	// poolDimensionKey is the dimension key for pool.
	poolDimensionKey = "pool"
)

// taskCreator creates Swarming tasks
type taskCreator struct {
	// Client is Swarming API Client
	client *sw.Client
	// SwarmingService is a path to Swarming API
	swarmingService string
	// Session is an ID that is used to mark tasks and for tracking all of the tasks created in a logical session.
	session string
	// Authenticator is used to get user info
	authenticator *auth.Authenticator
	// LogdogService is the logdog service for the task logs
	LogdogService string
	// logdogTaskCode keeps unique code for each creating task. Please call GenerateLogdogTaskCode() for each task.
	logdogTaskCode string
	// SwarmingServiceAccount is the service account to be used.
	SwarmingServiceAccount string
	// LUCIProject is the name of the project used to create the task.
	LUCIProject string
}

// taskInfo contains information of the created task.
type taskInfo struct {
	// id of the created task in the Swarming.
	id string
	// taskURL provides the URL to the created task in Swarming.
	taskURL string
}

// getID gets an ID and returns "" by default.
func (t *taskInfo) getID() string {
	if t == nil {
		return ""
	}
	return t.id
}

// NewTaskCreator creates and initialize the TaskCreator.
func NewTaskCreator(ctx context.Context, authFlags *authcli.Flags, swarmingService string) (*taskCreator, error) {
	a, err := cmdlib.NewAuthenticator(ctx, authFlags)
	if err != nil {
		return nil, errors.Annotate(err, "failed to create TaskCreator. Authenticator error").Err()
	}
	h, err := a.Client()
	if err != nil {
		return nil, errors.Annotate(err, "failed to create TaskCreator. Cannot create http client").Err()
	}

	service, err := sw.NewClient(h, swarmingService)
	if err != nil {
		return nil, errors.Annotate(err, "failed to create TaskCreator. Cannot create API client").Err()
	}

	tc := &taskCreator{
		client:          service,
		swarmingService: swarmingService,
		session:         uuid.New().String(),
		LUCIProject:     luciProject,
		authenticator:   a,
	}
	return tc, nil
}
