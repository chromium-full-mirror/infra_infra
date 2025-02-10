// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:generate-b395127903-disabled go install infra/cmd/bqexport
//go:generate-b395127903-disabled bqexport -path ../../../tools/bqschemaupdater/raw_events/buildevent_completed_build_legacy.pb.txt
//go:generate-b395127903-disabled bqexport -path ../../../tools/bqschemaupdater/raw_events/buildevent_completed_step_legacy.pb.txt
//go:generate-b395127903-disabled bqexport -path ../../../tools/bqschemaupdater/builds/builds.pb.txt
//go:generate-b395127903-disabled bqexport -path ../../../tools/bqschemaupdater/builds/steps.pb.txt

package buildevent
