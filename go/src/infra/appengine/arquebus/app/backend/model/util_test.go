// Copyright 2019 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	"infra/appengine/arquebus/app/config"
	"infra/appengine/arquebus/app/util"
)

// updateAndGetAllAssigners stores Assigners entities based on given configs,
// and returns all the Assigner entities stored in datastore.
func updateAndGetAllAssigners(c context.Context, t *ftt.Test, rev string, cfgs ...*config.Assigner) []*Assigner {
	err := UpdateAssigners(c, cfgs, rev)
	assert.Loosely(t, err, should.BeNil)
	assigners, err := GetAllAssigners(c)
	assert.Loosely(t, err, should.BeNil)

	return assigners
}

// createConfig creates a sample, valid Assigner config to be used in tests.
func createConfig(t *ftt.Test, id string) *config.Assigner {
	var cfg config.Assigner
	assert.Loosely(t, proto.UnmarshalText(util.SampleValidAssignerCfg, &cfg), should.BeNil)
	cfg.Id = id
	return &cfg
}

func createTasks(c context.Context, t *ftt.Test, assigner *Assigner, status TaskStatus, startTimes ...time.Time) []*Task {
	var tasks []*Task
	for _, s := range startTimes {
		tasks = append(tasks, &Task{
			AssignerKey:   GenAssignerKey(c, assigner),
			Status:        status,
			ExpectedStart: s,
		})
	}
	assert.Loosely(t, datastore.Put(c, tasks), should.BeNil)
	return tasks
}
