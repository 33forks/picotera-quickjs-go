// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quickjs // import "modernc.org/quickjs"

import (
	"fmt"
)

// Multiple concurrent Javascript virtual machines communicating via Go channels.
func Example_ping() {
	m1, _ := NewVM()
	defer m1.Close()
	m2, _ := NewVM()
	defer m2.Close()
	ch1 := make(chan string, 1)
	ch2 := make(chan string, 1)
	registerFuncs(m1, ch1, ch2)
	registerFuncs(m2, ch2, ch1)
	go func() {
		m2.Eval("tx(rx()+' reply');", EvalGlobal)
	}()
	fmt.Println(m1.Eval("tx('ping'); rx();", EvalGlobal))
	// Output:
	// ping reply <nil>
}

func registerFuncs(m *VM, ch1, ch2 chan string) {
	m.RegisterFunc("tx", func(s string) { ch1 <- s }, false)
	m.RegisterFunc("rx", func() string { return <-ch2 }, false)
}
