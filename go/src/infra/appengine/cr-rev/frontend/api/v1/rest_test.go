// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
//
//go:generate mockgen -source=service.pb.go -package api -destination service.mock.go CrrevServer
package api

import (
	"net/http"
	"net/url"
	"testing"

	gomock "github.com/golang/mock/gomock"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/server/router"
)

func TestRest(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mock := NewMockCrrevServer(mockCtrl)
	s := &restAPIServer{
		grpcServer: mock,
	}

	ftt.Run("Numbering request", t, func(t *ftt.Test) {
		t.Run("non-chromim/src", func(t *ftt.Test) {
			expectedReq := &NumberingRequest{
				Host:           "chromium",
				Repository:     "foo",
				PositionRef:    "refs/heads/main",
				PositionNumber: int64(1),
			}
			mock.EXPECT().Numbering(gomock.Any(), gomock.Eq(expectedReq)).Times(1)

			url, _ := url.Parse("/?project=chromium&repo=foo&numbering_identifier=refs/heads/main&number=1")
			c := &router.Context{
				Request: &http.Request{
					URL: url,
				},
			}
			s.handleNumbering(c)
		})

		t.Run("chromim/src", func(t *ftt.Test) {
			t.Run("before migration", func(t *ftt.Test) {
				t.Run("using old ref", func(t *ftt.Test) {
					expectedReq := &NumberingRequest{
						Host:           "chromium",
						Repository:     "chromium/src",
						PositionRef:    "refs/heads/master",
						PositionNumber: int64(1),
					}
					mock.EXPECT().Numbering(gomock.Any(), gomock.Eq(expectedReq)).Times(1)

					url, _ := url.Parse("/?project=chromium&repo=chromium/src&numbering_identifier=refs/heads/master&number=1")
					c := &router.Context{
						Request: &http.Request{
							URL: url,
						},
					}
					s.handleNumbering(c)
				})
				t.Run("using new ref", func(t *ftt.Test) {
					expectedReq := &NumberingRequest{
						Host:           "chromium",
						Repository:     "chromium/src",
						PositionRef:    "refs/heads/master",
						PositionNumber: int64(1),
					}
					mock.EXPECT().Numbering(gomock.Any(), gomock.Eq(expectedReq)).Times(1)

					url, _ := url.Parse("/?project=chromium&repo=chromium/src&numbering_identifier=refs/heads/main&number=1")
					c := &router.Context{
						Request: &http.Request{
							URL: url,
						},
					}
					s.handleNumbering(c)
				})
			})
			t.Run("chromim/src after migration", func(t *ftt.Test) {
				t.Run("using old ref", func(t *ftt.Test) {
					expectedReq := &NumberingRequest{
						Host:           "chromium",
						Repository:     "chromium/src",
						PositionRef:    "refs/heads/main",
						PositionNumber: int64(1000000),
					}
					mock.EXPECT().Numbering(gomock.Any(), gomock.Eq(expectedReq)).Times(1)

					url, _ := url.Parse("/?project=chromium&repo=chromium/src&numbering_identifier=refs/heads/master&number=1000000")
					c := &router.Context{
						Request: &http.Request{
							URL: url,
						},
					}
					s.handleNumbering(c)
				})
				t.Run("using new ref", func(t *ftt.Test) {
					expectedReq := &NumberingRequest{
						Host:           "chromium",
						Repository:     "chromium/src",
						PositionRef:    "refs/heads/main",
						PositionNumber: int64(1000000),
					}
					mock.EXPECT().Numbering(gomock.Any(), gomock.Eq(expectedReq)).Times(1)

					url, _ := url.Parse("/?project=chromium&repo=chromium/src&numbering_identifier=refs/heads/main&number=1000000")
					c := &router.Context{
						Request: &http.Request{
							URL: url,
						},
					}
					s.handleNumbering(c)
				})
			})
		})
	})
}
