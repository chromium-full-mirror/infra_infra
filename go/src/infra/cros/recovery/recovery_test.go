// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package recovery

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/recovery/config"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/logger"
	"go.chromium.org/infra/cros/recovery/tlw"
	"go.chromium.org/infra/libs/skylab/buildbucket"
)

// Test cases for TestDUTPlans
var dutPlansCases = []struct {
	name         string
	setupType    tlw.DUTSetupType
	taskName     buildbucket.TaskName
	expPlanNames []string
	ok           bool
}{
	{
		"default no task",
		tlw.DUTSetupType_UNSPECIFIED,
		buildbucket.TaskName(""),
		nil,
		false,
	},
	{
		"default recovery",
		tlw.DUTSetupType_UNSPECIFIED,
		buildbucket.Recovery,
		nil,
		false,
	},
	{
		"default MH recovery",
		tlw.DUTSetupType_UNSPECIFIED,
		buildbucket.MHRecovery,
		nil,
		false,
	},
	{
		"default deploy",
		tlw.DUTSetupType_UNSPECIFIED,
		buildbucket.Deploy,
		nil,
		false,
	},
	{
		"default custom",
		tlw.DUTSetupType_UNSPECIFIED,
		buildbucket.Custom,
		nil,
		false,
	},
	{
		"cros no task",
		tlw.DUTSetupType_CROS,
		buildbucket.TaskName(""),
		nil,
		false,
	},
	{
		"cros recovery",
		tlw.DUTSetupType_CROS,
		buildbucket.Recovery,
		[]string{
			config.PlanCrOSBase,
			config.PlanDolos,
			config.PlanServo,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanHMR,
			config.PlanAMT,
			config.PlanClosing},
		true,
	},
	{
		"cros recovery",
		tlw.DUTSetupType_CLANK_ONLY,
		buildbucket.Recovery,
		[]string{
			config.PlanCrOSBase,
			config.PlanDolos,
			config.PlanServo,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanHMR,
			config.PlanAMT,
			config.PlanClosing},
		true,
	},
	{
		"MH recovery",
		tlw.DUTSetupType_CROS,
		buildbucket.MHRecovery,
		[]string{
			config.PlanCrOSBase,
			config.PlanDolos,
			config.PlanServo,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanHMR,
			config.PlanAMT,
			config.PlanClosing},
		true,
	},
	{
		"cros deploy",
		tlw.DUTSetupType_CROS,
		buildbucket.Deploy,
		[]string{
			config.PlanCrOSBase,
			config.PlanServoFwUpdate,
			config.PlanDolos,
			config.PlanServo,
			config.PlanCrOSDeploy,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanHMR,
			config.PlanAMT,
			config.PlanClosing,
		},
		true,
	},
	{
		"cros custom",
		tlw.DUTSetupType_CROS,
		buildbucket.Custom,
		nil,
		false,
	},
	{
		"labstation no task",
		tlw.DUTSetupType_CROS,
		buildbucket.TaskName(""),
		nil,
		false,
	},
	{
		"labstation recovery",
		tlw.DUTSetupType_LABSTATION,
		buildbucket.Recovery,
		[]string{config.PlanCrOS},
		true,
	},
	{
		"labstation deploy",
		tlw.DUTSetupType_LABSTATION,
		buildbucket.Deploy,
		[]string{config.PlanCrOS},
		true,
	},
	{
		"labstation custom",
		tlw.DUTSetupType_LABSTATION,
		buildbucket.Custom,
		nil,
		false,
	},
	{
		"android no task",
		tlw.DUTSetupType_ANDROID,
		buildbucket.TaskName(""),
		nil,
		false,
	},
	{
		"android recovery",
		tlw.DUTSetupType_ANDROID,
		buildbucket.Recovery,
		[]string{config.PlanAndroid, config.PlanClosing},
		true,
	},
	{
		"android deploy",
		tlw.DUTSetupType_ANDROID,
		buildbucket.Deploy,
		[]string{config.PlanAndroid, config.PlanClosing},
		true,
	},
	{
		"android custom",
		tlw.DUTSetupType_ANDROID,
		buildbucket.Custom,
		nil,
		false,
	},
	{
		"android no task",
		tlw.DUTSetupType_ANDROID,
		buildbucket.TaskName(""),
		nil,
		false,
	},
	{
		"chromeos audit RPM",
		tlw.DUTSetupType_CROS,
		buildbucket.AuditRPM,
		[]string{
			config.PlanCrOSBase,
			config.PlanServo,
			config.PlanCrOSAudit,
			config.PlanClosing,
		},
		true,
	},
	{
		"chromeos audit USB-key",
		tlw.DUTSetupType_CROS,
		buildbucket.AuditUSB,
		[]string{
			config.PlanCrOSBase,
			config.PlanServo,
			config.PlanCrOSAudit,
			config.PlanClosing,
		},
		true,
	},
	{
		"chromeos audit storage",
		tlw.DUTSetupType_CROS,
		buildbucket.AuditStorage,
		[]string{
			config.PlanCrOSBase,
			config.PlanCrOSAudit,
		},
		true,
	},
	{
		"labstation does not have audit RPM",
		tlw.DUTSetupType_LABSTATION,
		buildbucket.AuditRPM,
		nil,
		false,
	},
	{
		"android does not have audit RPM",
		tlw.DUTSetupType_ANDROID,
		buildbucket.AuditRPM,
		nil,
		false,
	},
	{
		"cros deep recovery",
		tlw.DUTSetupType_CROS,
		buildbucket.DeepRecovery,
		[]string{
			config.PlanCrOSBase,
			config.PlanDolos,
			config.PlanServoDeepRepair,
			config.PlanCrOSDeepRepair,
			config.PlanServo,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanHMR,
			config.PlanAMT,
			config.PlanClosing,
		},
		true,
	},
	{
		"cros deep recovery",
		tlw.DUTSetupType_LABSTATION,
		buildbucket.DeepRecovery,
		[]string{config.PlanCrOS},
		true,
	},
	{
		"cros browser DUT recovery",
		tlw.DUTSetupType_CROS_BROWSER,
		buildbucket.Recovery,
		[]string{
			config.PlanCrOSBase,
			config.PlanDolos,
			config.PlanServo,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanHMR,
			config.PlanAMT,
			config.PlanClosing,
		},
		true,
	},
	{
		"cros browser DUT deep recovery",
		tlw.DUTSetupType_CROS_BROWSER,
		buildbucket.DeepRecovery,
		[]string{
			config.PlanCrOSBase,
			config.PlanDolos,
			config.PlanServoDeepRepair,
			config.PlanCrOSDeepRepair,
			config.PlanServo,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanHMR,
			config.PlanAMT,
			config.PlanClosing,
		},
		true,
	},
	{
		"cros browser DUT deploy",
		tlw.DUTSetupType_CROS_BROWSER,
		buildbucket.Deploy,
		[]string{
			config.PlanCrOSBase,
			config.PlanServoFwUpdate,
			config.PlanDolos,
			config.PlanServo,
			config.PlanCrOSDeploy,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanHMR,
			config.PlanAMT,
			config.PlanClosing,
		},
		true,
	},
	{
		"cros dry run",
		tlw.DUTSetupType_CROS_BROWSER,
		buildbucket.DryRun,
		nil,
		true,
	},
	{
		"android dry run",
		tlw.DUTSetupType_ANDROID,
		buildbucket.DryRun,
		nil,
		true,
	},
	{
		"labstation dry run",
		tlw.DUTSetupType_LABSTATION,
		buildbucket.DryRun,
		nil,
		true,
	},
	{
		"cros post test",
		tlw.DUTSetupType_CROS,
		buildbucket.PostTest,
		[]string{
			config.PlanCrOSBase,
			config.PlanDolos,
			config.PlanServo,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanHMR,
			config.PlanAMT,
			config.PlanClosing,
		},
		true,
	},
	{
		"cros browser lightweight verifier",
		tlw.DUTSetupType_CROS_BROWSER,
		buildbucket.PostTest,
		nil,
		false,
	},
	{
		"android lightweight verifier",
		tlw.DUTSetupType_ANDROID,
		buildbucket.PostTest,
		nil,
		false,
	},
	{
		"android labstation verifier",
		tlw.DUTSetupType_LABSTATION,
		buildbucket.PostTest,
		nil,
		false,
	},
}

// TestLoadConfiguration tests default configuration used for recovery flow is loading right and parsibale without any issue.
//
// Goals:
//  1. Parsed without any issue
//  2. plan using only existing execs
//  3. configuration contain all required plans in order.
func TestLoadConfiguration(t *testing.T) {
	t.Parallel()
	for _, c := range dutPlansCases {
		cs := c
		t.Run(cs.name, func(t *testing.T) {
			ctx := context.Background()
			args := &RunArgs{}
			if c.taskName != "" {
				args.TaskName = c.taskName
			}
			dut := &tlw.Dut{SetupType: c.setupType}
			got, err := loadConfiguration(ctx, dut, args)
			if cs.ok {
				if err != nil {
					t.Errorf("encountered unexpected error %q in test %q", err, cs.name)
				}
				if !cmp.Equal(got.GetPlanNames(), cs.expPlanNames) {
					t.Errorf("%q ->want: %v\n got: %v: %s", cs.name, cs.expPlanNames, got.GetPlanNames(), err)
				}
				if _, err := config.Validate(ctx, got, execs.Exist); err != nil {
					t.Errorf("%q -> fail to validate configuration with error: %s", cs.name, err)
				}
			} else {
				if err == nil {
					t.Errorf("%q -> expected to finish with error but passed", cs.name)
				}
				if len(got.GetPlanNames()) != 0 {
					t.Errorf("%q -> want: %v\n got: %v", cs.name, cs.expPlanNames, got.GetPlanNames())
				}
			}
		})
	}
}

// TestParsedDefaultConfiguration tests default configurations are loading right and parsibale without any issue.
//
// Goals:
//  1. Parsed without any issue
//  2. plan using only existing execs
//  3. configuration contain all required plans in order.
func TestParsedDefaultConfiguration(t *testing.T) {
	t.Parallel()
	for _, c := range dutPlansCases {
		cs := c
		t.Run(cs.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := ParsedDefaultConfiguration(ctx, c.taskName, c.setupType)
			if cs.ok {
				if !cmp.Equal(got.GetPlanNames(), cs.expPlanNames) {
					t.Errorf("%q ->want: %v\n got: %v", cs.name, cs.expPlanNames, got.GetPlanNames())
				}
			} else {
				if err == nil {
					t.Errorf("%q -> expected to finish with error but passed", cs.name)
				}
				if len(got.GetPlanNames()) != 0 {
					t.Errorf("%q -> want: %v\n got: %v", cs.name, cs.expPlanNames, got.GetPlanNames())
				}
			}
		})
	}
}

func TestRunDUTPlan(t *testing.T) {
	t.Parallel()
	ftt.Run("bad cases", t, func(t *ftt.Test) {
		ctx := context.Background()
		dut := &tlw.Dut{
			Name: "test_dut",
			Chromeos: &tlw.ChromeOS{
				Servo: &tlw.ServoHost{
					Name: "servo_host",
				},
			},
		}
		args := &RunArgs{
			Logger: logger.NewLogger(),
		}
		execArgs := &execs.RunArgs{
			DUT:    dut,
			Logger: args.Logger,
		}
		c := &config.Configuration{}
		t.Run("fail when no plans in config", func(t *ftt.Test) {
			c.Plans = map[string]*config.Plan{
				"something": nil,
			}
			c.PlanNames = []string{"my_plan"}
			err := runDUTPlans(ctx, dut, c, args)
			if err == nil {
				t.Errorf("Expected fail but passed")
			} else {
				assert.Loosely(t, err.Error(), should.ContainSubstring("run dut \"test_dut\" plans:"))
				assert.Loosely(t, err.Error(), should.ContainSubstring("not found in configuration"))
			}
		})
		t.Run("fail when one plan fail of plans fail", func(t *ftt.Test) {
			c.Plans = map[string]*config.Plan{
				config.PlanServo: {
					CriticalActions: []string{"sample_fail"},
					Actions: map[string]*config.Action{
						"sample_fail": {
							ExecName: "sample_fail",
						},
					},
				},
				config.PlanCrOS: {
					CriticalActions: []string{"sample_pass"},
					Actions: map[string]*config.Action{
						"sample_pass": {
							ExecName: "sample_pass",
						},
					},
				},
			}
			c.PlanNames = []string{config.PlanServo, config.PlanCrOS}
			err := runDUTPlans(ctx, dut, c, args)
			if err == nil {
				t.Errorf("Expected fail but passed")
			} else {
				assert.Loosely(t, err.Error(), should.ContainSubstring("run plan \"servo\" for \"servo_host\":"))
				assert.Loosely(t, err.Error(), should.ContainSubstring("failed"))
			}
		})
		t.Run("fail when bad action in the plan", func(t *ftt.Test) {
			plan := &config.Plan{
				CriticalActions: []string{"sample_fail"},
				Actions: map[string]*config.Action{
					"sample_fail": {
						ExecName: "sample_fail",
					},
				},
			}
			err := runDUTPlanPerResource(ctx, "test_dut", config.PlanCrOS, plan, execArgs, nil)
			if err == nil {
				t.Errorf("Expected fail but passed")
			} else {
				assert.Loosely(t, err.Error(), should.ContainSubstring("run plan \"cros\" for \"test_dut\":"))
				assert.Loosely(t, err.Error(), should.ContainSubstring(": failed"))
			}
		})
	})
	ftt.Run("Happy path", t, func(t *ftt.Test) {
		ctx := context.Background()
		dut := &tlw.Dut{
			Name: "test_dut",
			Chromeos: &tlw.ChromeOS{
				Servo: &tlw.ServoHost{
					Name: "servo_host",
				},
			},
		}
		args := &RunArgs{
			Logger: logger.NewLogger(),
		}
		execArgs := &execs.RunArgs{
			DUT: dut,
		}
		t.Run("Run good plan", func(t *ftt.Test) {
			plan := &config.Plan{
				CriticalActions: []string{"sample_pass"},
				Actions: map[string]*config.Action{
					"sample_pass": {
						ExecName: "sample_pass",
					},
				},
			}
			if err := runDUTPlanPerResource(ctx, "DUT3", config.PlanCrOS, plan, execArgs, nil); err != nil {
				t.Errorf("Expected pass but failed: %s", err)
			}
		})
		t.Run("Run all good plans", func(t *ftt.Test) {
			c := &config.Configuration{
				Plans: map[string]*config.Plan{
					config.PlanCrOS: {
						CriticalActions: []string{"sample_pass"},
						Actions: map[string]*config.Action{
							"sample_pass": {
								ExecName: "sample_pass",
							},
						},
					},
					config.PlanServo: {
						CriticalActions: []string{"sample_pass"},
						Actions: map[string]*config.Action{
							"sample_pass": {
								ExecName: "sample_pass",
							},
						},
					},
				},
			}
			if err := runDUTPlans(ctx, dut, c, args); err != nil {
				t.Errorf("Expected pass but failed: %s", err)
			}
		})
		t.Run("Run all plans even one allow to fail", func(t *ftt.Test) {
			c := &config.Configuration{
				Plans: map[string]*config.Plan{
					config.PlanCrOS: {
						CriticalActions: []string{"sample_fail"},
						Actions: map[string]*config.Action{
							"sample_fail": {
								ExecName: "sample_fail",
							},
						},
						AllowFail: true,
					},
					config.PlanServo: {
						CriticalActions: []string{"sample_pass"},
						Actions: map[string]*config.Action{
							"sample_pass": {
								ExecName: "sample_pass",
							},
						},
					},
				},
			}
			if err := runDUTPlans(ctx, dut, c, args); err != nil {
				t.Errorf("Expected pass but failed: %s", err)
			}
		})
		t.Run("Do not fail even if closing plan failed", func(t *ftt.Test) {
			c := &config.Configuration{
				Plans: map[string]*config.Plan{
					config.PlanCrOS: {
						CriticalActions: []string{},
					},
					config.PlanServo: {
						CriticalActions: []string{},
					},
					config.PlanClosing: {
						CriticalActions: []string{"sample_fail"},
						Actions: map[string]*config.Action{
							"sample_fail": {
								ExecName: "sample_fail",
							},
						},
					},
				},
			}
			if err := runDUTPlans(ctx, dut, c, args); err != nil {
				t.Errorf("Expected pass but failed: %s", err)
			}
		})
	})
}

// TestVerify is a smoke test for the verify method.
func TestVerify(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   *RunArgs
		good bool
	}{
		{
			"nil",
			nil,
			false,
		},
		{
			"empty",
			&RunArgs{},
			false,
		},
		{
			"missing tlw client",
			&RunArgs{
				UnitName: "a",
				LogRoot:  "b",
			},
			false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expected := tt.good
			e := tt.in.verify()
			actual := (e == nil)

			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("unexpected diff (-want +got): %s", diff)
			}
		})
	}
}

// Test cases for TestDUTPlans
var customConfigurationTestCases = []struct {
	name      string
	getConfig func() *config.Configuration
}{
	{
		"Reserve DUT",
		func() *config.Configuration {
			return config.ReserveDutConfig()
		},
	},
	{
		"Recover CBI With Contents From Inventory",
		func() *config.Configuration {
			return config.RecoverCBIFromInventoryConfig()
		},
	},
	{
		"Custom dowload image to USB drive",
		func() *config.Configuration {
			return config.DownloadImageToServoUSBDrive("image_path", "image_name")
		},
	},
	{
		"Battery cut-off",
		func() *config.Configuration {
			return config.FixBatteryCutOffConfig()
		},
	},
	{
		"Serial console enable plan",
		func() *config.Configuration {
			return config.EnableSerialConsoleConfig()
		},
	},
	{
		"Update fw targets plan",
		func() *config.Configuration {
			return config.SetFwTargets("ec-target", "ap-target")
		},
	},
	{
		"BTpeer provision plan",
		func() *config.Configuration {
			return config.ProvisionBtpeerConfig("test_url")
		},
	},
}

// TestOtherConfigurations tests other known configurations used anywhere.
//
// Goals:
//  1. Parsed without any issue
//  2. plan using only existing execs
//  3. configuration contain all required plans in order.
func TestOtherConfigurations(t *testing.T) {
	t.Parallel()
	for _, c := range customConfigurationTestCases {
		cs := c
		t.Run(cs.name, func(t *testing.T) {
			ctx := context.Background()
			configuration := cs.getConfig()
			if _, err := config.Validate(ctx, configuration, execs.Exist); err != nil {
				t.Errorf("%q -> fail to validate configuration with error: %s", cs.name, err)
			}
		})
	}
}

// Testing dutPlans method.
func TestGetConfiguration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		in     string
		isNull bool
	}{
		{
			"no Data",
			"",
			true,
		},
		{
			"Some data",
			`{
			"Field":"something",
			"number': 765
		}`,
			false,
		},
		{
			"strange data",
			"!@#$%^&*()__)(*&^%$#retyuihjo{:>\"?{",
			false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a := &RunArgs{}
			b64 := base64.StdEncoding
			buf := make([]byte, b64.EncodedLen(len(c.in)))
			b64.Encode(buf, []byte(c.in))
			err := a.UseConfigBase64(string(buf))
			if err != nil {
				panic(err.Error())
			}
			r := a.configReader

			if err != nil {
				t.Errorf("Case %s: %s", c.name, err)
			}
			if c.isNull {
				if r != nil {
					t.Errorf("Case %s: expected nil", c.name)
				}
			} else {
				got := []byte{}
				err := errors.New("config reader cannot be nil")
				if r != nil {
					got, err = io.ReadAll(r)
				}
				if err != nil {
					t.Errorf("Case %s: %s", c.name, err)
				}
				if !cmp.Equal(string(got), c.in) {
					t.Errorf("got: %v\nwant: %v", string(got), c.in)
				}
			}
		})
	}
}

// TestcollectResourcesForPlan tests collectResourcesForPlan function.
func TestCollectResourcesForPlan(t *testing.T) {
	t.Parallel()
	dut := &tlw.Dut{
		Name: "dut-name",
		Chromeos: &tlw.ChromeOS{
			Servo: &tlw.ServoHost{
				Name: "servo-host",
			},
			BluetoothPeers: []*tlw.BluetoothPeer{
				{Name: "bluetooth-peers1"},
				{Name: "bluetooth-peers2"},
				{Name: "bluetooth-peers3"},
			},
			WifiRouters: []*tlw.WifiRouterHost{
				{Name: "wifi-router1"},
				{Name: "wifi-router2"},
				{Name: "wifi-router3"},
			},
			Chameleon: &tlw.Chameleon{
				Name: "chameleon",
			},
			HumanMotionRobot: &tlw.HumanMotionRobot{
				Name: "hmr",
			},
			Dolos: &tlw.Dolos{
				Hostname: "dolos",
			},
			AmtManager: &tlw.AMTManager{
				Hostname: "amt",
			},
		},
		Android: &tlw.Android{
			AssociatedHostname: "android-name",
		},
	}
	cases := []struct {
		plan string
		out  []string
	}{
		{config.PlanAndroid, []string{"dut-name"}},
		{config.PlanCrOS, []string{"dut-name"}},
		{config.PlanCrOSAudit, []string{"dut-name"}},
		{config.PlanCrOSDeepRepair, []string{"dut-name"}},
		{config.PlanServo, []string{"servo-host"}},
		{config.PlanServoDeepRepair, []string{"servo-host"}},
		{config.PlanChameleon, []string{"chameleon"}},
		{config.PlanBluetoothPeer, []string{"bluetooth-peers1", "bluetooth-peers2", "bluetooth-peers3"}},
		{config.PlanWifiRouter, []string{"wifi-router1", "wifi-router2", "wifi-router3"}},
		{config.PlanHMR, []string{"hmr"}},
		{config.PlanDolos, []string{"dolos"}},
		{config.PlanAMT, []string{"amt"}},
		{config.PlanClosing, []string{"dut-name"}},
		{"cros_bla", []string{"dut-name"}},
		{"servo_bla", []string{"servo-host"}},
		{"", []string{"dut-name"}},
		{"empty", []string{"dut-name"}},
	}

	for _, c := range cases {
		t.Run(c.plan, func(t *testing.T) {
			t.Parallel()
			out := collectResourcesForPlan(c.plan, dut)
			if len(out) == 0 {
				t.Errorf("%s: did not get reources", c.plan)
			}
			if len(out) != len(c.out) {
				t.Errorf("%s: did not get expeceted list of resource %d!= %d", c.plan, len(out), len(c.out))
			}
			for i, v := range c.out {
				if !cmp.Equal(out[i], v) {
					t.Errorf("%s: got:%v  want: %v", c.plan, out[i], v)
				}
			}
		})
	}
}
