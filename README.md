# quickjs

Package quickjs is an idiomatic Go wrapper for [modernc.org/libquickjs], an
embeddable, CGo-free Javascript engine.

See also the original [C quickjs] library.

# Supported platforms and architectures

These combinations of GOOS and GOARCH are currently supported

    OS      Arch
    -------------
    linux   amd64
    linux   loong64

# Builders

Builder results are available at:

https://modern-c.appspot.com/-/builder/?importpath=modernc.org%2fquickjs

# Performance

This package @ v0.8.0

vs https://pkg.go.dev/github.com/dop251/goja @v0.0.0-20240220182346-e401ed450204

    go test -timeout 24h -run @ -bench . 2>&1 | tee log-benchmark
    goos: linux
    goarch: amd64
    pkg: modernc.org/quickjs
    cpu: AMD Ryzen 9 3900X 12-Core Processor            
    BenchmarkArewefastyet/ccgo-24    1    115820232777 ns/op          22680 B/op            67 allocs/op
    BenchmarkArewefastyet/goja-24    1    184796170244 ns/op    28111172328 B/op    1755493570 allocs/op
    PASS
    ok  modernc.org/quickjs 300.630s

# Notes

Parts of the documentation were copied from the quickjs documentation, see
LICENSE-QUICKJS for details.

[C quickjs]: https://bellard.org/quickjs
[modernc.org/libquickjs]: https://pkg.go.dev/modernc.org/libquickjs
