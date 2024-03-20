// Copyright 2024 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quickjs // import "modernc.org/bquickjs"

import (
	"os"
	"runtime"
	"testing"
)

var (
	goos   = runtime.GOOS
	goarch = runtime.GOARCH
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func Test0(t *testing.T) {
	rt, err := NewRuntime()
	if err != nil {
		t.Fatal(err)
	}

	ctx, err := rt.NewContext()
	if err != nil {
		t.Fatal(err)
	}

	v := ctx.Eval(`42 * 314`, "test.js", 0)
	if g, e := v.val.Ftag, int64(0); /* JS_TAG_INT */ g != e {
		t.Logf("%+v", v)
		t.Fatal(g, e)
	}

	if g, e := v.val.Fu.Fint321, int32(42*314); g != e {
		t.Logf("%+v", v)
		t.Fatal(g, e)
	}
}
