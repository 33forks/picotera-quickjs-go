// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quickjs // import "modernc.org/quickjs"

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"runtime/debug"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	util "modernc.org/fileutil/ccgo"
)

var (
	memgrind bool
)

//lint:ignore U1000 debug helper
func stack() []byte { return debug.Stack() }

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestEval(t *testing.T) {
	t.Run("eval1", testEval1)
	t.Run("eval2", testEval2)
	t.Run("eval3", testEval3)
}

func testEval1(t *testing.T) {
	m, err := NewVM()
	if err != nil {
		t.Fatal(err)
	}

	defer m.Close()

	for _, test := range []struct {
		js string
		v  any
	}{
		{"42*314;", 42 * 314},
		{"'foo'+'bar';", "foobar"},
		{"42 < 314;", true},
		{"null;", nil},
		{"undefined;", Undefined{}},
		{"throw new Error('FAIL')", fmt.Errorf("Error: FAIL")},
	} {
		v, err := m.Eval(test.js, EvalGlobal)
		t.Logf("%s: %T(%[1]v) %v", test.js, v, err)
		if err != nil {
			switch x := test.v.(type) {
			case error:
				if g, e := err.Error(), x.Error(); g != e {
					t.Fatalf("FAIL %s: %v %v", test.js, g, e)
				}

				continue
			default:
				t.Errorf("FAIL %s: %v", test.js, err)
				continue
			}
		}

		if g, e := v, test.v; g != e {
			t.Fatalf("FAIL %s: %T(%[1]v) %T(%[2]v)", test.js, g, e)
		}
	}
}

func testEval2(t *testing.T) {
	m, err := NewVM()
	if err != nil {
		t.Fatal(err)
	}

	defer m.Close()

	m.AddIntrinsicBigFloat()
	m.AddIntrinsicBigDecimal()

	for _, test := range []struct {
		js string
		v  any
		sv string
	}{
		{"BigInt('1234567890123456789')", newBigInt(t, "1234567890123456789"), "1234567890123456789"},
		{"BigInt('-1234567890123456789')", newBigInt(t, "-1234567890123456789"), "-1234567890123456789"},
		{"1234567890123456789n", newBigInt(t, "1234567890123456789"), "1234567890123456789"},
		{"-1234567890123456789n", newBigInt(t, "-1234567890123456789"), "-1234567890123456789"},
		{"BigFloat('1234567890.123456789e+5')", newBigFloat(t, "1234567890.123456789e+5"), "1.234567890123456789e+14"},
		{"BigFloat('-1234567890.123456789e+5')", newBigFloat(t, "-1234567890.123456789e+5"), "-1.23456789012345678899999999999999994e+14"},
		{"BigDecimal('1234567890.123456789')", newBigDecimal(t, "1234567890.123456789"), "1234567890.123456789"},
		{"BigDecimal('1234567890.123456789')", newBigDecimal(t, "1234567890.123456789"), "1234567890.123456789"},
		{"1234567890.123456789m", newBigDecimal(t, "1234567890.123456789"), "1234567890.123456789"},
		{"-1234567890.123456789m", newBigDecimal(t, "-1234567890.123456789"), "-1234567890.123456789"},
	} {
		v, err := m.Eval(test.js, EvalGlobal)
		t.Logf("%s: %T(%[1]v) %v", test.js, v, err)
		if err != nil {
			t.Errorf("FAIL %s: %v", test.js, err)
			continue
		}

		if g, e := fmt.Sprintf("%T", v), fmt.Sprintf("%T", test.v); g != e {
			t.Errorf("FAIL %s: %T(%[1]v) %T(%[2]v)", test.js, g, e)
			continue
		}

		if g, e := fmt.Sprint(v), test.sv; g != e {
			t.Errorf("FAIL %v: %T(%[1]v) %T(%[2]v)", test.js, g, e)
			continue
		}
	}
}

func testEval3(t *testing.T) {
	m, err := NewVM()
	if err != nil {
		t.Fatal(err)
	}

	defer m.Close()

	for _, test := range []struct {
		js string
		v  string
	}{
		{`a = {}; a;`, `{}`},
		{`a = {foo: 'bar', baz: "qux", i: 42}; a;`, `{"foo":"bar","baz":"qux","i":42}`},
		{`a = []; a;`, `[]`},
		{`a = [1, 2, 3]; a;`, `[1,2,3]`},
	} {
		v, err := m.Eval(test.js, EvalGlobal)
		t.Logf("js=`%s`: v=%T(%[2]v) err=%T(%[3]v)", test.js, v, err)
		if err != nil {
			t.Errorf("FAIL js=`%s`: err=%v", test.js, err)
			continue
		}

		switch x := v.(type) {
		case *Object:
			if g, e := string(x.json), test.v; g != e {
				t.Errorf("got=`%s` expected=`%s`", g, e)
			}
		default:
			t.Errorf("unexpected result type: %T", x)
		}
	}
}

func TestCall0(t *testing.T) {
	t.Run("call1", testCall1)
	t.Run("call2", testCall2)
	t.Run("call3", testCall3)
}

func testCall1(t *testing.T) {
	m, err := NewVM()
	if err != nil {
		t.Fatal(err)
	}

	defer m.Close()

	for _, test := range []struct {
		js string
	}{
		{`42;`},
		{`var a = 42; a;`},
		{`var a = { }; a;`},
	} {
		v, err := m.Call(test.js)
		t.Logf("js=`%s`: v=%T(%[2]v) err=%T(%[3]v)", test.js, v, err)
		if err == nil {
			t.Errorf("FAIL js=`%s`: expected non nil err", test.js)
		}
	}
}

func testCall2(t *testing.T) {
	m, err := NewVM()
	if err != nil {
		t.Fatal(err)
	}

	defer m.Close()

	m.AddIntrinsicBigFloat()
	m.AddIntrinsicBigDecimal()

	for _, test := range []struct {
		js string
		v  any
	}{
		{`function f() { return null; }; f`, nil},
		{`function f() { return undefined; }; f`, Undefined{}},
		{`function f() { return "foo"; }; f`, "foo"},
		{`function f() { return 42; }; f`, 42},
		{`function f() { return true; }; f`, true},
		{`function f() { return 3.14; }; f`, 3.14},
		{`function f() { return 1n; }; f`, 1},
		{`function f() { return BigFloat('0.5'); }; f`, 0.5},
		{`function f() { return 12.34m; }; f`, "12.34"},
		{`function f() { return {1:2,3:4}; }; f`, `{"1":2,"3":4}`},
	} {
		v, err := m.Call(test.js)
		t.Logf("js=`%s`: v=%T(%[2]v) err=%T(%[3]v)", test.js, v, err)
		if err != nil {
			t.Errorf("FAIL js=`%s`: err=%v", test.js, err)
		}

		switch x := v.(type) {
		case nil:
			if test.v != nil {
				t.Errorf("expected nil")
			}
		case Undefined, string, int, bool, float64:
			if g, e := x, test.v; g != e {
				t.Errorf("got %v, expected %v", g, e)
			}
		case *big.Int:
			if g, e := x, big.NewInt(int64(test.v.(int))); g.Cmp(e) != 0 {
				t.Errorf("got %v, expected %v", g, e)
			}
		case *big.Float:
			if g, e := x, big.NewFloat(float64(test.v.(float64))); g.Cmp(e) != 0 {
				t.Errorf("got %v, expected %v", g, e)
			}
		case decimal.Decimal:
			if g, e := x.String(), test.v; g != e {
				t.Errorf("got %v, expected %v", g, e)
			}
		case *Object:
			if g, e := x.json, test.v; g != e {
				t.Errorf("got %v, expected %v", g, e)
			}
		default:
			panic(todo("%T", x))
		}
	}
}

func testCall3(t *testing.T) {
	m, err := NewVM()
	if err != nil {
		t.Fatal(err)
	}

	defer m.Close()

	m.AddIntrinsicBigFloat()
	m.AddIntrinsicBigDecimal()

	type T struct {
		A int
		B string
	}

	for _, test := range []struct {
		js   string
		args []any
		v    any
	}{
		{`function f(a) { return a; }; f`, nil, Undefined{}},
		{`function f(a) { return a; }; f`, []any{nil}, nil},
		{`function f(a) { return a; }; f`, []any{Undefined{}}, Undefined{}},
		{`function f(a) { return a; }; f`, []any{123}, 123},
		{`function f(a) { return a; }; f`, []any{1234567890123}, 1.234567890123e+12},
		{`function f(a) { return a; }; f`, []any{true}, true},
		{`function f(a) { return a; }; f`, []any{"foo"}, "foo"},
		{`function f(a) { return a; }; f`, []any{big.NewInt(42)}, 42},
		{`function f(a) { return a; }; f`, []any{big.NewFloat(0.5)}, 0.5},
		{`function f(a) { return a; }; f`, []any{decimal.NewFromInt(42)}, "42"},
		{`function f(a, b, c, d) { return {a: a, b: b, c: c, d: d}; }; f`, []any{"aa", 11, "cc", 22}, `{"a":"aa","b":11,"c":"cc","d":22}`},
		{`function f(a) { return a; }; f`, []any{T{11, "aa"}}, `{"A":11,"B":"aa"}`},
		{`function f(a) { return a.A; }; f`, []any{T{11, "aa"}}, 11},
		{`function f(a) { return a; }; f`, []any{&T{11, "aa"}}, `{"A":11,"B":"aa"}`},
		{`var obj={"a":42, foo() { return obj.a; }}; obj.foo`, nil, 42},
	} {
		v, err := m.Call(test.js, test.args...)
		t.Logf("js=`%s`: v=%T(%[2]v) err=%T(%[3]v)", test.js, v, err)
		if err != nil {
			t.Errorf("FAIL js=`%s`: err=%v", test.js, err)
		}

		switch x := v.(type) {
		case nil:
			if test.v != nil {
				t.Errorf("expected nil")
			}
		case Undefined, string, int, bool, float64:
			if g, e := x, test.v; g != e {
				t.Errorf("got %v, expected %v", g, e)
			}
		case *big.Int:
			if g, e := x, big.NewInt(int64(test.v.(int))); g.Cmp(e) != 0 {
				t.Errorf("got %v, expected %v", g, e)
			}
		case *big.Float:
			if g, e := x, big.NewFloat(float64(test.v.(float64))); g.Cmp(e) != 0 {
				t.Errorf("got %v, expected %v", g, e)
			}
		case decimal.Decimal:
			if g, e := x.String(), test.v; g != e {
				t.Errorf("got %v, expected %v", g, e)
			}
		case *Object:
			if g, e := x.json, test.v; g != e {
				t.Errorf("got %v, expected %v", g, e)
			}
		default:
			panic(todo("%T", x))
		}
	}
}

func newBigInt(t *testing.T, s string) *big.Int {
	n := big.NewInt(0)
	if _, ok := n.SetString(s, 10); !ok {
		t.Fatalf("big.Int.SetString(%q, 10) failed", s)
	}

	return n
}

func newBigFloat(t *testing.T, s string) *big.Float {
	n := big.NewFloat(0)
	n.SetPrec(128)
	if _, ok := n.SetString(s); !ok {
		t.Fatalf("big.Float.SetString(%q) failed", s)
	}

	return n
}

func newBigDecimal(t *testing.T, s string) decimal.Decimal {
	n, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("decimal.NewFromString(%q) failed", s)
	}

	return n
}

func TestMem(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}

	if !memgrind {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)

		defer cancel()

		if _, err := util.Shell(ctx, "go", "test", fmt.Sprintf("-short=%v", testing.Short()), "-v", "-tags", "libc.memgrind", "-timeout", "12h", "-run", "TestMemgrind"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRegisterGoFunc(t *testing.T) {
	t.Run("fnFail", testRegisterGoFuncMustFail)
	t.Run("fnOK", testRegisterGoFuncOK)
}

func testRegisterGoFuncMustFail(t *testing.T) {
	m, err := NewVM()
	if err != nil {
		t.Fatal(err)
	}

	defer m.Close()

	for _, test := range []struct {
		wantThis bool
		f        any
	}{
		{false, 42},       // Not a function
		{true, func() {}}, // Wants this but no parameters
	} {
		switch err := m.RegisterFunc("myfunc", test.f, test.wantThis); {
		case err != nil:
			t.Logf("registering function '%T(%[1]v)' failed as expected: err=%v", test, err)
		default:
			t.Errorf("registering should have failed: %T", test.f)
		}
	}
}

func testRegisterGoFuncOK(t *testing.T) {
	m, err := NewVM()
	if err != nil {
		t.Fatal(err)
	}

	defer m.Close()

	m.AddIntrinsicBigFloat()
	m.AddIntrinsicBigDecimal()

	obj, err := m.Eval("obj = {a: 42, b: 'foo'}; obj;", EvalGlobal)
	if err != nil {
		t.Fatalf("obj: %v", err)
	}

	for i, test := range []struct {
		nm       string
		wantThis bool
		f        any
		args     []any
		call     string
		v        any
	}{
		{"", false, func() {}, nil, "", Undefined{}},
		{"", false, func() any { return nil }, nil, "", nil},
		{"", false, func() any { return Undefined{} }, nil, "", Undefined{}},
		{"", false, func() any { return fmt.Errorf("abc") }, nil, "", "abc"},
		{"", false, func() any { return error(nil) }, nil, "", nil},
		{"", false, func() error { return fmt.Errorf("abc") }, nil, "", "abc"},
		{"", false, func() error { return nil }, nil, "", nil},
		{"", false, func() *int { return nil }, nil, "", nil},
		{"", false, func() any { i := 42; return &i }, nil, "", 42},
		{"", false, func() *int { i := 42; return &i }, nil, "", 42},
		{"", false, func() float64 { return 0.5 }, nil, "", 0.5},
		{"", false, func() float32 { return 2.5 }, nil, "", 2.5},
		{"", false, func() string { return "2.5" }, nil, "", "2.5"},
		{"", false, func() any { return big.NewInt(42) }, nil, "", 42},
		{"", false, func() *big.Int { return big.NewInt(42) }, nil, "", 42},
		{"", false, func() any { return big.NewFloat(0.5) }, nil, "", 0.5},
		{"", false, func() *big.Float { return big.NewFloat(0.5) }, nil, "", 0.5},
		{"", false, func() any { return decimal.NewFromInt(42) }, nil, "", "42"},
		{"", false, func() decimal.Decimal { return decimal.NewFromInt(42) }, nil, "", "42"},
		{"", false, func() any { return obj }, nil, "", `{"a":42,"b":"foo"}`},
		{"", false, func() any { return []any{42, "foo"} }, nil, "", `[42,"foo"]`},
		{"", false, func() []any { return []any{42, "foo"} }, nil, "", `[42,"foo"]`},
		{"", false, func() []int { return []int{42, 314} }, nil, "", `[42,314]`},
		{"", false, func() []any { return []any{42, obj, "foo"} }, nil, "", `[42,{"a":42,"b":"foo"},"foo"]`},
		{"", false, func() (int, string) { return 42, "foo" }, nil, "", `[42,"foo"]`},
		{"", true, func(any) {}, nil, "", Undefined{}},
		{"g1", true, func(this any) any { return this }, nil, "var a = {foo: 42, bar: g1}; a.bar();", `{"foo":42}`},
		{"g2", false, func(in any) any { return in }, nil, "g2(obj)", `{"a":42,"b":"foo"}`},
		{"g3", false, func(i int, s string) (string, int) { return s, i }, nil, "g3(42, 'foo')", `["foo",42]`},
		{"g4", false, func(in ...any) []any { return in }, nil, "g4(42, 'foo')", `[42,"foo"]`},
		{"g5", false, func(s string, args ...any) string { return fmt.Sprintf(s, args...) }, nil, "g5('hello %v %q', 42, 'foo')", `hello 42 "foo"`},
		{"g6", false, func(in ...any) []any { return in }, nil, "g4()", `[]`},
		{"g7", false, func(s string, args ...any) string { return fmt.Sprintf(s, args...) }, nil, "g5('hello %v')", `hello %!v(MISSING)`},
	} {
		nm := test.nm
		if nm == "" {
			nm = fmt.Sprintf("f%v", i)
		}
		if err := m.RegisterFunc(nm, test.f, test.wantThis); err != nil {
			t.Errorf("registering function '%T(%[1]v)': %v", test, err)
			continue
		}

		call := test.call
		if call == "" {
			call = fmt.Sprintf("%s()", nm)
		}
		rv, err := m.Eval(call, EvalGlobal)
		t.Logf("%s: %T(%[2]v)", call, rv)
		if err != nil {
			t.Errorf("calling %s: err=%v", nm, err)
			continue
		}

		switch x := rv.(type) {
		case nil:
			if test.v != nil {
				t.Errorf("expected nil")
			}
		case Undefined, string, int, bool, float64:
			if g, e := x, test.v; g != e {
				t.Errorf("got %v, expected %v", g, e)
			}
		case *big.Int:
			if g, e := x, big.NewInt(int64(test.v.(int))); g.Cmp(e) != 0 {
				t.Errorf("got %v, expected %v", g, e)
			}
		case *big.Float:
			if g, e := x, big.NewFloat(float64(test.v.(float64))); g.Cmp(e) != 0 {
				t.Errorf("got %v, expected %v", g, e)
			}
		case decimal.Decimal:
			if g, e := x.String(), test.v; g != e {
				t.Errorf("got %v, expected %v", g, e)
			}
		case *Object:
			if g, e := x.json, test.v; g != e {
				t.Errorf("%s: got %v, expected %v", nm, g, e)
			}
		default:
			panic(todo("%T", x))
		}
	}
}

func TestEvalValue(t *testing.T) {
	m, err := NewVM()
	if err != nil {
		t.Fatal(err)
	}

	defer m.Close()

	v, err := m.EvalValue("obj = {foo:42,bar:'baz'}; obj;", EvalGlobal)
	if err != nil {
		t.Fatal(err)
	}

	defer v.Free()
}
