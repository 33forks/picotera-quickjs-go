// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quickjs // import "modernc.org/bquickjs"

import (
	"os"
	"runtime"
	"testing"

	"github.com/dop251/goja"
	lib "modernc.org/libquickjs"
)

var (
	goos   = runtime.GOOS
	goarch = runtime.GOARCH
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func Test0(t *testing.T) {
	rt, err := NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	ctx, err := rt.NewContext()
	if err != nil {
		t.Fatal(err)
	}

	v := ctx.Eval(`42 * 314`, "test.js", 0)
	if g, e := v.val.Ftag, int64(lib.EJS_TAG_INT); g != e {
		t.Logf("%+v", v)
		t.Fatal(g, e)
	}

	if g, e := v.val.Fu.Fint321, int32(42*314); g != e {
		t.Logf("%+v", v)
		t.Fatal(g, e)
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

func TestFibCCGo(t *testing.T) {
	rt, err := NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	ctx, err := rt.NewContext()
	if err != nil {
		t.Fatal(err)
	}

	v := ctx.Eval(fib, "fib.js", 0)
	if g, e := v.val.Ftag, int64(lib.EJS_TAG_INT); g != e {
		t.Logf("%+v", v)
		t.Fatal(g, e)
	}

	if g, e := v.val.Fu.Fint321, int32(55); g != e {
		t.Logf("%+v", v)
		t.Fatal(g, e)
	}
}

func TestFibGoja(t *testing.T) {
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
		ctx.Eval(fib, "fib.js", 0)
	}
}

func benchmarkFibGoja(b *testing.B) {
	rt := goja.New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.RunString(fib)
	}
}
