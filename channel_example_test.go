// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quickjs // import "modernc.org/quickjs"

// Disabled for now. Turns out the original C libray is not thread safe.

//TODO generate libquickjs while moving all items with storage class 'static'
// to an instance, like was done in libqbe.

// import (
// 	"fmt"
// 	"sync"
// )
//
// // Multiple VMs communicating using Go channels.
// func Example_ping() {
// 	ch := make(chan string, 10)
// 	m1, _ := NewVM()
// 	defer m1.Close()
// 	registerFuncs(m1, ch)
// 	m2, _ := NewVM()
// 	defer m2.Close()
// 	registerFuncs(m2, ch)
// 	var wg sync.WaitGroup
// 	wg.Add(2)
// 	go func() {
// 		defer wg.Done()
// 		m2.Eval("tx(rx()+' reply');", EvalGlobal)
// 	}()
// 	go func() {
// 		defer wg.Done()
// 		fmt.Println(m1.Eval("tx('ping'); rx();", EvalGlobal))
// 	}()
// 	wg.Wait()
// 	// Output:
// 	// ping reply TOD <nil>
// }
//
// func registerFuncs(m *VM, ch chan string) {
// 	m.RegisterFunc("tx", func(s string) { ch <- s }, false)
// 	m.RegisterFunc("rx", func() string { return <-ch }, false)
// }
