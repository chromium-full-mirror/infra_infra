// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import "go.chromium.org/chromiumos/config/go/test/api"

// FindFirst builds out a `First` focal task finder for a given task type.
func FindFirst(taskType api.FocalTaskFinder_TaskType) *api.FocalTaskFinder {
	return &api.FocalTaskFinder{
		Finder: &api.FocalTaskFinder_First_{
			First: &api.FocalTaskFinder_First{
				TaskType: taskType,
			},
		},
	}
}

// FindLast builds out a `Last` focal task finder for a given task type.
func FindLast(taskType api.FocalTaskFinder_TaskType) *api.FocalTaskFinder {
	return &api.FocalTaskFinder{
		Finder: &api.FocalTaskFinder_Last_{
			Last: &api.FocalTaskFinder_Last{
				TaskType: taskType,
			},
		},
	}
}

// FindBeginning builds out a `Beginning` focal task finder.
// Index = 0.
func FindBeginning() *api.FocalTaskFinder {
	return &api.FocalTaskFinder{
		Finder: &api.FocalTaskFinder_Beginning_{
			Beginning: &api.FocalTaskFinder_Beginning{},
		},
	}
}

// FindEnd builds out an `End` focal task finder.
// Index = len(taskList)-1.
func FindEnd() *api.FocalTaskFinder {
	return &api.FocalTaskFinder{
		Finder: &api.FocalTaskFinder_End_{
			End: &api.FocalTaskFinder_End{},
		},
	}
}

// FindByDynamicIdentifier builds out a focal task finder that
// searches for the task that has the dynamic id provided.
func FindByDynamicIdentifier(dynamicId string) *api.FocalTaskFinder {
	return &api.FocalTaskFinder{
		Finder: &api.FocalTaskFinder_ByDynamicIdentifier_{
			ByDynamicIdentifier: &api.FocalTaskFinder_ByDynamicIdentifier{
				DynamicIdentifier: dynamicId,
			},
		},
	}
}
