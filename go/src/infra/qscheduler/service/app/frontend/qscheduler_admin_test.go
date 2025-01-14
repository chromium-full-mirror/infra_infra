// Copyright 2018 The LUCI Authors.
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

package frontend_test

import (
	"testing"

	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/appengine/gaetesting"
	. "go.chromium.org/luci/common/testing/truth/convey/facade"

	"infra/qscheduler/qslib/protos"
	qscheduler "infra/qscheduler/service/api/qscheduler/v1"
	"infra/qscheduler/service/app/frontend"
)

func TestCreateDeleteScheduler(t *testing.T) {
	Convey("Given an admin server running in a test context", t, func(t *T) {
		ctx := gaetesting.TestingContext()
		admin := &frontend.QSchedulerAdminServerImpl{}
		view := &frontend.QSchedulerViewServerImpl{}
		poolID := "Pool 1"
		req := qscheduler.CreateSchedulerPoolRequest{
			PoolId: poolID,
		}

		Convey("when CreateSchedulerPool is called with a config", t, func(t *T) {
			config := &protos.SchedulerConfig{}
			req.Config = config
			resp, err := admin.CreateSchedulerPool(ctx, &req)
			Convey("then an error is returned.", t, func(t *T) {
				So(t, resp, ShouldBeNil)
				So(t, err, ShouldNotBeNil)
			})
		})

		Convey("when CreateSchedulerPool is called", t, func(t *T) {
			resp, err := admin.CreateSchedulerPool(ctx, &req)
			Convey("then it returns without errors.", t, func(t *T) {
				So(t, resp, ShouldNotBeNil)
				So(t, err, ShouldBeNil)
			})

			Convey("when InspectPool is called, it succeeds.", t, func(t *T) {
				req := &qscheduler.InspectPoolRequest{PoolId: poolID}
				resp, err := view.InspectPool(ctx, req)
				So(t, err, ShouldBeNil)
				So(t, resp, ShouldNotBeNil)
			})

			Convey("when DeleteSchedulerPool is called to delete the scheduler", t, func(t *T) {
				req := &qscheduler.DeleteSchedulerPoolRequest{
					PoolId: poolID,
				}
				resp, err := admin.DeleteSchedulerPool(ctx, req)
				So(t, err, ShouldBeNil)
				So(t, resp, ShouldNotBeNil)
				Convey("when inspect is called, it fails to find scheduler.", t, func(t *T) {
					req := &qscheduler.InspectPoolRequest{PoolId: poolID}
					resp, err := view.InspectPool(ctx, req)
					So(t, resp, ShouldBeNil)
					So(t, err, ShouldNotBeNil)
				})
			})
		})
	})
}

func TestCreateListDeleteAccount(t *testing.T) {
	t.Skip("test is not shuffle safe, see b/389742273 for details")
	poolID := "Pool1"
	Convey("Given an admin server running in a test context", t, func(t *T) {
		ctx := gaetesting.TestingContext()
		admin := &frontend.QSchedulerAdminServerImpl{}
		view := &frontend.QSchedulerViewServerImpl{}
		Convey("when CreateAccount is called with a nonexistent pool", t, func(t *T) {
			req := qscheduler.CreateAccountRequest{
				PoolId: poolID,
			}
			resp, err := admin.CreateAccount(ctx, &req)
			// TODO(crbug.com/1027755): this should return NotFound instead of Unknown.
			Convey("then an error with code Unknown is returned.", t, func(t *T) {
				So(t, resp, ShouldBeNil)
				So(t, err, ShouldNotBeNil)
				s, ok := status.FromError(err)
				So(t, ok, ShouldBeTrue)
				So(t, s.Code(), ShouldEqual(codes.Unknown))
			})
		})

		Convey("when ListAccounts is called for nonexistent pool", t, func(t *T) {
			req := qscheduler.ListAccountsRequest{
				PoolId: poolID,
			}
			resp, err := view.ListAccounts(ctx, &req)
			// TODO(crbug.com/1027755): this should return NotFound instead of Unknown.
			Convey("then an error with code Unknown is returned.", t, func(t *T) {
				So(t, resp, ShouldBeNil)
				So(t, err, ShouldNotBeNil)
				s, ok := status.FromError(err)
				So(t, ok, ShouldBeTrue)
				So(t, s.Code(), ShouldEqual(codes.Unknown))
			})
		})

		Convey("with a scheduler pool", t, func(t *T) {
			req := qscheduler.CreateSchedulerPoolRequest{
				PoolId: poolID,
			}
			_, err := admin.CreateSchedulerPool(ctx, &req)
			So(t, err, ShouldBeNil)

			Convey("when ListAccounts is called for that pool", t, func(t *T) {
				req := qscheduler.ListAccountsRequest{
					PoolId: poolID,
				}
				resp, err := view.ListAccounts(ctx, &req)
				Convey("then it returns no results.", t, func(t *T) {
					So(t, resp.Accounts, ShouldBeEmpty)
					So(t, err, ShouldBeNil)
				})
			})

			Convey("when CreateAccount is called for that pool", t, func(t *T) {
				accountID := "Account1"
				req := qscheduler.CreateAccountRequest{
					AccountId: accountID,
					PoolId:    poolID,
					Config: &protos.AccountConfig{
						PerLabelTaskLimits: map[string]int32{"label-model": 4},
					},
				}
				resp, err := admin.CreateAccount(ctx, &req)
				Convey("then it succeeds.", t, func(t *T) {
					So(t, resp, ShouldResemble(&qscheduler.CreateAccountResponse{}))
					So(t, err, ShouldBeNil)
				})
				Convey("when ListAccounts is called for that pool", t, func(t *T) {
					req := qscheduler.ListAccountsRequest{
						PoolId: poolID,
					}
					resp, err := view.ListAccounts(ctx, &req)
					Convey("then it returns a list with that account.", t, func(t *T) {
						So(t, err, ShouldBeNil)
						So(t, resp.Accounts, ShouldContainKey(accountID))
						So(t, resp.Accounts, ShouldHaveLength(1))
						newLabelLimits := resp.Accounts[accountID].PerLabelTaskLimits
						So(t, newLabelLimits, ShouldResemble(map[string]int32{"label-model": 4}))
					})
				})
				Convey("when ModAccount is called to update the account", t, func(t *T) {
					expect := "foo"
					reqMod := qscheduler.ModAccountRequest{
						AccountId:          accountID,
						PoolId:             poolID,
						Description:        &wrappers.StringValue{Value: expect},
						PerLabelTaskLimits: map[string]int32{"label-model": 2},
					}
					respMod, err := admin.ModAccount(ctx, &reqMod)
					So(t, respMod, ShouldResemble(&qscheduler.ModAccountResponse{}))
					So(t, err, ShouldBeNil)

					Convey("then when account is listed, it reflects the modifications.", t, func(t *T) {
						reqList := qscheduler.ListAccountsRequest{
							PoolId: poolID,
						}
						respList, err := view.ListAccounts(ctx, &reqList)
						So(t, err, ShouldBeNil)
						So(t, respList.Accounts, ShouldContainKey(accountID))
						So(t, respList.Accounts, ShouldHaveLength(1))
						actual := respList.Accounts[accountID]
						So(t, actual.Description, ShouldEqual(expect))
						So(t, actual.PerLabelTaskLimits, ShouldResemble(map[string]int32{"label-model": 2}))
					})
				})
				Convey("when ModAccount is called to delete the account", t, func(t *T) {
					req := &qscheduler.DeleteAccountRequest{
						PoolId:    poolID,
						AccountId: accountID,
					}
					resp, err := admin.DeleteAccount(ctx, req)
					So(t, resp, ShouldNotBeNil)
					So(t, err, ShouldBeNil)
					Convey("when ListAccounts is called for that pool", t, func(t *T) {
						req := qscheduler.ListAccountsRequest{
							PoolId: poolID,
						}
						resp, err := view.ListAccounts(ctx, &req)
						Convey("then it returns no results.", t, func(t *T) {
							So(t, resp.Accounts, ShouldBeEmpty)
							So(t, err, ShouldBeNil)
						})
					})
				})
			})
		})
	})
}
