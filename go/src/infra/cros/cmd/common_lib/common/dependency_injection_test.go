// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common_test

import (
	"testing"

	"google.golang.org/protobuf/types/known/anypb"

	_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/artifact"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/common"
)

func TestDependencyInjectionJson(t *testing.T) {
	ftt.Run("Add execution metadata", t, func(t *ftt.Test) {
		originalProto := &api.TestTask{
			TestRequest: &api.CrosTestRequest{
				TestSuites: []*api.TestSuite{
					{
						ExecutionMetadata: &api.ExecutionMetadata{
							Args: []*api.Arg{
								{
									Flag:  "FirstFlag",
									Value: "FirstValue",
								},
							},
						},
					},
				},
			},
			DynamicDeps: []*api.DynamicDep{
				{
					Key:   "testRequest.testSuites.0.executionMetadata.args",
					Value: `JSON={"flag":"service-address","value":"${generic-service.address}:${generic-service.port}"}`,
				},
			},
		}

		storage := common.NewInjectableStorage()
		_ = storage.Set("generic-service", &labapi.IpEndpoint{
			Address: "localhost",
			Port:    1234,
		})

		assert.Loosely(t, common.InjectDependencies(originalProto, storage, originalProto.DynamicDeps), should.BeNil)
		assert.Loosely(t, len(originalProto.TestRequest.TestSuites[0].ExecutionMetadata.Args), should.Equal(2))
		assert.Loosely(t, originalProto.TestRequest.TestSuites[0].ExecutionMetadata.Args[1].Flag, should.Equal("service-address"))
		assert.Loosely(t, originalProto.TestRequest.TestSuites[0].ExecutionMetadata.Args[1].Value, should.Equal("localhost:1234"))
	})
}

func TestDependencyInjectionAny(t *testing.T) {
	ftt.Run("Concrete -> Any", t, func(t *ftt.Test) {
		publishMetadata, _ := anypb.New(&artifact.TestResult{})
		original_proto := &testapi.PublishRequest{
			Metadata: publishMetadata,
		}
		testResult := &artifact.TestResult{
			Version: 12,
			TestInvocation: &artifact.TestInvocation{
				IsCftRun: true,
			},
		}
		storage := common.NewInjectableStorage()
		assert.Loosely(t, storage.Set("testResult", testResult), should.BeNil)
		assert.Loosely(t, storage.LoadInjectables(), should.BeNil)
		assert.Loosely(t, common.Inject(original_proto, "metadata", storage, "ANY(type.googleapis.com/chromiumos.test.artifact.TestResult)=testResult"), should.BeNil)

		assert.Loosely(t, original_proto.Metadata.TypeUrl, should.Equal("type.googleapis.com/chromiumos.test.artifact.TestResult"))
		extractedTestResult := &artifact.TestResult{}
		assert.Loosely(t, original_proto.Metadata.UnmarshalTo(extractedTestResult), should.BeNil)
		assert.Loosely(t, extractedTestResult.Version, should.Equal(testResult.Version))
		assert.Loosely(t, extractedTestResult.TestInvocation.IsCftRun, should.Equal(testResult.TestInvocation.IsCftRun))
	})

	ftt.Run("Concrete -> Undefined Any", t, func(t *ftt.Test) {
		original_proto := &testapi.PublishRequest{}
		testResult := &artifact.TestResult{
			Version: 12,
			TestInvocation: &artifact.TestInvocation{
				IsCftRun: true,
			},
		}
		storage := common.NewInjectableStorage()
		assert.Loosely(t, storage.Set("testResult", testResult), should.BeNil)
		assert.Loosely(t, storage.LoadInjectables(), should.BeNil)
		assert.Loosely(t, common.Inject(original_proto, "metadata", storage, "ANY(type.googleapis.com/chromiumos.test.artifact.TestResult)=testResult"), should.BeNil)

		assert.Loosely(t, original_proto.Metadata.TypeUrl, should.Equal("type.googleapis.com/chromiumos.test.artifact.TestResult"))
		extractedTestResult := &artifact.TestResult{}
		assert.Loosely(t, original_proto.Metadata.UnmarshalTo(extractedTestResult), should.BeNil)
		assert.Loosely(t, extractedTestResult.Version, should.Equal(testResult.Version))
		assert.Loosely(t, extractedTestResult.TestInvocation.IsCftRun, should.Equal(testResult.TestInvocation.IsCftRun))
	})

	ftt.Run("Any -> Any", t, func(t *ftt.Test) {
		publishMetadata, _ := anypb.New(&artifact.TestResult{})
		original_proto := &testapi.PublishRequest{
			Metadata: publishMetadata,
		}
		testResult := &artifact.TestResult{
			Version: 12,
			TestInvocation: &artifact.TestInvocation{
				IsCftRun: true,
			},
		}
		testResultAny, _ := anypb.New(testResult)
		storage := common.NewInjectableStorage()
		assert.Loosely(t, storage.Set("testResult", testResultAny), should.BeNil)
		assert.Loosely(t, storage.LoadInjectables(), should.BeNil)
		assert.Loosely(t, common.Inject(original_proto, "metadata", storage, "testResult"), should.BeNil)

		assert.Loosely(t, original_proto.Metadata.TypeUrl, should.Equal("type.googleapis.com/chromiumos.test.artifact.TestResult"))
		extractedTestResult := &artifact.TestResult{}
		assert.Loosely(t, original_proto.Metadata.UnmarshalTo(extractedTestResult), should.BeNil)
		assert.Loosely(t, extractedTestResult.Version, should.Equal(testResult.Version))
		assert.Loosely(t, extractedTestResult.TestInvocation.IsCftRun, should.Equal(testResult.TestInvocation.IsCftRun))
	})

	ftt.Run("Overwrite with different Any", t, func(t *ftt.Test) {
		publishMetadata, _ := anypb.New(&artifact.TestResult{})
		original_proto := &testapi.PublishRequest{
			Metadata: publishMetadata,
		}
		testInvocation := &artifact.TestInvocation{
			IsCftRun: true,
		}
		testInvocationAny, _ := anypb.New(testInvocation)
		storage := common.NewInjectableStorage()
		assert.Loosely(t, storage.Set("testResult", testInvocationAny), should.BeNil)
		assert.Loosely(t, storage.LoadInjectables(), should.BeNil)
		assert.Loosely(t, common.Inject(original_proto, "metadata", storage, "testResult"), should.BeNil)

		assert.Loosely(t, original_proto.Metadata.TypeUrl, should.Equal("type.googleapis.com/chromiumos.test.artifact.TestInvocation"))
		extractedTestInvocation := &artifact.TestInvocation{}
		assert.Loosely(t, original_proto.Metadata.UnmarshalTo(extractedTestInvocation), should.BeNil)
		assert.Loosely(t, extractedTestInvocation.IsCftRun, should.Equal(testInvocation.IsCftRun))
	})
}

func TestDependencyInjectionBasic(t *testing.T) {
	ftt.Run("basic injection", t, func(t *ftt.Test) {
		original_proto := &testapi.CrosProvisionRequest{
			ProvisionState: &testapi.ProvisionState{
				Id: &testapi.ProvisionState_Id{
					Value: "Hello, World!",
				},
				PreventReboot: true,
			},
		}
		dut_address_proto := &labapi.IpEndpoint{
			Address: "localhost",
			Port:    4040,
		}
		storage := common.NewInjectableStorage()
		err := storage.Set("dut_primary", dut_address_proto)
		assert.Loosely(t, err, should.BeNil)
		err = storage.LoadInjectables()
		assert.Loosely(t, err, should.BeNil)

		err = common.Inject(original_proto, "dutServer", storage, "dut_primary")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, original_proto.DutServer, should.NotBeNil)
		assert.Loosely(t, original_proto.DutServer.Address, should.Equal(dut_address_proto.Address))
		assert.Loosely(t, original_proto.DutServer.Port, should.Equal(dut_address_proto.Port))
	})

	ftt.Run("IpEndpoint direct injection", t, func(t *ftt.Test) {
		original_proto := &labapi.IpEndpoint{}
		dut_address_proto := &labapi.IpEndpoint{
			Address: "localhost",
			Port:    4040,
		}
		storage := common.NewInjectableStorage()
		err := storage.Set("cros-dut", dut_address_proto)
		assert.Loosely(t, err, should.BeNil)
		err = storage.LoadInjectables()
		assert.Loosely(t, err, should.BeNil)

		err = common.Inject(original_proto, "", storage, "cros-dut")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, original_proto, should.NotBeNil)
		assert.Loosely(t, original_proto.Address, should.Equal(dut_address_proto.Address))
		assert.Loosely(t, original_proto.Port, should.Equal(dut_address_proto.Port))
	})

	ftt.Run("test injection", t, func(t *ftt.Test) {
		original_proto := &api.ContainerRequest{
			DynamicIdentifier: "cros-provision",
			Container: &api.Template{
				Container: &api.Template_CrosProvision{
					CrosProvision: &api.CrosProvisionTemplate{
						InputRequest: &api.CrosProvisionRequest{},
					},
				},
			},
			ContainerImageKey: "cros-provision",
			DynamicDeps: []*api.DynamicDep{
				{
					Key:   "crosProvision.inputRequest.dut",
					Value: "dut_primary",
				},
				{
					Key:   "crosProvision.inputRequest.dutServer",
					Value: "cros-dut",
				},
			},
		}
		dut_address_proto := &labapi.IpEndpoint{
			Address: "localhost",
			Port:    4040,
		}
		dut := &labapi.Dut{
			DutType: &labapi.Dut_Chromeos{
				Chromeos: &labapi.Dut_ChromeOS{
					Name: "hello",
				},
			},
		}
		storage := common.NewInjectableStorage()
		err := storage.Set("cros-dut", dut_address_proto)
		assert.Loosely(t, err, should.BeNil)
		err = storage.Set("dut_primary", dut)
		assert.Loosely(t, err, should.BeNil)
		err = storage.LoadInjectables()
		assert.Loosely(t, err, should.BeNil)

		for _, dep := range original_proto.DynamicDeps {
			err := common.Inject(original_proto.Container, dep.Key, storage, dep.Value)
			assert.Loosely(t, err, should.BeNil)
		}
	})
}

func TestDependencyInjectionArray(t *testing.T) {
	ftt.Run("array injection", t, func(t *ftt.Test) {
		original_proto := &testapi.CrosProvisionRequest{
			ProvisionState: &testapi.ProvisionState{
				Id: &testapi.ProvisionState_Id{
					Value: "Hello, World!",
				},
				PreventReboot: true,
			},
		}
		dut_address_protos := []*labapi.IpEndpoint{
			{
				Address: "not_expected",
				Port:    4040,
			},
			{
				Address: "expected",
				Port:    1234,
			},
		}

		storage := common.NewInjectableStorage()
		err := storage.Set("duts", dut_address_protos)
		assert.Loosely(t, err, should.BeNil)
		err = storage.LoadInjectables()
		assert.Loosely(t, err, should.BeNil)

		err = common.Inject(original_proto, "dutServer", storage, "duts.1")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, original_proto.DutServer, should.NotBeNil)
		assert.Loosely(t, original_proto.DutServer.Address, should.Equal(dut_address_protos[1].Address))
		assert.Loosely(t, original_proto.DutServer.Port, should.Equal(dut_address_protos[1].Port))
	})
}

func TestDependencyInjectionArrayAppend(t *testing.T) {
	ftt.Run("array injection", t, func(t *ftt.Test) {
		original_proto := &testapi.CrosProvisionRequest{
			ProvisionState: &testapi.ProvisionState{
				Id: &testapi.ProvisionState_Id{
					Value: "Hello, World!",
				},
				PreventReboot: true,
				Packages: []*testapi.ProvisionState_Package{
					{
						PackagePath: &_go.StoragePath{
							Path: "a",
						},
					},
					{
						PackagePath: &_go.StoragePath{
							Path: "b",
						},
					},
					{
						PackagePath: &_go.StoragePath{
							Path: "c",
						},
					},
				},
			},
		}
		new_package := &testapi.ProvisionState_Package{
			PackagePath: &_go.StoragePath{
				Path: "d",
			},
		}

		storage := common.NewInjectableStorage()
		err := storage.Set("package", new_package)
		assert.Loosely(t, err, should.BeNil)
		err = storage.LoadInjectables()
		assert.Loosely(t, err, should.BeNil)

		err = common.Inject(original_proto, "provisionState.packages", storage, "package")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, original_proto.ProvisionState.Packages, should.HaveLength(4))
		assert.Loosely(t, original_proto.ProvisionState.Packages[3].PackagePath.Path, should.Equal(new_package.PackagePath.Path))
	})
}

func TestDependencyInjectionArrayOverride(t *testing.T) {
	ftt.Run("array override injection", t, func(t *ftt.Test) {
		original_proto := &testapi.CrosProvisionRequest{
			ProvisionState: &testapi.ProvisionState{
				Id: &testapi.ProvisionState_Id{
					Value: "Hello, World!",
				},
				PreventReboot: true,
				Packages:      []*testapi.ProvisionState_Package{},
			},
		}
		new_packages := []*testapi.ProvisionState_Package{
			{
				PackagePath: &_go.StoragePath{
					Path: "d",
				},
			},
			{
				PackagePath: &_go.StoragePath{
					Path: "e",
				},
			},
			{
				PackagePath: &_go.StoragePath{
					Path: "f",
				},
			},
		}

		storage := common.NewInjectableStorage()
		err := storage.Set("packages", new_packages)
		assert.Loosely(t, err, should.BeNil)
		err = storage.LoadInjectables()
		assert.Loosely(t, err, should.BeNil)

		err = common.Inject(original_proto, "provisionState.packages", storage, "packages")

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, original_proto.ProvisionState.Packages, should.HaveLength(3))
		assert.Loosely(t, original_proto.ProvisionState.Packages[0].PackagePath.Path, should.Equal(new_packages[0].PackagePath.Path))
		assert.Loosely(t, original_proto.ProvisionState.Packages[1].PackagePath.Path, should.Equal(new_packages[1].PackagePath.Path))
		assert.Loosely(t, original_proto.ProvisionState.Packages[2].PackagePath.Path, should.Equal(new_packages[2].PackagePath.Path))
	})
}

func TestDependencyInjectionFullTest(t *testing.T) {
	storage := common.NewInjectableStorage()
	req := &api.CrosTestRunnerDynamicRequest{
		StartRequest: &api.CrosTestRunnerDynamicRequest_Build{
			Build: &api.BuildMode{},
		},
		Params: &api.CrosTestRunnerParams{},
	}
	err := storage.Set("req", req)
	if err != nil {
		t.Fatalf("%s", err)
	}

	ftt.Run("NoticeChangesInStateKeeper", t, func(t *ftt.Test) {
		req.GetParams().Keyvals = map[string]string{
			"build_target": "drallion",
		}
		err := storage.LoadInjectables()
		assert.Loosely(t, err, should.BeNil)

		buildTarget, err := storage.Get("req.params.keyvals.build_target")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, buildTarget, should.Equal("drallion"))
	})

	ftt.Run("CanAddStringToInjectables", t, func(t *ftt.Test) {
		err := storage.Set("hello", "world!")
		assert.Loosely(t, err, should.BeNil)
		err = storage.LoadInjectables()
		assert.Loosely(t, err, should.BeNil)

		str, err := storage.Get("hello")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, str, should.Equal("world!"))
	})

	ftt.Run("CanAddStringArrayToInjectables", t, func(t *ftt.Test) {
		err := storage.Set("hello", []string{"World1", "World2"})
		assert.Loosely(t, err, should.BeNil)
		err = storage.LoadInjectables()
		assert.Loosely(t, err, should.BeNil)

		arr, err := storage.Get("hello")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, arr, should.HaveLength(2))
	})

	ftt.Run("CanAddAndPullParams", t, func(t *ftt.Test) {
		req.Params.TestSuites = []*api.TestSuite{
			{
				Name: "Test1",
			}, {
				Name: "Test2",
			}, {
				Name: "Test3",
			},
		}
		assert.Loosely(t, err, should.BeNil)
		err = storage.LoadInjectables()
		assert.Loosely(t, err, should.BeNil)

		testSuites, err := storage.Get("req.params.testSuites")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, testSuites, should.HaveLength(3))

		req.Params.TestSuites = append(req.Params.TestSuites, &testapi.TestSuite{Name: "Test4"})
		err = storage.LoadInjectables()
		assert.Loosely(t, err, should.BeNil)
		testSuites, err = storage.Get("req.params.testSuites")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, testSuites, should.HaveLength(4))
	})

	ftt.Run("CanDoDependencyInjection", t, func(t *ftt.Test) {
		endpoint := &labapi.IpEndpoint{
			Address: "localhost",
			Port:    1234,
		}
		err := storage.Set("cros-test", endpoint)
		assert.Loosely(t, err, should.BeNil)
		req.Params.TestSuites = []*api.TestSuite{
			{
				Name: "Test1",
			}, {
				Name: "Test2",
			}, {
				Name: "Test3",
			},
		}
		testRequest := &api.TestTask{
			ServiceAddress: &labapi.IpEndpoint{},
			TestRequest:    &testapi.CrosTestRequest{},
			DynamicDeps: []*api.DynamicDep{
				{
					Key:   "serviceAddress",
					Value: "cros-test",
				},
				{
					Key:   "testRequest.testSuites",
					Value: "req.params.testSuites",
				},
			},
		}
		req.OrderedTasks = []*api.CrosTestRunnerDynamicRequest_Task{
			{
				Task: &api.CrosTestRunnerDynamicRequest_Task_Test{
					Test: testRequest,
				},
			},
		}

		err = common.InjectDependencies(testRequest, storage, testRequest.DynamicDeps)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, testRequest.ServiceAddress.Address, should.Equal(endpoint.Address))
		assert.Loosely(t, testRequest.ServiceAddress.Port, should.Equal(endpoint.Port))
		assert.Loosely(t, testRequest.TestRequest.TestSuites, should.HaveLength(len(req.Params.TestSuites)))
	})
}

func TestGenericContainerOutputAsDependency(t *testing.T) {
	storage := common.NewInjectableStorage()
	devboardServer, _ := anypb.New(&labapi.IpEndpoint{
		Address: "localhost",
		Port:    12345,
	})
	genericContainerOutput := &api.GenericStopResponse{
		Message: &testapi.GenericMessage{
			Values: map[string]*anypb.Any{
				"devboard-server": devboardServer,
			},
		},
	}
	err := storage.Set("user-container_stop", genericContainerOutput)
	if err != nil {
		t.Fatalf("%s", err)
	}

	ftt.Run("Anypb dep", t, func(t *ftt.Test) {
		testRequest := &api.TestTask{
			TestRequest: &api.CrosTestRequest{
				Primary:  &api.CrosTestRequest_Device{},
				Metadata: &anypb.Any{},
			},
			DynamicDeps: []*api.DynamicDep{
				{
					Key:   "testRequest.primary.devboardServer",
					Value: "user-container_stop.message.values.devboard-server",
				},
			},
		}

		err = common.InjectDependencies(testRequest, storage, testRequest.DynamicDeps)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, testRequest.TestRequest.Primary.DevboardServer.Address, should.Equal("localhost"))
		assert.Loosely(t, testRequest.TestRequest.Primary.DevboardServer.Port, should.Equal(12345))
	})
}

func TestHandlers(t *testing.T) {
	storage := common.NewInjectableStorage()

	ftt.Run("bool handler", t, func(t *ftt.Test) {
		valUntyped, err := storage.Get("BOOL=true")
		val, ok := valUntyped.(bool)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ok, should.BeTrue)
		assert.Loosely(t, val, should.BeTrue)

		valUntyped, err = storage.Get("BOOL=false")
		val, ok = valUntyped.(bool)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ok, should.BeTrue)
		assert.Loosely(t, val, should.BeFalse)
	})

	ftt.Run("fmt handler", t, func(t *ftt.Test) {
		endpoint := &labapi.IpEndpoint{
			Address: "localhost",
			Port:    1234,
		}
		assert.Loosely(t, storage.Set("endpoint", endpoint), should.BeNil)
		assert.Loosely(t, storage.LoadInjectables(), should.BeNil)

		valUntyped, err := storage.Get("FMT=${endpoint.address}:${endpoint.port}")
		val, ok := valUntyped.(string)

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, ok, should.BeTrue)
		assert.Loosely(t, val, should.Equal("localhost:1234"))
	})
}

func TestHandlersWithinDynamicDeps(t *testing.T) {
	storage := common.NewInjectableStorage()

	ftt.Run("init", t, func(t *ftt.Test) {
		endpoint := &labapi.IpEndpoint{
			Address: "localhost",
			Port:    1234,
		}
		assert.Loosely(t, storage.Set("endpoint", endpoint), should.BeNil)
	})

	ftt.Run("bool handler", t, func(t *ftt.Test) {
		crosProvisionMetadata, err := anypb.New(&api.CrOSProvisionMetadata{})
		assert.Loosely(t, err, should.BeNil)
		provisionTaskRequest := &api.ProvisionTask{
			InstallRequest: &api.InstallRequest{
				Metadata: crosProvisionMetadata,
			},
			DynamicDeps: []*api.DynamicDep{
				{
					Key:   "installRequest.metadata.updateFirmware",
					Value: "BOOL=true",
				},
			},
		}

		assert.Loosely(t, common.InjectDependencies(provisionTaskRequest, storage, provisionTaskRequest.DynamicDeps), should.BeNil)
		metadata := &api.CrOSProvisionMetadata{}
		assert.Loosely(t, provisionTaskRequest.GetInstallRequest().GetMetadata().UnmarshalTo(metadata), should.BeNil)
		assert.Loosely(t, metadata.UpdateFirmware, should.BeTrue)
	})

	ftt.Run("fmt handler", t, func(t *ftt.Test) {
		containerRequest := &api.ContainerRequest{
			Container: &api.Template{
				Container: &api.Template_Generic{
					Generic: &api.GenericTemplate{
						BinaryName: "example container",
						BinaryArgs: []string{
							"server", "-dutEndpoint", "", "-log", "/tmp/example",
						},
					},
				},
			},
			DynamicDeps: []*api.DynamicDep{
				{
					Key:   "generic.binaryArgs.2",
					Value: "FMT=${endpoint.address}:${endpoint.port}",
				},
			},
		}

		assert.Loosely(t, common.InjectDependencies(containerRequest.Container, storage, containerRequest.DynamicDeps), should.BeNil)
		assert.Loosely(t, containerRequest.GetContainer().GetGeneric().GetBinaryArgs()[2], should.Equal("localhost:1234"))
	})
}
