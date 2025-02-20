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

package nodestore_test

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"go.chromium.org/luci/appengine/gaetesting"
	. "go.chromium.org/luci/common/testing/truth/convey/facade"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/qscheduler/qslib/scheduler"
	"go.chromium.org/infra/qscheduler/service/app/state/nodestore"
	"go.chromium.org/infra/qscheduler/service/app/state/types"
)

type createUniqueAccounts struct {
	nAccounts int

	created int32
}

// createUniqueAccount implements nodestore.Operator
var _ nodestore.Operator = &createUniqueAccounts{}

func (n *createUniqueAccounts) Modify(ctx context.Context, s *types.QScheduler) error {
	for i := 0; i < n.nAccounts; i++ {
		s.Scheduler.AddAccount(ctx, scheduler.AccountID(uuid.New().String()), scheduler.NewAccountConfig(0, map[string]int32{"label-model": 3}, 0, nil, false, ""), nil)
	}
	return nil
}

func (n *createUniqueAccounts) Commit(_ context.Context) error {
	return nil
}

func (n *createUniqueAccounts) Finish(_ context.Context) {
	atomic.AddInt32(&n.created, 1)
}

func (n *createUniqueAccounts) Created() int {
	return int(n.created)
}

func addDatastoreIndexes(ctx context.Context) error {
	defs, err := datastore.FindAndParseIndexYAML(".")
	if err != nil {
		return err
	}
	datastore.GetTestable(ctx).AddIndexes(defs...)
	return nil
}

func TestBasicRun(t *testing.T) {
	t.Parallel()
	Convey("Given a testing context with a created entity", t, func(t *T) {
		ctx := gaetesting.TestingContext()

		store := nodestore.New("foo-pool")
		err := store.Create(ctx, time.Now())
		So(t, err, ShouldBeNil)

		Convey("duplicate creation attempts fail.", t, func(t *T) {
			err := store.Create(ctx, time.Now())
			So(t, err, ShouldNotBeNil)
		})

		Convey("operations run without error.", t, func(t *T) {
			err = store.Run(ctx, &createUniqueAccounts{nAccounts: 1})
			So(t, err, ShouldBeNil)

			err = store.Run(ctx, &createUniqueAccounts{nAccounts: 1})
			So(t, err, ShouldBeNil)

			err = store.Run(ctx, &createUniqueAccounts{nAccounts: 1})
			So(t, err, ShouldBeNil)

			s, err := store.Get(ctx)
			So(t, err, ShouldBeNil)
			So(t, len(s.Scheduler.Config().AccountConfigs), ShouldEqual(3))
		})
	})
}

// TestConflictingRun tests that two stores being used concurrently for the same
// qscheduler do not obliterate eachothers' writes.
func TestConflictingRun(t *testing.T) {
	t.Parallel()
	Convey("Given a testing context with a created entity and two stores using it", t, func(t *T) {
		ctx := gaetesting.TestingContext()

		storeA := nodestore.New("foo-pool")
		err := storeA.Create(ctx, time.Now())
		So(t, err, ShouldBeNil)

		storeB := nodestore.New("foo-pool")

		Convey("alternating null operations between both stores run without error.", t, func(t *T) {
			err = storeA.Run(ctx, &createUniqueAccounts{nAccounts: 1})
			So(t, err, ShouldBeNil)

			err = storeB.Run(ctx, &createUniqueAccounts{nAccounts: 1})
			So(t, err, ShouldBeNil)

			err = storeA.Run(ctx, &createUniqueAccounts{nAccounts: 1})
			So(t, err, ShouldBeNil)

			err = storeB.Run(ctx, &createUniqueAccounts{nAccounts: 1})
			So(t, err, ShouldBeNil)

			s, err := storeA.Get(ctx)
			So(t, err, ShouldBeNil)
			So(t, len(s.Scheduler.Config().AccountConfigs), ShouldEqual(4))
		})
	})
}

func TestClean(t *testing.T) {
	t.Parallel()
	Convey("Given a testing context with a created entity", t, func(t *T) {
		ctx := gaetesting.TestingContext()
		datastore.GetTestable(ctx).Consistent(true)
		err := addDatastoreIndexes(ctx)
		So(t, err, ShouldBeNil)

		store := nodestore.New("foo-pool")
		err = store.Create(ctx, time.Now())
		So(t, err, ShouldBeNil)

		Convey("an immediate clean should run without error.", t, func(t *T) {
			count, err := store.Clean(ctx)
			So(t, count, ShouldEqual(0))
			So(t, err, ShouldBeNil)
		})

		Convey("a clean after some operations runs without error, removes stale entities, and does not affect state.", t, func(t *T) {
			for i := 0; i < 200; i++ {
				store.Run(ctx, &createUniqueAccounts{nAccounts: 1})
				if err != nil {
					// This assert is guarded because we don't want to goconvey
					// to think we did 200 real asserts; that would pollute the UI.
					So(t, err, ShouldBeNil)
				}
			}

			beforeClean, err := store.Get(ctx)
			So(t, err, ShouldBeNil)

			count, err := datastore.Count(ctx, datastore.NewQuery("stateNode"))
			So(t, count, ShouldEqualInt64(201))

			delCount, err := store.Clean(ctx)
			So(t, err, ShouldBeNil)
			So(t, delCount, ShouldEqualInt64(100))
			afterClean, err := store.Get(ctx)
			So(t, err, ShouldBeNil)
			So(t, beforeClean, DangerousShouldResemble(afterClean))
			count, err = datastore.Count(ctx, datastore.NewQuery("stateNode"))
			So(t, count, ShouldEqualInt64(101))
		})
	})
}

func TestLargeState(t *testing.T) {
	t.Parallel()
	Convey("Given a testing context with a created entity", t, func(t *T) {
		ctx := gaetesting.TestingContext()
		datastore.GetTestable(ctx).Consistent(true)

		store := nodestore.New("foo-pool")
		err := store.Create(ctx, time.Now())
		So(t, err, ShouldBeNil)

		Convey("a very large state spanning 10 child nodes can be stored.", t, func(t *T) {
			// Given uuid size, 140k accounts causes state to be large enough to
			// be spread over 24 nodes.
			nAccounts := 140 * 1000
			err := store.Run(ctx, &createUniqueAccounts{nAccounts: nAccounts})
			So(t, err, ShouldBeNil)

			state, err := store.Get(ctx)
			So(t, err, ShouldBeNil)
			So(t, len(state.Scheduler.Config().AccountConfigs), ShouldEqual(nAccounts))

			count, err := datastore.Count(ctx, datastore.NewQuery("stateNode"))
			// 1 node for generation 0; 20 for generation 1.
			So(t, count, ShouldEqualInt64(24))
		})
	})
}

func TestCreateListDelete(t *testing.T) {
	t.Parallel()
	Convey("Given a testing context with a two created entities, List and Delete should work as expected.", t, func(t *T) {
		ctx := gaetesting.TestingContext()
		datastore.GetTestable(ctx).Consistent(true)

		storeA := nodestore.New("A")
		err := storeA.Create(ctx, time.Now())
		So(t, err, ShouldBeNil)

		storeB := nodestore.New("B")
		err = storeB.Create(ctx, time.Now())
		So(t, err, ShouldBeNil)

		IDs, err := nodestore.List(ctx)
		So(t, err, ShouldBeNil)
		So(t, IDs, ShouldHaveLength(2))
		So(t, IDs, ShouldContainString("A"))
		So(t, IDs, ShouldContainString("B"))

		err = storeB.Delete(ctx)
		So(t, err, ShouldBeNil)

		IDs, err = nodestore.List(ctx)
		So(t, err, ShouldBeNil)
		So(t, IDs, ShouldHaveLength(1))
		So(t, IDs, ShouldContainString("A"))

		err = storeA.Delete(ctx)
		So(t, err, ShouldBeNil)

		IDs, err = nodestore.List(ctx)
		So(t, err, ShouldBeNil)
		So(t, IDs, ShouldBeEmpty)
	})
}

func TestConcurrentRuns(t *testing.T) {
	t.Parallel()
	Convey("Given a testing context with a created entity", t, func(t *T) {
		ctx := gaetesting.TestingContext()

		store := nodestore.New("foo-pool")
		store.Create(ctx, time.Now())

		Convey("and 10 concurrent stores, with 10 concurrent operations each", t, func(t *T) {
			nStores := 10
			opsPerStore := 10

			wg := sync.WaitGroup{}
			wg.Add(nStores * opsPerStore)

			operator := &createUniqueAccounts{nAccounts: 1}

			for i := 0; i < nStores; i++ {
				store := nodestore.New("foo-pool")
				go func(store *nodestore.NodeStore) {
					for j := 0; j < opsPerStore; j++ {
						go func() {
							// Add jitter to run attempts, for an acceptable level
							// of contention.
							time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
							store.Run(ctx, operator)
							wg.Done()
						}()
					}
				}(store)
			}
			wg.Wait()

			Convey("the number of Finish calls matches the number of successful operations", t, func(t *T) {
				state, err := store.Get(ctx)
				So(t, err, ShouldBeNil)
				// The number of created accounts should match the number of Finish() calls.
				// With current parameters, this typically means ~50-75 successful calls,
				// out of 100 attempts, but the precise number is nondeterministic.
				So(t, len(state.Scheduler.Config().AccountConfigs), ShouldEqual(operator.Created()))
			})
		})
	})
}
