// Copyright 2014 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ninjalog

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

var (
	trueValue = true

	logTestCase = `# ninja log v6
76	187	0	resources/inspector/devtools_extension_api.js	75430546595be7c2
80	284	0	gen/autofill_regex_constants.cc	fa33c8d7ce1d8791
78	286	0	gen/angle/commit_id.py	4ede38e2c1617d8c
79	287	0	gen/angle/copy_compiler_dll.bat	9fb635ad5d2c1109
141	287	0	PepperFlash/manifest.json	324f0a0b77c37ef
142	288	0	PepperFlash/libpepflashplayer.so	1e2c2b7845a4d4fe
287	290	0	obj/third_party/angle/src/copy_scripts.actions_rules_copies.stamp	b211d373de72f455
`

	stepsTestCase = []Step{
		{
			End: 76 * time.Millisecond,
			Out: "ninja startup overhead",
		},
		{
			Start:   76 * time.Millisecond,
			End:     187 * time.Millisecond,
			Out:     "resources/inspector/devtools_extension_api.js",
			CmdHash: "75430546595be7c2",
		},
		{
			Start:   80 * time.Millisecond,
			End:     284 * time.Millisecond,
			Out:     "gen/autofill_regex_constants.cc",
			CmdHash: "fa33c8d7ce1d8791",
		},
		{
			Start:   78 * time.Millisecond,
			End:     286 * time.Millisecond,
			Out:     "gen/angle/commit_id.py",
			CmdHash: "4ede38e2c1617d8c",
		},
		{
			Start:   79 * time.Millisecond,
			End:     287 * time.Millisecond,
			Out:     "gen/angle/copy_compiler_dll.bat",
			CmdHash: "9fb635ad5d2c1109",
		},
		{
			Start:   141 * time.Millisecond,
			End:     287 * time.Millisecond,
			Out:     "PepperFlash/manifest.json",
			CmdHash: "324f0a0b77c37ef",
		},
		{
			Start:   142 * time.Millisecond,
			End:     288 * time.Millisecond,
			Out:     "PepperFlash/libpepflashplayer.so",
			CmdHash: "1e2c2b7845a4d4fe",
		},
		{
			Start:   287 * time.Millisecond,
			End:     290 * time.Millisecond,
			Out:     "obj/third_party/angle/src/copy_scripts.actions_rules_copies.stamp",
			CmdHash: "b211d373de72f455",
		},
	}

	stepsTestCaseLast = append(stepsTestCase, []Step{
		{
			Start:   900 * time.Millisecond,
			End:     1000 * time.Millisecond,
			Out:     "out/Release/obj/chrome/installer/mini_installer/previous_version_mini_installer.stamp",
			CmdHash: "647407ef506035a9",
		},
		{
			Start:   800 * time.Millisecond,
			End:     999 * time.Millisecond,
			Out:     "out/Release/obj/chrome/test/mini_installer/mini_installer_tests.stamp",
			CmdHash: "b40242acfdb206f6",
		},
	}...)

	stepsSorted = []Step{
		{
			End: 76 * time.Millisecond,
			Out: "ninja startup overhead",
		},
		{
			Start:   76 * time.Millisecond,
			End:     187 * time.Millisecond,
			Out:     "resources/inspector/devtools_extension_api.js",
			CmdHash: "75430546595be7c2",
		},
		{
			Start:   78 * time.Millisecond,
			End:     286 * time.Millisecond,
			Out:     "gen/angle/commit_id.py",
			CmdHash: "4ede38e2c1617d8c",
		},
		{
			Start:   79 * time.Millisecond,
			End:     287 * time.Millisecond,
			Out:     "gen/angle/copy_compiler_dll.bat",
			CmdHash: "9fb635ad5d2c1109",
		},
		{
			Start:   80 * time.Millisecond,
			End:     284 * time.Millisecond,
			Out:     "gen/autofill_regex_constants.cc",
			CmdHash: "fa33c8d7ce1d8791",
		},
		{
			Start:   141 * time.Millisecond,
			End:     287 * time.Millisecond,
			Out:     "PepperFlash/manifest.json",
			CmdHash: "324f0a0b77c37ef",
		},
		{
			Start:   142 * time.Millisecond,
			End:     288 * time.Millisecond,
			Out:     "PepperFlash/libpepflashplayer.so",
			CmdHash: "1e2c2b7845a4d4fe",
		},
		{
			Start:   287 * time.Millisecond,
			End:     290 * time.Millisecond,
			Out:     "obj/third_party/angle/src/copy_scripts.actions_rules_copies.stamp",
			CmdHash: "b211d373de72f455",
		},
	}

	metadataTestCase = Metadata{
		User:             "bob@google.com",
		BuildID:          12345,
		InvocationID:     "6dc52b4f-fdf9-4017-b542-8c6cf296677d",
		ExitCode:         1,
		BuildDurationSec: 100,
		BuildConfigs: map[string]string{
			"use_goma":           "true",
			"is_component_build": "true",
			"enable_nacl":        "false",
			"host_cpu":           "\"x64\"",
			"target_os":          "\"\"",
			"target_cpu":         "\"\"",
			"symbol_level":       "-1",
			"is_debug":           "false",
			"host_os":            "\"linux\"",
		},
		ExplicitBuildConfigKeys: []string{"is_debug", "use_goma"},
		Platform:                "Linux",
		Argv:                    []string{"../../../scripts/compile.py", "--target", "Release", "--clobber", "--compiler=goma", "--", "all"},
		Cwd:                     "/b/build/Linux_x64/build/src",
		Compiler:                "goma",
		Exit:                    0,
		StepName:                "compile",
		IsCloudtop:              &trueValue,
		GCEMachineType:          "n2d-standard-9999",
		IsCog:                   &trueValue,
		Env: map[string]string{
			"LANG":    "en_US.UTF-8",
			"SHELL":   "/bin/bash",
			"HOME":    "/home/chrome-bot",
			"PWD":     "/b/build/Linux_x64/build",
			"LOGNAME": "chrome-bot",
			"USER":    "chrome-bot",
			"PATH":    "/home/chrome-bot/bin:/b/depot_tools:/usr/bin:/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin",
		},
		Jobs:    50,
		Targets: []string{"all"},
	}
)

func TestStepsSort(t *testing.T) {
	steps := append([]Step{}, stepsTestCase...)
	sort.Sort(Steps(steps))
	if diff := cmp.Diff(steps, stepsSorted); diff != "" {
		t.Errorf("sort Steps mismatch (-want, +got):\n%s", diff)
	}
}

func TestStepsReverse(t *testing.T) {
	steps := []Step{
		{Out: "0"},
		{Out: "1"},
		{Out: "2"},
		{Out: "3"},
	}
	Steps(steps).Reverse()
	want := []Step{
		{Out: "3"},
		{Out: "2"},
		{Out: "1"},
		{Out: "0"},
	}
	if diff := cmp.Diff(steps, want); diff != "" {
		t.Errorf("steps.Reverse(...) mismatch (-want, +got):\n%s", diff)
	}
}

func TestParseSupportedVersion(t *testing.T) {
	for _, v := range []int{5, 6} {
		_, err := Parse(".ninja_log", strings.NewReader(fmt.Sprintf(`# ninja log v%d
0	1	0	foo	touch foo
`, v)))
		if err != nil {
			t.Errorf("Parse()=_, %v; want=_, <nil>", err)
		}
	}
}

func TestParseOldVersion(t *testing.T) {
	_, err := Parse(".ninja_log", strings.NewReader(`# ninja log v4
0	1	0	foo	touch foo
`))
	if err == nil {
		t.Error("Parse()=_, <nil>; want=_, error")
	}
}

func TestParseInvalidVersion(t *testing.T) {
	_, err := Parse(".ninja_log", strings.NewReader(`# ninja log vXXX
0	1	0	foo	touch foo
`))
	if err == nil {
		t.Error("Parse()=_, <nil>; want=_, error")
	}
}

func TestParseSimple(t *testing.T) {
	njl, err := Parse(".ninja_log", strings.NewReader(logTestCase))
	if err != nil {
		t.Errorf(`Parse()=_, %v; want=_, <nil>`, err)
	}

	want := &NinjaLog{
		Filename: ".ninja_log",
		Start:    1,
		Steps:    stepsTestCase,
	}
	if diff := cmp.Diff(njl, want); diff != "" {
		t.Errorf("Parse(...) mismatch (-want, +got):\n%s", diff)
	}
}

func TestParseEmptyLine(t *testing.T) {
	njl, err := Parse(".ninja_log", strings.NewReader(logTestCase+"\n"))
	if err != nil {
		t.Errorf(`Parse()=_, %v; want=_, <nil>`, err)
	}
	want := &NinjaLog{
		Filename: ".ninja_log",
		Start:    1,
		Steps:    stepsTestCase,
	}
	if diff := cmp.Diff(njl, want); diff != "" {
		t.Errorf("Parse(...) mismatch (-want, +got):\n%s", diff)
	}
}

func TestParseLast(t *testing.T) {
	njl, err := Parse(".ninja_log", strings.NewReader(`# ninja log v5
1020807	1020916	0	chrome.1	e101fd46be020cfc
84	9489	0	gen/libraries.cc	9001f3182fa8210e
1024369	1041522	0	chrome	aee9d497d56c9637
76	187	0	resources/inspector/devtools_extension_api.js	75430546595be7c2
80	284	0	gen/autofill_regex_constants.cc	fa33c8d7ce1d8791
78	286	0	gen/angle/commit_id.py	4ede38e2c1617d8c
79	287	0	gen/angle/copy_compiler_dll.bat	9fb635ad5d2c1109
141	287	0	PepperFlash/manifest.json	324f0a0b77c37ef
142	288	0	PepperFlash/libpepflashplayer.so	1e2c2b7845a4d4fe
287	290	0	obj/third_party/angle/src/copy_scripts.actions_rules_copies.stamp	b211d373de72f455
900	1000	0	out/Release/obj/chrome/installer/mini_installer/previous_version_mini_installer.stamp	647407ef506035a9
800	999	0	out/Release/obj/chrome/test/mini_installer/mini_installer_tests.stamp	b40242acfdb206f6
`))
	if err != nil {
		t.Errorf(`Parse()=_, %v; want=_, <nil>`, err)
	}

	want := &NinjaLog{
		Filename: ".ninja_log",
		Start:    4,
		Steps:    stepsTestCaseLast,
	}
	if diff := cmp.Diff(want, njl); diff != "" {
		t.Errorf("Parse() got diff; (-want +got):\n%s, ", diff)
	}
}

func TestParseWithMetadata(t *testing.T) {
	njl, err := Parse(".ninja_log", strings.NewReader(`# ninja log v5
1020807	1020916	0	chrome.1	e101fd46be020cfc
84	9489	0	gen/libraries.cc	9001f3182fa8210e
1024369	1041522	0	chrome	aee9d497d56c9637
76	187	0	resources/inspector/devtools_extension_api.js	75430546595be7c2
80	284	0	gen/autofill_regex_constants.cc	fa33c8d7ce1d8791
78	286	0	gen/angle/commit_id.py	4ede38e2c1617d8c
79	287	0	gen/angle/copy_compiler_dll.bat	9fb635ad5d2c1109
141	287	0	PepperFlash/manifest.json	324f0a0b77c37ef
142	288	0	PepperFlash/libpepflashplayer.so	1e2c2b7845a4d4fe
287	290	0	obj/third_party/angle/src/copy_scripts.actions_rules_copies.stamp	b211d373de72f455

# end of ninja log
{"exit_code": 1, "build_duration_sec": 100, "build_id": 12345, "invocation_id": "6dc52b4f-fdf9-4017-b542-8c6cf296677d", "user": "bob@google.com", "platform": "Linux", "argv": ["../../../scripts/compile.py", "--target", "Release", "--clobber", "--compiler=goma", "--", "all"], "exit": 0, "step_name": "compile", "is_cloudtop": true, "gce_machine_type": "n2d-standard-9999", "is_cog": true, "env": {"LANG": "en_US.UTF-8", "SHELL": "/bin/bash", "HOME": "/home/chrome-bot", "PWD": "/b/build/Linux_x64/build", "LOGNAME": "chrome-bot", "USER": "chrome-bot", "PATH": "/home/chrome-bot/bin:/b/depot_tools:/usr/bin:/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin" }, "cwd": "/b/build/Linux_x64/build/src", "compiler": "goma", "jobs": 50, "targets": ["all"], "build_configs": {"use_goma": "true", "target_cpu": "\"\"", "is_component_build": "true", "symbol_level": "-1", "is_debug": "false", "enable_nacl": "false", "host_cpu": "\"x64\"", "host_os": "\"linux\"", "target_os": "\"\""}, "explicit_build_config_keys": ["is_debug", "use_goma"]}
`))
	if err != nil {
		t.Errorf(`Parse()=_, %#v; want=_, <nil>`, err)
	}

	want := &NinjaLog{
		Filename: ".ninja_log",
		Start:    4,
		Steps:    stepsTestCase,
		Metadata: metadataTestCase,
	}
	njl.Metadata.Raw = ""

	if diff := cmp.Diff(want, njl); diff != "" {
		t.Errorf("Parse() mismatch (-want, +got):\n%s", diff)
	}
}

func TestDump(t *testing.T) {
	var b bytes.Buffer
	err := Dump(&b, stepsTestCase)
	if err != nil {
		t.Errorf("Dump()=%v; want=<nil>", err)
	}
	if diff := cmp.Diff(b.String(), logTestCase); diff != "" {
		t.Errorf("Dump(...) mismatch (-want, +got):\n%s", diff)
	}
}

func TestDedup(t *testing.T) {
	steps := append([]Step{}, stepsTestCase...)
	for _, out := range []string{
		"gen/ui/keyboard/webui/keyboard.mojom.cc",
		"gen/ui/keyboard/webui/keyboard.mojom.h",
		"gen/ui/keyboard/webui/keyboard.mojom.js",
		"gen/ui/keyboard/webui/keyboard.mojom-internal.h",
	} {
		steps = append(steps, Step{
			Start:   302 * time.Millisecond,
			End:     5764 * time.Millisecond,
			Out:     out,
			CmdHash: "a551cc46f8c21e5a",
		})
	}
	got := Dedup(steps)
	want := append([]Step{}, stepsSorted...)
	want = append(want, Step{
		Start: 302 * time.Millisecond,
		End:   5764 * time.Millisecond,
		Out:   "gen/ui/keyboard/webui/keyboard.mojom-internal.h",
		Outs: []string{
			"gen/ui/keyboard/webui/keyboard.mojom.cc",
			"gen/ui/keyboard/webui/keyboard.mojom.h",
			"gen/ui/keyboard/webui/keyboard.mojom.js",
		},
		CmdHash: "a551cc46f8c21e5a",
	})
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Dedup(...) mismatch (-want, +got):\n%s", diff)
	}
}

func TestFlow(t *testing.T) {
	steps := append([]Step{}, stepsTestCase...)
	steps = append(steps, Step{
		Start:   187 * time.Millisecond,
		End:     21304 * time.Millisecond,
		Out:     "obj/third_party/pdfium/core/src/fpdfdoc/fpdfdoc.doc_formfield.o",
		CmdHash: "2ac7111aa1ae86af",
	})

	flow := Flow(steps, false)

	wantSortByStart := [][]Step{
		{
			{
				End: 76 * time.Millisecond,
				Out: "ninja startup overhead",
			},
			{
				Start:   76 * time.Millisecond,
				End:     187 * time.Millisecond,
				Out:     "resources/inspector/devtools_extension_api.js",
				CmdHash: "75430546595be7c2",
			},
			{
				Start:   187 * time.Millisecond,
				End:     21304 * time.Millisecond,
				Out:     "obj/third_party/pdfium/core/src/fpdfdoc/fpdfdoc.doc_formfield.o",
				CmdHash: "2ac7111aa1ae86af",
			},
		},
		{
			{
				Start:   78 * time.Millisecond,
				End:     286 * time.Millisecond,
				Out:     "gen/angle/commit_id.py",
				CmdHash: "4ede38e2c1617d8c",
			},
			{
				Start:   287 * time.Millisecond,
				End:     290 * time.Millisecond,
				Out:     "obj/third_party/angle/src/copy_scripts.actions_rules_copies.stamp",
				CmdHash: "b211d373de72f455",
			},
		},
		{
			{
				Start:   79 * time.Millisecond,
				End:     287 * time.Millisecond,
				Out:     "gen/angle/copy_compiler_dll.bat",
				CmdHash: "9fb635ad5d2c1109",
			},
		},
		{
			{
				Start:   80 * time.Millisecond,
				End:     284 * time.Millisecond,
				Out:     "gen/autofill_regex_constants.cc",
				CmdHash: "fa33c8d7ce1d8791",
			},
		},
		{
			{
				Start:   141 * time.Millisecond,
				End:     287 * time.Millisecond,
				Out:     "PepperFlash/manifest.json",
				CmdHash: "324f0a0b77c37ef",
			},
		},
		{
			{
				Start:   142 * time.Millisecond,
				End:     288 * time.Millisecond,
				Out:     "PepperFlash/libpepflashplayer.so",
				CmdHash: "1e2c2b7845a4d4fe",
			},
		},
	}

	if diff := cmp.Diff(flow, wantSortByStart); diff != "" {
		t.Errorf("Flow(...) mismatch (-want, +got):\n%s", diff)
	}

	flow = Flow(steps, true)
	wantSortByEnd := [][]Step{
		{
			{
				End: 76 * time.Millisecond,
				Out: "ninja startup overhead",
			},
			{
				Start:   76 * time.Millisecond,
				End:     187 * time.Millisecond,
				Out:     "resources/inspector/devtools_extension_api.js",
				CmdHash: "75430546595be7c2",
			},
			{
				Start:   187 * time.Millisecond,
				End:     21304 * time.Millisecond,
				Out:     "obj/third_party/pdfium/core/src/fpdfdoc/fpdfdoc.doc_formfield.o",
				CmdHash: "2ac7111aa1ae86af",
			},
		},
		{
			{
				Start:   141 * time.Millisecond,
				End:     287 * time.Millisecond,
				Out:     "PepperFlash/manifest.json",
				CmdHash: "324f0a0b77c37ef",
			},
			{
				Start:   287 * time.Millisecond,
				End:     290 * time.Millisecond,
				Out:     "obj/third_party/angle/src/copy_scripts.actions_rules_copies.stamp",
				CmdHash: "b211d373de72f455",
			},
		},
		{
			{
				Start:   142 * time.Millisecond,
				End:     288 * time.Millisecond,
				Out:     "PepperFlash/libpepflashplayer.so",
				CmdHash: "1e2c2b7845a4d4fe",
			},
		},
		{
			{
				Start:   79 * time.Millisecond,
				End:     287 * time.Millisecond,
				Out:     "gen/angle/copy_compiler_dll.bat",
				CmdHash: "9fb635ad5d2c1109",
			},
		},
		{
			{
				Start:   78 * time.Millisecond,
				End:     286 * time.Millisecond,
				Out:     "gen/angle/commit_id.py",
				CmdHash: "4ede38e2c1617d8c",
			},
		},
		{
			{
				Start:   80 * time.Millisecond,
				End:     284 * time.Millisecond,
				Out:     "gen/autofill_regex_constants.cc",
				CmdHash: "fa33c8d7ce1d8791",
			},
		},
	}

	if diff := cmp.Diff(flow, wantSortByEnd); diff != "" {
		t.Errorf("Flow(...) mismatch (-want, +got):\n%s", diff)
	}
}

func TestWeightedTime(t *testing.T) {
	steps := []Step{
		{
			Start:   0 * time.Millisecond,
			End:     3 * time.Millisecond,
			Out:     "target-a",
			CmdHash: "hash-target-a",
		},
		{
			Start:   2 * time.Millisecond,
			End:     5 * time.Millisecond,
			Out:     "target-b",
			CmdHash: "hash-target-b",
		},
		{
			Start:   2 * time.Millisecond,
			End:     8 * time.Millisecond,
			Out:     "target-c",
			CmdHash: "hash-target-c",
		},
		{
			Start:   2 * time.Millisecond,
			End:     3 * time.Millisecond,
			Out:     "target-d",
			CmdHash: "hash-target-d",
		},
	}

	// 0 1 2 3 4 5 6 7 8
	// +-+-+-+-+-+-+-+-+
	// <--A-->
	//     <--B-->
	//     <------C---->
	//     <D>
	got := WeightedTime(steps)
	want := map[string]time.Duration{
		"target-a": 2*time.Millisecond + 1*time.Millisecond/4,
		"target-b": 1*time.Millisecond/4 + 2*time.Millisecond/2,
		"target-c": 1*time.Millisecond/4 + 2*time.Millisecond/2 + 3*time.Millisecond,
		"target-d": 1 * time.Millisecond / 4,
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("WeightedTime(%v) mismatch (-want, +got):\n%s", steps, diff)
	}
}

func TestParseMetadata(t *testing.T) {
	var m Metadata
	mJSON := `{"exit_code": 1, "build_duration_sec": 100, "jobs": 1000, "platform": "Linux", "cpu_core": 48, "targets": ["chrome"], "build_configs": {"use_goma": "true", "target_cpu": "\"\"", "is_component_build": "true", "symbol_level": "-1", "is_debug": "false", "enable_nacl": "false", "host_cpu": "\"x64\"", "host_os": "\"linux\"", "target_os": "\"\""}, "explicit_build_config_keys": ["target_os", "use_goma"]}`

	err := json.Unmarshal([]byte(mJSON), &m)

	if err != nil {
		t.Errorf("failed to parse medatadata %q: %v", mJSON, err)
	}

	want := Metadata{
		ExitCode:         1,
		BuildDurationSec: 100,
		Platform:         "Linux",
		CPUCore:          48,
		BuildConfigs: map[string]string{
			"use_goma":           "true",
			"is_component_build": "true",
			"enable_nacl":        "false",
			"host_cpu":           "\"x64\"",
			"target_os":          "\"\"",
			"target_cpu":         "\"\"",
			"symbol_level":       "-1",
			"is_debug":           "false",
			"host_os":            "\"linux\"",
		},
		ExplicitBuildConfigKeys: []string{"target_os", "use_goma"},
		Jobs:                    1000,
		Targets:                 []string{"chrome"},
	}

	if diff := cmp.Diff(m, want); diff != "" {
		t.Errorf("json.Unmarshal(%q, ...) mismatch (-want, +got):\n%s", mJSON, diff)
	}
}

func TestGetTargets(t *testing.T) {
	type testData struct {
		m    *Metadata
		want []string
	}
	tests := []testData{
		{
			m:    &Metadata{Targets: []string{"foo", "/path/to/bar"}},
			want: []string{"foo"},
		},
		{
			m:    &Metadata{Targets: []string{"foo", "C:\\path\\to\\bar"}},
			want: []string{"foo"},
		},
	}
	for _, test := range tests {
		if diff := cmp.Diff(test.want, test.m.getTargets()); diff != "" {
			t.Errorf("getTargets() mismatch (-want, +got):\n%s", diff)
		}
	}
}

//go:embed testdata/ninja_log
var ninjaLogData []byte

func BenchmarkParse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Parse(".ninja_log", bytes.NewReader(ninjaLogData))
		if err != nil {
			b.Errorf(`Parse()=_, %v; want=_, <nil>`, err)
		}
	}
}

func BenchmarkDedup(b *testing.B) {
	njl, err := Parse(".ninja_log", bytes.NewReader(ninjaLogData))
	if err != nil {
		b.Errorf(`Parse()=_, %v; want=_, <nil>`, err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		steps := make([]Step, len(njl.Steps))
		copy(steps, njl.Steps)
		Dedup(steps)
	}
}

func BenchmarkFlow(b *testing.B) {
	njl, err := Parse(".ninja_log", bytes.NewReader(ninjaLogData))
	if err != nil {
		b.Errorf(`Parse()=_, %v; want=_, <nil>`, err)
	}
	steps := Dedup(njl.Steps)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		flowInput := make([]Step, len(steps))
		copy(flowInput, steps)
		Flow(flowInput, false)
	}
}

func BenchmarkToTraces(b *testing.B) {
	njl, err := Parse(".ninja_log", bytes.NewReader(ninjaLogData))
	if err != nil {
		b.Errorf(`Parse()=_, %v; want=_, <nil>`, err)
	}
	steps := Dedup(njl.Steps)
	flow := Flow(steps, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ToTraces(flow, 1)
	}
}

func BenchmarkDedupFlowToTraces(b *testing.B) {
	for i := 0; i < b.N; i++ {
		njl, err := Parse(".ninja_log", bytes.NewReader(ninjaLogData))
		if err != nil {
			b.Errorf(`Parse()=_, %v; want=_, <nil>`, err)
		}

		steps := Dedup(njl.Steps)
		flow := Flow(steps, false)
		ToTraces(flow, 1)
	}
}
