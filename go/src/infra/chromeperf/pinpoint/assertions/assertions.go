// Copyright 2021 The Chromium Authors.
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

// Package assertions contains GoConvey assertions used by pinpoint.
// Like the GoConvey package, it should be dot-imported into tests.
package assertions

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/testing/truth/comparison"
	"go.chromium.org/luci/common/testing/truth/failure"
	"go.chromium.org/luci/common/testing/truth/should"
)

// ShouldbeStatusError checks that an error is a status error with the given status.
func ShouldBeStatusError(want codes.Code) comparison.Func[error] {
	return func(got error) *failure.Summary {
		s, ok := status.FromError(got)
		if !ok {
			sb := comparison.NewSummaryBuilder("should.BeStatusError")
			sb = sb.AddFindingf("error type", "error was not a Status error, found %T", got)
			return sb.Summary
		}
		return should.Equal(want)(s.Code())
	}
}
