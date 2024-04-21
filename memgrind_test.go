// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build libc.memgrind

package quickjs // import "modernc.org/bquickjs"

import (
	"os"
	"path/filepath"
	"testing"

	util "modernc.org/fileutil/ccgo"
	"modernc.org/libc"
)

func init() {
	memgrind = true
}

func TestMemgrind(t *testing.T) {
	// Force libc environ allocation that may otherwise skew the accounting.
	tls := libc.NewTLS()
	tls.Close()

	libc.MemAuditStart()
	util.InDir(filepath.Join("internal", "arewefastyet", "v8-v7"), func() error {
		var src []string
		for _, fn := range arewefastyetJS {
			s, err := os.ReadFile(fn)
			if err != nil {
				t.Fatal(fn, err)
			}

			src = append(src, string(s))
		}
		if testing.Short() {
			src = src[:2]
		}
		src = append(src, runArewefastyet)

		rt, err := NewRuntime()
		if err != nil {
			t.Fatal(err)
		}

		defer rt.Close()

		ctx, err := rt.NewContext()
		if err != nil {
			t.Fatal(err)
		}

		defer ctx.Close()

		for _, v := range src {
			if _, err := ctx.Eval(v, EvalGlobal); err != nil {
				t.Fatal(err)
			}
		}

		return nil
	})

	if err := libc.MemAuditReport(); err != nil {
		t.Fatal(err)
	}
}

func TestMemgrind2(t *testing.T) {
	// Force libc environ allocation that may otherwise skew the accounting.
	tls := libc.NewTLS()
	tls.Close()

	libc.MemAuditStart()

	t.Run("eval1", testEval1)
	t.Run("eval2", testEval2)
	t.Run("eval3", testEval3)
	t.Run("call1", testCall1)
	t.Run("call2", testCall2)
	t.Run("call3", testCall3)

	if err := libc.MemAuditReport(); err != nil {
		t.Fatal(err)
	}
}
