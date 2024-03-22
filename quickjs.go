// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package quickjs is a CGo-free wrapper of quickjs, a library
// implementing an embeddable Javascript engine.
//
// See also https://bellard.org/quickjs/
//
// Portions of documentation are copied from the quickjs documentaion, see
// LICENSE-QUICKJS for details.
package quickjs // import "modernc.org/quickjs"

import (
	"fmt"

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

// DupValue returns a copy of v while updating its associated reference count.
func (c *Context) DupValue(v Value) Value {
	return Value{val: lib.XDupValue(c.runtime.tls, c.context, v.val)}
}

// FreeValue updates the associated reference count of v and releases its
// resources when the count becomes zero.
func (c *Context) FreeValue(v Value) {
	lib.XFreeValue(c.runtime.tls, c.context, v.val)
}

// Value represents a Javascript value which can be a primitive type or an
// object. Reference counting is used, so it is important to explicitly
// duplicate (Context.DupValue(), increment the reference count) or free
// (Context.FreeValue(), decrement the reference count) Values.
//
// Using a directly copied Value is not supported.
type Value struct {
	val lib.TJSValue
}

// Eval evaluates a script or module source in 'js', pretending it originates
// from 'filename'.
func (c *Context) Eval(js, filename string, flags int) Value {
	tls := c.runtime.tls
	ps, err := libc.CString(js)
	if err != nil {
		panic(err)
	}

	defer libc.Xfree(tls, ps)

	pfilename, err := libc.CString(filename)
	if err != nil {
		panic(err)
	}

	defer libc.Xfree(tls, pfilename)

	return Value{val: lib.XJS_Eval(tls, c.context, ps, libc.Tsize_t(len(js)), pfilename, int32(flags))}
}
