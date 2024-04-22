// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quickjs // import "modernc.org/bquickjs"

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"

	"github.com/dop251/goja"
	"github.com/shopspring/decimal"
	util "modernc.org/fileutil/ccgo"
)

var (
	goos   = runtime.GOOS
	goarch = runtime.GOARCH

	memgrind bool
)

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

const fib = `
function fib(n) {
	if (n<2) {
		return n;
	}

	return fib(n-1)+fib(n-2);
}

fib(10);
`

func TestFib(t *testing.T) {
	t.Run("ccgo", testFibCCGo)
	t.Run("goja", testFibGoja)
}

func testFibCCGo(t *testing.T) {
	m, err := NewVM()
	if err != nil {
		t.Fatal(err)
	}

	defer m.Close()

	v, err := m.Eval(fib, EvalGlobal)
	if err != nil {
		t.Fatal(err)
	}

	if g, e := v, 55; g != e {
		t.Fatalf("%T(%[1]v) %v", g, e)
	}
}

func testFibGoja(t *testing.T) {
	rt := goja.New()
	v, err := rt.RunString(fib)
	if err != nil {
		t.Fatal(err)
	}

	if g, e := v.Export().(int64), int64(55); g != e {
		t.Fatal(g, e)
	}
}

func BenchmarkArewefastyet(b *testing.B) {
	util.InDir(filepath.Join("internal", "arewefastyet", "v8-v7"), func() error {
		var src []string
		for _, fn := range arewefastyetJS {
			s, err := os.ReadFile(fn)
			if err != nil {
				b.Fatal(fn, err)
			}

			src = append(src, string(s))
		}
		src = append(src, runArewefastyet)
		b.Run("ccgo", func(b *testing.B) { benchmarkArewefastyetCCGo(b, src) })
		b.Run("goja", func(b *testing.B) { benchmarkArewefastyetGoja(b, src) })
		return nil
	})
}

const runArewefastyet = `
BenchmarkSuite.RunSuites({
	NotifyError: function(name, error) {
		throw new Error("FAIL "+name+": "+error);
	}
});
`

var arewefastyetJS = []string{
	"base.js",
	"richards.js",
	"deltablue.js",
	"crypto.js",
	"raytrace.js",
	"earley-boyer.js",
	"regexp.js",
	"splay.js",
	"navier-stokes.js",
}

func benchmarkArewefastyetCCGo(b *testing.B, src []string) {
	m, err := NewVM()
	if err != nil {
		b.Fatal(err)
	}

	defer m.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range src {
			if _, err = m.Eval(v, EvalGlobal); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func benchmarkArewefastyetGoja(b *testing.B, src []string) {
	rt := goja.New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range src {
			if _, err := rt.RunString(v); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func TestMem(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}

	if !memgrind {
		if _, err := util.Shell(nil, "go", "test", fmt.Sprintf("-short=%v", testing.Short()), "-v", "-tags", "libc.memgrind", "-timeout", "12h", "-run", "TestMemgrind"); err != nil {
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
		{false, 42},                              // Not a function
		{false, func() (a, b, c int) { return }}, // Too many return values
		{false, func() (a, b int) { return }},    // Two return values but the second is not error
		{true, func() {}},                        // Wants this but no params
	} {
		switch err := m.RegisterFunc("myfunc", test.f, test.wantThis); {
		case err != nil:
			t.Logf("registering function '%T(%[1]v)' failed as expected: err=%v", test, err)
		default:
			t.Errorf("registering function '%T(%[1]v)' should failed", test)
		}
	}
}

func testRegisterGoFuncOK(t *testing.T) {
	m, err := NewVM()
	if err != nil {
		t.Fatal(err)
	}

	defer m.Close()

	for _, test := range []struct {
		nm       string
		wantThis bool
		f        any
		args     []any
		call     string
		v        any
	}{
		{"f1", false, func() {}, nil, "", Undefined{}},
		{"f2", false, func() any { return nil }, nil, "", nil},
		{"f3", false, func() *int { return nil }, nil, "", nil},
		{"f4", false, func() *int { i := 42; return &i }, nil, "", 42},
		{"f5", false, func() float64 { return 0.5 }, nil, "", 0.5},
		{"f6", false, func() float32 { return 2.5 }, nil, "", 2.5},
		{"f7", false, func() string { return "2.5" }, nil, "", "2.5"},
	} {
		if err := m.RegisterFunc(test.nm, test.f, test.wantThis); err != nil {
			t.Errorf("registering function '%T(%[1]v)': %v", test, err)
			continue
		}

		call := test.call
		if call == "" {
			call = fmt.Sprintf("%s();", test.nm)
		}
		rv, err := m.Eval(call, EvalGlobal)
		if err != nil {
			t.Errorf("calling %s: err=%v", test.nm, err)
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
		// case *big.Int:
		// 	if g, e := x, big.NewInt(int64(test.v.(int))); g.Cmp(e) != 0 {
		// 		t.Errorf("got %v, expected %v", g, e)
		// 	}
		// case *big.Float:
		// 	if g, e := x, big.NewFloat(float64(test.v.(float64))); g.Cmp(e) != 0 {
		// 		t.Errorf("got %v, expected %v", g, e)
		// 	}
		// case decimal.Decimal:
		// 	if g, e := x.String(), test.v; g != e {
		// 		t.Errorf("got %v, expected %v", g, e)
		// 	}
		// case *Object:
		// 	if g, e := x.json, test.v; g != e {
		// 		t.Errorf("got %v, expected %v", g, e)
		// 	}
		default:
			panic(todo("%T", x))
		}
	}
}
