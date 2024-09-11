// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ninjalog

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestAVROCodec(t *testing.T) {
	if _, err := avroCodec(); err != nil {
		t.Fatalf("failed to parse AVRO schema: %v", err)
	}
}

func TestToAVRO(t *testing.T) {
	outputTestCase := append([]Step{
		{
			Start:   76 * time.Millisecond,
			End:     187 * time.Millisecond,
			Out:     "resources/inspector/devtools_api.js",
			CmdHash: "75430546595be7c2",
		},
		{
			Start:   78 * time.Millisecond,
			End:     286 * time.Millisecond,
			Out:     "gen/angle/commit_id_2.py",
			CmdHash: "4ede38e2c1617d8c",
		},
		{
			Start:   78 * time.Millisecond,
			End:     286 * time.Millisecond,
			Out:     "gen/angle/commit_id_3.py",
			CmdHash: "4ede38e2c1617d8c",
		}}, stepsTestCase...)

	info := NinjaLog{
		Filename: ".ninja_log",
		Start:    1,
		Steps:    outputTestCase,
		Metadata: metadataTestCase,
	}

	createdTime := time.Unix(1514768400, 12345678)
	defer func() {
		timeNow = time.Now
	}()
	timeNow = func() time.Time { return createdTime }

	got, err := toAVRO(&info)
	if err != nil {
		t.Fatalf("got err %v, want nil", err)
	}
	if diff := cmp.Diff(map[string]any{
		"user": "bob@google.com",
		"build_configs": []map[string]any{
			{"key": "enable_nacl", "value": "false", "explicit": false},
			{"key": "host_cpu", "value": "\"x64\"", "explicit": false},
			{"key": "host_os", "value": "\"linux\"", "explicit": false},
			{"key": "is_component_build", "value": "true", "explicit": false},
			{"key": "is_debug", "value": "false", "explicit": true},
			{"key": "symbol_level", "value": "-1", "explicit": false},
			{"key": "target_cpu", "value": "\"\"", "explicit": false},
			{"key": "target_os", "value": "\"\"", "explicit": false},
			{"key": "use_goma", "value": "true", "explicit": true},
		},
		"build_id":           int64(12345),
		"invocation_id":      "6dc52b4f-fdf9-4017-b542-8c6cf296677d",
		"exit_code":          1,
		"build_duration_sec": 100,
		"cpu_core":           0,
		"created_at":         createdTime,
		"jobs":               50,
		"os":                 "LINUX",
		"step_name":          "compile",
		"targets":            []string{"all"},
		"log_entries": []map[string]any{
			{
				"end_duration_sec":      0.076,
				"outputs":               []string{"ninja startup overhead"},
				"start_duration_sec":    0.0,
				"weighted_duration_sec": 0.076,
			},
			{
				"end_duration_sec":      0.187,
				"outputs":               []string{"resources/inspector/devtools_api.js", "resources/inspector/devtools_extension_api.js"},
				"start_duration_sec":    0.076,
				"weighted_duration_sec": 0.025783333,
			},
			{
				"end_duration_sec":      0.286,
				"outputs":               []string{"gen/angle/commit_id.py", "gen/angle/commit_id_2.py", "gen/angle/commit_id_3.py"},
				"start_duration_sec":    0.078,
				"weighted_duration_sec": 0.043683333,
			},
			{
				"end_duration_sec":      0.287,
				"outputs":               []string{"gen/angle/copy_compiler_dll.bat"},
				"start_duration_sec":    0.079,
				"weighted_duration_sec": 0.043516666,
			},
			{
				"end_duration_sec":      0.284,
				"outputs":               []string{"gen/autofill_regex_constants.cc"},
				"start_duration_sec":    0.08,
				"weighted_duration_sec": 0.04235,
			},
			{
				"end_duration_sec":      0.287,
				"outputs":               []string{"PepperFlash/manifest.json"},
				"start_duration_sec":    0.141,
				"weighted_duration_sec": 0.027933333,
			},
			{
				"end_duration_sec":      0.288,
				"outputs":               []string{"PepperFlash/libpepflashplayer.so"},
				"start_duration_sec":    0.142,
				"weighted_duration_sec": 0.028233333,
			},
			{
				"end_duration_sec":      0.29,
				"outputs":               []string{"obj/third_party/angle/src/copy_scripts.actions_rules_copies.stamp"},
				"start_duration_sec":    0.287,
				"weighted_duration_sec": 0.0025,
			},
		},
	}, got); diff != "" {
		t.Errorf("ToAVRO(%v) mismatch (-want +got):\n%s", info, diff)
	}
}
