// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quickjs // import "modernc.org/quickjs"

// https://sangupta.com/tech/parsing-typescript-in-go-with-quickjs.html

import (
	"fmt"
)

// JSON marshalling.
func ExampleObject_MarshalJSON() {
	m, _ := NewVM()
	defer m.Close()
	obj, _ := m.Eval("obj = {a: 42+314, b: 'foo'}; obj;", EvalGlobal)
	s, _ := (obj.(*Object).MarshalJSON())
	fmt.Printf("%s\n", s)
	// Output:
	// {"a":356,"b":"foo"}
}

// JSON marshalling.
func ExampleObject_String() {
	m, _ := NewVM()
	defer m.Close()
	obj, _ := m.Eval("obj = {a: 42+314, b: 'foo'}; obj;", EvalGlobal)
	fmt.Printf("%s\n", obj)
	// Output:
	// {"a":356,"b":"foo"}
}

// Enable BigDecimal.
func ExampleVM_AddIntrinsicBigDecimal() {
	m, _ := NewVM()
	defer m.Close()
	fmt.Println(m.Eval("BigDecimal('1234567890.123456789');", EvalGlobal))
	m.AddIntrinsicBigDecimal()
	fmt.Println(m.Eval("BigDecimal('1234567890.123456789');", EvalGlobal))
	// Output:
	// <nil> ReferenceError: 'BigDecimal' is not defined
	// 1234567890.123456789 <nil>
}

// Enable BigFloat.
func ExampleVM_AddIntrinsicBigFloat() {
	m, _ := NewVM()
	defer m.Close()
	fmt.Println(m.Eval("BigFloat('1234567890.123456789e+5');", EvalGlobal))
	m.AddIntrinsicBigFloat()
	fmt.Println(m.Eval("BigFloat('1234567890.123456789e+5');", EvalGlobal))
	// Output:
	// <nil> ReferenceError: 'BigFloat' is not defined
	// 1.234567890123456789e+14 <nil>
}

// Call a Javascript function.
func ExampleVM_Call_function() {
	m, _ := NewVM()
	defer m.Close()
	fmt.Println(m.Call("parseInt", "1234"))
	// Output:
	// 1234 <nil>
}

// Call a Javascript method.
func ExampleVM_Call_method() {
	m, _ := NewVM()
	defer m.Close()
	fmt.Println(m.Call("Math.abs", -1234))
	// Output:
	// 1234 <nil>
}

// Getting exception.
func ExampleVM_Eval_exception() {
	m, _ := NewVM()
	defer m.Close()
	fmt.Println(m.Eval("throw new Error('failed');", EvalGlobal))
	// Output:
	// <nil> Error: failed
}

// Evaluate a simple Javascript expression.
func ExampleVM_Eval_expression() {
	m, _ := NewVM()
	defer m.Close()
	fmt.Println(m.Eval("1+2", EvalGlobal))
	// Output:
	// 3 <nil>
}

// Object example.
func ExampleVM_Eval_object() {
	m, _ := NewVM()
	defer m.Close()
	fmt.Println(m.Eval("obj = {a: 42+314, b: 'foo'}; obj;", EvalGlobal))
	// Output:
	// {"a":356,"b":"foo"} <nil>
}

// Use the std module.
func ExampleVM_InitModuleStd() {
	m, _ := NewVM()
	defer m.Close()
	m.InitModuleStd()
	m.Eval(`
import * as std from 'std';
globalThis.std = std;
`, EvalModule)
	fmt.Println(m.Call("std.sprintf", "%s %i", "hello", 42))
	// Output:
	// hello 42 <nil>
}

// Add std helpers.
func ExampleVM_AddStdHelpers() {
	m, _ := NewVM()
	defer m.Close()
	fmt.Println(m.Eval("console.toString();", EvalGlobal))
	m.AddStdHelpers()
	fmt.Println(m.Eval("console.toString();", EvalGlobal))
	fmt.Println(m.Eval("console.log.toString();", EvalGlobal))
	// Output:
	// <nil> ReferenceError: 'console' is not defined
	// [object Object] <nil>
	// function log() {
	//     [native code]
	// } <nil>
}

// Call error returning Go function from Javascript.
func ExampleVM_RegisterFunc_error() {
	m, _ := NewVM()
	defer m.Close()
	m.RegisterFunc("gofunc", func(a int) error {
		if a < 0 {
			return fmt.Errorf("negative")
		}
		return nil
	}, false)
	fmt.Println(m.Eval("gofunc(-1)", EvalGlobal))
	fmt.Println(m.Eval("gofunc(1)", EvalGlobal))
	// Output:
	// negative <nil>
	// <nil> <nil>
}

// Call multiple return Go function from Javascript.
func ExampleVM_RegisterFunc_multipleReturn() {
	m, _ := NewVM()
	defer m.Close()
	m.RegisterFunc("gofunc", func(a, b int) (int, int) { return 10 * a, 100 * b }, false)
	fmt.Println(m.Eval("gofunc(2, 3)", EvalGlobal))
	// Output:
	// [20,300] <nil>
}

// Call single return Go function from Javascript.
func ExampleVM_RegisterFunc_singleReturn() {
	m, _ := NewVM()
	defer m.Close()
	m.RegisterFunc("gofunc", func(a, b, c int) int { return a + b*c }, false)
	fmt.Println(m.Eval("gofunc(2, 3, 5)", EvalGlobal))
	// Output:
	// 17 <nil>
}

// Call void Go function from Javascript.
func ExampleVM_RegisterFunc_void() {
	m, _ := NewVM()
	defer m.Close()
	m.RegisterFunc("gofunc", func(a, b, c int) { fmt.Println(a + b*c) }, false)
	fmt.Println(m.Eval("gofunc(2, 3, 5)", EvalGlobal))
	// Output:
	// 17
	// undefined <nil>
}

// Passing undefined Javascript 'this' to a Go function.
func ExampleVM_RegisterFunc_thisNull() {
	m, _ := NewVM()
	defer m.Close()
	m.RegisterFunc("gofunc", func(this any) any { return this }, true)
	fmt.Println(m.Eval("gofunc()", EvalGlobal))
	// Output:
	// undefined <nil>
}

// Passing undefined Javascript 'this' to a Go function.
func ExampleVM_RegisterFunc_thisNull2() {
	m, _ := NewVM()
	defer m.Close()
	m.RegisterFunc("gofunc", func(this any, n int) (any, int) { return this, 10 * n }, true)
	fmt.Println(m.Eval("gofunc(42)", EvalGlobal))
	// Output:
	// [null,420] <nil>
}

// Passing Javascript 'this' to a Go function.
func ExampleVM_RegisterFunc_thisNonNull() {
	m, _ := NewVM()
	defer m.Close()
	m.RegisterFunc("gofunc", func(this any) any { return this }, true)
	fmt.Println(m.Eval("var obj = { foo: 314, method: gofunc }; obj.method()", EvalGlobal))
	// Output:
	// {"foo":314} <nil>
}

// Passing Javascript 'this' to a Go function.
func ExampleVM_RegisterFunc_thisNonNull2() {
	m, _ := NewVM()
	defer m.Close()
	m.RegisterFunc("gofunc", func(this any, n int) (any, int) { return this, 10 * n }, true)
	fmt.Println(m.Eval("var obj = { foo: 314, method: gofunc }; obj.method(42)", EvalGlobal))
	// Output:
	// [{"foo":314},420] <nil>
}
