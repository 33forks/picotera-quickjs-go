// Copyright 2025 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"golang.org/x/mod/modfile"
)

var (
	goos   = runtime.GOOS
	goarch = runtime.GOARCH
	target = fmt.Sprintf("%s_%s", goos, goarch) + ".txt"
)

func fail(rc int, s string, args ...any) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(s, args...))
	os.Exit(rc)
}

func main() {
	fn := filepath.Join("compare", "go.mod")
	b, err := os.ReadFile(fn)
	if err != nil {
		fail(1, "%v", err)
	}

	mod, err := modfile.ParseLax(filepath.Join(fn), b, nil)
	if err != nil {
		fail(1, "%v", err)
	}

	versionsByPath := map[string]string{} // path: version
	for _, v := range mod.Require {
		versionsByPath[v.Mod.Path] = v.Mod.Version
	}
	// fmt.Println(versionsByPath)
	quickjsVersion := versionsByPath["modernc.org/quickjs"]
	// fmt.Println(quickjsVersion)
	results := filepath.Join("testdata", "benchmarks", quickjsVersion, target)
	// fmt.Println(results)
	if fi, err := os.Stat(results); err == nil && fi.Mode().IsRegular() {
		return // done
	}

	os.Remove(results)
	os.MkdirAll(filepath.Dir(results), 0775)
	if out, err := exec.Command("sh", "-c", fmt.Sprintf("make benchmark > %s", results)).CombinedOutput(); err != nil {
		fail(1, "err=%v out=%s", err, out)
	}

	//TODO update package godocs using results.
}
