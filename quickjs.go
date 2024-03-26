// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package quickjs is a CGo-free wrapper of quickjs, a library
// implementing an embeddable Javascript engine.
//
// See also https://bellard.org/quickjs/
//
// Parts of the documentation were copied from the quickjs documentation, see
// LICENSE-QUICKJS for details.
package quickjs // import "modernc.org/quickjs"

import (
	"fmt"
	"unsafe"

	"modernc.org/libc"
	lib "modernc.org/libquickjs"
)

// Runtime represents a Javascript runtime corresponding to an object heap.
// Several Runtimes can exist at the same time but they cannot exchange
// objects. Inside a given Runtime, no multi-threading is supported.
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
type Context struct {
	context uintptr // lib.TJSContext
	runtime *Runtime
}

// NewContext returns a newly created Context.
func (r *Runtime) NewContext() (*Context, error) {
	context := lib.XJS_NewContext(r.tls, r.runtime)
	if context == 0 {
		return nil, fmt.Errorf("failed to create new Javascript context")
	}

	return &Context{context: context, runtime: r}, nil
}

// Free releases the resources held by c.
func (c *Context) Free() error {
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
func (c *Context) Eval(js string, flags int) (any, error) {
	tls := c.runtime.tls
	ps, err := libc.CString(js)
	if err != nil {
		panic(err)
	}

	defer libc.Xfree(tls, ps)

	return c.value(lib.XJS_Eval(tls, c.context, ps, libc.Tsize_t(len(js)), uintptr(unsafe.Pointer(&evalFN)), int32(flags)))
}

// Unsupported represent an unsupported javascript value.
type Unsupported struct{}

// Undefined represents the javascript value "undefined".
type Undefined struct{}

// value "unpacks" 'v' into (<go-value>, <exception>) and frees 'v'.
func (c *Context) value(v lib.TJSValue) (any, error) {
	defer lib.XFreeValue(c.runtime.tls, c.context, v)
	switch v.Ftag {
	/* all tags with a reference count are negative */
	// case lib.EJS_TAG_BIG_DECIMAL: // -11,
	// case lib.EJS_TAG_BIG_INT: // -10,
	// case lib.EJS_TAG_BIG_FLOAT: // -9,
	// case lib.EJS_TAG_SYMBOL: // -8,
	case lib.EJS_TAG_STRING: // -7,
		p := lib.XToCString(c.runtime.tls, c.context, v)

		defer lib.XJS_FreeCString(c.runtime.tls, c.context, p)

		return libc.GoString(p), nil
	// case lib.EJS_TAG_MODULE: // -3, /* used internally */
	// case lib.EJS_TAG_FUNCTION_BYTECODE: //  -2, /* used internally */
	// case lib.EJS_TAG_OBJECT: // -1,
	case lib.EJS_TAG_INT: //  0,
		return *(*int32)(unsafe.Pointer(&v)), nil
	case lib.EJS_TAG_BOOL: //  1,
		return *(*int32)(unsafe.Pointer(&v)) != 0, nil
	case lib.EJS_TAG_NULL: //  2,
		return nil, nil
	case lib.EJS_TAG_UNDEFINED: //  3,
		return Undefined{}, nil
	// case lib.EJS_TAG_UNINITIALIZED: // 4,
	// case lib.EJS_TAG_CATCH_OFFSET: // 5,
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
