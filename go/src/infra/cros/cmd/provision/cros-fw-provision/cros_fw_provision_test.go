// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	conf "go.chromium.org/chromiumos/config/go"
	build_api "go.chromium.org/chromiumos/config/go/build/api"
	longrunning "go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/localonly"

	"go.chromium.org/infra/cros/cmd/provision/cros-fw-provision/cli"
	firmwareservice "go.chromium.org/infra/cros/cmd/provision/cros-fw-provision/service"
	state_machine "go.chromium.org/infra/cros/cmd/provision/cros-fw-provision/state-machine"
	"go.chromium.org/infra/cros/cmd/provision/mock_api"
)

type curlMatcher struct {
	target      string
	fileMatcher gomock.Matcher
	log         *log.Logger
}

func newCurlMatcher(target string, fileMatcher gomock.Matcher, log *log.Logger) curlMatcher {
	return curlMatcher{
		target:      target,
		fileMatcher: fileMatcher,
		log:         log,
	}
}
func (c curlMatcher) Matches(x interface{}) bool {
	req, ok := x.(*api.ExecCommandRequest)
	if ok {
		if req.Command != "curl" {
			return false
		}
		if len(req.Args) != 5 {
			return false
		}
		if req.Args[0] != "-f" || req.Args[1] != "-S" || req.Args[2] != "-o" || req.Args[3] != c.target {
			return false
		}
		idx := strings.Index(req.Args[4], "?file=")
		if idx < 0 {
			return false
		}
		filename, err := url.QueryUnescape(req.Args[4][idx+6:])
		if err != nil {
			panic(err)
		}
		// Remove trailing quote
		filename = filename[:len(filename)-1]
		return c.fileMatcher.Matches(filename)
	}
	return false
}

func (c curlMatcher) String() string {
	return fmt.Sprintf("curl -f -S -o %s url?file=(%s)", c.target, c.fileMatcher)
}

type rpcMsg struct {
	msg proto.Message
}

func (r *rpcMsg) Matches(msg interface{}) bool {
	m, ok := msg.(proto.Message)
	if !ok {
		return false
	}
	return proto.Equal(m, r.msg)
}

func (r *rpcMsg) String() string {
	return fmt.Sprintf("is %s", prototext.Format(r.msg))
}

func (r *rpcMsg) Got(got interface{}) string {
	return prototext.Format(got.(proto.Message))
}

type dutServiceExecCommandClient struct {
	Response *api.ExecCommandResponse
	grpc.ClientStream
}

func (x *dutServiceExecCommandClient) Recv() (*api.ExecCommandResponse, error) {
	if x.Response != nil {
		tmp := x.Response
		x.Response = nil
		return tmp, nil
	}
	return nil, io.EOF
}

func newResponse(pb *api.ExecCommandResponse) *dutServiceExecCommandClient {
	return &dutServiceExecCommandClient{
		Response: pb,
	}
}

type dutServiceFetchFileClient struct {
	Response *api.File
	grpc.ClientStream
}

func (x *dutServiceFetchFileClient) Recv() (*api.File, error) {
	if x.Response != nil {
		tmp := x.Response
		x.Response = nil
		return tmp, nil
	}
	return nil, io.EOF
}

func newFileResponse(pb *api.File) *dutServiceFetchFileClient {
	return &dutServiceFetchFileClient{
		Response: pb,
	}
}

func TestDetailedRequestSSHStates(t *testing.T) {
	localonly.Because(t, "b/402551644")
	fakeGSPath := "gs://chromeos-image-archive/board-firmware-branch/R123-12345.0.0/board/firmware_from_source.tar.bz2"

	makeRequest := func(main_rw, main_ro, ec_ro, ec_rw bool) *api.InstallRequest {
		fakePayload := &build_api.FirmwarePayload{FirmwareImage: &build_api.FirmwarePayload_FirmwareImagePath{FirmwareImagePath: &conf.StoragePath{HostType: conf.StoragePath_GS, Path: fakeGSPath}}}
		FirmwareConfig := build_api.FirmwareConfig{}
		if main_rw {
			FirmwareConfig.MainRwPayload = fakePayload
		}
		if main_ro {
			FirmwareConfig.MainRoPayload = fakePayload
		}
		if ec_ro {
			FirmwareConfig.EcRoPayload = fakePayload
		}
		if ec_rw {
			FirmwareConfig.EcRwPayload = fakePayload
		}
		any, err := anypb.New(&api.FirmwareProvisionInstallMetadata{
			FirmwareConfig: &FirmwareConfig,
		})
		if err != nil {
			t.Fatal("any.new failed: ", err)
		}

		req := &api.InstallRequest{
			Metadata: any,
		}

		return req
	}

	checkStateName := func(st state_machine.ServiceState, expectedStateName string) {
		if st == nil {
			if len(expectedStateName) > 0 {
				t.Fatalf("expected state %v. got: nil state", expectedStateName)
			}
			return
		}
		stateName := st.Name()
		if stateName != expectedStateName {
			t.Fatalf("expected state %v. got: %v", expectedStateName, stateName)
		}
	}

	type TestCase struct {
		// inputs
		mainRw, mainRo, ecRo, ecRw bool
		configYAML                 string
		// These are the first files that will exist in the archive.
		apImageWithinArchive   string
		ecImageWithinArchive   string
		npcxImageWithinArchive string
		ecConfigWithinArchive  string
		// expected outputs
		updateRw, updateRo     bool
		expectConstructorError bool
		expectedResponse       *api.FirmwareProvisionResponse
	}

	fullVersion := &api.FirmwareProvisionResponse{
		ApRoVersion: "roversion",
		ApRwVersion: "rwversion",
		EcRoVersion: "ecroversion",
		EcRwVersion: "ecrwversion",
	}
	roOnlyVersion := &api.FirmwareProvisionResponse{
		ApRoVersion: "roversion",
		ApRwVersion: "roversioninrw",
		EcRoVersion: "ecroversion",
		EcRwVersion: "ecroversioninrw",
	}

	testCases := []TestCase{
		{ /*in*/ false, false, false, false, "", "image.bin", "ec.bin", "npcx_monitor.bin", "ec.config" /*out*/, false, false /*err*/, true, fullVersion},
		{ /*in*/ true, false, false, false, "", "image.bin", "ec.bin", "npcx_monitor.bin", "ec.config" /*out*/, true, false /*err*/, false, fullVersion},
		{ /*in*/ false, true, false, false, "", "image.bin", "ec.bin", "npcx_monitor.bin", "ec.config" /*out*/, false, true /*err*/, false, roOnlyVersion},
		{ /*in*/ false, false, true, false, "", "image.bin", "ec.bin", "npcx_monitor.bin", "ec.config" /*out*/, false, true /*err*/, false, roOnlyVersion},
		{ /*in*/ false, true, true, false, "", "image.bin", "ec.bin", "npcx_monitor.bin", "ec.config" /*out*/, false, true /*err*/, false, roOnlyVersion},
		{ /*in*/ true, true, true, false, "", "image.bin", "ec.bin", "npcx_monitor.bin", "ec.config" /*out*/, true, true /*err*/, false, fullVersion},
		{ /*in*/ true, true, false, false, "", "image.bin", "ec.bin", "npcx_monitor.bin", "ec.config" /*out*/, true, true /*err*/, false, fullVersion},
		{ /*in*/ true, true, true, false, `{
			"chromeos": {
				"configs": [{
					"firmware": {
						"build-targets": {
							"coreboot": "fromcoreboot",
							"ec": "fromec",
							"zephyr-ec": "fromzephyr"
						},
						"image-name": "fromimagename"
					},
					"name": "test_model"
				}]
			}
		}`, "image-fromcoreboot.bin", "fromcoreboot/ec.bin", "fromcoreboot/npcx_monitor.bin", "fromcoreboot/ec.config" /*out*/, true, true /*err*/, false, fullVersion},
	}

	log, _ := cli.SetUpLog(cli.DefaultLogDirectory)

	for _, testCase := range testCases {
		// Set up the mock.
		ctrl := gomock.NewController(t)
		dsc := mock_api.NewMockDutServiceClient(ctrl)

		// Create FirmwareService.
		ctx := context.Background()
		req := makeRequest(testCase.mainRw, testCase.mainRo, testCase.ecRo, testCase.ecRw)
		log.Printf("  Test Case: %#v\n  Request: {%s}", testCase, req.String())
		cacheServer := url.URL{
			Scheme: "http",
			Host:   "1.2.3.4:5678",
		}
		fws, err := firmwareservice.NewFirmwareService(
			ctx,
			dsc,
			nil,
			cacheServer,
			"test_board",
			"test_model",
			false,
			req, nil,
		)
		// Check if init error is expected/got.
		if err != nil {
			if testCase.expectConstructorError {
				continue
			}
			t.Fatalf("failed to create FirmwareService with test case %#v: %v", testCase, err)
		}
		if err == nil && testCase.expectConstructorError {
			t.Fatalf("expected constructor error for test case %#v. got: %v", testCase, err)
		}
		// Check expected states.
		if testCase.updateRo != fws.UpdateRo() {
			t.Fatalf("test case %#v expects updateRo to be %v. got: %v.", testCase, testCase.updateRo, fws.UpdateRo())
		}
		if testCase.updateRw != fws.UpdateRw() {
			t.Fatalf("test case %#v expects updateRw to be %v. got: %v.", testCase, testCase.updateRw, fws.UpdateRw())
		}

		// Start with the first state of the service.
		st := state_machine.NewFirmwarePrepareState(fws)
		// Confirm state name is Prepare.
		checkStateName(st, state_machine.PrepareStateName)

		// Set mock expectations.
		if testCase.configYAML == "" {
			gomock.InOrder(
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "true",
				}}).MinTimes(1).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "crosid",
				}}).MinTimes(1).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("CONFIG_INDEX='0'")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Args: []string{"[", "-e", "/usr/share/chromeos-config/yaml/config.yaml", "]", "&&", "echo", "-n", "1", "||", "echo", "-n", "0"},
				}}).MinTimes(1).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("0")}), nil),
			)
		} else {
			td := t.TempDir()
			t.Logf("Writing %s", path.Join(td, "config.yaml"))
			if err := os.WriteFile(path.Join(td, "config.yaml"), []byte(testCase.configYAML), 0666); err != nil {
				t.Fatal("Failed to write config.yaml: ", err)
			}
			if out, err := exec.Command("tar", "-c", "--mode=a+rw", "--gzip", "-C", td, "-f", path.Join(td, "config.yaml.tar.gz"), "config.yaml").CombinedOutput(); err != nil {
				t.Fatalf("Failed to tar -config.yaml: %v\n%s", err, string(out))
			}
			yamlTar, err := os.ReadFile(path.Join(td, "config.yaml.tar.gz"))
			if err != nil {
				t.Fatal("Failed to read config.yaml.tar.gz: ", err)
			}
			gomock.InOrder(
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "true",
				}}).MinTimes(1).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "crosid",
				}}).MinTimes(1).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("CONFIG_INDEX='0'")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Args: []string{"[", "-e", "/usr/share/chromeos-config/yaml/config.yaml", "]", "&&", "echo", "-n", "1", "||", "echo", "-n", "0"},
				}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("1")}), nil),
				dsc.EXPECT().FetchFile(gomock.Any(), &api.FetchFileRequest{
					File: "/usr/share/chromeos-config/yaml/config.yaml",
				}).Return(newFileResponse(&api.File{File: yamlTar}), nil),
			)
		}
		stagingURL := "'http://1.2.3.4:5678/stage/?archive_url=gs%3A%2Fchromeos-image-archive%2Fboard-firmware-branch%2FR123-12345.0.0%2Fboard&files=firmware_from_source.tar.bz2'"
		mkdirCall := dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
			Command: "mktemp",
			Args:    []string{"-d", "--tmpdir=/var/tmp", "'cros-fw-provision.XXXXXXXXX.board'"},
		}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("/var/tmp/some TempDir\n")}), nil)

		if testCase.ecRo {
			dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "ectool",
				Args:    []string{"flashprotect"},
			}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 0}, Stdout: []byte("Flash protect flags: 0x0\n")}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "mkdir",
				Args:    []string{"-p", "'/var/tmp/some TempDir'"},
			}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "test",
				Args:    []string{"-f", "'/var/tmp/some TempDir/ec.bin'"},
			}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}}), nil)
			stagingCall := dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "curl",
				Args:    []string{"-f", "-S", stagingURL},
			}}).MinTimes(1).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/ec.bin'", gomock.Eq("test_board/ec.bin"), log)).
				After(stagingCall).After(mkdirCall).MaxTimes(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}, Stderr: []byte("The requested URL returned error: 404\n")}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/ec.bin'", gomock.Eq("test_model/ec.bin"), log)).
				After(stagingCall).After(mkdirCall).MaxTimes(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}, Stderr: []byte("The requested URL returned error: 404\n")}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/ec.bin'", gomock.Eq(testCase.ecImageWithinArchive), log)).
				After(stagingCall).After(mkdirCall).Times(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/npcx_monitor.bin'", gomock.Eq(testCase.npcxImageWithinArchive), log)).
				After(stagingCall).After(mkdirCall).Times(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/ec.config'", gomock.Eq(testCase.ecConfigWithinArchive), log)).
				After(stagingCall).After(mkdirCall).Times(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
		}
		if testCase.mainRo {
			dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "mkdir",
				Args:    []string{"-p", "'/var/tmp/some TempDir'"},
			}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "which",
				Args:    []string{"ifdtool"},
			}}).MinTimes(1).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}}), nil)
			stagingCall := dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "curl",
				Args:    []string{"-f", "-S", stagingURL},
			}}).MinTimes(1).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "test",
				Args:    []string{"-f", "'/var/tmp/some TempDir/bios.bin'"},
			}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/bios.bin'", gomock.Eq("image-test_model.bin"), log)).
				After(stagingCall).After(mkdirCall).MaxTimes(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}, Stderr: []byte("The requested URL returned error: 404\n")}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/bios.bin'", gomock.Eq("image-test_board.bin"), log)).
				After(stagingCall).After(mkdirCall).MaxTimes(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}, Stderr: []byte("The requested URL returned error: 404\n")}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/bios.bin'", gomock.Eq(testCase.apImageWithinArchive), log)).
				After(stagingCall).After(mkdirCall).Times(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
		}
		if testCase.mainRw {
			dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "mkdir",
				Args:    []string{"-p", "'/var/tmp/some TempDir'"},
			}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
			stagingCall := dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "curl",
				Args:    []string{"-f", "-S", stagingURL},
			}}).MinTimes(1).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "test",
				Args:    []string{"-f", "'/var/tmp/some TempDir/bios.bin'"},
			}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/bios.bin'", gomock.Eq("image-test_model.bin"), log)).
				After(stagingCall).After(mkdirCall).MaxTimes(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}, Stderr: []byte("The requested URL returned error: 404\n")}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/bios.bin'", gomock.Eq("image-test_board.bin"), log)).
				After(stagingCall).After(mkdirCall).MaxTimes(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}, Stderr: []byte("The requested URL returned error: 404\n")}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/bios.bin'", gomock.Eq(testCase.apImageWithinArchive), log)).
				After(stagingCall).After(mkdirCall).Times(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
		}
		if testCase.ecRw {
			dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "mkdir",
				Args:    []string{"-p", "'/var/tmp/some TempDir'"},
			}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
			stagingCall := dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "curl",
				Args:    []string{"-f", "-S", stagingURL},
			}}).MinTimes(1).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/ec.bin'", gomock.Eq("test_board/ec.bin"), log)).
				After(stagingCall).After(mkdirCall).MaxTimes(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}, Stderr: []byte("The requested URL returned error: 404\n")}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/ec.bin'", gomock.Eq("test_model/ec.bin"), log)).
				After(stagingCall).After(mkdirCall).MaxTimes(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}, Stderr: []byte("The requested URL returned error: 404\n")}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/ec.bin'", gomock.Eq(testCase.ecImageWithinArchive), log)).
				After(stagingCall).After(mkdirCall).Times(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/npcx_monitor.bin'", gomock.Eq(testCase.npcxImageWithinArchive), log)).
				After(stagingCall).After(mkdirCall).Times(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
			dsc.EXPECT().ExecCommand(gomock.Any(), newCurlMatcher("'/var/tmp/some TempDir/ec.config'", gomock.Eq(testCase.ecConfigWithinArchive), log)).
				After(stagingCall).After(mkdirCall).Times(1).
				Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil)
		}

		unexpected := dsc.EXPECT().ExecCommand(gomock.Any(), gomock.Any()).AnyTimes().Do(func(ctx context.Context, in *api.ExecCommandRequest, opts ...grpc.CallOption) interface{} {
			t.Fatalf("Unexpected ExecCommand: %s", in.String())
			return newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{Status: 1}, Stderr: []byte("Unmocked call\n"), Stdout: []byte("Unmocked call\n")})
		})

		// Execute the state and proceed.
		_, _, err = st.Execute(ctx, log)
		if err != nil {
			t.Fatal(err)
		}
		st = st.Next()

		unexpected.Times(0)

		if testCase.updateRo {
			// Confirm state name is RO.
			checkStateName(st, state_machine.UpdateRoStateName)

			// Set mock expectations.
			expectedFutilityArgs := []string{"update", "--mode=recovery"}
			expectedFutilityPyArgs := "'update', '--mode=recovery'"
			expectedFutilityImageArgs := []string{}
			if testCase.ecRo {
				expectedFutilityImageArgs = append(expectedFutilityImageArgs, "'--ec_image=/var/tmp/some TempDir/ec.bin'")
				expectedFutilityPyArgs += ", '--ec_image=/var/tmp/some TempDir/ec.bin'"
			}
			if testCase.mainRo {
				expectedFutilityImageArgs = append(expectedFutilityImageArgs, "'--image=/var/tmp/some TempDir/bios.bin'")
				expectedFutilityPyArgs += ", '--image=/var/tmp/some TempDir/bios.bin'"
			}
			expectedFutilityArgs = append(expectedFutilityArgs, expectedFutilityImageArgs...)
			expectedFutilityArgs = append(expectedFutilityArgs, "--wp=0")
			expectedFutilityPyArgs += ", '--wp=0'"
			expectedFutilityManifestArgs := append([]string{"update", "--manifest"}, expectedFutilityImageArgs...)
			gomock.InOrder(
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "futility",
					Args:    expectedFutilityManifestArgs,
				}}).Times(1).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte(`{
					"default": {
						"host": {
							"versions": { "ro": "roversion", "rw": "roversioninrw" }
						},
						"ec": {
							"versions": { "ro": "ecroversion", "rw": "ecroversioninrw" }
						}
					}
				}`)}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "crossystem",
					Args:    []string{"ro_fwid", "fwid"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("old_roversion old_rwversion")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "ectool",
					Args:    []string{"version"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("RO version: oldecroversion\nRW version: oldecrwversion\n")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "ectool",
					Args:    []string{"echash"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("status:  done\ntype:    SHA-256\noffset:  0x00040000\nsize:    0x00033b80\nhash:    4873526f4ff3af9062687c30911309ee12a5ceec235519a8baf6cdbe6361ba92\n")}), nil),
			)
			if testCase.mainRo {
				gomock.InOrder(
					dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
						Command: "cbfstool",
						Args:    []string{"'/var/tmp/some TempDir/bios.bin'", "extract", "-r", "FW_MAIN_A", "-n", "ecrw.hash", "-f", "'/var/tmp/some TempDir/bios.bin-ecrw.hash'"},
					}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil),
					dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
						Command: "od",
						Args:    []string{"-A", "n", "-t", "x1", "'/var/tmp/some TempDir/bios.bin-ecrw.hash'"},
					}}).Return(newResponse(&api.ExecCommandResponse{
						ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte(" 38 73 52 6f 4f f3 af 90 62 68 7c 30 91 13 09 ee\n 12 a5 ce ec 23 55 19 a8 ba f6 cd be 63 61 ba 92\n")}), nil),
				)
			}
			if testCase.ecRo {
				gomock.InOrder(
					dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
						Command: "cat",
						Args:    []string{">", "'/var/tmp/some TempDir/futility.py'"},
						Stdin: []byte(fmt.Sprintf("#!/usr/bin/env python3\n\nimport subprocess\n\n"+
							"with open(\"/var/tmp/some TempDir/futility.log\", \"wb\", buffering=0) as outFile:\n"+
							"  rc = subprocess.run([\"futility\", %s], stdout=outFile, stderr=outFile, check=False, bufsize=0)\n"+
							"  outFile.write(f\"EXIT CODE: {rc.returncode}\\n\".encode(\"utf-8\"))\n  outFile.flush()\n"+
							"  subprocess.run([\"sync\", \"-d\", \"/var/tmp\", \"/usr/local/tmp\"])\n  subprocess.run([\"reboot\"])\n", expectedFutilityPyArgs)),
					}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil),
					dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
						Command: "bash",
						Args:    []string{"-c", "'nohup python3 '\"'\"'/var/tmp/some TempDir/futility.py'\"'\"' </dev/null >&'\"'\"'/var/tmp/some TempDir/futility.start'\"'\"' & exit'"},
					}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil),
					dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
						Command: "cat",
						Args:    []string{"'/var/tmp/some TempDir/futility.log'"},
					}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("EXIT CODE: 0\n")}), nil),
					dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
						Command: "rm",
						Args:    []string{"-f", "'/var/tmp/some TempDir/futility.log'"},
					}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil),
				)
			} else {
				gomock.InOrder(
					dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
						Command: "futility",
						Args:    expectedFutilityArgs,
					}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil),
				)
			}

			// Execute the state and proceed.
			_, _, err := st.Execute(ctx, log)
			if err != nil {
				t.Fatal(err)
			}
			st = st.Next()
		}

		if testCase.updateRw {
			// Confirm state name is RW.
			checkStateName(st, state_machine.UpdateRwStateName)

			// Set mock expectations.
			expectedFutilityArgs := []string{"update", "--mode=recovery", "'--image=/var/tmp/some TempDir/bios.bin'", "--wp=1"}
			expectedFutilityManifestArgs := []string{"update", "--manifest", "'--image=/var/tmp/some TempDir/bios.bin'"}
			gomock.InOrder(
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "futility",
					Args:    expectedFutilityManifestArgs,
				}}).Times(1).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte(`{
					"default": {
						"host": {
							"versions": { "ro": "wrongro", "rw": "rwversion" }
						}
					}
				}`)}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "cbfstool",
					Args:    []string{"'/var/tmp/some TempDir/bios.bin'", "extract", "-r", "FW_MAIN_A", "-n", "ecrw.hash", "-f", "'/var/tmp/some TempDir/bios.bin-ecrw.hash'"},
				}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "od",
					Args:    []string{"-A", "n", "-t", "x1", "'/var/tmp/some TempDir/bios.bin-ecrw.hash'"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte(" 38 73 52 6f 4f f3 af 90 62 68 7c 30 91 13 09 ee\n 12 a5 ce ec 23 55 19 a8 ba f6 cd be 63 61 ba 92\n")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "crossystem",
					Args:    []string{"ro_fwid", "fwid"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("old_roversion old_rwversion")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "ectool",
					Args:    []string{"version"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("RO version: ecroversion\nRW version: oldecrwversion\n")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "ectool",
					Args:    []string{"echash"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("status:  done\ntype:    SHA-256\noffset:  0x00040000\nsize:    0x00033b80\nhash:    4873526f4ff3af9062687c30911309ee12a5ceec235519a8baf6cdbe6361ba92\n")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "futility",
					Args:    expectedFutilityArgs,
				}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil),
			)

			/* OD output:
			    38 73 52 6f 4f f3 af 90 62 68 7c 30 91 13 09 ee
			    12 a5 ce ec 23 55 19 a8 ba f6 cd be 63 61 ba 92

			    ectool output:
			   status:  done
			   type:    SHA-256
			   offset:  0x00040000
			   size:    0x00033b80
			   hash:    3873526f4ff3af9062687c30911309ee12a5ceec235519a8baf6cdbe6361ba92
			*/

			// Execute the state and proceed.
			_, _, err := st.Execute(ctx, log)
			if err != nil {
				t.Fatal(err)
			}
			st = st.Next()
		}

		// Confirm state name is postinstall.
		checkStateName(st, state_machine.PostInstallStateName)
		// Set mock expectations.
		gomock.InOrder(
			dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
				Command: "rm",
				Args:    []string{"-rf", "'/var/tmp/some TempDir'"},
			}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil),
		)
		if !testCase.ecRo || testCase.updateRw {
			gomock.InOrder(
				dsc.EXPECT().Restart(gomock.Any(), gomock.Any()).Return(&longrunning.Operation{Done: true}, nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "true",
				}}).Return(nil, fmt.Errorf("ssh timeout")),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "true",
				}}).Return(newResponse(&api.ExecCommandResponse{ExitInfo: &api.ExecCommandResponse_ExitInfo{}}), nil),
			)
		}
		if testCase.updateRo && testCase.updateRw {
			gomock.InOrder(
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "crossystem",
					Args:    []string{"ro_fwid", "fwid"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("roversion rwversion")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "ectool",
					Args:    []string{"version"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("RO version: ecroversion\nRW version: ecrwversion\n")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "ectool",
					Args:    []string{"echash"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("status:  done\ntype:    SHA-256\noffset:  0x00040000\nsize:    0x00033b80\nhash:    3873526f4ff3af9062687c30911309ee12a5ceec235519a8baf6cdbe6361ba92\n")}), nil),
			)
		} else if testCase.updateRo {
			gomock.InOrder(
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "crossystem",
					Args:    []string{"ro_fwid", "fwid"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("roversion roversioninrw")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "ectool",
					Args:    []string{"version"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("RO version: ecroversion\nRW version: ecroversioninrw\n")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "ectool",
					Args:    []string{"echash"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("status:  done\ntype:    SHA-256\noffset:  0x00040000\nsize:    0x00033b80\nhash:    3873526f4ff3af9062687c30911309ee12a5ceec235519a8baf6cdbe6361ba92\n")}), nil),
			)
		} else if testCase.updateRw {
			gomock.InOrder(
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "crossystem",
					Args:    []string{"ro_fwid", "fwid"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("roversion rwversion")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "ectool",
					Args:    []string{"version"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("RO version: ecroversion\nRW version: ecrwversion\n")}), nil),
				dsc.EXPECT().ExecCommand(gomock.Any(), &rpcMsg{msg: &api.ExecCommandRequest{
					Command: "ectool",
					Args:    []string{"echash"},
				}}).Return(newResponse(&api.ExecCommandResponse{
					ExitInfo: &api.ExecCommandResponse_ExitInfo{}, Stdout: []byte("status:  done\ntype:    SHA-256\noffset:  0x00040000\nsize:    0x00033b80\nhash:    3873526f4ff3af9062687c30911309ee12a5ceec235519a8baf6cdbe6361ba92\n")}), nil),
			)
		}
		// Execute the state and proceed.
		response, _, err := st.Execute(ctx, log)
		if err != nil {
			t.Fatal(err)
		}
		st = st.Next()

		// Confirm no states left.
		checkStateName(st, "")

		// Check for missing mock calls.
		ctrl.Finish()

		if response == nil {
			response = fws.GetVersions()
		}
		if !proto.Equal(response, testCase.expectedResponse) {
			t.Fatalf("Expected response, got %s, want %s", response.String(), testCase.expectedResponse.String())
		}
	}
}
