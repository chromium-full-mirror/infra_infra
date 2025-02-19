// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Code generated file.
// Code is copied from http://gpaste/6461300373979136 and then modified to
// make sure the imports work with bare minimum of structs needed.
package common

import (
	gensupport "infra/cros/cmd/common_lib/ants/gensupport"

	googleapi "google.golang.org/api/googleapi"
)

// BuildMessage: Build
// message. This is a map to the build info comes from the build pubsub. It
// includes a build's information.
type BuildMessage struct {
	AttemptId         string `json:"attemptId,omitempty"`
	Branch            string `json:"branch,omitempty"`
	BuildAlias        string `json:"buildAlias,omitempty"`
	BuildFlavor       string `json:"buildFlavor,omitempty"`
	BuildId           string `json:"buildId,omitempty"`
	BuildOs           string `json:"buildOs,omitempty"`
	BuildPlatform     string `json:"buildPlatform,omitempty"`
	BuildTarget       string `json:"buildTarget,omitempty"`
	BuildType         string `json:"buildType,omitempty"`
	CreationTimestamp string `json:"creationTimestamp,omitempty"`
	// ForceSendFields is a list of field names (e.g. "AttemptId") to
	// unconditionally include in API requests. By default, fields with empty or
	// default values are omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-ForceSendFields for more
	// details.
	ForceSendFields []string `json:"-"`
	// NullFields is a list of field names (e.g. "AttemptId") to include in API
	// requests with the JSON null value. By default, fields with empty values are
	// omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-NullFields for more details.
	NullFields []string `json:"-"`
}

func (s *BuildMessage) MarshalJSON() ([]byte, error) {
	type NoMethod BuildMessage
	return gensupport.MarshalJSON(NoMethod(*s), s.ForceSendFields, s.NullFields)
}

// KeyValuesMessage: Key
// values message.
type KeyValuesMessage struct {
	Key    string   `json:"key,omitempty"`
	Values []string `json:"values,omitempty"`
	// ForceSendFields is a list of field names (e.g. "Key") to unconditionally
	// include in API requests. By default, fields with empty or default values are
	// omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-ForceSendFields for more
	// details.
	ForceSendFields []string `json:"-"`
	// NullFields is a list of field names (e.g. "Key") to include in API requests
	// with the JSON null value. By default, fields with empty values are omitted
	// from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-NullFields for more details.
	NullFields []string `json:"-"`
}

func (s *KeyValuesMessage) MarshalJSON() ([]byte, error) {
	type NoMethod KeyValuesMessage
	return gensupport.MarshalJSON(NoMethod(*s), s.ForceSendFields, s.NullFields)
}

// ResourceUsage:
// ResourceUsage message. It contains information regarding the usage of a
// specific resource.
type ResourceUsage struct {
	// Possible values:
	//   "CPU"
	//   "DEVICE"
	//   "DISK_IO"
	//   "UNKNOWN"
	Measure string `json:"measure,omitempty"`
	Value   int64  `json:"value,omitempty"`
	// ForceSendFields is a list of field names (e.g. "Measure") to unconditionally
	// include in API requests. By default, fields with empty or default values are
	// omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-ForceSendFields for more
	// details.
	ForceSendFields []string `json:"-"`
	// NullFields is a list of field names (e.g. "Measure") to include in API
	// requests with the JSON null value. By default, fields with empty values are
	// omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-NullFields for more details.
	NullFields []string `json:"-"`
}

func (s *ResourceUsage) MarshalJSON() ([]byte, error) {
	type NoMethod ResourceUsage
	return gensupport.MarshalJSON(NoMethod(*s), s.ForceSendFields, s.NullFields)
}

// TestBenchMessage:
// TestBench message. TestBench is where actually the test should be run.
type TestBenchMessage struct {
	Attributes []string `json:"attributes,omitempty"`
	Cluster    string   `json:"cluster,omitempty"`
	// Properties: Key values message.
	Properties []*KeyValuesMessage `json:"properties,omitempty"`
	RunTarget  string              `json:"runTarget,omitempty"`
	// Possible values:
	//   "ANDROIDX_FTL"
	//   "ANDROIDX_PLAYGROUND"
	//   "ANDROID_EMU"
	//   "ANDROID_G3"
	//   "AUTOTEST"
	//   "BATCAVE"
	//   "CTP"
	//   "MOBILE_HARNESS"
	//   "NO_OP"
	//   "PIXEL_CAMERA_FLEXTAPE"
	//   "PIXEL_CAMERA_PRESIL"
	//   "SWARMING"
	//   "TRADEFED"
	//   "UNITY_INTEGRATION"
	//   "UNKNOWN_RUNNER"
	//   "VERSIONED_TF"
	Scheduler string `json:"scheduler,omitempty"`
	// ForceSendFields is a list of field names (e.g. "Attributes") to
	// unconditionally include in API requests. By default, fields with empty or
	// default values are omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-ForceSendFields for more
	// details.
	ForceSendFields []string `json:"-"`
	// NullFields is a list of field names (e.g. "Attributes") to include in API
	// requests with the JSON null value. By default, fields with empty values are
	// omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-NullFields for more details.
	NullFields []string `json:"-"`
}

func (s *TestBenchMessage) MarshalJSON() ([]byte, error) {
	type NoMethod TestBenchMessage
	return gensupport.MarshalJSON(NoMethod(*s), s.ForceSendFields, s.NullFields)
}

// TestConfigMessage:
// TestConfig message. TestConfig is the basic config for a TestJob.
type TestConfigMessage struct {
	// Args: Key values message.
	Args               []*KeyValuesMessage `json:"args,omitempty"`
	Command            string              `json:"command,omitempty"`
	Name               string              `json:"name,omitempty"`
	PrimaryBuildDevice string              `json:"primaryBuildDevice,omitempty"`
	RunCount           int64               `json:"runCount,omitempty"`
	Shards             int64               `json:"shards,omitempty"`
	// ForceSendFields is a list of field names (e.g. "Args") to unconditionally
	// include in API requests. By default, fields with empty or default values are
	// omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-ForceSendFields for more
	// details.
	ForceSendFields []string `json:"-"`
	// NullFields is a list of field names (e.g. "Args") to include in API requests
	// with the JSON null value. By default, fields with empty values are omitted
	// from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-NullFields for more details.
	NullFields []string `json:"-"`
}

func (s *TestConfigMessage) MarshalJSON() ([]byte, error) {
	type NoMethod TestConfigMessage
	return gensupport.MarshalJSON(NoMethod(*s), s.ForceSendFields, s.NullFields)
}

// TestJobEventMessage:
// TestJobEvent message. TestJobEvent is a TestJob event. Every time the status
// of a TestJob changes, the test runner should report a TestJobEvent back.
type TestJobEventMessage struct {
	DeviceLostDetected int64    `json:"deviceLostDetected,omitempty"`
	EventTimestamp     string   `json:"eventTimestamp,omitempty"`
	FailedTestCount    int64    `json:"failedTestCount"`
	FailedTestRunCount int64    `json:"failedTestRunCount"`
	ResultLinks        []string `json:"resultLinks,omitempty"` // Deprecated/Unused
	// Possible values:
	//   "CANCELED"
	//   "COMPLETED"
	//   "ERROR"
	//   "FATAL"
	//   "QUEUED"
	//   "RUNNING"
	//   "UNKNOWN"
	State   string `json:"state,omitempty"`
	Summary string `json:"summary,omitempty"`
	// TestJob: TestJob message. TestJob is a test job running in the test runner.
	// This message has information about test job's config, state and result. A
	// TestJob can be executed by creating one or multiple TestTasks based on the
	// test's configuration. Attributes: id: a test job ID. user: an email of a
	// user who created the test job. runner: a test job runner type. test: a
	// TestConfigMessage object. build: a BuildMessage object. testBench: a
	// TestBenchMessage object. testJobState: a state of a test job.
	// creationTimestamp: time when a test job was created. startTimestamp: time
	// when a test job was started. updateTimestamp: time when a test job was last
	// updated. endTimestamp: time when a test job was ended. tasks: a list of
	// TestTaskMessage objects. jobInfo: extra information about a test job.
	// priority: a priority of a test job. Must be in range [0, 1000].
	// runnerOptions: test runner options. context: context for the test job
	// context has gerrit_cl and test_result_id. stateReason: a state reason of a
	// test job when error/cancel happen. extra_builds: a list of extra builds.
	// plugin_data: a list of KeyValuesMessage for plugin data. utilization: a list
	// of ResourceUsage resultLinks: a list of result links
	TestJob         *TestJobMessage `json:"testJob,omitempty"`
	TestJobId       string          `json:"testJobId,omitempty"`
	TotalRunTimeSec int64           `json:"totalRunTimeSec,omitempty"`
	TotalTestCount  int64           `json:"totalTestCount"`
	// Possible values:
	//   "STATE_CHANGED"
	Type string `json:"type,omitempty"`
	// ForceSendFields is a list of field names (e.g. "DeviceLostDetected") to
	// unconditionally include in API requests. By default, fields with empty or
	// default values are omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-ForceSendFields for more
	// details.
	ForceSendFields []string `json:"-"`
	// NullFields is a list of field names (e.g. "DeviceLostDetected") to include
	// in API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-NullFields for more details.
	NullFields []string `json:"-"`
}

func (s *TestJobEventMessage) MarshalJSON() ([]byte, error) {
	type NoMethod TestJobEventMessage
	return gensupport.MarshalJSON(NoMethod(*s), s.ForceSendFields, s.NullFields)
}

// TestJobMessage: TestJob
// message. TestJob is a test job running in the test runner. This message has
// information about test job's config, state and result. A TestJob can be
// executed by creating one or multiple TestTasks based on the test's
// configuration. Attributes: id: a test job ID. user: an email of a user who
// created the test job. runner: a test job runner type. test: a
// TestConfigMessage object. build: a BuildMessage object. testBench: a
// TestBenchMessage object. testJobState: a state of a test job.
// creationTimestamp: time when a test job was created. startTimestamp: time
// when a test job was started. updateTimestamp: time when a test job was last
// updated. endTimestamp: time when a test job was ended. tasks: a list of
// TestTaskMessage objects. jobInfo: extra information about a test job.
// priority: a priority of a test job. Must be in range [0, 1000].
// runnerOptions: test runner options. context: context for the test job
// context has gerrit_cl and test_result_id. stateReason: a state reason of a
// test job when error/cancel happen. extra_builds: a list of extra builds.
// plugin_data: a list of KeyValuesMessage for plugin data. utilization: a list
// of ResourceUsage
type TestJobMessage struct {
	// Build: Build message. This is a map to the build info comes from the build
	// pubsub. It includes a build's information.
	Build *BuildMessage `json:"build,omitempty"`
	// Context: Key values message.
	Context           []*KeyValuesMessage `json:"context,omitempty"`
	CreationTimestamp string              `json:"creationTimestamp,omitempty"`
	EndTimestamp      string              `json:"endTimestamp,omitempty"`
	// ExtraBuilds: Build message. This is a map to the build info comes from the
	// build pubsub. It includes a build's information.
	ExtraBuilds []*BuildMessage `json:"extraBuilds,omitempty"`
	Id          string          `json:"id,omitempty"`
	// JobInfo: Key values message.
	JobInfo []*KeyValuesMessage `json:"jobInfo,omitempty"`
	// PluginData: Key values message.
	PluginData  []*KeyValuesMessage `json:"plugin_data,omitempty"`
	Priority    int64               `json:"priority,omitempty"`
	ResultLinks []string            `json:"resultLinks,omitempty"`
	// Possible values:
	//   "ANDROIDX_FTL"
	//   "ANDROIDX_PLAYGROUND"
	//   "ANDROID_EMU"
	//   "ANDROID_G3"
	//   "AUTOTEST"
	//   "BATCAVE"
	//   "CTP"
	//   "MOBILE_HARNESS"
	//   "NO_OP"
	//   "PIXEL_CAMERA_FLEXTAPE"
	//   "PIXEL_CAMERA_PRESIL"
	//   "SPELUNKY"
	//   "SWARMING"
	//   "TRADEFED"
	//   "UNITY_INTEGRATION"
	//   "UNKNOWN_RUNNER"
	//   "VERSIONED_TF"
	Runner string `json:"runner,omitempty"`
	// RunnerOptions: Key values message.
	RunnerOptions  []*KeyValuesMessage `json:"runnerOptions,omitempty"`
	StartTimestamp string              `json:"startTimestamp,omitempty"`
	StateMessage   string              `json:"stateMessage,omitempty"`
	// Possible values:
	//   "ASSOCIATED_TEST_LABEL_REMOVED"
	//   "CANARY_TEST_FAILED"
	//   "CANCELED_BY_REQUEST_API"
	//   "CANCELED_BY_TEST_RUN_API"
	//   "EXTRA_BUILD_FAILED"
	//   "INFRA_ERROR"
	//   "MISSING_INVOCATION_FROM_RUNNER"
	//   "OBSOLETED"
	//   "OUT_OF_QUOTA"
	//   "QUEUE_TIMEOUT"
	//   "RUNNER_INTERNAL_FAILURE"
	//   "TEST_ERROR"
	//   "TEST_RUN_CONFIG_BLOCKLISTED"
	//   "TEST_RUN_CONFIG_SUSPENDED"
	//   "UNKNOWN"
	StateReason string `json:"stateReason,omitempty"`
	// Tasks: TestTask message. A single TestJob will create one or multiple
	// TestTasks. TestTaskMessage has information about a TestTask. A TestTask can
	// be executed one or multiple times.
	Tasks []*TestTaskMessage `json:"tasks,omitempty"`
	// Test: TestConfig message. TestConfig is the basic config for a TestJob.
	Test *TestConfigMessage `json:"test,omitempty"`
	// TestBench: TestBench message. TestBench is where actually the test should be
	// run.
	TestBench *TestBenchMessage `json:"testBench,omitempty"`
	// Possible values:
	//   "CANCELED"
	//   "COMPLETED"
	//   "ERROR"
	//   "FATAL"
	//   "QUEUED"
	//   "RUNNING"
	//   "UNKNOWN"
	TestJobState    string `json:"testJobState,omitempty"`
	UpdateTimestamp string `json:"updateTimestamp,omitempty"`
	User            string `json:"user,omitempty"`
	// Utilization: ResourceUsage message. It contains information regarding the
	// usage of a specific resource.
	Utilization []*ResourceUsage `json:"utilization,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the server.
	googleapi.ServerResponse `json:"-"`
	// ForceSendFields is a list of field names (e.g. "Build") to unconditionally
	// include in API requests. By default, fields with empty or default values are
	// omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-ForceSendFields for more
	// details.
	ForceSendFields []string `json:"-"`
	// NullFields is a list of field names (e.g. "Build") to include in API
	// requests with the JSON null value. By default, fields with empty values are
	// omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-NullFields for more details.
	NullFields []string `json:"-"`
}

func (s *TestJobMessage) MarshalJSON() ([]byte, error) {
	type NoMethod TestJobMessage
	return gensupport.MarshalJSON(NoMethod(*s), s.ForceSendFields, s.NullFields)
}

// TestTaskAttemptMessage:
// TestTaskAttempt message. A TestTaskAttempt is a single attempt for a
// TestTask. TestTask can be executed multiple times and each
// TestTaskAttemptMessage will have the information about each TestTaskAttempt.
type TestTaskAttemptMessage struct {
	// AttemptInfo: Key values message.
	AttemptInfo        []*KeyValuesMessage `json:"attemptInfo,omitempty"`
	CreationTimestamp  string              `json:"creationTimestamp,omitempty"`
	EndTimestamp       string              `json:"endTimestamp,omitempty"`
	FailedTestCount    int64               `json:"failedTestCount"`
	FailedTestRunCount int64               `json:"failedTestRunCount"`
	Id                 string              `json:"id,omitempty"`
	StartTimestamp     string              `json:"startTimestamp,omitempty"`
	// Possible values:
	//   "CANCELED"
	//   "COMPLETED"
	//   "ERROR"
	//   "FATAL"
	//   "QUEUED"
	//   "RUNNING"
	//   "UNKNOWN"
	TestTaskAttemptState string `json:"testTaskAttemptState,omitempty"`
	TotalTestCount       int64  `json:"totalTestCount"`
	UpdateTimestamp      string `json:"updateTimestamp,omitempty"`
	// ForceSendFields is a list of field names (e.g. "AttemptInfo") to
	// unconditionally include in API requests. By default, fields with empty or
	// default values are omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-ForceSendFields for more
	// details.
	ForceSendFields []string `json:"-"`
	// NullFields is a list of field names (e.g. "AttemptInfo") to include in API
	// requests with the JSON null value. By default, fields with empty values are
	// omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-NullFields for more details.
	NullFields []string `json:"-"`
}

func (s *TestTaskAttemptMessage) MarshalJSON() ([]byte, error) {
	type NoMethod TestTaskAttemptMessage
	return gensupport.MarshalJSON(NoMethod(*s), s.ForceSendFields, s.NullFields)
}

// TestTaskMessage:
// TestTask message. A single TestJob will create one or multiple TestTasks.
// TestTaskMessage has information about a TestTask. A TestTask can be executed
// one or multiple times.
type TestTaskMessage struct {
	// Attempts: TestTaskAttempt message. A TestTaskAttempt is a single attempt for
	// a TestTask. TestTask can be executed multiple times and each
	// TestTaskAttemptMessage will have the information about each TestTaskAttempt.
	Attempts          []*TestTaskAttemptMessage `json:"attempts,omitempty"`
	CreationTimestamp string                    `json:"creationTimestamp,omitempty"`
	EndTimestamp      string                    `json:"endTimestamp,omitempty"`
	Id                string                    `json:"id,omitempty"`
	ShardIndex        int64                     `json:"shardIndex,omitempty"`
	Shards            int64                     `json:"shards,omitempty"`
	StartTimestamp    string                    `json:"startTimestamp,omitempty"`
	// TaskInfo: Key values message.
	TaskInfo []*KeyValuesMessage `json:"taskInfo,omitempty"`
	// Possible values:
	//   "CANCELED"
	//   "COMPLETED"
	//   "ERROR"
	//   "FATAL"
	//   "QUEUED"
	//   "RUNNING"
	//   "UNKNOWN"
	TestTaskState   string `json:"testTaskState,omitempty"`
	UpdateTimestamp string `json:"updateTimestamp,omitempty"`
	// ForceSendFields is a list of field names (e.g. "Attempts") to
	// unconditionally include in API requests. By default, fields with empty or
	// default values are omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-ForceSendFields for more
	// details.
	ForceSendFields []string `json:"-"`
	// NullFields is a list of field names (e.g. "Attempts") to include in API
	// requests with the JSON null value. By default, fields with empty values are
	// omitted from API requests. See
	// https://pkg.go.dev/google.golang.org/api#hdr-NullFields for more details.
	NullFields []string `json:"-"`
}

func (s *TestTaskMessage) MarshalJSON() ([]byte, error) {
	type NoMethod TestTaskMessage
	return gensupport.MarshalJSON(NoMethod(*s), s.ForceSendFields, s.NullFields)
}
