# Copyright 2024 The quickjs-go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

.PHONY:	all clean edit editor work test cpu mem short-test benchmark cover

all: editor
	golint 2>&1
	staticcheck 2>&1

benchmark:
	go test -vet=off -timeout 24h -run @ -bench . 2>&1 | tee log-benchmark

clean:
	rm -f log-* cpu.test mem.test *.out go.work*
	go clean

edit:
	@touch log
	@if [ -f "Session.vim" ]; then gvim -S & else gvim -p Makefile go.mod *.go & fi

editor:
	gofmt -l -s -w . 2>&1 | tee log-editor
	go test -c -o /dev/null ./... 2>&1 | tee -a log-editor
	go install -v  ./... 2>&1 | tee -a log-editor
	staticcheck 2>&1

test:
	go test -failfast -v -timeout 24h -count=1 ./... 2>&1 | tee log-test
	grep -a 'TRC\|TODO\|ERRORF\|FAIL' log-test || true 2>&1 | tee -a log-test

short-test:
	go test -failfast -v -short -timeout 24h -count=1 ./... 2>&1 | tee log-test
	grep -a 'TRC\|TODO\|ERRORF\|FAIL' log-test || true 2>&1 | tee -a log-test

work:
	rm -f go.work*
	go work init
	go work use .
	go work use ../../libc
	go work use ../../libquickjs

cpu: clean
	go test -run @ -bench BenchmarkArewefastyet/ccgo -cpuprofile cpu.out
	go tool pprof -web *.test cpu.out

mem: clean
	go test -run @ -bench BenchmarkArewefastyet/ccgo -memprofile mem.out -memprofilerate 1 -timeout 24h
	go tool pprof -lines -web -alloc_space *.test mem.out

leak:
	go test -v -run TestMemgrind2 -tags=libc.memgrind 2>&1 | tee log-memgrind

cover:
	cover=$(shell mktemp) ; \
	      go test -v -short -coverprofile=$$cover ; \
	      go tool cover -html=$$cover
