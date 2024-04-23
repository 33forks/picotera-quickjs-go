// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quickjs // import "modernc.org/quickjs"

import (
	"fmt"
)

// Evaluate a simple Javascript expression.
func ExampleVM_Eval_expression() {
	m, _ := NewVM()
	defer m.Close()
	fmt.Println(m.Eval("1+2", EvalGlobal))
	// Output:
	// 3 <nil>
}

// Call a Javascript function.
func ExampleVM_Call_function() {
	m, _ := NewVM()
	defer m.Close()
	fmt.Println(m.Call("parseInt", "1234"))
	// Output:
	// 1234 <nil>
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

// Call a Javascript method.
func ExampleVM_Call_method() {
	m, _ := NewVM()
	defer m.Close()
	fmt.Println(m.Call("Math.abs", -1234))
	// Output:
	// 1234 <nil>
}

// Object example.
func ExampleVM_Eval_object() {
	m, _ := NewVM()
	defer m.Close()
	fmt.Println(m.Eval("obj = {a: 42+314, b: 'foo'}; obj;", EvalGlobal))
	// Output:
	// {"a":356,"b":"foo"} <nil>
}

// Call Go from Javascript.
func ExampleVM_RegisterFunc() {
	m, _ := NewVM()
	defer m.Close()
	m.RegisterFunc("gofun", func(a, b, c int) int { return a + b*c }, false)
	fmt.Println(m.Eval("gofun(2, 3, 5)", EvalGlobal))
	// Output:
	// 17 <nil>
}
