// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package driver

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gopkg.in/yaml.v2"

	"go.chromium.org/chromiumos/config/go/test/api"

	"infra/cros/cmd/cft/execution/cros-test/internal/device"
)

// TestNewTastArgs makes sure newTastArgs creates the correct arguments for tast.
func TestNewTastArgs(t *testing.T) {
	primary := &device.DutInfo{Addr: dut1, Role: ""}
	reportsServerValue := ":5555"
	varsFileValue := "/tmp/test.yaml"
	buildArtifactsURLValue := "gs://chromeos-image-archive/corsola-release/R119-15611.0.0/"
	dutLabConfigFile := "dutlabconfig.jsonpb"
	varName := "vn"
	varValue := "vv"

	expectedArgs := runArgs{
		primary:  primary,
		patterns: []string{test1, test2, test3, test4, test5},
		tastFlags: map[string]string{
			verboseFlag: "true",
			logTimeFlag: "false",
		},
		runFlags: []flagValue{
			{sshRetriesFlag, "2"},
			{buildFlag, "false"},
			{downloadPrivateBundlesFlag, "true"},
			{resultsDirFlag, workDir1},
			{reportsServerFlag, reportsServerValue},
			{varsFileFlag, varsFileValue},
			{systemServicesTimeoutFlag, defaultSysServicesTimeout},
			{dutLabConfigFlag, dutLabConfigFile},
			{buildArtifactsURLFlag, buildArtifactsURLValue},
			{varFlag, fmt.Sprintf("%s=%s", varName, varValue)},
			{testRetriesFlag, "1"},
		},
	}

	args := newTastArgs(
		primary, nil, nil, expectedArgs.patterns,
		workDir1, reportsServerValue,
		varsFileValue,
		[]*api.Arg{
			{Flag: buildArtifactsURLFlag, Value: buildArtifactsURLValue},
		},
		[]*api.Arg{
			{Flag: varName, Value: varValue},
		},
		dutLabConfigFile,
	)
	if diff := cmp.Diff(args, &expectedArgs,
		cmp.AllowUnexported(runArgs{}, flagValue{}),
		cmpopts.IgnoreUnexported(api.Arg{}),
		cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Got unexpected argument from newTastArgs (-got +want):\n%s\n%v\n--\n%v\n", diff, args, expectedArgs)
	}
}

// TestNewTastArgsRepeatsAndRetries makes sure newTestArgs omit retries flag when
// the repeats flag is specified by the user.
func TestNewTastArgsRepeatsAndRetries(t *testing.T) {
	primary := &device.DutInfo{Addr: dut1, Role: ""}
	reportsServerValue := ":127.0.0.1:3333"
	varsFileValue := "/tmp/test.yaml"
	dutLabConfigFile := "dutlabconfig.jsonpb"
	repeatsCount := "2"

	expectedArgs := runArgs{
		primary:  primary,
		patterns: []string{test1, test2, test3, test4, test5},
		tastFlags: map[string]string{
			verboseFlag: "true",
			logTimeFlag: "false",
		},
		runFlags: []flagValue{
			{sshRetriesFlag, "2"},
			{buildFlag, "false"},
			{downloadPrivateBundlesFlag, "true"},
			{resultsDirFlag, workDir1},
			{reportsServerFlag, reportsServerValue},
			{varsFileFlag, varsFileValue},
			{systemServicesTimeoutFlag, defaultSysServicesTimeout},
			{dutLabConfigFlag, dutLabConfigFile},
			{testRepeatsFlag, repeatsCount},
		},
	}

	args := newTastArgs(
		primary, nil, nil, expectedArgs.patterns,
		workDir1, reportsServerValue,
		varsFileValue,
		[]*api.Arg{
			{Flag: testRepeatsFlag, Value: repeatsCount},
		},
		[]*api.Arg{},
		dutLabConfigFile,
	)

	if diff := cmp.Diff(args, &expectedArgs, cmp.AllowUnexported(runArgs{}, flagValue{}), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Got unexpected argument from newTastArgs (-got +want):\n%s", diff)
	}
}

// TestNewTastArgsCompanions makes sure newTastArgs creates the correct arguments for tast with companion DUTs.
func TestNewTastArgsCompanions(t *testing.T) {
	primary := &device.DutInfo{Addr: dut1, Role: ""}
	reportsServerValue := ":127.0.0.1:3333"
	varsFileValue := "/tmp/test.yaml"
	buildArtifactsURLValue := "gs://chromeos-image-archive/corsola-release/R119-15611.0.0/"
	companions := []*device.DutInfo{
		{
			Addr: "companion_dut1_address",
			Role: "cd1",
		},
		{
			Addr: "companion_dut2_address",
			Role: "cd2",
		},
	}
	androids := []*device.AndroidInfo{
		{
			AssoicateAddr: "host1",
			Serial:        "ABC123",
			ModelName:     "pixel7",
		},
	}
	expectedArgs := runArgs{
		primary:  primary,
		patterns: []string{test1, test2, test3, test4, test5},
		tastFlags: map[string]string{
			verboseFlag: "true",
			logTimeFlag: "false",
		},
		runFlags: []flagValue{
			{sshRetriesFlag, "2"},
			{buildFlag, "false"},
			{downloadPrivateBundlesFlag, "true"},
			{resultsDirFlag, workDir1},
			{reportsServerFlag, reportsServerValue},
			{varsFileFlag, varsFileValue},
			{systemServicesTimeoutFlag, defaultSysServicesTimeout},
			{buildArtifactsURLFlag, buildArtifactsURLValue},
			{testRetriesFlag, "1"},
		},
		companions: companions,
		androids:   androids,
	}

	args := newTastArgs(
		primary, companions, androids, expectedArgs.patterns, workDir1,
		reportsServerValue, varsFileValue,
		[]*api.Arg{
			{Flag: buildArtifactsURLFlag, Value: buildArtifactsURLValue},
		},
		nil,
		"",
	)
	if diff := cmp.Diff(args, &expectedArgs, cmp.AllowUnexported(runArgs{}, flagValue{})); diff != "" {
		t.Errorf("Got unexpected argument from newTastArgs (-got +want):\n%s", diff)
	}
}

// TestGenArgList makes sure genArgList generates the correct list of argument for tast.
func TestGenArgList(t *testing.T) {
	primary := &device.DutInfo{Addr: dut1, Role: ""}
	reportsServerValue := ":5555"
	varsFileValue := "/tmp/test.yaml"
	buildArtifactsURLValue := "gs://chromeos-image-archive/corsola-release/R119-15611.0.0/"
	companions := []*device.DutInfo{
		{
			Addr: "companion_dut1_address",
			Role: "cd1",
		},
		{
			Addr: "companion_dut2_address",
			Role: "cd2",
		},
	}
	androids := []*device.AndroidInfo{
		{
			AssoicateAddr: "host1",
			Serial:        "ABC123",
			ModelName:     "pixel6",
		},
		{
			AssoicateAddr: "host1",
			Serial:        "DEF456",
			ModelName:     "pixel7",
		},
	}
	args := runArgs{
		primary:  primary,
		patterns: []string{test1, test2},
		tastFlags: map[string]string{
			verboseFlag: "true",
			logTimeFlag: "false",
		},
		runFlags: []flagValue{
			{sshRetriesFlag, "2"},
			{buildFlag, "false"},
			{downloadPrivateBundlesFlag, "true"},
			{resultsDirFlag, workDir1},
			{reportsServerFlag, reportsServerValue},
			{varsFileFlag, varsFileValue},
			{systemServicesTimeoutFlag, defaultSysServicesTimeout},
			{buildArtifactsURLFlag, buildArtifactsURLValue},
			{testRetriesFlag, "1"},
		},
		companions: companions,
		androids:   androids,
	}

	var expectedArgList []string
	for key, value := range args.tastFlags {
		expectedArgList = append(expectedArgList, fmt.Sprintf("%v=%v", key, value))
	}
	runIndex := len(expectedArgList)
	expectedArgList = append(expectedArgList, "run")
	for _, kv := range args.runFlags {
		expectedArgList = append(expectedArgList, fmt.Sprintf("%v=%v", kv.flag, kv.value))
	}
	for i, c := range companions {
		expectedArgList = append(expectedArgList, fmt.Sprintf("%v=cd%v:%v", companionDUTFlag, i+1, c.Addr))
	}
	var androidsInfo []string
	for _, a := range androids {
		androidsInfo = append(androidsInfo,
			fmt.Sprintf("%s:%s:%s", a.AssoicateAddr, a.Serial, a.ModelName))
	}
	if len(androidsInfo) > 0 {
		expectedArgList = append(expectedArgList,
			fmt.Sprintf("-var=android.companions=%s", strings.Join(androidsInfo, ",")))
	}

	dutIndex := len(expectedArgList)
	expectedArgList = append(expectedArgList, dut1)
	expectedArgList = append(expectedArgList, test1)
	expectedArgList = append(expectedArgList, test2)

	argList := genArgList(&args)

	// Sort both lists so that we can compare them.
	sort.Strings(expectedArgList[0:runIndex])
	sort.Strings(argList[0:runIndex])
	sort.Strings(expectedArgList[runIndex+1 : dutIndex])
	sort.Strings(argList[runIndex+1 : dutIndex])

	if diff := cmp.Diff(argList, expectedArgList, cmp.AllowUnexported(runArgs{})); diff != "" {
		t.Errorf("Got unexpected argument from genArgList (-got %v +want %v):\n%s", argList, expectedArgList, diff)
	}
}

// TestGenArgListWithServers makes sure genArgList generates the correct list of argument for tast
// when various servers are defined.
func TestGenArgListWithServers(t *testing.T) {
	primary := &device.DutInfo{
		Addr:            dut1,
		Role:            "",
		Servo:           "servo0",
		DutServer:       "dutserver0",
		LibsServer:      "libsserver0",
		ProvisionServer: "provisionserver0",
	}
	reportsServerValue := ":127.0.0.1:3333"
	companions := []*device.DutInfo{
		{
			Addr:            "companion_dut1_address",
			Role:            "cd1",
			Servo:           "servo1",
			DutServer:       "dutserver1",
			ProvisionServer: "provisionserver1",
		},
		{
			Addr:            "companion_dut2_address",
			Role:            "cd2",
			Servo:           "servo2",
			DutServer:       "dutserver2",
			ProvisionServer: "provisionserver2",
		},
	}
	args := runArgs{
		primary:  primary,
		patterns: []string{test1, test2},
		tastFlags: map[string]string{
			verboseFlag: "true",
			logTimeFlag: "false",
		},
		runFlags: []flagValue{
			{sshRetriesFlag, "2"},
			{buildFlag, "false"},
			{downloadPrivateBundlesFlag, "true"},
			{resultsDirFlag, workDir1},
			{reportsServerFlag, reportsServerValue},
			{systemServicesTimeoutFlag, defaultSysServicesTimeout},
			{testRetriesFlag, "1"},
		},
		companions: companions,
	}

	var expectedArgList []string
	for key, value := range args.tastFlags {
		expectedArgList = append(expectedArgList, fmt.Sprintf("%v=%v", key, value))
	}
	runIndex := len(expectedArgList)
	expectedArgList = append(expectedArgList, "run")
	for _, kv := range args.runFlags {
		expectedArgList = append(expectedArgList, fmt.Sprintf("%v=%v", kv.flag, kv.value))
	}
	for i, c := range companions {
		expectedArgList = append(expectedArgList, fmt.Sprintf("%v=cd%v:%v", companionDUTFlag, i+1, c.Addr))
	}
	// Creates servo list.
	servoStr := fmt.Sprintf(":%s", primary.Servo)
	for _, c := range companions {
		servoStr = fmt.Sprintf("%s,%s:%s", servoStr, c.Role, c.Servo)
	}
	expectedArgList = append(expectedArgList, fmt.Sprintf("-var=servers.servo=%v", servoStr))
	expectedArgList = append(expectedArgList, fmt.Sprintf("-var=servo=%v", primary.Servo))

	// Creates DUT server list.
	dutServerStr := fmt.Sprintf(":%s", primary.DutServer)
	for _, c := range companions {
		dutServerStr = fmt.Sprintf("%s,%s:%s", dutServerStr, c.Role, c.DutServer)
	}
	expectedArgList = append(expectedArgList, fmt.Sprintf("-var=servers.dut=%v", dutServerStr))

	// Creates Libsserver.
	libsServerStr := fmt.Sprintf(":%s", primary.LibsServer)
	expectedArgList = append(expectedArgList, fmt.Sprintf("-var=servers.libs=%v", libsServerStr))

	// Creates DUT server list.
	ProvisionServerStr := fmt.Sprintf(":%s", primary.ProvisionServer)
	for _, c := range companions {
		ProvisionServerStr = fmt.Sprintf("%s,%s:%s", ProvisionServerStr, c.Role, c.ProvisionServer)
	}
	expectedArgList = append(expectedArgList, fmt.Sprintf("-var=servers.provision=%v", ProvisionServerStr))

	dutIndex := len(expectedArgList)
	expectedArgList = append(expectedArgList, dut1)
	expectedArgList = append(expectedArgList, test1)
	expectedArgList = append(expectedArgList, test2)

	argList := genArgList(&args)

	// Sort both lists so that we can compare them.
	sort.Strings(expectedArgList[0:runIndex])
	sort.Strings(argList[0:runIndex])
	sort.Strings(expectedArgList[runIndex+1 : dutIndex])
	sort.Strings(argList[runIndex+1 : dutIndex])

	if diff := cmp.Diff(argList, expectedArgList, cmp.AllowUnexported(runArgs{})); diff != "" {
		t.Errorf("Got unexpected argument from genArgList (-got %v +want %v):\n%s", argList, expectedArgList, diff)
	}
}

// TestGenArgListWithDevboard makes sure genArgList generates the correct list of argument for tast
// when various servers are defined.
func TestGenArgListWithDevboard(t *testing.T) {
	primary := &device.DutInfo{
		Addr:           "",
		Role:           "",
		DevboardServer: "devboardserver0",
	}
	reportsServerValue := ":127.0.0.1:3333"
	companions := []*device.DutInfo{
		{
			Addr:           "",
			Role:           "cd1",
			DevboardServer: "devboardserver1",
		},
	}
	args := runArgs{
		primary:  primary,
		patterns: []string{test1, test2},
		tastFlags: map[string]string{
			verboseFlag: "true",
			logTimeFlag: "false",
		},
		runFlags: []flagValue{
			{sshRetriesFlag, "2"},
			{buildFlag, "false"},
			{downloadPrivateBundlesFlag, "true"},
			{resultsDirFlag, workDir1},
			{reportsServerFlag, reportsServerValue},
			{systemServicesTimeoutFlag, defaultSysServicesTimeout},
			{testRetriesFlag, "1"},
		},
		companions: companions,
	}

	var expectedArgList []string
	for key, value := range args.tastFlags {
		expectedArgList = append(expectedArgList, fmt.Sprintf("%v=%v", key, value))
	}
	runIndex := len(expectedArgList)
	expectedArgList = append(expectedArgList, "run")
	for _, kv := range args.runFlags {
		expectedArgList = append(expectedArgList, fmt.Sprintf("%v=%v", kv.flag, kv.value))
	}

	// Creates devboard list.
	devboardStr := fmt.Sprintf(":%s", primary.DevboardServer)
	for _, c := range companions {
		devboardStr = fmt.Sprintf("%s,%s:%s", devboardStr, c.Role, c.DevboardServer)
	}
	expectedArgList = append(expectedArgList, fmt.Sprintf("-var=servers.devboard=%v", devboardStr))

	dutIndex := len(expectedArgList)
	expectedArgList = append(expectedArgList, "-")
	expectedArgList = append(expectedArgList, test1)
	expectedArgList = append(expectedArgList, test2)

	argList := genArgList(&args)

	// Sort both lists so that we can compare them.
	sort.Strings(expectedArgList[0:runIndex])
	sort.Strings(argList[0:runIndex])
	sort.Strings(expectedArgList[runIndex+1 : dutIndex])
	sort.Strings(argList[runIndex+1 : dutIndex])

	if diff := cmp.Diff(argList, expectedArgList, cmp.AllowUnexported(runArgs{})); diff != "" {
		t.Errorf("Got unexpected argument from genArgList (-got %v +want %v):\n%s", argList, expectedArgList, diff)
	}
}

// TestgenHostInfoYAML tests memes.
func TestGenHostInfoYAML(t *testing.T) {
	primary := &device.DutInfo{
		Addr:  "foo",
		Phase: "aphase",
		Sku:   "asku",
		Model: "amodel",
		Board: "aboard",
	}
	expected := make(map[string]string)
	expected["phase"] = primary.Phase
	expected["sku"] = primary.Sku
	expected["model"] = primary.Model
	expected["board"] = primary.Board

	fn, err := genHostInfoYAML(primary)

	if err != nil {
		t.Errorf("Got unexpected err: %v", err)
	}

	fmt.Println(fn)
	defer os.Remove(fn)
	yfile, err := os.ReadFile(fn)

	if err != nil {
		t.Error(err)
	}

	var data Labels
	err = yaml.Unmarshal(yfile, &data)
	if err != nil {
		log.Fatal(err)
	}

	for k, v := range expected {
		testStr := fmt.Sprintf("\"%v:%v\"", k, v)
		if !strings.Contains(data.AutotestHostInfoLabels, testStr) {
			t.Errorf("Missing |%v| in: %v", testStr, data.AutotestHostInfoLabels)
		}
	}
}
