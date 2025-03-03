// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bootstrap

import (
	"time"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/errors/errtag"
)

type sleepBeforeExitingTag struct {
	Key errors.TagKey
}

func (s sleepBeforeExitingTag) With(sleepDuration time.Duration) errors.TagValue {
	return errors.TagValue{
		Key:   s.Key,
		Value: sleepDuration,
	}
}

func (s sleepBeforeExitingTag) In(err error) (sleepDuration time.Duration, ok bool) {
	v, ok := errors.TagValueIn(s.Key, err)
	if ok {
		sleepDuration = v.(time.Duration)
	}
	return
}

var (
	// PatchRejected indicates that some portion of a patch was rejected.
	PatchRejected = errtag.Make("the patch could not be applied", true)
	// SleepBeforeExiting indicates that the top-level code should sleep before returning
	// control to the calling process, with the duration of the sleep being the tag's value.
	SleepBeforeExiting = sleepBeforeExitingTag{Key: errors.NewTagKey("the properties file does not exist in the dependency project")}
)
