// Copyright 2025 The quickjs-go Authors. All rights reserved.
// Use of the source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

const (
	docFn = "quickjs.go"
	qemu  = "    (qemu)"
)

var (
	goos   = runtime.GOOS
	goarch = runtime.GOARCH
	target = fmt.Sprintf("%s_%s", goos, goarch) + ".txt"
	notes  = map[string]string{
		"freebsd/amd64": qemu,
		"freebsd/arm64": qemu,
		"linux/386":     qemu,
		"linux/s390x":   qemu,
		"openbsd/amd64": qemu,
		"windows/386":   qemu,
	}
)

func fail(rc int, s string, args ...any) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(s, args...))
	os.Exit(rc)
}

func main() {
	fn := path.Join("compare", "go.mod")
	b, err := os.ReadFile(fn)
	if err != nil {
		fail(1, "%v", err)
	}

	mod, err := modfile.ParseLax(fn, b, nil)
	if err != nil {
		fail(1, "%v", err)
	}

	versionsByPath := map[string]string{} // path: version
	for _, v := range mod.Require {
		versionsByPath[v.Mod.Path] = v.Mod.Version
	}
	quickjsVersion := versionsByPath["modernc.org/quickjs"]
	resultsDir := path.Join("testdata", "benchmarks", quickjsVersion)
	resultsFn := path.Join(resultsDir, target)
	if fi, err := os.Stat(resultsFn); err == nil && fi.Mode().IsRegular() {
		return // done
	}

	os.Remove(resultsFn)
	os.MkdirAll(resultsDir, 0775)

	out, err := exec.Command("sh", "-c", fmt.Sprintf("make benchmark > %s", resultsFn)).CombinedOutput()
	fmt.Printf("err=%v out=%s\n", err, out)
	if err != nil {
		fail(1, "err=%v out=%s", err, out)
	}

	m, err := filepath.Glob(path.Join(resultsDir, "*.txt"))
	if err != nil {
		fail(1, "err=%v", err)
	}

	for i, v := range m {
		m[i] = filepath.ToSlash(v)
	}
	var results [][]string
	for _, fn := range m {
		result, err := os.ReadFile(fn)
		if err != nil {
			fail(1, "err=%v", err)
		}

		key := path.Base(fn)                    // os_arch.txt
		key = key[:len(key)-len(".txt")]        // os_arch
		key = strings.Replace(key, "_", "/", 1) // os/arch
		a := strings.Split(string(result), "\n")
		for _, v := range a {
			if f := strings.Fields(v); len(f) >= 4 && f[0] == "geomean" { // geomean 1.000 1.019 0.848
				f[0] = key
				results = append(results, f)
				continue
			}
		}
	}

	slices.SortFunc(results, func(a, b []string) int {
		switch {
		case a[0] < b[0]:
			return -1
		case a[0] == b[0]:
			return 0
		default:
			return 1
		}
	})
	doc, err := os.ReadFile(docFn)
	if err != nil {
		fail(1, "err=%v", err)
	}

	a := strings.Split(string(doc), "\n")
	buf := bytes.NewBuffer(nil)
	state := 0
	ver := func(ip string) string {
		return fmt.Sprintf("%s@%s", ip, versionsByPath[ip])
	}
	for _, v := range a {
		switch state {
		case 0:
			fmt.Fprintf(buf, "%s\n", v)
			if strings.HasPrefix(v, "// # Performance") {
				state++
			}
		case 1:
			fmt.Fprintf(buf, `//
// Geomeans over a set of benchmarks, relative to CCGO. Detailed results available
// in the testdata/benchmarks directory.
//
//  CCGO: %s
//  GOJA: %s
//   QJS: %s 
//  
//	                        CCGO     GOJA     QJS
//	-----------------------------------------------
`,
				ver("modernc.org/quickjs"),
				ver("github.com/dop251/goja"),
				ver("github.com/fastschema/qjs"),
			)
			for _, v := range results {
				fmt.Fprintf(buf, "//	%20s%9s%9s%9s%s\n", v[0], v[1], v[2], v[3], notes[v[0]])
			}
			fmt.Fprintf(buf, `//	-----------------------------------------------
//	                        CCGO     GOJA     QJS
`)
			state++
		case 2:
			switch {
			case strings.HasPrefix(v, "// # Notes"):
				fmt.Fprintf(buf, "%s\n", v)
				state++
			}
		case 3:
			fmt.Fprintf(buf, "%s\n", v)
		}
	}
	if err := os.WriteFile(docFn, buf.Bytes(), 0660); err != nil {
		fail(1, "err=%v", err)
	}
}
