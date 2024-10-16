// Copyright 2021 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/assertions"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/convey"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestEnsureArgsValid(t *testing.T) {
	t.Parallel()

	r := &goRun{}
	ftt.Run(`does not alter correct command`, t, func(t *ftt.Test) {
		args := strings.Split("go test -json infra/tools/result_adapter", " ")
		validArgs, err := r.ensureArgsValid(args)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, validArgs, should.Resemble(args))
	})
	ftt.Run(`adds -json flag`, t, func(t *ftt.Test) {
		args := strings.Split("go test infra/tools/result_adapter", " ")
		validArgs, err := r.ensureArgsValid(args)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, validArgs, should.Resemble(strings.Split("go test -json infra/tools/result_adapter", " ")))
	})
	ftt.Run(`passes plausible command through as is`, t, func(t *ftt.Test) {
		args := strings.Split("GOROOT/src/run.bash -json", " ")
		plausibleArgs, err := r.ensureArgsValid(args)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, plausibleArgs, should.Resemble(args))
	})
	ftt.Run(`reports unlikely command`, t, func(t *ftt.Test) {
		args := strings.Split("not_the_right_thing --at=all", " ")
		_, err := r.ensureArgsValid(args)
		assert.Loosely(t, err, should.ErrLike("Expected command to be an invocation of `go test -json` or equivalent:"))
	})
}

func TestGenerateTestResults(t *testing.T) {
	t.Parallel()

	r := &goRun{}

	ftt.Run(`parses output`, t, func(t *ftt.Test) {
		trs, err := r.generateTestResults(context.Background(),
			[]byte(`
			{"Time":"2021-06-17T15:59:10.536701-07:00","Action":"start","Package":"infra/tools/result_adapter"}
			{"Time":"2021-06-17T15:59:10.536706-07:00","Action":"run","Package":"infra/tools/result_adapter","Test":"TestEnsureArgsValid"}
			{"Time":"2021-06-17T15:59:10.537037-07:00","Action":"output","Package":"infra/tools/result_adapter","Test":"TestEnsureArgsValid","Output":"=== RUN   TestEnsureArgsValid\n"}
			{"Time":"2021-06-17T15:59:10.537058-07:00","Action":"output","Package":"infra/tools/result_adapter","Test":"TestEnsureArgsValid","Output":"=== PAUSE TestEnsureArgsValid\n"}
			{"Time":"2021-06-17T15:59:10.537064-07:00","Action":"pause","Package":"infra/tools/result_adapter","Test":"TestEnsureArgsValid"}
			{"Time":"2021-06-17T15:59:10.537178-07:00","Action":"cont","Package":"infra/tools/result_adapter","Test":"TestEnsureArgsValid"}
			{"Time":"2021-06-17T15:59:10.537183-07:00","Action":"output","Package":"infra/tools/result_adapter","Test":"TestEnsureArgsValid","Output":"=== CONT  TestEnsureArgsValid\n"}
			{"Time":"2021-06-17T15:59:10.537309-07:00","Action":"output","Package":"infra/tools/result_adapter","Test":"TestEnsureArgsValid","Output":"--- PASS: TestEnsureArgsValid (0.00s)\n"}
			{"Time":"2021-06-17T15:59:10.537672-07:00","Action":"pass","Package":"infra/tools/result_adapter","Test":"TestEnsureArgsValid","Elapsed":0}
			{"Time":"2021-06-17T15:59:10.540475-07:00","Action":"output","Package":"infra/tools/result_adapter","Output":"PASS\n"}
			{"Time":"2021-06-17T15:59:10.541301-07:00","Action":"output","Package":"infra/tools/result_adapter","Output":"ok  \tinfra/tools/result_adapter\t0.143s\n"}
			{"Time":"2021-06-17T15:59:10.541324-07:00","Action":"pass","Package":"infra/tools/result_adapter","Elapsed":0.143}`),
		)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, trs, should.HaveLength(2))
		assert.Loosely(t, trs[0], convey.Adapt(assertions.ShouldResembleProtoText)(
			`test_id:  "infra/tools/result_adapter"
			expected:  true
			status:  PASS
			summary_html:  "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			start_time:  {
		  		seconds:  1623970750
		  		nanos:  536701000
			}
			duration:  {
				nanos:  143000000
			}
			artifacts:  {
		  		key:  "output"
		  		value:  {
					contents:  "PASS\nok  	infra/tools/result_adapter	0.143s\n"
		  		}
			}`))
		assert.Loosely(t, trs[1], convey.Adapt(assertions.ShouldResembleProtoText)(
			`test_id:  "infra/tools/result_adapter.TestEnsureArgsValid"
			expected:  true
			status:  PASS
			summary_html:  "<p><text-artifact artifact-id=\"output\"></p>"
			start_time:  {
		  		seconds:  1623970750
		  		nanos:  536706000
			}
			duration:  {}
			artifacts:  {
		  		key:  "output"
		  		value:  {
					contents:  "=== RUN   TestEnsureArgsValid\n=== PAUSE TestEnsureArgsValid\n=== CONT  TestEnsureArgsValid\n--- PASS: TestEnsureArgsValid (0.00s)\n"
		  		}
			}`))
	})

	// Test that output is associated with the test that produced it, and only that test.
	//
	// These test events were generated by running 'go test -json' on the following tests:
	//
	//	package test
	//
	//	import "testing"
	//
	//	func TestA(t *testing.T) {
	//		t.Log("TestA line 1 of 1")
	//	}
	//
	//	func TestB(t *testing.T) {
	//		t.Log("TestB line 1 of 2")
	//		t.Log("TestB line 2 of 2")
	//	}
	//
	//	func TestAB(t *testing.T) {
	//		t.Log("TestAB line 1 of 3")
	//		t.Log("TestAB line 2 of 3")
	//		t.Log("TestAB line 3 of 3")
	//	}
	//
	ftt.Run(`test output separate`, t, func(t *ftt.Test) {
		trs, err := r.generateTestResults(context.Background(),
			[]byte(`{"Time":"2023-04-03T12:44:58.511534-04:00","Action":"start","Package":"example/pkg"}
{"Time":"2023-04-03T12:44:58.73917-04:00","Action":"run","Package":"example/pkg","Test":"TestA"}
{"Time":"2023-04-03T12:44:58.739261-04:00","Action":"output","Package":"example/pkg","Test":"TestA","Output":"=== RUN   TestA\n"}
{"Time":"2023-04-03T12:44:58.739323-04:00","Action":"output","Package":"example/pkg","Test":"TestA","Output":"    main_test.go:6: TestA line 1 of 1\n"}
{"Time":"2023-04-03T12:44:58.739339-04:00","Action":"output","Package":"example/pkg","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n"}
{"Time":"2023-04-03T12:44:58.739345-04:00","Action":"pass","Package":"example/pkg","Test":"TestA","Elapsed":0}
{"Time":"2023-04-03T12:44:58.739352-04:00","Action":"run","Package":"example/pkg","Test":"TestB"}
{"Time":"2023-04-03T12:44:58.739354-04:00","Action":"output","Package":"example/pkg","Test":"TestB","Output":"=== RUN   TestB\n"}
{"Time":"2023-04-03T12:44:58.739357-04:00","Action":"output","Package":"example/pkg","Test":"TestB","Output":"    main_test.go:10: TestB line 1 of 2\n"}
{"Time":"2023-04-03T12:44:58.739359-04:00","Action":"output","Package":"example/pkg","Test":"TestB","Output":"    main_test.go:11: TestB line 2 of 2\n"}
{"Time":"2023-04-03T12:44:58.739362-04:00","Action":"output","Package":"example/pkg","Test":"TestB","Output":"--- PASS: TestB (0.00s)\n"}
{"Time":"2023-04-03T12:44:58.739365-04:00","Action":"pass","Package":"example/pkg","Test":"TestB","Elapsed":0}
{"Time":"2023-04-03T12:44:58.739367-04:00","Action":"run","Package":"example/pkg","Test":"TestAB"}
{"Time":"2023-04-03T12:44:58.73937-04:00","Action":"output","Package":"example/pkg","Test":"TestAB","Output":"=== RUN   TestAB\n"}
{"Time":"2023-04-03T12:44:58.739373-04:00","Action":"output","Package":"example/pkg","Test":"TestAB","Output":"    main_test.go:15: TestAB line 1 of 3\n"}
{"Time":"2023-04-03T12:44:58.739375-04:00","Action":"output","Package":"example/pkg","Test":"TestAB","Output":"    main_test.go:16: TestAB line 2 of 3\n"}
{"Time":"2023-04-03T12:44:58.739379-04:00","Action":"output","Package":"example/pkg","Test":"TestAB","Output":"    main_test.go:17: TestAB line 3 of 3\n"}
{"Time":"2023-04-03T12:44:58.739577-04:00","Action":"output","Package":"example/pkg","Test":"TestAB","Output":"--- PASS: TestAB (0.00s)\n"}
{"Time":"2023-04-03T12:44:58.739582-04:00","Action":"pass","Package":"example/pkg","Test":"TestAB","Elapsed":0}
{"Time":"2023-04-03T12:44:58.739598-04:00","Action":"output","Package":"example/pkg","Output":"PASS\n"}
{"Time":"2023-04-03T12:44:58.73966-04:00","Action":"output","Package":"example/pkg","Output":"ok  \texample/pkg\t0.228s\n"}
{"Time":"2023-04-03T12:44:58.739667-04:00","Action":"pass","Package":"example/pkg","Elapsed":0.228}`),
		)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, trs, should.HaveLength(4))
		assert.Loosely(t, trs[0], convey.Adapt(assertions.ShouldResembleProtoText)(
			`test_id: "example/pkg"
			expected: true
			status: PASS
			summary_html:  "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1680540298
			  nanos: 511534000
			}
			duration: {
			  nanos: 228000000
			}
			artifacts: {
			  key: "output"
			  value: {
				contents:  "PASS\nok  	example/pkg	0.228s\n"
			  }
			}`))
		assert.Loosely(t, trs[1], convey.Adapt(assertions.ShouldResembleProtoText)(
			`test_id: "example/pkg.TestA"
			expected: true
			status: PASS
			summary_html: "<p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1680540298
			  nanos: 739170000
			}
			duration: {}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "=== RUN   TestA\n    main_test.go:6: TestA line 1 of 1\n--- PASS: TestA (0.00s)\n"
			  }
			}`))
		assert.Loosely(t, trs[2], convey.Adapt(assertions.ShouldResembleProtoText)(
			`test_id: "example/pkg.TestB"
			expected: true
			status: PASS
			summary_html: "<p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1680540298
			  nanos: 739352000
			}
			duration: {}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "=== RUN   TestB\n    main_test.go:10: TestB line 1 of 2\n    main_test.go:11: TestB line 2 of 2\n--- PASS: TestB (0.00s)\n"
			  }
			}`))
		assert.Loosely(t, trs[3], convey.Adapt(assertions.ShouldResembleProtoText)(
			`test_id:  "example/pkg.TestAB"
			expected:  true
			status:  PASS
			summary_html:  "<p><text-artifact artifact-id=\"output\"></p>"
			start_time:  {
			  seconds:  1680540298
			  nanos:  739367000
			}
			duration:  {}
			artifacts:  {
			  key:  "output"
			  value:  {
			    contents:  "=== RUN   TestAB\n    main_test.go:15: TestAB line 1 of 3\n    main_test.go:16: TestAB line 2 of 3\n    main_test.go:17: TestAB line 3 of 3\n--- PASS: TestAB (0.00s)\n"
			  }
			}`))
	})

	ftt.Run(`parses skipped package`, t, func(t *ftt.Test) {
		trs, err := r.generateTestResults(context.Background(),
			[]byte(`{"Time":"2021-06-17T16:11:01.086366-07:00","Action":"output","Package":"go.chromium.org/luci/resultdb/internal/permissions","Output":"?   \tgo.chromium.org/luci/resultdb/internal/permissions\t[no test files]\n"}
			{"Time":"2021-06-17T16:11:01.086381-07:00","Action":"skip","Package":"go.chromium.org/luci/resultdb/internal/permissions","Elapsed":0}`),
		)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, trs, should.HaveLength(1))
		assert.Loosely(t, trs[0], convey.Adapt(assertions.ShouldResembleProtoText)(
			`test_id: "go.chromium.org/luci/resultdb/internal/permissions"
			expected: true
			status: SKIP
			summary_html:  "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			duration: {}
			artifacts: {
			  key: "output"
			  value: {
				contents:  "?   	go.chromium.org/luci/resultdb/internal/permissions	[no test files]\n"
			  }
			}`))
	})
}

var goTestJSONSimple = []byte(`
{"Action":"start","Package":"example/pkg"}
{"Action":"run","Package":"example/pkg","Test":"TestA"}
{"Action":"output","Package":"example/pkg","Test":"TestA","Output":"=== RUN   TestA\n"}
{"Action":"output","Package":"example/pkg","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n"}
{"Action":"output","Package":"example/pkg","Output":"PASS\n"}
{"Action":"pass","Package":"example/pkg","Test":"TestA"}
{"Action":"output","Package":"example/pkg","Output":"ok  \texample/pkg\t0.123s\n"}
{"Action":"pass","Package":"example/pkg"}
`)

var goTestJSONInterleaved = []byte(`
{"Action":"start","Package":"example/pkg1"}
{"Action":"start","Package":"example/pkg2"}
{"Action":"run","Package":"example/pkg2","Test":"TestB"}
{"Action":"run","Package":"example/pkg1","Test":"TestA"}
{"Action":"output","Package":"example/pkg1","Test":"TestA","Output":"=== RUN   TestA\n"}
{"Action":"output","Package":"example/pkg1","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n"}
{"Action":"output","Package":"example/pkg2","Test":"TestB","Output":"=== RUN   TestB\n"}
{"Action":"output","Package":"example/pkg2","Test":"TestB","Output":"--- PASS: TestB (0.00s)\n"}
{"Action":"output","Package":"example/pkg2","Output":"PASS\n"}
{"Action":"pass","Package":"example/pkg2","Test":"TestB"}
{"Action":"output","Package":"example/pkg1","Output":"PASS\n"}
{"Action":"output","Package":"example/pkg2","Output":"ok  \texample/pkg2\t0.123s\n"}
{"Action":"pass","Package":"example/pkg2"}
{"Action":"pass","Package":"example/pkg1","Test":"TestA"}
{"Action":"output","Package":"example/pkg1","Output":"ok  \texample/pkg1\t0.123s\n"}
{"Action":"pass","Package":"example/pkg1"}
`)

var goTestJSONPkgLevelOutputPass = []byte(`
{"Action":"start","Package":"example/pkg"}
{"Action":"run","Package":"example/pkg","Test":"TestA"}
{"Action":"output","Package":"example/pkg","Test":"TestA","Output":"=== RUN   TestA\n"}
{"Action":"output","Package":"example/pkg","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n"}
{"Action":"output","Package":"example/pkg","Output":"PASS\n"}
{"Action":"pass","Package":"example/pkg","Test":"TestA"}
{"Action":"output","Package":"example/pkg","Output":"ok  \texample/pkg\t0.123s\n"}
{"Action":"output","Package":"example/pkg","Output":"hello world!\n"}
{"Action":"pass","Package":"example/pkg"}
`)

var goTestJSONPkgLevelOutputFail = []byte(`
{"Action":"start","Package":"example/pkg"}
{"Action":"run","Package":"example/pkg","Test":"TestA"}
{"Action":"output","Package":"example/pkg","Test":"TestA","Output":"=== RUN   TestA\n"}
{"Action":"output","Package":"example/pkg","Test":"TestA","Output":"--- FAIL: TestA (0.00s)\n"}
{"Action":"output","Package":"example/pkg","Output":"FAIL\n"}
{"Action":"fail","Package":"example/pkg","Test":"TestA"}
{"Action":"output","Package":"example/pkg","Output":"FAIL\texample/pkg\t0.123s\n"}
{"Action":"output","Package":"example/pkg","Output":"hello world!\n"}
{"Action":"fail","Package":"example/pkg"}
`)

func TestCopyTestOutput(t *testing.T) {
	type test struct {
		name    string
		verbose bool
		input   []byte
		expect  string
	}
	for _, test := range []test{
		{
			name:    "SimpleVerbose",
			verbose: true,
			input:   goTestJSONSimple,
			expect: `=== RUN   TestA
--- PASS: TestA (0.00s)
PASS
ok  	example/pkg	0.123s
`,
		},
		{
			name:    "SimpleNonVerbose",
			verbose: false,
			input:   goTestJSONSimple,
			expect: `ok  	example/pkg	0.123s
`,
		},
		{
			name:    "InterleavedVerbose",
			verbose: true,
			input:   goTestJSONInterleaved,
			expect: `=== RUN   TestA
--- PASS: TestA (0.00s)
PASS
ok  	example/pkg1	0.123s
=== RUN   TestB
--- PASS: TestB (0.00s)
PASS
ok  	example/pkg2	0.123s
`,
		},
		{
			name:    "InterleavedNonVerbose",
			verbose: false,
			input:   goTestJSONInterleaved,
			expect: `ok  	example/pkg1	0.123s
ok  	example/pkg2	0.123s
`,
		},
		{
			name:    "PkgLevelPassVerbose",
			verbose: true,
			input:   goTestJSONPkgLevelOutputPass,
			expect: `=== RUN   TestA
--- PASS: TestA (0.00s)
PASS
ok  	example/pkg	0.123s
hello world!
`,
		},
		{
			name:    "PkgLevelPassNonVerbose",
			verbose: false,
			input:   goTestJSONPkgLevelOutputPass,
			expect: `ok  	example/pkg	0.123s
`,
		},
		{
			name:    "PkgLevelFailVerbose",
			verbose: true,
			input:   goTestJSONPkgLevelOutputFail,
			expect: `=== RUN   TestA
--- FAIL: TestA (0.00s)
FAIL
FAIL	example/pkg	0.123s
hello world!
`,
		},
		{
			name:    "PkgLevelFailNonVerbose",
			verbose: false,
			input:   goTestJSONPkgLevelOutputFail,
			expect: `--- FAIL: TestA (0.00s)
FAIL
FAIL	example/pkg	0.123s
hello world!
`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := &goRun{CopyTestOutput: &buf, VerboseTestOutput: test.verbose}
			_, err := r.generateTestResults(context.Background(), test.input)
			if err != nil {
				t.Fatal(err)
			}
			got, want := buf.String(), test.expect
			if got != want {
				t.Errorf("test output copy doesn't match:\ngot  %q\nwant %q", got, want)
			}
		})
	}
}
