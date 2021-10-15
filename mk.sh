#!/bin/bash

temp=$(mktemp -d)
unzip -d $temp quickjs-master.zip
rm -rf lib/ cdb.json
mkdir lib
cd $temp/quickjs-master
export CC=$(which gcc) AR=$(which gcc-ar)
ccgo -compiledb $OLDPWD/cdb.json make libquickjs.a
cd -
ccgo \
	-o lib/quickjs.go \
	-pkgname quickjs \
	-trace-translation-units \
	cdb.json libquickjs.a
rm -rf $temp
