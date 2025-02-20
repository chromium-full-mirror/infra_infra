// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package urlpath

import (
	"context"
	"fmt"
	"net/url"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/scopes"
)

// EnrichWithTrackingIds enrich URL with Swarming and Buildbucket task ID.
//
// Expected result:
// http://Addr:8082/swarming/<swarming_task_id>/bbid/<bbid>/<original URL>
// Order is critical as server has hardcoded regexes to extra values.
func EnrichWithTrackingIds(ctx context.Context, urlpath string) (string, error) {
	if urlpath == "" {
		return urlpath, nil
	}
	parsedURL, err := url.Parse(urlpath)
	if err != nil {
		return "", errors.Annotate(err, "enrich with tracking ids").Err()
	}
	var swarmingTaskId, bbId string
	if id, ok := scopes.GetParam(ctx, scopes.ParamKeySwarmingTaskID); ok && id != "" {
		swarmingTaskId = fmt.Sprintf("%s", id)
	} else {
		swarmingTaskId = "none"
	}
	if id, ok := scopes.GetParam(ctx, scopes.ParamKeyBuildbucketID); ok && id != "" {
		bbId = fmt.Sprintf("%s", id)
	} else {
		bbId = "none"
	}
	parsedURL.Path, err = url.JoinPath("swarming", swarmingTaskId, "bbid", bbId, parsedURL.Path)
	if err != nil {
		return "", errors.Annotate(err, "enrich with tracking ids").Err()
	}
	return parsedURL.String(), nil
}
