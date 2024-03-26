// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quickjs // import "modernc.org/bquickjs"

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dop251/goja"
	util "modernc.org/fileutil/ccgo"
)

var (
	goos   = runtime.GOOS
	goarch = runtime.GOARCH
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestEval(t *testing.T) {
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
		{"42*314;", int32(42 * 314)},
		{"'foo'+'bar';", "foobar"},
		{"42 < 314;", true},
		{"null;", nil},
		{"undefined;", Undefined{}},
		{"throw new Error('FAIL')", fmt.Errorf("Error: FAIL")},
	} {
		v, err := ctx.Eval(test.js, EvalGlobal)
		t.Logf("%T(%[1]v) %v", v, err)
		if err != nil {
			switch x := test.v.(type) {
			case error:
				if g, e := err.Error(), x.Error(); g != e {
					t.Fatal(g, e)
				}

				continue
			default:
				t.Fatal(err)
			}
		}

		if g, e := v, test.v; g != e {
			t.Fatal(g, e)
		}
	}
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

	v, err := ctx.Eval(fib, 0)
	if err != nil {
		t.Fatal(err)
	}

	if g, e := v, int32(55); g != e {
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

func BenchmarkFib(b *testing.B) {
	b.Run("ccgo", benchmarkFibCCGO)
	b.Run("goja", benchmarkFibGoja)
}

func benchmarkFibCCGO(b *testing.B) {
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
		if _, err := ctx.Eval(fib, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkFibGoja(b *testing.B) {
	rt := goja.New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := rt.RunString(fib); err != nil {
			b.Fatal(err)
		}
	}
}

func TestArewefastyet(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}

	if err := util.InDir(filepath.Join("internal", "arewefastyet", "v8-v7"), func() error {
		t.Run("ccgo", testArewefastyetCCGo)
		t.Run("goja", testArewefastyetGoja)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

const run = `
var printbuf = [];

function print(s) {
	printbuf.push(s);
}

var success = true;

function PrintResult(name, result) {
  print(name + ': ' + result);
}


function PrintError(name, error) {
  throw new Error("FAIL "+name+": "+error);
  // PrintResult(name, error);
  // success = false;
}


function PrintScore(score) {
  if (success) {
    print('----');
    print('Score (version ' + BenchmarkSuite.version + '): ' + score);
  }
}


BenchmarkSuite.RunSuites({ NotifyResult: PrintResult,
                           NotifyError: PrintError,
                           NotifyScore: PrintScore });

printbuf.join("\n");
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

func testArewefastyetCCGo(t *testing.T) {
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

	for _, fn := range arewefastyetJS {
		b, err := os.ReadFile(fn)
		if err != nil {
			t.Fatal(fn, err)
		}

		if _, err = ctx.Eval(string(b), EvalGlobal); err != nil {
			t.Fatal(err)
		}
	}

	v, err := ctx.Eval(run, 0)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(v)
}

func testArewefastyetGoja(t *testing.T) {
	rt := goja.New()
	for _, v := range arewefastyetJS {
		b, err := os.ReadFile(v)
		if err != nil {
			t.Fatal(v, err)
		}

		if _, err = rt.RunString(string(b)); err != nil {
			t.Fatal(err)
		}
	}
	v, err := rt.RunString(run)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(v.Export())
}
