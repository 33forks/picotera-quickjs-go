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

	memgrind bool
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
		{"42*314;", 42 * 314},
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
	if !memgrind {
		if _, err := util.Shell(nil, "go", "test", fmt.Sprintf("-short=%v", testing.Short()), "-v", "-tags", "libc.memgrind", "-timeout", "12h", "-run", "TestMemgrind"); err != nil {
			t.Fatal(err)
		}
	}
}
