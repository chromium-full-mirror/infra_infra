// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

// This follows the proto definition of mobileharness.gateway.GetSessionDetailResponse in Google3,
// to allow deserializing the json marshaled GetSessionDetailResponse.
// see go/omnilab-gateway for more details.
type sessionDetailResponse struct {
	SessionDetail *sessionDetail `json:"sessionDetail,omitempty"`
}

type sessionDetail struct {
	SessionSummary *sessionSummary `json:"sessionSummary,omitempty"`
	JobDetail      []*jobDetail    `json:"jobDetail,omitempty"`
}

type sessionSummary struct {
	ID                    string         `json:"id,omitempty"`
	Status                string         `json:"status,omitempty"`
	Result                string         `json:"result,omitempty"`
	StartTimeMs           string         `json:"startTimeMs,omitempty"`
	EndTimeMs             string         `json:"endTimeMs,omitempty"`
	Error                 []*errorInfo   `json:"error,omitempty"`
	User                  string         `json:"user,omitempty"`
	Client                string         `json:"client,omitempty"`
	CustomConfig          customConfig   `json:"customConfig,omitempty"`
	ExecutorSpec          string         `json:"executorSpec,omitempty"`
	SessionConfig         sessionConfig  `json:"sessionConfig,omitempty"`
	ExecutorVersion       string         `json:"executorVersion,omitempty"`
	JobResults            []string       `json:"jobResults,omitempty"`
	Property              []*strPair     `json:"property,omitempty"`
	GenFileWithLength     []*file        `json:"genFileWithLength,omitempty"`
	CreateTimeMs          string         `json:"createTimeMs,omitempty"`
	OriginalSessionConfig *sessionConfig `json:"originalSessionConfig,omitempty"`
}

type jobSummary struct {
	ID                string       `json:"id,omitempty"`
	Name              string       `json:"name,omitempty"`
	Status            string       `json:"status,omitempty"`
	Result            string       `json:"result,omitempty"`
	StartTimeMs       string       `json:"startTimeMs,omitempty"`
	EndTimeMs         string       `json:"endTimeMs,omitempty"`
	Error             []string     `json:"error,omitempty"`
	Property          []*strPair   `json:"property,omitempty"`
	Retry             retry        `json:"retry,omitempty"`
	RunFileWithLength []*file      `json:"runFileWithLength,omitempty"`
	GenFileWithLength []*file      `json:"genFileWithLength,omitempty"`
	Log               string       `json:"log,omitempty"`
	TotalTestCount    int          `json:"totalTestCount,omitempty"`
	ErrorInfo         []*errorInfo `json:"errorInfo,omitempty"`
	CreateTimeMs      string       `json:"createTimeMs,omitempty"`
	Type              *jobType     `json:"type,omitempty"`
}

type testDetail struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	// ENUM: "NEW", "ASSIGNED", "RUNNING", "DONE", "SUSPENDED"
	Status string `json:"status,omitempty"`
	// ENUM: "UNKNOWN", "PASS", "FAIL", "ERROR", "TIMEOUT", "ABORT", "INFRA_ERROR", "ALLOC_FAIL", "ALLOC_ERROR"
	Result            string        `json:"result,omitempty"`
	Property          []*strPair    `json:"property,omitempty"`
	Error             []string      `json:"error,omitempty"`
	Log               string        `json:"log,omitempty"`
	StartTimeMs       string        `json:"startTimeMs,omitempty"`
	EndTimeMs         string        `json:"endTimeMs,omitempty"`
	Lab               *labLocator   `json:"lab,omitempty"`
	DeviceSerial      []string      `json:"deviceSerial,omitempty"`
	ServerSpec        string        `json:"serverSpec,omitempty"`
	GenFileWithLength []*file       `json:"genFileWithLength,omitempty"`
	TestDetail        []*testDetail `json:"testDetail,omitempty"`
	DurationTimeMs    string        `json:"durationTimeMs,omitempty"`
	ErrorInfo         []*errorInfo  `json:"errorInfo,omitempty"`
	JobID             string        `json:"jobId,omitempty"`
}

type jobDetail struct {
	JobSummary *jobSummary   `json:"jobSummary,omitempty"`
	TestDetail []*testDetail `json:"testDetail,omitempty"`
}

// Lab address and port information.
type labLocator struct {
	IP       string     `json:"ip,omitempty"`
	HostName string     `json:"hostName,omitempty"`
	Port     []*labPort `json:"port,omitempty"`
	Location string     `json:"location,omitempty"`
}

// Lab port type and number.
type labPort struct {
	Type string `json:"type,omitempty"`
	Num  int    `json:"num,omitempty"`
}

// Type of a job, including the device type, driver and decorator information.
type jobType struct {
	Device    string   `json:"device,omitempty"`
	Driver    string   `json:"driver,omitempty"`
	Decorator []string `json:"decorator,omitempty"`
}

type file struct {
	Path   string `json:"path,omitempty"`
	Length string `json:"length,omitempty"`
}

type errorInfo struct {
	Code       int    `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
	StackTrace string `json:"stackTrace,omitempty"`
	Name       string `json:"name,omitempty"`
}

type sessionConfig struct {
	JobConfig                           []*jobConfig     `json:"jobConfig,omitempty"`
	BlazeTarget                         []*blazeTarget   `json:"blazeTarget,omitempty"`
	Serial                              bool             `json:"serial,omitempty"`
	Client                              string           `json:"client,omitempty"`
	Label                               string           `json:"label,omitempty"`
	CustomConfig                        *customConfig    `json:"customConfig,omitempty"`
	Name                                string           `json:"name,omitempty"`
	User                                string           `json:"user,omitempty"`
	MasterSpec                          string           `json:"masterSpec,omitempty"`
	ExecutorCells                       string           `json:"executorCells,omitempty"`
	MaxRunningSessionCountForThisClient int              `json:"maxRunningSessionCountForThisClient,omitempty"`
	RerunWhenExecutorShutdown           bool             `json:"rerunWhenExecutorShutdown,omitempty"`
	MaxConcurrentRunningJobsPerSession  int              `json:"maxConcurrentRunningJobsPerSession,omitempty"`
	ExecutorType                        string           `json:"executorType,omitempty"`
	GuitarParameter                     *guitarParameter `json:"guitarParameter,omitempty"`
	CloudPubSubTopic                    string           `json:"cloudPubSubTopic,omitempty"`
	DryRun                              bool             `json:"dryRun,omitempty"`
}

type customConfig struct {
}

type guitarParameter struct {
	GuitarClusterName string `json:"guitarClusterName,omitempty"`
}

type jobConfig struct {
	Name                 string               `json:"name,omitempty"`
	User                 string               `json:"user,omitempty"`
	Type                 *Type                `json:"type,omitempty"`
	Dimension            []*strPair           `json:"dimension,omitempty"`
	Param                []*strPair           `json:"param,omitempty"`
	File                 []*strPair           `json:"file,omitempty"`
	Test                 []string             `json:"test,omitempty"`
	Timeout              *timeout             `json:"timeout,omitempty"`
	Retry                *retry               `json:"retry,omitempty"`
	Repeat               *repeat              `json:"repeat,omitempty"`
	SubDeviceSpec        *mhSubDeviceSpec     `json:"subDeviceSpec,omitempty"`
	SharedDimensionNames []string             `json:"sharedDimensionNames,omitempty"`
	Property             []*strPair           `json:"property,omitempty"`
	JobSpec              []*jobSpec           `json:"jobSpec,omitempty"`
	JobConfigFromTarget  *jobConfigFromTarget `json:"jobConfigFromTarget,omitempty"`
	MasterSpec           string               `json:"masterSpec,omitempty"`
	Priority             string               `json:"priority,omitempty"`
}

type jobSpec struct {
	DriverSpec    *driverSpec    `json:"driverSpec,omitempty"`
	DecoratorSpec *decoratorSpec `json:"decoratorSpec,omitempty"`
}

type driverSpec struct {
}

type decoratorSpec struct {
}

type Type struct {
	Device string `json:"device,omitempty"`
	Driver string `json:"driver,omitempty"`
}

type timeout struct {
	TestTimeoutMs string `json:"testTimeoutMs,omitempty"`
}

type retry struct {
	TestAttempts int `json:"testAttempts,omitempty"`
}

type repeat struct {
}

type blazeTarget struct {
	TargetName string `json:"targetName,omitempty"`
	TestFlags  string `json:"testFlags,omitempty"`
}

type strPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type jobConfigFromTarget struct {
	MhJobConfig     *mhJobConfig `json:"mhJobConfig,omitempty"`
	NonstandardFlag []string     `json:"nonstandardFlag,omitempty"`
	GenDirPath      string       `json:"genDirPath,omitempty"`
}

// mhJobConfig contains all necessary settings to initialize mobileharness client.
type mhJobConfig struct {
	Name                   string            `json:"name,omitempty"`
	Driver                 *mhDriver         `json:"driver,omitempty"`
	Device                 *mhDeviceList     `json:"device,omitempty"`
	Params                 map[string]any    `json:"params,omitempty"`
	Files                  *mhFileConfigList `json:"files,omitempty"`
	NeedCheckBuiltFiles    bool              `json:"needCheckBuiltFiles,omitempty"`
	Tests                  *stringList       `json:"tests,omitempty"`
	ExecMode               string            `json:"execMode,omitempty"`
	RunAs                  string            `json:"runAs,omitempty"`
	JobTimeoutSec          string            `json:"jobTimeoutSec,omitempty"`
	TestTimeoutSec         string            `json:"testTimeoutSec,omitempty"`
	StartTimeoutSec        string            `json:"startTimeoutSec,omitempty"`
	TestAttempts           int               `json:"testAttempts,omitempty"`
	RetryLevel             string            `json:"retryLevel,omitempty"`
	RepeatedRuns           int               `json:"repeatedRuns,omitempty"`
	Priority               string            `json:"priority,omitempty"`
	GenFileDir             string            `json:"genFileDir,omitempty"`
	MasterSpec             string            `json:"masterSpec,omitempty"`
	TargetLocations        map[string]any    `json:"targetLocations,omitempty"`
	SpecFiles              *stringList       `json:"specFiles,omitempty"`
	RemoteFileDir          string            `json:"remoteFileDir,omitempty"`
	Tags                   *stringList       `json:"tags,omitempty"`
	AllocationExitStrategy string            `json:"allocationExitStrategy,omitempty"`
	SharedDimensionNames   *stringList       `json:"sharedDimensionNames,omitempty"`
	ParamStats             map[string]any    `json:"paramStats,omitempty"`
	BlazeTarget            string            `json:"blazeTarget,omitempty"`
}

type stringList struct {
	Content []string `json:"content,omitempty"`
}

type mhFileConfigList struct {
	Content []*mhFileConfig `json:"content,omitempty"`
}
type mhFileConfig struct {
	Tag  string   `json:"tag,omitempty"`
	Path []string `json:"path,omitempty"`
}

type mhDeviceList struct {
	SubDeviceSpec       []*mhSubDeviceSpec `json:"subDeviceSpec,omitempty"`
	SharedDimensionName string             `json:"sharedDimensionName,omitempty"`
}

type mhDriver struct {
	Name  string `json:"name,omitempty"`
	Param string `json:"param,omitempty"`
}

type mhSubDeviceSpec struct {
	Type       string           `json:"type,omitempty"`
	Dimensions map[string]any   `json:"dimensions,omitempty"`
	Decorators *mhDecoratorList `json:"decorators,omitempty"`
}

type mhDecoratorList struct {
	Driver []*mhDriver `json:"driver,omitempty"`
}
