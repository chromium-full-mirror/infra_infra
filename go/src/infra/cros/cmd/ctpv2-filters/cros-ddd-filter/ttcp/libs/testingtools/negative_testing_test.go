// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testingtools

import (
	"testing"
)

func TestExpectToFail(t *testing.T) {
	a := 1
	ExpectToFail(
		func(t testing.TB) {
			a = 2
			t.Fail()
			a = 3
		},
		t)
	Equivalent(t, a, 3)
}

func TestExpectToFailNow(t *testing.T) {
	a := 1
	ExpectToFail(
		func(t testing.TB) {
			a = 2
			t.Logf("This test will be interrupted")
			t.FailNow()
			a = 3
		},
		t)
	Equivalent(t, a, 2)
}

func TestExpectToFatal(t *testing.T) {
	a := 1
	ExpectToFail(
		func(t testing.TB) {
			a = 2
			t.Fatal()
			a = 3
		},
		t)
	Equivalent(t, a, 2)
}

func TestExpectPanic(t *testing.T) {
	a := 1
	ExpectToFail(
		func(t testing.TB) {
			a = 2
			panic("test panic")
		},
		t)
	Equivalent(t, a, 2)
}

func TestCleanupAndHelper(t *testing.T) {
	ExpectToFail(func(t testing.TB) {
		func() {
			t.Helper()
		}()
		t.Cleanup(func() {
			t.Log("Clean up test", t.Name())
		})
		t.Setenv("testKey", "testVal")
		panic("Clean up")
	},
		t)
}

func TestError(t *testing.T) {
	ExpectToFail(func(t testing.TB) {
		t.Error("error")
	}, t)
}

func TestErrorf(t *testing.T) {
	ExpectToFail(func(t testing.TB) {
		t.Errorf("%s", "error")
	}, t)
}

func TestSkip(t *testing.T) {
	ExpectToFail(func(t testing.TB) {
		t.Log("Temp dir", t.TempDir())
		t.Skipf("Skip")
		panic("skip")
	},
		t)
}

func TestExpectSkipNow(t *testing.T) {
	a := 1
	ExpectToFail(
		func(t testing.TB) {
			a = 2
			t.SkipNow()
			a = 3
		},
		t)
	Equivalent(t, a, 2)
}

func TestExpectFailFail(t *testing.T) {
	ExpectToFail(
		func(tt testing.TB) {
			ExpectToFail(
				func(ttt testing.TB) {
					Equal(ttt, 1, 1)
				},
				tt)
		},
		t)
}
