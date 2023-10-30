#!/usr/bin/env bash

set -x

build_for() {
  GOOS=$2
  go build -C $1 ../... || { exit 1; }
  go test -C $1 -c -o /dev/null -tags cit || { exit 1; }
}

build() {
  build_for $1 linux
  build_for $1 windows
}

if golint &> /dev/null; then
  golint -set_exit_status ./...
fi

build cmd/manager/
build cmd/wrapper/

for suite in `find test_suites/* -maxdepth 0 -type d`; do
  build $suite
done
