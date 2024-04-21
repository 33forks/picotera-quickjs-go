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

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestEval(t *testing.T) {
	t.Run("eval1", testEval1)
	t.Run("eval2", testEval2)
	t.Run("eval3", testEval3)
}

func testEval1(t *testing.T) {
	rt, err := NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	defer rt.Free()

	ctx, err := rt.NewContext()
	if err != nil {
		t.Fatal(err)
	}

	defer ctx.Free()

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
		v, err := ctx.Eval(test.js, EvalGlobal)
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
	rt, err := NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	defer rt.Free()

	ctx, err := rt.NewContext()
	if err != nil {
		t.Fatal(err)
	}

	defer ctx.Free()

	ctx.AddIntrinsicBigFloat()
	ctx.AddIntrinsicBigDecimal()

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
		v, err := ctx.Eval(test.js, EvalGlobal)
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
	rt, err := NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	defer rt.Free()

	ctx, err := rt.NewContext()
	if err != nil {
		t.Fatal(err)
	}

	defer ctx.Free()

	for _, test := range []struct {
		js string
		v  string
	}{
		{`a = {}; a;`, `{}`},
		{`a = {foo: 'bar', baz: "qux", i: 42}; a;`, `{"foo":"bar","baz":"qux","i":42}`},
		{`a = []; a;`, `[]`},
		{`a = [1, 2, 3]; a;`, `[1,2,3]`},
	} {
		v, err := ctx.Eval(test.js, EvalGlobal)
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
	rt, err := NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	defer rt.Free()

	ctx, err := rt.NewContext()
	if err != nil {
		t.Fatal(err)
	}

	defer ctx.Free()

	v, err := ctx.Eval(fib, EvalGlobal)
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
	// 202403271432
	// jnml@e5-1650:~/src/modernc.org/quickjs$ make bench
	// go test -run @ -bench .
	// goos: linux
	// goarch: amd64
	// pkg: modernc.org/quickjs
	// cpu: Intel(R) Xeon(R) CPU E5-1650 v2 @ 3.50GHz
	// BenchmarkArewefastyet/ccgo-12         	       1	149414713712 ns/op	      22496 B/op	        52 allocs/op
	// BenchmarkArewefastyet/goja-12         	       1	259292254549 ns/op	27425119432 B/op	1733858709 allocs/op
	// PASS
	// ok  	modernc.org/quickjs	408.723s
	// jnml@e5-1650:~/src/modernc.org/quickjs$
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
	rt, err := NewRuntime()
	if err != nil {
		b.Fatal(err)
	}

	defer rt.Free()

	ctx, err := rt.NewContext()
	if err != nil {
		b.Fatal(err)
	}

	defer ctx.Free()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range src {
			if _, err = ctx.Eval(v, EvalGlobal); err != nil {
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
