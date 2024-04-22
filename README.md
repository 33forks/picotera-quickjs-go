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

# Preliminary performance results

This package vs https://pkg.go.dev/github.com/dop251/goja

    goos: linux
    goarch: amd64
    pkg: modernc.org/quickjs
    cpu: AMD Ryzen 9 3900X 12-Core Processor
    BenchmarkArewefastyet/ccgo-24    1    114833264962 ns/op          22808 B/op            70 allocs/op
    BenchmarkArewefastyet/goja-24    1    188090359173 ns/op    28283063392 B/op    1771005768 allocs/op
    PASS
    ok  modernc.org/quickjs 302.936s

# Notes

Parts of the documentation were copied from the quickjs documentation, see
LICENSE-QUICKJS for details.

[C quickjs]: https://bellard.org/quickjs
[modernc.org/libquickjs]: https://pkg.go.dev/modernc.org/libquickjs
