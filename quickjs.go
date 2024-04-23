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

	staticMagic atomic.Int32

	goFuncsMu sync.Mutex
	goFuncs   = map[int32]*goFunc{}
	freeMagic = map[int32]struct{}{}
)

// runtime represents a Javascript runtime.
//
// Note: runtime is not safe for concurrent use by multiple goroutines.
type runtime struct {
	cRuntime uintptr // lib.TJSRuntime
	tls      *libc.TLS
}

// newRuntime returns a newly create Runtime
func newRuntime() (*runtime, error) {
	tls := libc.NewTLS()
	r := lib.XJS_NewRuntime(tls)
	if r == 0 {
		tls.Close()
		return nil, fmt.Errorf("failed to create a new Javascript runtime")
	}

	return &runtime{cRuntime: r, tls: tls}, nil
}

// close releases the resources held by r.
func (r *runtime) close() error {
	lib.XJS_FreeRuntime(r.tls, r.cRuntime)
	r.tls.Close()
	*r = runtime{}
	return nil
}

// newVM returns a newly created VM.
func (r *runtime) newVM() (*VM, error) {
	argv := libc.Xcalloc(r.tls, 2, libc.Tsize_t(unsafe.Sizeof(lib.TJSValue{})))
	if argv == 0 {
		return nil, fmt.Errorf("OOM")
	}

	c := lib.XJS_NewContext(r.tls, r.cRuntime)
	if c == 0 {
		return nil, fmt.Errorf("failed to create a new Javascript context")
	}

	return &VM{
		cContext:     c,
		int32_2:      lib.XNewInt32(r.tls, c, 2),
		int32_16:     lib.XNewInt32(r.tls, c, 16),
		runtime:      r,
		toStringArgv: argv,
	}, nil
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

func (m *VM) newString(s string) (r lib.TJSValue) {
	tls := m.runtime.tls
	ctx := m.cContext
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

func isException(v lib.TJSValue) bool {
	return v.Ftag == lib.EJS_TAG_EXCEPTION
}

var (
	jsonFNC = [...]byte{'J', 'S', 'O', 'N', 0}
	jsonFN  = uintptr(unsafe.Pointer(&jsonFNC[0]))
)

func (m *VM) parseJSON(s string, flags int32) (r lib.TJSValue, err error) {
	tls := m.runtime.tls
	ctx := m.cContext

	cs := mustCString(s)

	defer libc.Xfree(tls, cs)

	r = lib.XJS_ParseJSON2(tls, ctx, cs, lib.Tsize_t(len(s)), jsonFN, flags)
	if isException(r) {
		err = m.errFromException()
	}
	return r, err
}

// Unsupported represents an unsupported Javascript value.
type Unsupported struct{}

// Undefined represents the Javascript value "undefined".
type Undefined struct{}

// Object represents the value of a Javascript object, but not the javascript
// object instance itself.
type Object struct {
	json               string
	forceNonComparable []byte
}

// String implements fmt.Stringer.
func (o *Object) String() string {
	return o.json
}

// MarshalJSON implements encoding/json.Marshaler.
func (o *Object) MarshalJSON() (r []byte, err error) {
	return []byte(o.json), nil
}

// VM represents a Javascript context (or Realm). Each VM has its
// own global objects and system objects.
//
// Note: VM is not safe for concurrent use by multiple goroutines.
type VM struct {
	// Safe to share, not reference counted
	int32_2      lib.TJSValue
	int32_16     lib.TJSValue
	cContext     uintptr // lib.TJSContext
	goFuncs      map[string]int32
	runtime      *runtime
	toStringArgv uintptr
}

// NewVM returns a newly created VM.
func NewVM() (*VM, error) {
	r, err := newRuntime()
	if err != nil {
		return nil, err
	}

	return r.newVM()
}

// Close releases the resources held by 'm'.
func (m *VM) Close() error {
	tls := m.runtime.tls
	var a []int32
	var b []*goFunc
	for _, k := range m.goFuncs {
		a = append(a, k)
	}
	goFuncsMu.Lock()
	for _, k := range a {
		b = append(b, goFuncs[k])
		freeMagic[k] = struct{}{}
		delete(goFuncs, k)
	}
	goFuncsMu.Unlock()

	for _, v := range b {
		libc.Xfree(tls, v.cName)
		libc.Xfree(tls, v.tab)
	}

	libc.Xfree(tls, m.toStringArgv)
	lib.XJS_FreeContext(m.runtime.tls, m.cContext)
	r := m.runtime
	*m = VM{}
	return r.close()
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
func (m *VM) Eval(javascript string, flags int) (r any, err error) {
	tls := m.runtime.tls
	ps, err := libc.CString(javascript)
	if err != nil {
		panic(err)
	}

	defer libc.Xfree(tls, ps)

	return m.value(lib.XJS_Eval(tls, m.cContext, ps, libc.Tsize_t(len(javascript)), uintptr(unsafe.Pointer(&evalFN)), int32(flags)), true)
}

func (m *VM) eval(js string, flags int) lib.TJSValue {
	tls := m.runtime.tls
	ctx := m.cContext
	ps, err := libc.CString(js)
	if err != nil {
		panic(err)
	}

	defer libc.Xfree(tls, ps)

	return lib.XJS_Eval(tls, ctx, ps, libc.Tsize_t(len(js)), uintptr(unsafe.Pointer(&evalFN)), int32(flags))
}

func (m *VM) globalObject() lib.TJSValue {
	return lib.XJS_GetGlobalObject(m.runtime.tls, m.cContext)
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
func (m *VM) Call(function string, args ...any) (r any, err error) {
	tls := m.runtime.tls
	ctx := m.cContext
	ps, err := libc.CString(function)
	if err != nil {
		panic(err)
	}

	defer libc.Xfree(tls, ps)

	f := lib.XJS_Eval(tls, ctx, ps, libc.Tsize_t(len(function)), uintptr(unsafe.Pointer(&evalFN)), int32(EvalGlobal))

	defer lib.XFreeValue(tls, ctx, f)

	g := m.globalObject()

	defer lib.XFreeValue(tls, ctx, g)

	return m.call(f, g, args...)
}

func (m *VM) call(f, this lib.TJSValue, args ...any) (r any, err error) {
	tls := m.runtime.tls
	ctx := m.cContext
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
			jv := m.eval(s, EvalGlobal)

			defer lib.XFreeValue(tls, ctx, jv)

			jsArgs = append(jsArgs, jv)
		case *big.Float:
			s := fmt.Sprintf("BigFloat('%s')", x.String())
			jv := m.eval(s, EvalGlobal)

			defer lib.XFreeValue(tls, ctx, jv)

			jsArgs = append(jsArgs, jv)
		case decimal.Decimal:
			s := x.String() + "m"
			jv := m.eval(s, EvalGlobal)

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
	return m.value(lib.XJS_Call(tls, ctx, f, this, int32(len(jsArgs)), argv), true)
}

func (m *VM) newObject(v lib.TJSValue) *Object {
	tls := m.runtime.tls
	ctx := m.cContext
	json := lib.XJS_JSONStringify(tls, ctx, v, undefined, undefined)

	defer lib.XFreeValue(tls, ctx, json)

	p := lib.XToCString(tls, ctx, json)

	defer lib.XJS_FreeCString(tls, ctx, p)

	return &Object{json: libc.GoString(p)}
}

var (
	toStringC = [...]byte{'t', 'o', 'S', 't', 'r', 'i', 'n', 'g', 0}
	toString  = uintptr(unsafe.Pointer(&toStringC[0]))
)

// value "unpacks" 'v'. FreeValue(v) is called before returning, 'v' must not
// be used afterwards.
func (m *VM) value(v lib.TJSValue, free bool) (r any, err error) {
	tls := m.runtime.tls
	ctx := m.cContext
	if free && v.Ftag < 0 {
		// all tags with a reference count are negative
		defer lib.XFreeValue(tls, ctx, v)
	}

	switch v.Ftag {
	case lib.EJS_TAG_BIG_DECIMAL: // -11,
		ps := lib.XJS_GetPropertyStr(tls, ctx, v, toString)
		if ps.Ftag != lib.EJS_TAG_OBJECT {
			panic("failed to get BigDecimal.toString()")
		}

		defer lib.XFreeValue(tls, ctx, ps)

		jsString := lib.XJS_Call(tls, ctx, ps, v, 0, m.toStringArgv)

		defer lib.XFreeValue(tls, ctx, jsString)

		p := lib.XToCString(tls, ctx, jsString)

		defer lib.XJS_FreeCString(tls, ctx, p)

		n, err := decimal.NewFromString(libc.GoString(p))
		if err != nil {
			panic(todo("decimal.NewFromString failed"))
		}

		return n, nil
	case lib.EJS_TAG_BIG_INT: // -10,
		ps := lib.XJS_GetPropertyStr(tls, ctx, v, toString)
		if ps.Ftag != lib.EJS_TAG_OBJECT {
			panic("failed to get BigInt.toString()")
		}

		defer lib.XFreeValue(tls, ctx, ps)

		*(*lib.TJSValue)(unsafe.Pointer(m.toStringArgv)) = m.int32_16
		jsString := lib.XJS_Call(tls, ctx, ps, v, 1, m.toStringArgv)

		defer lib.XFreeValue(tls, ctx, jsString)

		p := lib.XToCString(tls, ctx, jsString)

		defer lib.XJS_FreeCString(tls, ctx, p)

		n := big.NewInt(0)
		if _, ok := n.SetString(libc.GoString(p), 16); !ok {
			panic(todo("big.Int.SetString failed"))
		}

		return n, nil
	case lib.EJS_TAG_BIG_FLOAT: // -9,
		ps := lib.XJS_GetPropertyStr(tls, ctx, v, toString)
		if ps.Ftag != lib.EJS_TAG_OBJECT {
			panic("failed to get BigInt.toString()")
		}

		defer lib.XFreeValue(tls, ctx, ps)

		*(*lib.TJSValue)(unsafe.Pointer(m.toStringArgv)) = m.int32_16
		jsString := lib.XJS_Call(tls, ctx, ps, v, 1, m.toStringArgv)

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
		return m.newObject(v), nil
	case lib.EJS_TAG_INT: //  0,
		return int(*(*int32)(unsafe.Pointer(&v))), nil
	case lib.EJS_TAG_BOOL: //  1,
		return *(*int32)(unsafe.Pointer(&v)) != 0, nil
	case lib.EJS_TAG_NULL: //  2,
		return nil, nil
	case lib.EJS_TAG_UNDEFINED: //  3,
		return Undefined{}, nil
	case lib.EJS_TAG_EXCEPTION: // 6,
		return nil, m.errFromException()
	case lib.EJS_TAG_FLOAT64: // 7,
		return *(*float64)(unsafe.Pointer(&v)), nil
	}
	return Unsupported{}, nil
}

func (m *VM) errFromException() error {
	tls := m.runtime.tls
	ctx := m.cContext
	e := lib.XJS_GetException(tls, ctx)

	defer lib.XFreeValue(tls, ctx, e)

	p := lib.XToCString(tls, ctx, e)

	defer lib.XJS_FreeCString(tls, ctx, p)

	return fmt.Errorf("%s", libc.GoString(p))
}

// AddIntrinsicBigFloat adds the BigFloat object to 'm'.
func (m *VM) AddIntrinsicBigFloat() {
	lib.XJS_AddIntrinsicBigFloat(m.runtime.tls, m.cContext)
}

// AddIntrinsicBigDecimal adds the BigDecimal object to 'm'.
func (m *VM) AddIntrinsicBigDecimal() {
	lib.XJS_AddIntrinsicBigDecimal(m.runtime.tls, m.cContext)
}

var (
	stdC = [...]byte{'s', 't', 'd', 0}
	std  = uintptr(unsafe.Pointer(&stdC[0]))
)

// InitModuleStd adds the "std" module to 'm'.
func (m *VM) InitModuleStd() error {
	tls := m.runtime.tls
	ctx := m.cContext
	if lib.Xjs_init_module_std(tls, ctx, std) == 0 {
		return fmt.Errorf("module initialization failed")
	}

	return nil
}

// AddStdHelpers adds the 'print' and 'console' global objects to 'm'.
func (m *VM) AddStdHelpers() error {
	tls := m.runtime.tls
	ctx := m.cContext
	lib.Xjs_std_add_helpers(tls, ctx, -1, 0)
	return nil
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
	cName        uintptr
	f            reflect.Value
	in           []reflect.Type
	name         string
	out          []reflect.Type
	tab          uintptr
	variadicType reflect.Type
	vm           *VM

	minArgs int
	maxArgs int

	wantThis bool
}

// RegisterFunc registers a Go function 'f' and makes it callable from
// Javascript.
//
// The 'f' argument can be a regular Go function, a closure, a method
// expression or a method value. All of them are called 'Go function' below.
//
// The Go function can have zero or more parameters. If 'wantThis' is true then
// the first parameter of the Go function will get the Javascript value of
// 'this'.  Depending on context, 'this' can be Javascript null or undefined.
//
// Go functions with multiple results return them as an Javascript array.
//
// Go nil errors are converted to Javascript null.
//
// Go non-nil errors are converted to Javascript strings using the Error()
// method.
//
// Any Go -> Javascript or Javascript -> Go type conversion between arguments
// and return values that fails throws a Javascript exception.
//
// There is a process-wide limit on the total number of currently registered Go
// functions.
//
// Note: The 'name' argument should be a valid Javascript identifier. It is not
// currently enforced but this may change later.
func (m *VM) RegisterFunc(name string, f any, wantThis bool) (err error) {
	if name == "" {
		return fmt.Errorf("func name cannot be empty")
	}

	if _, ok := m.goFuncs[name]; ok {
		return fmt.Errorf("func already registered: %s", name)
	}

	tls := m.runtime.tls
	ctx := m.cContext
	val := reflect.ValueOf(f)
	typ := reflect.TypeOf(f)
	if typ.Kind() != reflect.Func {
		return fmt.Errorf("%s is not a function", name)
	}

	if wantThis && typ.NumIn() == 0 {
		return fmt.Errorf("'wantThis' is true but 'f' has zero parameters")
	}

	minArgs := typ.NumIn()
	if wantThis {
		minArgs--
	}
	maxArgs := minArgs
	var vt reflect.Type
	if typ.IsVariadic() {
		maxArgs = -1
		sliceT := typ.In(typ.NumIn() - 1)
		vt = sliceT.Elem()
		minArgs--
	}
	info := &goFunc{
		f:            val,
		maxArgs:      maxArgs,
		minArgs:      minArgs,
		name:         name,
		variadicType: vt,
		vm:           m,
		wantThis:     wantThis,
	}
	for i := 0; i < typ.NumIn(); i++ {
		info.in = append(info.in, typ.In(i))
	}
	if maxArgs < 0 {
		info.in = info.in[:len(info.in)-1]
	}
	for i := 0; i < typ.NumOut(); i++ {
		info.out = append(info.out, typ.Out(i))
	}

	magic := int32(-1)
	goFuncsMu.Lock()
	for k := range freeMagic {
		magic = k
		delete(freeMagic, k)
		break
	}
	if magic < 0 {
		magic = staticMagic.Add(1)
		if magic > math.MaxInt16 {
			staticMagic.Add(-1)
			goFuncsMu.Unlock()
			return fmt.Errorf("too many registered functions")
		}

	}
	goFuncs[magic] = info
	goFuncsMu.Unlock()
	if m.goFuncs == nil {
		m.goFuncs = map[string]int32{}
	}
	m.goFuncs[name] = magic

	cName, err := libc.CString(name)
	if err != nil {
		return err
	}

	info.cName = cName
	argc := int32(typ.NumIn()) - 1
	if argc < 0 {
		argc = 0
	}
	tab := libc.Xcalloc(tls, 1, libc.Tsize_t(unsafe.Sizeof(lib.TJSCFunctionListEntry{})))
	info.tab = tab

	g := m.globalObject()

	defer lib.XFreeValue(tls, ctx, g)

	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fname = cName
	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fprop_flags = lib.MJS_PROP_WRITABLE | lib.MJS_PROP_CONFIGURABLE
	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fdef_type = lib.MJS_DEF_CFUNC
	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fmagic = int16(magic)
	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fu.Ffunc1.Flength = uint8(argc)
	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fu.Ffunc1.Fcproto = lib.EJS_CFUNC_generic_magic
	(*lib.TJSCFunctionListEntry)(unsafe.Pointer(tab)).Fu.Ffunc1.Fcfunc.Fgeneric = fp(callGo)
	lib.XJS_SetPropertyFunctionList(tls, ctx, g, tab, 1)
	return nil
}

func fp(f interface{}) uintptr {
	type iface [2]uintptr
	return (*iface)(unsafe.Pointer(&f))[1]
}

// typedef JSValue JSCFunctionMagic(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv, int magic);
func callGo(tls *libc.TLS, ctx uintptr, this lib.TJSValue, argc int32, argv uintptr, magic int32) (r lib.TJSValue) {
	goFuncsMu.Lock()
	info := goFuncs[magic]
	goFuncsMu.Unlock()

	if info == nil {
		return throwInternalError(tls, ctx, "callback id=%i not registered", magic)
	}

	m := info.vm
	haveArgs := int(argc)
	if info.wantThis {
		haveArgs--
		if haveArgs < 0 {
			haveArgs = 0
		}
	}
	if haveArgs < info.minArgs {
		trc("wantThis=%v haveArgs=%v minArgs=%v maxArgs=%v argc=%v", info.wantThis, haveArgs, info.minArgs, info.maxArgs, argc)
		return throwTypeError(tls, ctx, fmt.Sprintf("not enough arguments in call to %s", info.name))
	}

	if info.maxArgs >= 0 && haveArgs > info.maxArgs {
		// trc("wantThis=%v haveArgs=%v minArgs=%v maxArgs=%v argc=%v", info.wantThis, haveArgs, info.minArgs, info.maxArgs, argc)
		return throwTypeError(tls, ctx, fmt.Sprintf("too many arguments in call to %s", info.name))
	}

	var in []reflect.Value
	i := 0
	if info.wantThis {
		v, err := m.value(this, false)
		if err != nil {
			return throwTypeError(tls, ctx, fmt.Sprintf("calling %s: argument conversion error: %v", info.name, err))
		}

		rv := reflect.ValueOf(v)
		if !rv.Type().AssignableTo(info.in[0]) {
			return throwTypeError(tls, ctx, fmt.Sprintf("calling %s: cannot assign %s to %s", info.name, rv.Type(), info.in[0]))
		}

		in = append(in, rv)
		i++
	}
	types := info.in[i:]
	sz := unsafe.Sizeof(lib.TJSValue{})
	for i := 0; i < int(argc); i++ {
		var typ reflect.Type
		switch {
		case i < len(types):
			typ = types[i]
		default:
			typ = info.variadicType
		}
		v, err := m.value(*(*lib.TJSValue)(unsafe.Pointer(argv)), false)
		if err != nil {
			return throwTypeError(tls, ctx, fmt.Sprintf("calling %s: argument conversion error: %v", info.name, err))
		}

		rv := reflect.ValueOf(v)
		if !rv.Type().AssignableTo(typ) {
			return throwTypeError(tls, ctx, fmt.Sprintf("calling %s: cannot assign %s to %s", info.name, rv.Type(), info.in[0]))
		}

		in = append(in, rv)
		argv += sz
	}

	out := info.f.Call(in)
	var err error
	switch len(info.out) {
	case 0:
		return undefined
	case 1:
		r, err = m.jsValue(out[0])
	default:
		r, err = m.jsArray(out)
	}
	if err != nil {
		return throwTypeError(tls, ctx, fmt.Sprintf("callback %s: %v", info.name, err))
	}

	return r
}

func (m *VM) jsValue(in reflect.Value) (out lib.TJSValue, err error) {
	defer func() {
		if isException(out) && err == nil {
			err = m.errFromException()
		}
	}()

	switch x := in.Interface().(type) {
	case nil:
		return null, nil
	case Undefined:
		return undefined, nil
	case error:
		return m.newString(x.Error()), nil
	}

	switch in.Kind() {
	case reflect.Pointer:
		return m.jsPtrValue(in)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return jsInt(in.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return jsUint(in.Uint()), nil
	case reflect.Float64, reflect.Float32:
		return newFloat(in.Float()), nil
	case reflect.String:
		return m.newString(in.String()), nil
	case reflect.Struct:
		switch x := in.Interface().(type) {
		case decimal.Decimal:
			s := x.String() + "m"
			return m.eval(s, EvalGlobal), nil
		}

		panic(todo("%T", in.Interface()))
	case reflect.Interface:
		return m.jsInterfaceValue(in)
	case reflect.Slice:
		var a []reflect.Value
		for i := 0; i < in.Len(); i++ {
			a = append(a, in.Index(i))
		}
		return m.jsArray(a)
	default:
		panic(todo("", in.Kind()))
	}
}

func (m *VM) jsInterfaceValue(in reflect.Value) (out lib.TJSValue, err error) {
	if in.IsNil() {
		return null, nil
	}

	return m.jsValue(in.Elem())
}

func (m *VM) jsPtrValue(in reflect.Value) (out lib.TJSValue, err error) {
	if in.IsNil() {
		return null, nil
	}

	switch x := in.Interface().(type) {
	case *Object:
		return m.parseJSON(x.json, 0)
	case *big.Int:
		s := x.String() + "n"
		return m.eval(s, EvalGlobal), nil
	case *big.Float:
		s := fmt.Sprintf("BigFloat('%s')", x.String())
		return m.eval(s, EvalGlobal), nil
	default:
		ev := in.Elem()
		if ev.Kind() == reflect.Pointer { // avoid unbound recursion
			return out, fmt.Errorf("type not supported: %T", x)
		}

		return m.jsValue(ev)
	}
}

func (m *VM) jsArray(a []reflect.Value) (out lib.TJSValue, err error) {
	var s []lib.TJSValue
	for _, v := range a {
		jv, err := m.jsValue(v)
		if err != nil {
			return out, err
		}

		s = append(s, jv)
	}
	return m.jsArrayOf(s)
}

func (m *VM) jsArrayOf(a []lib.TJSValue) (out lib.TJSValue, err error) {
	tls := m.runtime.tls
	ctx := m.cContext
	sz := unsafe.Sizeof(lib.TJSValue{})
	argv := libc.Xmalloc(tls, lib.Tsize_t(len(a))*lib.Tsize_t(sz))

	defer libc.Xfree(tls, argv)

	p := argv
	for _, v := range a {
		*(*lib.TJSValue)(unsafe.Pointer(p)) = v
		p += sz
	}
	out = lib.ArrayOf(tls, ctx, int32(len(a)), argv)
	if isException(out) {
		err = m.errFromException()
	}
	for _, v := range a {
		lib.XFreeValue(tls, ctx, v)
	}
	return out, err
}
