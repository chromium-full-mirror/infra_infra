// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testingtools

import (
	"testing"
)

type RectI interface {
	area() int
}

type Rect struct {
	x int
	y int
}

func (r Rect) area() int {
	return r.x * r.y
}

func TestValueNilPointer(t *testing.T) {
	ExpectToFail(func(t testing.TB) {
		a := 5
		var expected = &a
		Equivalent(t, 5, expected)
	},
		t)
}

func TestNilNil(t *testing.T) {
	Equivalent(t, nil, nil)
}

func TestPointerSame(t *testing.T) {
	a := Rect{x: 1, y: 2}
	b := &a
	Equivalent(t, &a, b)
}

func TestPointerComparable(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			a := Rect{x: 1, y: 2}
			b := Rect{x: 1, y: 2}
			Equivalent(t, &a, &b)
		},
		t)
}

func TestNilPNilP(t *testing.T) {
	var a *Rect
	var b *Rect
	a = nil
	b = nil
	Equivalent(t, a, b)
}

func TestNilPNil(t *testing.T) {
	var a interface{}
	Equivalent(t, a, nil)
}

func TestNilNilP(t *testing.T) {
	var a interface{}
	Equivalent(t, nil, a)
}

func TestNilPStruct1(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			var a *Rect
			var b *Rect
			a = nil
			b = &Rect{x: 1, y: 2}
			Equivalent(t, a, b)
		},
		t)
}
func TestNilPStruct2(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			var a *Rect
			var b *Rect
			a = nil
			b = &Rect{x: 1, y: 2}
			Equivalent(t, b, a)
		},
		t)
}

func TestNilPPrimitive(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			var a *Rect
			var b *Rect
			a = nil
			b = &Rect{x: 2, y: 3}
			Equivalent(t, &a, &b)
		},
		t)
}

func TestNilPrimitive1(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			Equivalent(t, nil, 1)
		},
		t)
}
func TestNilPrimitive2(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			Equivalent(t, 1, nil)
		},
		t)
}

func TestNilPointerPrimitive(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			a := struct{}{}
			Equivalent(t, &a, 1)
		},
		t)
}

func TestStructEquivalent(t *testing.T) {
	a := Rect{x: 1, y: 2}
	b := Rect{x: 1, y: 2}
	Equivalent(t, a, b)
}

func TestStructnotEquivalent(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			a := Rect{x: 1, y: 4}
			b := Rect{x: 1, y: 2}
			Equivalent(t, a, b)
		},
		t)
}

type BoxedRect struct {
	a Rect
}

func TestStructWithPointerEquivalent(t *testing.T) {
	a := Rect{x: 1, y: 2}
	b := BoxedRect{a: a}
	c := BoxedRect{a: a}
	Equivalent(t, b, c)
}

func TestStructWithPointernotEquivalent(t *testing.T) {
	a := Rect{x: 1, y: 2}
	b := Rect{x: 1, y: 2}
	c := BoxedRect{a: a}
	d := BoxedRect{a: b}
	Equivalent(t, c, d)
}
func TestBoolIntEqual(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			Equal(t, true, 1)
		},
		t)
}
func TestIntIntFalseEqual(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			Equal(t, 1, 2)
		},
		t)
}
func TestIntIntTrueEqual(t *testing.T) {
	Equal(t, 1, 1)
}
func TestBoolUint64Equal(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			var expected uint64 = 32
			Equal(t, true, expected)
		},
		t)
}
func TestUint64Uint64FalseEqual(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			var expected uint64 = 32
			var computed uint64 = 31
			Equal(t, computed, expected)
		},
		t)
}
func TestUint64Uint64TrueEqual(t *testing.T) {
	var expected uint64 = 32
	var computed uint64 = 32
	Equal(t, computed, expected)
}

func TestBoolInt64Equal(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			var expected int64 = 32
			Equal(t, true, expected)
		},
		t)
}
func TestInt64Int64FalseEqual(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			var expected int64 = 32
			var computed int64 = 31
			Equal(t, computed, expected)
		},
		t)
}
func TestInt64Int64TrueEqual(t *testing.T) {
	var expected int64 = 32
	var computed int64 = 32
	Equal(t, computed, expected)
}

func TestBoolStringEqual(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			Equal(t, true, "test")
		},
		t)
}
func TestStringStringLenDiffEqual(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			Equal(t, "test long", "test")
		},
		t)
}
func TestStringStringLenEqualDiffEqual(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			Equal(t, "test", "tset")
		},
		t)
}
func TestIsNilOrInvalidFail(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			IsNilOrInvalid(t, "test")
		},
		t)
}
func TestIsNotNilValueFail(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			IsNotNil(t, nil)
		},
		t)
}
func TestIsNotNilPointerFail(t *testing.T) {
	ExpectToFail(
		func(t testing.TB) {
			var v *int = nil
			IsNotNil(t, v)
		},
		t)
}
