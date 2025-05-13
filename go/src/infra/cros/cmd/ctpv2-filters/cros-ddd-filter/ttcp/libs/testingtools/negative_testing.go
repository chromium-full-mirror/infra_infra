// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testingtools

import (
	"runtime"
	"testing"
)

type mockedTesting struct {
	TestFailed bool
	t          testing.TB
	testing.TB
	Cleaner func()
}

func (m *mockedTesting) Fail() {
	m.TestFailed = true
}

func (m *mockedTesting) Errorf(format string, args ...any) {
	m.t.Logf(format, args...)
	m.Fail()
}

func (m *mockedTesting) Cleanup(cleaner func()) {
	m.Cleaner = cleaner
}

func (m *mockedTesting) Error(args ...any) {
	m.t.Log(args...)
	m.Fail()
}

func (m *mockedTesting) FailNow() {
	m.Fail()
	runtime.Goexit()
}

func (m *mockedTesting) Failed() bool {
	return m.TestFailed
}

func (m *mockedTesting) Fatal(args ...any) {
	m.t.Log(args...)
	m.TestFailed = true
	runtime.Goexit()
}

func (m *mockedTesting) Fatalf(format string, args ...any) {
	m.t.Logf(format, args...)
	m.TestFailed = true
	runtime.Goexit()
}

func (m *mockedTesting) Helper() {
	m.t.Helper()
}

func (m *mockedTesting) Log(args ...any) {
	m.t.Log(args...)
}

func (m *mockedTesting) Logf(format string, args ...any) {
	m.t.Logf(format, args...)
}

func (m *mockedTesting) Name() string {
	return m.t.Name()
}

func (m *mockedTesting) Setenv(key, value string) {
	m.t.Setenv(key, value)
}

func (m *mockedTesting) Skip(args ...any) {
	m.t.Skip()
}

func (m *mockedTesting) SkipNow() {
	m.t.Skip()
	runtime.Goexit()
}

func (m *mockedTesting) Skipf(format string, args ...any) {
	m.t.Logf(format, args...)
	m.Skip()
}

func (m *mockedTesting) Skipped() bool {
	return m.t.Skipped()
}

func (m *mockedTesting) TempDir() string {
	return m.t.TempDir()
}

// ExpectToFail wraps a section of test code and checks that signals a test
// failure. If the code signals an error via t.Fail, t.ErrorF, t.Fatal or panic
// it will be caught and the test t will not be marked as failed. If the code
// does not call t.Fail or t.ErrorF, ExpectedToFail will mark the test t as
// failed.
func ExpectToFail(test func(t testing.TB), t testing.TB) {
	boxedTesting := mockedTesting{t: t}
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				boxedTesting.TestFailed = true
			}
			done <- struct{}{}
		}()
		test(&boxedTesting)
	}()
	<-done
	if boxedTesting.Cleaner != nil {
		boxedTesting.Cleaner()
	}
	if !boxedTesting.Skipped() {
		if !boxedTesting.Failed() {
			t.Errorf("  The test was expected to fail but did not.")
		}
	}
}
