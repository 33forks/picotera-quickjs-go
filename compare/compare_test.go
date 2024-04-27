// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compare // import "modernc.org/quickjs/compare"

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dop251/goja"
	util "modernc.org/fileutil/ccgo"
	"modernc.org/quickjs"
)

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
	m, err := quickjs.NewVM()
	if err != nil {
		t.Fatal(err)
	}

	defer m.Close()

	v, err := m.Eval(fib, quickjs.EvalGlobal)
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
	util.InDir(filepath.Join("..", "internal", "arewefastyet", "v8-v7"), func() error {
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
	m, err := quickjs.NewVM()
	if err != nil {
		b.Fatal(err)
	}

	defer m.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range src {
			if _, err = m.Eval(v, quickjs.EvalGlobal); err != nil {
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
