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

	"google.golang.org/protobuf/encoding/prototext"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

func TestEnsureArgsValid(t *testing.T) {
	t.Parallel()

	r := &goRun{}
	ftt.Run(`does not alter correct command`, t, func(t *ftt.Test) {
		args := strings.Split("go test -json infra/tools/result_adapter", " ")
		validArgs, err := r.ensureArgsValid(args)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, validArgs, should.Match(args))
	})
	ftt.Run(`adds -json flag`, t, func(t *ftt.Test) {
		args := strings.Split("go test infra/tools/result_adapter", " ")
		validArgs, err := r.ensureArgsValid(args)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, validArgs, should.Match(strings.Split("go test -json infra/tools/result_adapter", " ")))
	})
	ftt.Run(`passes plausible command through as is`, t, func(t *ftt.Test) {
		args := strings.Split("GOROOT/src/run.bash -json", " ")
		plausibleArgs, err := r.ensureArgsValid(args)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, plausibleArgs, should.Match(args))
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
		assert.That(t, trs[0], should.Match(mustParseTestResult(
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
			}`)))
		assert.That(t, trs[1], should.Match(mustParseTestResult(
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
			}`)))
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
		assert.That(t, trs[0], should.Match(mustParseTestResult(
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
			}`)))
		assert.That(t, trs[1], should.Match(mustParseTestResult(
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
			}`)))
		assert.That(t, trs[2], should.Match(mustParseTestResult(
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
			}`)))
		assert.That(t, trs[3], should.Match(mustParseTestResult(
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
			}`)))
	})

	ftt.Run("test JSON build errors", t, func(t *ftt.Test) {
		trs, err := r.generateTestResults(context.Background(), []byte(goTestJSONBuildError))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, trs, should.HaveLength(6))
		assert.That(t, trs[0], should.Match(mustParseTestResult(
			`test_id: "test/a"
			expected: true
			status: PASS
			summary_html: "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1734113176
			  nanos:   230477000
			}
			duration: {
			  nanos: 400000000
			}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "PASS\nok  	test/a	0.400s\n"
			  }
			}`)))
		assert.That(t, trs[1], should.Match(mustParseTestResult(
			`test_id: "test/a.TestA"
			expected: true
			status: PASS
			summary_html: "<p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1734113176
			  nanos:   630042000
			}
			duration: {}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "=== RUN   TestA\n    a_test.go:6: A is okay\n--- PASS: TestA (0.00s)\n"
			  }
			}`)))
		assert.That(t, trs[2], should.Match(mustParseTestResult(
			`test_id: "test/b"
			status: FAIL
			summary_html: "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1734113176
			  nanos:   230551000
			}
			duration: {}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "FAIL	test/b [build failed]\n# test/b [test/b.test]\nb/b_test.go:5:8: \"os\" imported and not used\n"
			  }
			}`)))
		assert.That(t, trs[3], should.Match(mustParseTestResult(
			`test_id: "test/c"
			status: FAIL
			summary_html: "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1734113176
			  nanos:   230575000
			}
			duration: {}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "FAIL	test/c [build failed]\n# test/c\nc/c.go:3:8: \"os\" imported and not used\n"
			  }
			}`)))
		assert.That(t, trs[4], should.Match(mustParseTestResult(
			`test_id: "test/d"
			expected: true
			status: PASS
			summary_html: "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1734113176
			  nanos:   230633000
			}
			duration: {
			  nanos: 232000000
			}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "PASS\nok  	test/d	0.232s\n"
			  }
			}`)))
		assert.That(t, trs[5], should.Match(mustParseTestResult(
			`test_id: "test/d.TestD"
			expected: true
			status: PASS
			summary_html: "<p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1734113176
			  nanos:   462169000
			}
			duration: {}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "=== RUN   TestD\n    d_test.go:8: D is okay\n--- PASS: TestD (0.00s)\n"
			  }
			}`)))
	})
	ftt.Run("test JSON build errors (Go 1.23 and older)", t, func(t *ftt.Test) {
		trs, err := r.generateTestResults(context.Background(), []byte(goTestJSONBuildErrorGo123))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, trs, should.HaveLength(6))
		assert.That(t, trs[0], should.Match(mustParseTestResult(
			`test_id: "test/a"
			expected: true
			status: PASS
			summary_html: "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1734116530
			  nanos:   21159000
			}
			duration: {
			  nanos: 234000000
			}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "PASS\nok  	test/a	0.234s\n"
			  }
			}`)))
		assert.That(t, trs[1], should.Match(mustParseTestResult(
			`test_id: "test/a.TestA"
			expected: true
			status: PASS
			summary_html: "<p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1734116530
			  nanos:   254709000
			}
			duration: {}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "=== RUN   TestA\n    a_test.go:6: A is okay\n--- PASS: TestA (0.00s)\n"
			  }
			}`)))
		assert.That(t, trs[2], should.Match(mustParseTestResult(
			`test_id: "test/b"
			status: FAIL
			summary_html: "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1734116530
			  nanos:   21296000
			}
			duration: {}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "FAIL	test/b [build failed]\n"
			  }
			}`)))
		assert.That(t, trs[3], should.Match(mustParseTestResult(
			`test_id: "test/c"
			status: FAIL
			summary_html: "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1734116530
			  nanos:   21361000
			}
			duration: {}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "FAIL	test/c [build failed]\n"
			  }
			}`)))
		assert.That(t, trs[4], should.Match(mustParseTestResult(
			`test_id: "test/d"
			expected: true
			status: PASS
			summary_html: "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1734116530
			  nanos:   21398000
			}
			duration: {
			  nanos: 398000000
			}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "PASS\nok  	test/d	0.398s\n"
			  }
			}`)))
		assert.That(t, trs[5], should.Match(mustParseTestResult(
			`test_id: "test/d.TestD"
			expected: true
			status: PASS
			summary_html: "<p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1734116530
			  nanos:   418623000
			}
			duration: {}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "=== RUN   TestD\n    d_test.go:8: D is okay\n--- PASS: TestD (0.00s)\n"
			  }
			}`)))
	})

	ftt.Run("test JSON build warnings", t, func(t *ftt.Test) {
		trs, err := r.generateTestResults(context.Background(), []byte(goTestJSONBuildWarning))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, trs, should.HaveLength(2))
		assert.That(t, trs[0], should.Match(mustParseTestResult(
			`test_id: "test"
			expected: true
			status: PASS
			summary_html: "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1736812206
			  nanos:   318980000
			}
			duration: {
			  nanos: 214000000
			}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "testing: warning: no tests to run\nPASS\nok  	test	0.214s [no tests to run]\n"
			  }
			}`)))
		assert.That(t, trs[1], should.Match(mustParseTestResult(
			`test_id: "test.test"
			expected: true
			status: PASS
			summary_html: "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			duration: {}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "# test.test\nld: warning: ignoring duplicate libraries: '-lobjc'\n"
			  }
			}`)))
	})
	ftt.Run("test JSON build warnings (Go 1.23 and older)", t, func(t *ftt.Test) {
		trs, err := r.generateTestResults(context.Background(), []byte(goTestJSONBuildWarningGo123))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, trs, should.HaveLength(1))
		assert.That(t, trs[0], should.Match(mustParseTestResult(
			`test_id: "test"
			expected: true
			status: PASS
			summary_html: "<p>Result only captures package setup and teardown. Tests within the package have their own result.</p><p><text-artifact artifact-id=\"output\"></p>"
			start_time: {
			  seconds: 1736812228
			  nanos:   261264000
			}
			duration: {
			  nanos: 220000000
			}
			artifacts: {
			  key: "output"
			  value: {
			    contents: "testing: warning: no tests to run\nPASS\nok  	test	0.220s [no tests to run]\n"
			  }
			}`)))
	})

	ftt.Run(`parses skipped package`, t, func(t *ftt.Test) {
		trs, err := r.generateTestResults(context.Background(),
			[]byte(`{"Time":"2021-06-17T16:11:01.086366-07:00","Action":"output","Package":"go.chromium.org/luci/resultdb/internal/permissions","Output":"?   \tgo.chromium.org/luci/resultdb/internal/permissions\t[no test files]\n"}
			{"Time":"2021-06-17T16:11:01.086381-07:00","Action":"skip","Package":"go.chromium.org/luci/resultdb/internal/permissions","Elapsed":0}`),
		)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, trs, should.HaveLength(1))
		assert.That(t, trs[0], should.Match(mustParseTestResult(
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
			}`)))
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

const (
	// goTestJSONBuildError holds stdout output from 'go test -json -count=1 ./...'
	// that corresponds to running the following tests that involve build errors:
	//
	//	$ go test -v -count=1 ./...
	//	# test/c
	//	c/c.go:3:8: "os" imported and not used
	//	# test/b [test/b.test]
	//	b/b_test.go:5:8: "os" imported and not used
	//	=== RUN   TestA
	//	    a_test.go:6: A is okay
	//	--- PASS: TestA (0.00s)
	//	PASS
	//	ok  	test/a	0.240s
	//	FAIL	test/b [build failed]
	//	FAIL	test/c [build failed]
	//	=== RUN   TestD
	//	    d_test.go:8: D is okay
	//	--- PASS: TestD (0.00s)
	//	PASS
	//	ok  	test/d	0.392s
	//	FAIL
	//
	//	$ go test -count=1 ./...
	//	# test/c
	//	c/c.go:3:8: "os" imported and not used
	//	# test/b [test/b.test]
	//	b/b_test.go:5:8: "os" imported and not used
	//	ok  	test/a	0.395s
	//	FAIL	test/b [build failed]
	//	FAIL	test/c [build failed]
	//	ok  	test/d	0.268s
	//	FAIL
	//
	goTestJSONBuildError = `{"ImportPath":"test/c","Action":"build-output","Output":"# test/c\n"}
{"ImportPath":"test/c","Action":"build-output","Output":"c/c.go:3:8: \"os\" imported and not used\n"}
{"ImportPath":"test/c","Action":"build-fail"}
{"ImportPath":"test/b [test/b.test]","Action":"build-output","Output":"# test/b [test/b.test]\n"}
{"ImportPath":"test/b [test/b.test]","Action":"build-output","Output":"b/b_test.go:5:8: \"os\" imported and not used\n"}
{"ImportPath":"test/b [test/b.test]","Action":"build-fail"}
{"Time":"2024-12-13T13:06:16.230477-05:00","Action":"start","Package":"test/a"}
{"Time":"2024-12-13T13:06:16.230551-05:00","Action":"start","Package":"test/b"}
{"Time":"2024-12-13T13:06:16.230562-05:00","Action":"output","Package":"test/b","Output":"FAIL\ttest/b [build failed]\n"}
{"Time":"2024-12-13T13:06:16.230566-05:00","Action":"fail","Package":"test/b","Elapsed":0,"FailedBuild":"test/b [test/b.test]"}
{"Time":"2024-12-13T13:06:16.230575-05:00","Action":"start","Package":"test/c"}
{"Time":"2024-12-13T13:06:16.230585-05:00","Action":"output","Package":"test/c","Output":"FAIL\ttest/c [build failed]\n"}
{"Time":"2024-12-13T13:06:16.230588-05:00","Action":"fail","Package":"test/c","Elapsed":0,"FailedBuild":"test/c"}
{"Time":"2024-12-13T13:06:16.230633-05:00","Action":"start","Package":"test/d"}
{"Time":"2024-12-13T13:06:16.462169-05:00","Action":"run","Package":"test/d","Test":"TestD"}
{"Time":"2024-12-13T13:06:16.462322-05:00","Action":"output","Package":"test/d","Test":"TestD","Output":"=== RUN   TestD\n"}
{"Time":"2024-12-13T13:06:16.462356-05:00","Action":"output","Package":"test/d","Test":"TestD","Output":"    d_test.go:8: D is okay\n"}
{"Time":"2024-12-13T13:06:16.46237-05:00","Action":"output","Package":"test/d","Test":"TestD","Output":"--- PASS: TestD (0.00s)\n"}
{"Time":"2024-12-13T13:06:16.462375-05:00","Action":"pass","Package":"test/d","Test":"TestD","Elapsed":0}
{"Time":"2024-12-13T13:06:16.462381-05:00","Action":"output","Package":"test/d","Output":"PASS\n"}
{"Time":"2024-12-13T13:06:16.463066-05:00","Action":"output","Package":"test/d","Output":"ok  \ttest/d\t0.232s\n"}
{"Time":"2024-12-13T13:06:16.463087-05:00","Action":"pass","Package":"test/d","Elapsed":0.232}
{"Time":"2024-12-13T13:06:16.630042-05:00","Action":"run","Package":"test/a","Test":"TestA"}
{"Time":"2024-12-13T13:06:16.630076-05:00","Action":"output","Package":"test/a","Test":"TestA","Output":"=== RUN   TestA\n"}
{"Time":"2024-12-13T13:06:16.630109-05:00","Action":"output","Package":"test/a","Test":"TestA","Output":"    a_test.go:6: A is okay\n"}
{"Time":"2024-12-13T13:06:16.630118-05:00","Action":"output","Package":"test/a","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n"}
{"Time":"2024-12-13T13:06:16.630123-05:00","Action":"pass","Package":"test/a","Test":"TestA","Elapsed":0}
{"Time":"2024-12-13T13:06:16.63013-05:00","Action":"output","Package":"test/a","Output":"PASS\n"}
{"Time":"2024-12-13T13:06:16.630519-05:00","Action":"output","Package":"test/a","Output":"ok  \ttest/a\t0.400s\n"}
{"Time":"2024-12-13T13:06:16.630537-05:00","Action":"pass","Package":"test/a","Elapsed":0.4}
`
	// goTestJSONBuildErrorGo123 is like goTestJSONBuildError, except it was generated with Go 1.23.
	//
	// Build output and failures are not present here, because they're sent to stderr.
	goTestJSONBuildErrorGo123 = `{"Time":"2024-12-13T14:02:10.021159-05:00","Action":"start","Package":"test/a"}
{"Time":"2024-12-13T14:02:10.021296-05:00","Action":"start","Package":"test/b"}
{"Time":"2024-12-13T14:02:10.021337-05:00","Action":"output","Package":"test/b","Output":"FAIL\ttest/b [build failed]\n"}
{"Time":"2024-12-13T14:02:10.021347-05:00","Action":"fail","Package":"test/b","Elapsed":0}
{"Time":"2024-12-13T14:02:10.021361-05:00","Action":"start","Package":"test/c"}
{"Time":"2024-12-13T14:02:10.02137-05:00","Action":"output","Package":"test/c","Output":"FAIL\ttest/c [build failed]\n"}
{"Time":"2024-12-13T14:02:10.021373-05:00","Action":"fail","Package":"test/c","Elapsed":0}
{"Time":"2024-12-13T14:02:10.021398-05:00","Action":"start","Package":"test/d"}
{"Time":"2024-12-13T14:02:10.254709-05:00","Action":"run","Package":"test/a","Test":"TestA"}
{"Time":"2024-12-13T14:02:10.254779-05:00","Action":"output","Package":"test/a","Test":"TestA","Output":"=== RUN   TestA\n"}
{"Time":"2024-12-13T14:02:10.254808-05:00","Action":"output","Package":"test/a","Test":"TestA","Output":"    a_test.go:6: A is okay\n"}
{"Time":"2024-12-13T14:02:10.254837-05:00","Action":"output","Package":"test/a","Test":"TestA","Output":"--- PASS: TestA (0.00s)\n"}
{"Time":"2024-12-13T14:02:10.254839-05:00","Action":"pass","Package":"test/a","Test":"TestA","Elapsed":0}
{"Time":"2024-12-13T14:02:10.254843-05:00","Action":"output","Package":"test/a","Output":"PASS\n"}
{"Time":"2024-12-13T14:02:10.255042-05:00","Action":"output","Package":"test/a","Output":"ok  \ttest/a\t0.234s\n"}
{"Time":"2024-12-13T14:02:10.255062-05:00","Action":"pass","Package":"test/a","Elapsed":0.234}
{"Time":"2024-12-13T14:02:10.418623-05:00","Action":"run","Package":"test/d","Test":"TestD"}
{"Time":"2024-12-13T14:02:10.418652-05:00","Action":"output","Package":"test/d","Test":"TestD","Output":"=== RUN   TestD\n"}
{"Time":"2024-12-13T14:02:10.418672-05:00","Action":"output","Package":"test/d","Test":"TestD","Output":"    d_test.go:8: D is okay\n"}
{"Time":"2024-12-13T14:02:10.418677-05:00","Action":"output","Package":"test/d","Test":"TestD","Output":"--- PASS: TestD (0.00s)\n"}
{"Time":"2024-12-13T14:02:10.41868-05:00","Action":"pass","Package":"test/d","Test":"TestD","Elapsed":0}
{"Time":"2024-12-13T14:02:10.418684-05:00","Action":"output","Package":"test/d","Output":"PASS\n"}
{"Time":"2024-12-13T14:02:10.419001-05:00","Action":"output","Package":"test/d","Output":"ok  \ttest/d\t0.398s\n"}
{"Time":"2024-12-13T14:02:10.419021-05:00","Action":"pass","Package":"test/d","Elapsed":0.398}
`

	// goTestJSONBuildWarning holds stdout output from 'go test -json -count=1 ./...'
	// that corresponds to running the following tests that involve build warnings:
	//
	//	$ go test -v -count=1 ./...
	//	# test.test
	//	ld: warning: ignoring duplicate libraries: '-lobjc'
	//	testing: warning: no tests to run
	//	PASS
	//	ok  	test	0.225s [no tests to run]
	//
	//	$ go test -count=1 ./...
	//	# test.test
	//	ld: warning: ignoring duplicate libraries: '-lobjc'
	//	ok  	test	0.216s [no tests to run]
	//
	goTestJSONBuildWarning = `{"ImportPath":"test.test","Action":"build-output","Output":"# test.test\nld: warning: ignoring duplicate libraries: '-lobjc'\n"}
{"Time":"2025-01-13T18:50:06.31898-05:00","Action":"start","Package":"test"}
{"Time":"2025-01-13T18:50:06.532973-05:00","Action":"output","Package":"test","Output":"testing: warning: no tests to run\n"}
{"Time":"2025-01-13T18:50:06.533027-05:00","Action":"output","Package":"test","Output":"PASS\n"}
{"Time":"2025-01-13T18:50:06.533313-05:00","Action":"output","Package":"test","Output":"ok  \ttest\t0.214s [no tests to run]\n"}
{"Time":"2025-01-13T18:50:06.533339-05:00","Action":"pass","Package":"test","Elapsed":0.214}
`
	// goTestJSONBuildWarningGo123 is like goTestJSONBuildWarning, except it was generated with Go 1.23.
	//
	// Build output and failures are not present here, because they're sent to stderr.
	goTestJSONBuildWarningGo123 = `{"Time":"2025-01-13T18:50:28.261264-05:00","Action":"start","Package":"test"}
{"Time":"2025-01-13T18:50:28.48058-05:00","Action":"output","Package":"test","Output":"testing: warning: no tests to run\n"}
{"Time":"2025-01-13T18:50:28.480678-05:00","Action":"output","Package":"test","Output":"PASS\n"}
{"Time":"2025-01-13T18:50:28.48109-05:00","Action":"output","Package":"test","Output":"ok  \ttest\t0.220s [no tests to run]\n"}
{"Time":"2025-01-13T18:50:28.481111-05:00","Action":"pass","Package":"test","Elapsed":0.22}
`
)

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
		{
			name:    "BuildErrorVerbose",
			verbose: true,
			input:   []byte(goTestJSONBuildError),
			expect: `=== RUN   TestA
    a_test.go:6: A is okay
--- PASS: TestA (0.00s)
PASS
ok  	test/a	0.400s
FAIL	test/b [build failed]
# test/b [test/b.test]
b/b_test.go:5:8: "os" imported and not used
FAIL	test/c [build failed]
# test/c
c/c.go:3:8: "os" imported and not used
=== RUN   TestD
    d_test.go:8: D is okay
--- PASS: TestD (0.00s)
PASS
ok  	test/d	0.232s
`,
		},
		{
			name:    "BuildErrorNonVerbose",
			verbose: false,
			input:   []byte(goTestJSONBuildError),
			expect: `ok  	test/a	0.400s
FAIL	test/b [build failed]
# test/b [test/b.test]
b/b_test.go:5:8: "os" imported and not used
FAIL	test/c [build failed]
# test/c
c/c.go:3:8: "os" imported and not used
ok  	test/d	0.232s
`,
		},
		{
			name:    "BuildErrorVerboseGo123",
			verbose: true,
			input:   []byte(goTestJSONBuildErrorGo123),
			expect: `=== RUN   TestA
    a_test.go:6: A is okay
--- PASS: TestA (0.00s)
PASS
ok  	test/a	0.234s
FAIL	test/b [build failed]
FAIL	test/c [build failed]
=== RUN   TestD
    d_test.go:8: D is okay
--- PASS: TestD (0.00s)
PASS
ok  	test/d	0.398s
`,
		},
		{
			name:    "BuildErrorNonVerboseGo123",
			verbose: false,
			input:   []byte(goTestJSONBuildErrorGo123),
			expect: `ok  	test/a	0.234s
FAIL	test/b [build failed]
FAIL	test/c [build failed]
ok  	test/d	0.398s
`,
		},
		{
			name:    "BuildWarningVerbose",
			verbose: true,
			input:   []byte(goTestJSONBuildWarning),
			expect: `testing: warning: no tests to run
PASS
ok  	test	0.214s [no tests to run]
# test.test
ld: warning: ignoring duplicate libraries: '-lobjc'
`,
		},
		{
			name:    "BuildWarningNonVerbose",
			verbose: false,
			input:   []byte(goTestJSONBuildWarning),
			expect: `ok  	test	0.214s [no tests to run]
`,
		},
		{
			name:    "BuildWarningVerboseGo123",
			verbose: true,
			input:   []byte(goTestJSONBuildWarningGo123),
			expect: `testing: warning: no tests to run
PASS
ok  	test	0.220s [no tests to run]
`,
		},
		{
			name:    "BuildWarningNonVerboseGo123",
			verbose: false,
			input:   []byte(goTestJSONBuildWarningGo123),
			expect: `ok  	test	0.220s [no tests to run]
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

func mustParseTestResult(msg string) *sinkpb.TestResult {
	var data sinkpb.TestResult
	if err := prototext.Unmarshal([]byte(msg), &data); err != nil {
		panic(err)
	}
	return &data
}
