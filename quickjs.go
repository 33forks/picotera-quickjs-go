// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package quickjs is an idiomatic Go wrapper for [modernc.org/libquickjs], an
// embeddable, CGo-free Javascript engine.
//
// See also the original [C quickjs] library.
//
// # Supported platforms and architectures
//
// These combinations of GOOS and GOARCH are currently supported
//
//	OS      Arch
//	-------------
//	linux	amd64
//	linux	loong64
//
// # Builders
//
// Builder results are available at:
//
// https://modern-c.appspot.com/-/builder/?importpath=modernc.org%2fquickjs
//
// # Preliminary performance results
//
// This package vs https://pkg.go.dev/github.com/dop251/goja
//
//	goos: linux
//	goarch: amd64
//	pkg: modernc.org/quickjs
//	cpu: AMD Ryzen 9 3900X 12-Core Processor
//	BenchmarkArewefastyet/ccgo-24   1       109049381989 ns/op            22456 B/op                47 allocs/op
//	BenchmarkArewefastyet/goja-24   1       189426235514 ns/op      28172865888 B/op        1765994482 allocs/op
//	PASS
//	ok  	modernc.org/quickjs	298.488s
//
// # Notes
//
// Parts of the documentation were copied from the quickjs documentation, see
// LICENSE-QUICKJS for details.
//
// [C quickjs]: https://bellard.org/quickjs
// [modernc.org/libquickjs]: https://pkg.go.dev/modernc.org/libquickjs
package quickjs // import "modernc.org/quickjs"

import (
	"encoding/json"
	"fmt"
	"math/big"
	"unsafe"

	"github.com/shopspring/decimal"
	"modernc.org/libc"
	lib "modernc.org/libquickjs"
)

var (
	_ json.Marshaler = (*Object)(nil)

	null      = lib.TJSValue{Ftag: lib.EJS_TAG_NULL}
	undefined = lib.TJSValue{Ftag: lib.EJS_TAG_UNDEFINED}
)

// Runtime represents a Javascript runtime corresponding to an object heap.
// Several Runtimes can exist at the same time but they cannot exchange
// objects.
//
// Note: Runtime is not safe for concurrent use by multiple goroutines.
type Runtime struct {
	runtime uintptr // lib.TJSRuntime
	tls     *libc.TLS
}

// NewRuntime returns a newly create Runtime
func NewRuntime() (*Runtime, error) {
	tls := libc.NewTLS()
	runtime := lib.XJS_NewRuntime(tls)
	if runtime == 0 {
		tls.Close()
		return nil, fmt.Errorf("failed to create new Javascript runtime")
	}

	return &Runtime{runtime: runtime, tls: tls}, nil
}

// Free releases the resources held by r.
func (r *Runtime) Free() error {
	lib.XJS_FreeRuntime(r.tls, r.runtime)
	r.tls.Close()
	*r = Runtime{}
	return nil
}

// Context represents a Javascript context (or Realm). Each JSContext has its
// own global objects and system objects. There can be several Contexts per
// Runtime and they can share objects, similar to frames of the same origin
// sharing Javascript objects in a web browser.
//
// Note: Context is not safe for concurrent use by multiple goroutines.
type Context struct {
	toStringArgv uintptr
	context      uintptr // lib.TJSContext
	runtime      *Runtime
	// Safe to share, not reference counted
	int32_2  lib.TJSValue
	int32_16 lib.TJSValue
}

// NewContext returns a newly created Context.
func (r *Runtime) NewContext() (*Context, error) {
	argv := libc.Xcalloc(r.tls, 2, libc.Tsize_t(unsafe.Sizeof(lib.TJSValue{})))
	if argv == 0 {
		return nil, fmt.Errorf("OOM")
	}

	context := lib.XJS_NewContext(r.tls, r.runtime)
	if context == 0 {
		return nil, fmt.Errorf("failed to create new Javascript context")
	}

	return &Context{
		context:      context,
		int32_2:      lib.XNewInt32(r.tls, context, 2),
		int32_16:     lib.XNewInt32(r.tls, context, 16),
		runtime:      r,
		toStringArgv: argv,
	}, nil
}

// Free releases the resources held by c.
func (c *Context) Free() error {
	libc.Xfree(c.runtime.tls, c.toStringArgv)
	lib.XJS_FreeContext(c.runtime.tls, c.context)
	*c = Context{}
	return nil
}

// Eval flags.
const (
	EvalGlobal = lib.MJS_EVAL_TYPE_GLOBAL // global code
	EvalModule = lib.MJS_EVAL_TYPE_MODULE // module code
)

var evalFN = [...]byte{'<', 'e', 'v', 'a', 'l', '>', 0}

// Eval evaluates a script or module source in 'js'.
//
//	QuickJS type    result Go type                          result error
//	--------------------------------------------------------------------
//	exception       nil                                     non-nil
//	null            nil                                     nil
//	undefined       Undefined                               nil
//	string          string                                  nil
//	int             int                                     nil
//	bool            bool                                    nil
//	float64         floa64                                  nil
//	BigInt          *math/big.Int                           nil
//	BigFloat        *math/big.Float                         nil
//	BigDecimal      github.com/shopspring/decimal.Decimal   nil
//	object          *Object                                 nil
//	any other type  Unsupported                             nil
func (c *Context) Eval(js string, flags int) (r any, err error) {
	tls := c.runtime.tls
	ps, err := libc.CString(js)
	if err != nil {
		panic(err)
	}

	defer libc.Xfree(tls, ps)

	return c.value(lib.XJS_Eval(tls, c.context, ps, libc.Tsize_t(len(js)), uintptr(unsafe.Pointer(&evalFN)), int32(flags)))
}

// Object represents a Javascript object.
type Object struct {
	json string
}

// MarshalJSON implements encoding/json.Marshaler.
func (o *Object) MarshalJSON() (r []byte, err error) {
	return []byte(o.json), nil
}

func (c *Context) newObject(v lib.TJSValue) *Object {
	json := lib.XJS_JSONStringify(c.runtime.tls, c.context, v, undefined, undefined)

	defer lib.XFreeValue(c.runtime.tls, c.context, json)

	p := lib.XToCString(c.runtime.tls, c.context, json)

	defer lib.XJS_FreeCString(c.runtime.tls, c.context, p)

	return &Object{json: libc.GoString(p)}
}

// Unsupported represents an unsupported javascript value.
type Unsupported struct{}

// Undefined represents the javascript value "undefined".
type Undefined struct{}

var (
	toStringC = [...]byte{'t', 'o', 'S', 't', 'r', 'i', 'n', 'g', 0}
	toString  = uintptr(unsafe.Pointer(&toStringC[0]))
)

// value "unpacks" 'v'. FreeValue(v) is called before returning, 'v' must not
// be used afterwards.
func (c *Context) value(v lib.TJSValue) (r any, err error) {
	if v.Ftag < 0 {
		// all tags with a reference count are negative
		defer lib.XFreeValue(c.runtime.tls, c.context, v)
	}

	switch v.Ftag {
	case lib.EJS_TAG_BIG_DECIMAL: // -11,
		m := lib.XJS_GetPropertyStr(c.runtime.tls, c.context, v, toString)
		if m.Ftag != lib.EJS_TAG_OBJECT {
			panic("failed to get BigDecimal.toString()")
		}

		defer lib.XFreeValue(c.runtime.tls, c.context, m)

		jsString := lib.XJS_Call(c.runtime.tls, c.context, m, v, 0, c.toStringArgv)

		defer lib.XFreeValue(c.runtime.tls, c.context, jsString)

		p := lib.XToCString(c.runtime.tls, c.context, jsString)

		defer lib.XJS_FreeCString(c.runtime.tls, c.context, p)

		n, err := decimal.NewFromString(libc.GoString(p))
		if err != nil {
			panic(todo("decimal.NewFromString failed"))
		}

		return n, nil
	case lib.EJS_TAG_BIG_INT: // -10,
		m := lib.XJS_GetPropertyStr(c.runtime.tls, c.context, v, toString)
		if m.Ftag != lib.EJS_TAG_OBJECT {
			panic("failed to get BigInt.toString()")
		}

		defer lib.XFreeValue(c.runtime.tls, c.context, m)

		*(*lib.TJSValue)(unsafe.Pointer(c.toStringArgv)) = c.int32_16
		jsString := lib.XJS_Call(c.runtime.tls, c.context, m, v, 1, c.toStringArgv)

		defer lib.XFreeValue(c.runtime.tls, c.context, jsString)

		p := lib.XToCString(c.runtime.tls, c.context, jsString)

		defer lib.XJS_FreeCString(c.runtime.tls, c.context, p)

		n := big.NewInt(0)
		if _, ok := n.SetString(libc.GoString(p), 16); !ok {
			panic(todo("big.Int.SetString failed"))
		}

		return n, nil
	case lib.EJS_TAG_BIG_FLOAT: // -9,
		m := lib.XJS_GetPropertyStr(c.runtime.tls, c.context, v, toString)
		if m.Ftag != lib.EJS_TAG_OBJECT {
			panic("failed to get BigInt.toString()")
		}

		defer lib.XFreeValue(c.runtime.tls, c.context, m)

		*(*lib.TJSValue)(unsafe.Pointer(c.toStringArgv)) = c.int32_16
		jsString := lib.XJS_Call(c.runtime.tls, c.context, m, v, 1, c.toStringArgv)

		defer lib.XFreeValue(c.runtime.tls, c.context, jsString)

		p := lib.XToCString(c.runtime.tls, c.context, jsString)

		defer lib.XJS_FreeCString(c.runtime.tls, c.context, p)

		s := libc.GoString(p)
		n := big.NewFloat(0)
		n.SetPrec(uint(4 * len(s)))
		if _, base, err := n.Parse(s, 16); base != 16 || err != nil {
			panic(todo("big.Float.Parse failed"))
		}

		return n, nil
	case lib.EJS_TAG_STRING: // -7,
		p := lib.XToCString(c.runtime.tls, c.context, v)

		defer lib.XJS_FreeCString(c.runtime.tls, c.context, p)

		return libc.GoString(p), nil
	case lib.EJS_TAG_OBJECT: // -1
		return c.newObject(v), nil
	case lib.EJS_TAG_INT: //  0,
		return int(*(*int32)(unsafe.Pointer(&v))), nil
	case lib.EJS_TAG_BOOL: //  1,
		return *(*int32)(unsafe.Pointer(&v)) != 0, nil
	case lib.EJS_TAG_NULL: //  2,
		return nil, nil
	case lib.EJS_TAG_UNDEFINED: //  3,
		return Undefined{}, nil
	case lib.EJS_TAG_EXCEPTION: // 6,
		e := lib.XJS_GetException(c.runtime.tls, c.context)

		defer lib.XFreeValue(c.runtime.tls, c.context, e)

		p := lib.XToCString(c.runtime.tls, c.context, e)

		defer lib.XJS_FreeCString(c.runtime.tls, c.context, p)

		return nil, fmt.Errorf("%s", libc.GoString(p))
	case lib.EJS_TAG_FLOAT64: // 7,
		return *(*float64)(unsafe.Pointer(&v)), nil
	}
	return Unsupported{}, nil
}

// AddIntrinsicBigFloat adds the BigFloat object.
func (c *Context) AddIntrinsicBigFloat() {
	lib.XJS_AddIntrinsicBigFloat(c.runtime.tls, c.context)
}

// AddIntrinsicBigDecimal adds the BigDecimal object.
func (c *Context) AddIntrinsicBigDecimal() {
	lib.XJS_AddIntrinsicBigDecimal(c.runtime.tls, c.context)
}
