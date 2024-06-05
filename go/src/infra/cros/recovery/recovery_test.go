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
	. "github.com/smartystreets/goconvey/convey"

	"infra/cros/recovery/config"
	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/logger"
	"infra/cros/recovery/tlw"
	"infra/libs/skylab/buildbucket"
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
		tlw.DUTSetupTypeUnspecified,
		buildbucket.TaskName(""),
		nil,
		false,
	},
	{
		"default recovery",
		tlw.DUTSetupTypeUnspecified,
		buildbucket.Recovery,
		nil,
		false,
	},
	{
		"default deploy",
		tlw.DUTSetupTypeUnspecified,
		buildbucket.Deploy,
		nil,
		false,
	},
	{
		"default custom",
		tlw.DUTSetupTypeUnspecified,
		buildbucket.Custom,
		nil,
		false,
	},
	{
		"cros no task",
		tlw.DUTSetupTypeCros,
		buildbucket.TaskName(""),
		nil,
		false,
	},
	{
		"cros recovery",
		tlw.DUTSetupTypeCros,
		buildbucket.Recovery,
		[]string{
			config.PlanServo,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanHMR,
			config.PlanDolos,
			config.PlanClosing},
		true,
	},
	{
		"cros deploy",
		tlw.DUTSetupTypeCros,
		buildbucket.Deploy,
		[]string{
			config.PlanServo,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanHMR,
			config.PlanDolos,
			config.PlanClosing},
		true,
	},
	{
		"cros custom",
		tlw.DUTSetupTypeCros,
		buildbucket.Custom,
		nil,
		false,
	},
	{
		"labstation no task",
		tlw.DUTSetupTypeCros,
		buildbucket.TaskName(""),
		nil,
		false,
	},
	{
		"labstation recovery",
		tlw.DUTSetupTypeLabstation,
		buildbucket.Recovery,
		[]string{config.PlanCrOS},
		true,
	},
	{
		"labstation deploy",
		tlw.DUTSetupTypeLabstation,
		buildbucket.Deploy,
		[]string{config.PlanCrOS},
		true,
	},
	{
		"labstation custom",
		tlw.DUTSetupTypeLabstation,
		buildbucket.Custom,
		nil,
		false,
	},
	{
		"android no task",
		tlw.DUTSetupTypeAndroid,
		buildbucket.TaskName(""),
		nil,
		false,
	},
	{
		"android recovery",
		tlw.DUTSetupTypeAndroid,
		buildbucket.Recovery,
		[]string{config.PlanAndroid, config.PlanClosing},
		true,
	},
	{
		"android deploy",
		tlw.DUTSetupTypeAndroid,
		buildbucket.Deploy,
		[]string{config.PlanAndroid, config.PlanClosing},
		true,
	},
	{
		"android custom",
		tlw.DUTSetupTypeAndroid,
		buildbucket.Custom,
		nil,
		false,
	},
	{
		"android no task",
		tlw.DUTSetupTypeAndroid,
		buildbucket.TaskName(""),
		nil,
		false,
	},
	{
		"chromeos audit RPM",
		tlw.DUTSetupTypeCros,
		buildbucket.AuditRPM,
		[]string{config.PlanServo, config.PlanCrOSAudit, config.PlanClosing},
		true,
	},
	{
		"chromeos audit USB-key",
		tlw.DUTSetupTypeCros,
		buildbucket.AuditUSB,
		[]string{config.PlanServo, config.PlanCrOSAudit, config.PlanClosing},
		true,
	},
	{
		"chromeos audit storage",
		tlw.DUTSetupTypeCros,
		buildbucket.AuditStorage,
		[]string{config.PlanCrOSAudit},
		true,
	},
	{
		"labstation does not have audit RPM",
		tlw.DUTSetupTypeLabstation,
		buildbucket.AuditRPM,
		nil,
		false,
	},
	{
		"android does not have audit RPM",
		tlw.DUTSetupTypeAndroid,
		buildbucket.AuditRPM,
		nil,
		false,
	},
	{
		"cros deep recovery",
		tlw.DUTSetupTypeCros,
		buildbucket.DeepRecovery,
		[]string{
			config.PlanServoDeepRepair,
			config.PlanCrOSDeepRepair,
			config.PlanServo,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanHMR,
			config.PlanDolos,
			config.PlanClosing,
		},
		true,
	},
	{
		"cros deep recovery",
		tlw.DUTSetupTypeLabstation,
		buildbucket.DeepRecovery,
		[]string{config.PlanCrOS},
		true,
	},
	{
		"cros browser DUT recovery",
		tlw.DUTSetupTypeCrosBrowser,
		buildbucket.Recovery,
		[]string{config.PlanCrOS},
		true,
	},
	{
		"cros browser DUT deep recovery",
		tlw.DUTSetupTypeCrosBrowser,
		buildbucket.DeepRecovery,
		[]string{config.PlanCrOS},
		true,
	},
	{
		"cros browser DUT deploy",
		tlw.DUTSetupTypeCrosBrowser,
		buildbucket.Deploy,
		[]string{config.PlanCrOS},
		true,
	},
	{
		"cros dry run",
		tlw.DUTSetupTypeCrosBrowser,
		buildbucket.DryRun,
		nil,
		true,
	},
	{
		"android dry run",
		tlw.DUTSetupTypeAndroid,
		buildbucket.DryRun,
		nil,
		true,
	},
	{
		"labstation dry run",
		tlw.DUTSetupTypeLabstation,
		buildbucket.DryRun,
		nil,
		true,
	},
	{
		"cros post test",
		tlw.DUTSetupTypeCros,
		buildbucket.PostTest,
		[]string{
			config.PlanServo,
			config.PlanBluetoothPeer,
			config.PlanWifiRouter,
			config.PlanCrOS,
			config.PlanChameleon,
			config.PlanHMR,
			config.PlanDolos,
			config.PlanClosing,
		},
		true,
	},
	{
		"cros browser lightweight verifier",
		tlw.DUTSetupTypeCrosBrowser,
		buildbucket.PostTest,
		nil,
		false,
	},
	{
		"android lightweight verifier",
		tlw.DUTSetupTypeAndroid,
		buildbucket.PostTest,
		nil,
		false,
	},
	{
		"android labstation verifier",
		tlw.DUTSetupTypeLabstation,
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
	Convey("bad cases", t, func() {
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
		Convey("fail when no plans in config", func() {
			c.Plans = map[string]*config.Plan{
				"something": nil,
			}
			c.PlanNames = []string{"my_plan"}
			err := runDUTPlans(ctx, dut, c, args)
			if err == nil {
				t.Errorf("Expected fail but passed")
			} else {
				So(err.Error(), ShouldContainSubstring, "run dut \"test_dut\" plans:")
				So(err.Error(), ShouldContainSubstring, "not found in configuration")
			}
		})
		Convey("fail when one plan fail of plans fail", func() {
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
				So(err.Error(), ShouldContainSubstring, "run plan \"servo\" for \"servo_host\":")
				So(err.Error(), ShouldContainSubstring, "failed")
			}
		})
		Convey("fail when bad action in the plan", func() {
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
				So(err.Error(), ShouldContainSubstring, "run plan \"cros\" for \"test_dut\":")
				So(err.Error(), ShouldContainSubstring, ": failed")
			}
		})
	})
	Convey("Happy path", t, func() {
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
		Convey("Run good plan", func() {
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
		Convey("Run all good plans", func() {
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
		Convey("Run all plans even one allow to fail", func() {
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
		Convey("Do not fail even if closing plan failed", func() {
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
		tt := tt
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
		c := c
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
