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
//	linux   amd64
//	linux   loong64
//
// # Builders
//
// Builder results are available at:
//
// https://modern-c.appspot.com/-/builder/?importpath=modernc.org%2fquickjs
//
// # Performance
//
// This package vs https://pkg.go.dev/github.com/dop251/goja
//
//	goos: linux
//	goarch: amd64
//	pkg: modernc.org/quickjs
//	cpu: AMD Ryzen 9 3900X 12-Core Processor
//	BenchmarkArewefastyet/ccgo-24    1    114833264962 ns/op          22808 B/op            70 allocs/op
//	BenchmarkArewefastyet/goja-24    1    188090359173 ns/op    28283063392 B/op    1771005768 allocs/op
//	PASS
//	ok  modernc.org/quickjs 302.936s
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
	"math"
	"math/big"
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/shopspring/decimal"
	"modernc.org/libc"
	lib "modernc.org/libquickjs"
)

var (
	_ json.Marshaler = (*Object)(nil)

	errorInterface = reflect.TypeOf((*error)(nil)).Elem()
	null           = lib.TJSValue{Ftag: lib.EJS_TAG_NULL}
	undefined      = lib.TJSValue{Ftag: lib.EJS_TAG_UNDEFINED}

	globalMagic atomic.Int32

	goFuncsMu sync.Mutex
	goFuncs   = map[int32]*goFunc{}
)

// VM represents a single Context Runtime. It has all Context methods promoted.
type VM struct {
	*Context
}

// NewVM returns a newly created VM.
func NewVM() (*VM, error) {
	r, err := NewRuntime()
	if err != nil {
		return nil, err
	}

	c, err := r.NewContext()
	if err != nil {
		r.Close()
		return nil, err
	}

	return &VM{Context: c}, nil
}

// Close releases the resources held by 'm'.
func (m *VM) Close() (err error) {
	r := m.runtime

	if err = m.Context.Close(); err != nil {
		return err
	}

	return r.Close()
}

// Runtime represents a Javascript runtime corresponding to an object heap.
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

// Close releases the resources held by r.
func (r *Runtime) Close() error {
	lib.XJS_FreeRuntime(r.tls, r.runtime)
	r.tls.Close()
	*r = Runtime{}
	return nil
}

// Context represents a Javascript context (or Realm). Each Context has its
// own global objects and system objects. There can be several Contexts per
// Runtime.
//
// Note: Context is not safe for concurrent use by multiple goroutines.
type Context struct {
	// Safe to share, not reference counted
	int32_2      lib.TJSValue
	int32_16     lib.TJSValue
	context      uintptr // lib.TJSContext
	goFuncs      map[string]int32
	runtime      *Runtime
	toStringArgv uintptr
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

// Close releases the resources held by c.
func (c *Context) Close() error {
	tls := c.runtime.tls
	var a []int32
	var b []*goFunc
	for _, k := range c.goFuncs {
		a = append(a, k)
	}
	goFuncsMu.Lock()
	for _, k := range a {
		b = append(b, goFuncs[k])
		delete(goFuncs, k)
	}
	goFuncsMu.Unlock()

	for _, v := range b {
		libc.Xfree(tls, v.cname)
		libc.Xfree(tls, v.tab)
	}

	libc.Xfree(tls, c.toStringArgv)
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

// Eval evaluates a script or module source in 'javascript'.
//
//	Javascript result type  Go result type                          Go result error
//	-------------------------------------------------------------------------------
//	exception               nil                                     non-nil
//	null                    nil                                     nil
//	undefined               Undefined                               nil
//	string                  string                                  nil
//	int                     int                                     nil
//	bool                    bool                                    nil
//	float64                 float64                                 nil
//	BigInt                  *math/big.Int                           nil
//	BigFloat                *math/big.Float                         nil
//	BigDecimal              github.com/shopspring/decimal.Decimal   nil
//	object                  *Object                                 nil
//	any other type          Unsupported                             nil
func (c *Context) Eval(javascript string, flags int) (r any, err error) {
	tls := c.runtime.tls
	ps, err := libc.CString(javascript)
	if err != nil {
		panic(err)
	}

	defer libc.Xfree(tls, ps)

	return c.value(lib.XJS_Eval(tls, c.context, ps, libc.Tsize_t(len(javascript)), uintptr(unsafe.Pointer(&evalFN)), int32(flags)))
}

func (c *Context) eval(js string, flags int) lib.TJSValue {
	tls := c.runtime.tls
	ctx := c.context
	ps, err := libc.CString(js)
	if err != nil {
		panic(err)
	}

	defer libc.Xfree(tls, ps)

	return lib.XJS_Eval(tls, ctx, ps, libc.Tsize_t(len(js)), uintptr(unsafe.Pointer(&evalFN)), int32(flags))
}

func (c *Context) globalObject() lib.TJSValue {
	return lib.XJS_GetGlobalObject(c.runtime.tls, c.context)
}

// Call evaluates 'function(args...)' and returns the resulting (value, error).
//
// Argument types must be one of:
//
//	Go argument type                        Javascript argument type
//	----------------------------------------------------------------
//	nil                                     null
//	Undefined                               undefined
//	string                                  string
//	int*/uint* (value in int32 range)       int
//	int*/uint* (value out of int32 range)   float
//	bool                                    bool
//	float64                                 float64
//	*math/big.Int                           BigInt
//	*math/big.Float                         BigFloat
//	github.com/shopspring/decimal.Decimal   BigDecimal
//	*Object                                 object
//	any other type                          object from JSON produced by encoding.json/Marshall(arg)
func (c *Context) Call(function string, args ...any) (r any, err error) {
	tls := c.runtime.tls
	ctx := c.context
	ps, err := libc.CString(function)
	if err != nil {
		panic(err)
	}

	defer libc.Xfree(tls, ps)

	f := lib.XJS_Eval(tls, ctx, ps, libc.Tsize_t(len(function)), uintptr(unsafe.Pointer(&evalFN)), int32(EvalGlobal))

	defer lib.XFreeValue(tls, ctx, f)

	g := c.globalObject()

	defer lib.XFreeValue(tls, ctx, g)

	return c.call(f, g, args...)
}

func newInt32(n int32) (r lib.TJSValue) {
	*(*int32)(unsafe.Pointer(&r)) = n
	r.Ftag = lib.EJS_TAG_INT
	return r
}

func newFloat(n float64) (r lib.TJSValue) {
	*(*float64)(unsafe.Pointer(&r)) = n
	r.Ftag = lib.EJS_TAG_FLOAT64
	return r
}

func (c *Context) newString(s string) (r lib.TJSValue) {
	tls := c.runtime.tls
	ctx := c.context
	p := mustCString(s)

	defer libc.Xfree(tls, p)

	return lib.XJS_NewStringLen(tls, ctx, p, lib.Tsize_t(len(s)))
}

func newBool(n bool) (r lib.TJSValue) {
	switch {
	case n:
		*(*int32)(unsafe.Pointer(&r)) = 1
	default:
		*(*int32)(unsafe.Pointer(&r)) = 0
	}
	r.Ftag = lib.EJS_TAG_BOOL
	return r
}

func (c *Context) call(f, this lib.TJSValue, args ...any) (r any, err error) {
	tls := c.runtime.tls
	ctx := c.context
	if lib.XJS_IsFunction(tls, ctx, f) == 0 {
		return nil, fmt.Errorf("cannot call a non-function")
	}

	var jsArgs []lib.TJSValue
	for _, v := range args {
		switch x := v.(type) {
		case nil:
			jsArgs = append(jsArgs, null)
		case Undefined:
			jsArgs = append(jsArgs, undefined)
		case int8:
			jsArgs = append(jsArgs, newInt32(int32(x)))
		case uint8:
			jsArgs = append(jsArgs, newInt32(int32(x)))
		case int16:
			jsArgs = append(jsArgs, newInt32(int32(x)))
		case uint16:
			jsArgs = append(jsArgs, newInt32(int32(x)))
		case int32:
			jsArgs = append(jsArgs, newInt32(int32(x)))
		case uint32:
			switch {
			case x <= math.MaxInt32:
				jsArgs = append(jsArgs, newInt32(int32(x)))
			default:
				jsArgs = append(jsArgs, newFloat(float64(x)))
			}
		case int:
			switch {
			case x >= math.MinInt32 && x <= math.MaxInt32:
				jsArgs = append(jsArgs, newInt32(int32(x)))
			default:
				jsArgs = append(jsArgs, newFloat(float64(x)))
			}
		case uint:
			switch {
			case x <= math.MaxInt32:
				jsArgs = append(jsArgs, newInt32(int32(x)))
			default:
				jsArgs = append(jsArgs, newFloat(float64(x)))
			}
		case int64:
			switch {
			case x >= math.MinInt32 && x <= math.MaxInt32:
				jsArgs = append(jsArgs, newInt32(int32(x)))
			default:
				jsArgs = append(jsArgs, newFloat(float64(x)))
			}
		case uint64:
			switch {
			case x <= math.MaxInt32:
				jsArgs = append(jsArgs, newInt32(int32(x)))
			default:
				jsArgs = append(jsArgs, newFloat(float64(x)))
			}
		case bool:
			jsArgs = append(jsArgs, newBool(x))
		case string:
			p, err := libc.CString(x)
			if err != nil {
				return nil, err
			}

			defer libc.Xfree(tls, p)

			jv := lib.XJS_NewStringLen(tls, ctx, p, libc.Tsize_t(len(x)))

			defer lib.XFreeValue(tls, ctx, jv)

			jsArgs = append(jsArgs, jv)
		case *big.Int:
			s := x.String() + "n"
			jv := c.eval(s, EvalGlobal)

			defer lib.XFreeValue(tls, ctx, jv)

			jsArgs = append(jsArgs, jv)
		case *big.Float:
			s := fmt.Sprintf("BigFloat('%s')", x.String())
			jv := c.eval(s, EvalGlobal)

			defer lib.XFreeValue(tls, ctx, jv)

			jsArgs = append(jsArgs, jv)
		case decimal.Decimal:
			s := x.String() + "m"
			jv := c.eval(s, EvalGlobal)

			defer lib.XFreeValue(tls, ctx, jv)

			jsArgs = append(jsArgs, jv)
		default:
			b, err := json.Marshal(x)
			if err != nil {
				return nil, err
			}

			p, err := libc.CString(string(b))
			if err != nil {
				return nil, err
			}

			defer libc.Xfree(tls, p)

			jv := lib.XJS_ParseJSON(tls, ctx, p, libc.Tsize_t(len(b)), uintptr(unsafe.Pointer(&evalFN)))

			defer lib.XFreeValue(tls, ctx, jv)

			jsArgs = append(jsArgs, jv)
		}
	}

	var argv uintptr
	if len(jsArgs) != 0 {
		sz := libc.Tsize_t(unsafe.Sizeof(lib.TJSValue{}) * uintptr(len(jsArgs)))
		argv = libc.Xmalloc(tls, sz)

		defer libc.Xfree(tls, argv)

		for i, v := range jsArgs {
			unsafe.Slice((*lib.TJSValue)(unsafe.Pointer(argv)), len(jsArgs))[i] = v
		}

	}
	return c.value(lib.XJS_Call(tls, ctx, f, this, int32(len(jsArgs)), argv))
}

// Object represents the value of a Javascript object, but not the javascript
// object instance itself.
type Object struct {
	json               string
	forceNonComparable []byte
}

// MarshalJSON implements encoding/json.Marshaler.
func (o *Object) MarshalJSON() (r []byte, err error) {
	return []byte(o.json), nil
}

func (c *Context) newObject(v lib.TJSValue) *Object {
	tls := c.runtime.tls
	ctx := c.context
	json := lib.XJS_JSONStringify(tls, ctx, v, undefined, undefined)

	defer lib.XFreeValue(tls, ctx, json)

	p := lib.XToCString(tls, ctx, json)

	defer lib.XJS_FreeCString(tls, ctx, p)

	return &Object{json: libc.GoString(p)}
}

// Unsupported represents an unsupported Javascript value.
type Unsupported struct{}

// Undefined represents the Javascript value "undefined".
type Undefined struct{}

var (
	toStringC = [...]byte{'t', 'o', 'S', 't', 'r', 'i', 'n', 'g', 0}
	toString  = uintptr(unsafe.Pointer(&toStringC[0]))
)

// value "unpacks" 'v'. FreeValue(v) is called before returning, 'v' must not
// be used afterwards.
func (c *Context) value(v lib.TJSValue) (r any, err error) {
	tls := c.runtime.tls
	ctx := c.context
	if v.Ftag < 0 {
		// all tags with a reference count are negative
		defer lib.XFreeValue(tls, ctx, v)
	}

	switch v.Ftag {
	case lib.EJS_TAG_BIG_DECIMAL: // -11,
		m := lib.XJS_GetPropertyStr(tls, ctx, v, toString)
		if m.Ftag != lib.EJS_TAG_OBJECT {
			panic("failed to get BigDecimal.toString()")
		}

		defer lib.XFreeValue(tls, ctx, m)

		jsString := lib.XJS_Call(tls, ctx, m, v, 0, c.toStringArgv)

		defer lib.XFreeValue(tls, ctx, jsString)

		p := lib.XToCString(tls, ctx, jsString)

		defer lib.XJS_FreeCString(tls, ctx, p)

		n, err := decimal.NewFromString(libc.GoString(p))
		if err != nil {
			panic(todo("decimal.NewFromString failed"))
		}

		return n, nil
	case lib.EJS_TAG_BIG_INT: // -10,
		m := lib.XJS_GetPropertyStr(tls, ctx, v, toString)
		if m.Ftag != lib.EJS_TAG_OBJECT {
			panic("failed to get BigInt.toString()")
		}

		defer lib.XFreeValue(tls, ctx, m)

		*(*lib.TJSValue)(unsafe.Pointer(c.toStringArgv)) = c.int32_16
		jsString := lib.XJS_Call(tls, ctx, m, v, 1, c.toStringArgv)

		defer lib.XFreeValue(tls, ctx, jsString)

		p := lib.XToCString(tls, ctx, jsString)

		defer lib.XJS_FreeCString(tls, ctx, p)

		n := big.NewInt(0)
		if _, ok := n.SetString(libc.GoString(p), 16); !ok {
			panic(todo("big.Int.SetString failed"))
		}

		return n, nil
	case lib.EJS_TAG_BIG_FLOAT: // -9,
		m := lib.XJS_GetPropertyStr(tls, ctx, v, toString)
		if m.Ftag != lib.EJS_TAG_OBJECT {
			panic("failed to get BigInt.toString()")
		}

		defer lib.XFreeValue(tls, ctx, m)

		*(*lib.TJSValue)(unsafe.Pointer(c.toStringArgv)) = c.int32_16
		jsString := lib.XJS_Call(tls, ctx, m, v, 1, c.toStringArgv)

		defer lib.XFreeValue(tls, ctx, jsString)

		p := lib.XToCString(tls, ctx, jsString)

		defer lib.XJS_FreeCString(tls, ctx, p)

		s := libc.GoString(p)
		n := big.NewFloat(0)
		n.SetPrec(uint(4 * len(s)))
		if _, base, err := n.Parse(s, 16); base != 16 || err != nil {
			panic(todo("big.Float.Parse failed"))
		}

		return n, nil
	case lib.EJS_TAG_STRING: // -7,
		p := lib.XToCString(tls, ctx, v)

		defer lib.XJS_FreeCString(tls, ctx, p)

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
		e := lib.XJS_GetException(tls, ctx)

		defer lib.XFreeValue(tls, ctx, e)

		p := lib.XToCString(tls, ctx, e)

		defer lib.XJS_FreeCString(tls, ctx, p)

		return nil, fmt.Errorf("%s", libc.GoString(p))
	case lib.EJS_TAG_FLOAT64: // 7,
		return *(*float64)(unsafe.Pointer(&v)), nil
	}
	return Unsupported{}, nil
}

// AddIntrinsicBigFloat adds the BigFloat object to 'c'.
func (c *Context) AddIntrinsicBigFloat() {
	lib.XJS_AddIntrinsicBigFloat(c.runtime.tls, c.context)
}

// AddIntrinsicBigDecimal adds the BigDecimal object to 'c'.
func (c *Context) AddIntrinsicBigDecimal() {
	lib.XJS_AddIntrinsicBigDecimal(c.runtime.tls, c.context)
}

func throwTypeError(tls *libc.TLS, ctx uintptr, msg string, args ...any) lib.TJSValue {
	bp := tls.Alloc(8 + 8*len(args))

	defer tls.Free(8 + 8*len(args))

	p := mustCString(msg)

	defer libc.Xfree(tls, p)

	return lib.XJS_ThrowTypeError(tls, ctx, p, libc.VaList(bp+8, args...))
}

func throwInternalError(tls *libc.TLS, ctx uintptr, msg string, args ...any) lib.TJSValue {
	bp := tls.Alloc(8 + 8*len(args))

	defer tls.Free(8 + 8*len(args))

	p := mustCString(msg)

	defer libc.Xfree(tls, p)

	return lib.XJS_ThrowInternalError(tls, ctx, p, libc.VaList(bp+8, args...))
}

func mustCString(s string) uintptr {
	p, err := libc.CString(s)
	if err != nil {
		panic(fmt.Errorf("OOM: %v", err))
	}

	return p
}

type goFunc struct {
	cname        uintptr
	ctx          *Context
	f            reflect.Value
	in           []reflect.Type
	name         string
	out          []reflect.Type
	tab          uintptr
	variadicType reflect.Type

	minArgs int
	maxArgs int

	wantThis bool
}

// RegisterFunc registers a Go function 'f' and makes it callable from
// Javascript.
//
// The function must have zero, one or two return values. If it has two return
// values, the second one must be of type error. In such case a non-nil
// returned error is translated to a Javascript exception.
//
// The function can have zero or more parameters. If 'wantThis' is true then
// the first parameter of the Go function will get the Javascript value of
// 'this'.  Depending on context, 'this' can be nil.
//
// Any Go -> Javascript or Javascript -> Go type conversion between arguments
// and return values that fails throws a Javascript exception.
//
// If the Javascript arguments cannot be converted to Go types then a
// Javascript exception is raised.
func (c *Context) RegisterFunc(name string, f any, wantThis bool) (err error) {
	if name == "" {
		return fmt.Errorf("func name cannot be empty")
	}

	if _, ok := c.goFuncs[name]; ok {
		return fmt.Errorf("func already registered: %s", name)
	}

	tls := c.runtime.tls
	ctx := c.context
	val := reflect.ValueOf(f)
	typ := reflect.TypeOf(f)
	if typ.Kind() != reflect.Func {
		return fmt.Errorf("%s is not a function", name)
	}

	switch typ.NumOut() {
	case 0:
		if wantThis {
			return fmt.Errorf("'wantThis' is true but 'f' has zero parameters")
		}
	case 1:
		// ok
	case 2:
		// Second return value must be error
		if t := typ.Out(1); t.Kind() != reflect.Interface || !t.Implements(errorInterface) {
			return fmt.Errorf("%s has two return values, the second one must implement error: %v", name, t)
		}
	default:
		return fmt.Errorf("%s has more than two return values", name)
	}

	minArgs := typ.NumIn()
	if wantThis {
		minArgs++
	}
	maxArgs := minArgs
	var vt reflect.Type
	if typ.IsVariadic() {
		maxArgs = -1
		sliceT := typ.In(typ.NumIn() - 1)
		vt = sliceT.Elem()
	}
	info := &goFunc{
		ctx:          c,
		f:            val,
		maxArgs:      maxArgs,
		minArgs:      minArgs,
		name:         name,
		variadicType: vt,
		wantThis:     wantThis,
	}
	for i := 0; i < typ.NumIn(); i++ {
		info.in = append(info.in, typ.In(i))
	}
	for i := 0; i < typ.NumOut(); i++ {
		info.out = append(info.out, typ.Out(i))
	}
	magic := globalMagic.Add(1)
	goFuncsMu.Lock()
	goFuncs[magic] = info
	goFuncsMu.Unlock()
	if c.goFuncs == nil {
		c.goFuncs = map[string]int32{}
	}
	c.goFuncs[name] = magic

	cname, err := libc.CString(name)
	if err != nil {
		return err
	}

	info.cname = cname
	argc := int32(typ.NumIn()) - 1
	if argc < 0 {
		argc = 0
	}
	tab := libc.Xcalloc(tls, 1, libc.Tsize_t(unsafe.Sizeof(lib.TJSCFunctionListEntry{})))
	info.tab = tab

	g := c.globalObject()

	defer lib.XFreeValue(tls, ctx, g)

	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fname = cname
	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fprop_flags = lib.MJS_PROP_WRITABLE | lib.MJS_PROP_CONFIGURABLE
	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fdef_type = lib.MJS_DEF_CFUNC
	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fmagic = int16(magic)
	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fu.Ffunc1.Flength = uint8(argc)
	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fu.Ffunc1.Fcproto = lib.EJS_CFUNC_generic_magic
	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fu.Ffunc1.Fcfunc.Fgeneric = fp(call)

	lib.XJS_SetPropertyFunctionList(tls, ctx, g, tab, 1)
	return nil
}

func fp(f interface{}) uintptr {
	type iface [2]uintptr
	return (*iface)(unsafe.Pointer(&f))[1]
}

// typedef JSValue JSCFunctionMagic(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv, int magic);
func call(tls *libc.TLS, ctx uintptr, this lib.TJSValue, argc int32, argv uintptr, magic int32) (r lib.TJSValue) {
	goFuncsMu.Lock()
	info := goFuncs[magic]
	goFuncsMu.Unlock()

	if info == nil {
		return throwInternalError(tls, ctx, "callback id=%i not registered", magic)
	}

	c := info.ctx
	if int(argc) < info.minArgs {
		return throwTypeError(tls, ctx, fmt.Sprintf("not enough arguments in call to %s", info.name))
	}

	if info.maxArgs >= 0 && int(argc) > info.maxArgs {
		return throwTypeError(tls, ctx, fmt.Sprintf("too many arguments in call to %s", info.name))
	}

	var err error
	var in []reflect.Value
	for i, typ := range info.in {
		var v lib.TJSValue
		switch {
		case info.wantThis && i == 0:
			_ = typ
			_ = v
			panic(todo(""))
		default:
			panic(todo(""))
		}
	}

	out := info.f.Call(in)
	switch len(info.out) {
	case 2:
		_ = out
		panic(todo(""))
	case 1:
		if r, err = c.jsValue(out[0]); err != nil {
			return throwTypeError(tls, ctx, fmt.Sprintf("callback id=%v: %v", magic, err))
		}
	default:
		return undefined
	}
	return r
}

func (c *Context) jsValue(in reflect.Value) (out lib.TJSValue, err error) {
	switch in.Kind() {
	case reflect.Interface:
		switch in.Interface().(type) {
		case Undefined:
			return undefined, nil
		case nil:
			return null, nil
		}

		typ := reflect.TypeOf(in)
		panic(todo("", typ.Kind()))
	case reflect.Pointer:
		if in.IsNil() {
			return null, nil
		}

		switch x := in.Interface().(type) {
		case *Object:
			panic(todo("", x))
		case nil:
			return null, nil
		case *big.Int:
			panic(todo(""))
		case *big.Float:
			panic(todo(""))
		case decimal.Decimal:
			panic(todo(""))
		default:
			ev := in.Elem()
			if ev.Kind() == reflect.Pointer { // avoid unbound recursion
				return out, fmt.Errorf("type not supported: %s", ev.Type())
			}

			return c.jsValue(ev)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return jsInt(in.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return jsUint(in.Uint()), nil
	case reflect.Float64, reflect.Float32:
		return newFloat(in.Float()), nil
	case reflect.String:
		return c.newString(in.String()), nil
	default:
		panic(todo("", in.Kind()))
	}
}

func jsInt(n int64) lib.TJSValue {
	switch {
	case n >= math.MinInt32 && n <= math.MaxInt32:
		return newInt32(int32(n))
	default:
		return newFloat(float64(n))
	}
}

func jsUint(n uint64) lib.TJSValue {
	switch {
	case n <= math.MaxInt32:
		return newInt32(int32(n))
	default:
		return newFloat(float64(n))
	}
}
