#!/bin/bash
# Copyright 2017 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -xe

BUILD_DIR=$1
[[ -n $BUILD_DIR ]] && cd $BUILD_DIR/imagetest

RET=0

GOARCH=amd64
GOOS=linux
CGO_ENABLED=0
GO111MODULE=on
go mod download || RET=$?

go vet --structtag=false ./... || RET=$?

# Test the testworkflow package and generate code coverage
go test -v -coverprofile=/tmp/coverage.out . >${ARTIFACTS}/go-test.txt || RET=$?
go tool cover -func=/tmp/coverage.out | grep ^total | awk '{print $NF}' | cut -d'.' -f1 > ${ARTIFACTS}/coverage.txt
cat ${ARTIFACTS}/go-test.txt | go-junit-report > ${ARTIFACTS}/junit.xml

# Build wrapper for all necessary architectures
go build -o /dev/null ./cmd/wrapper/main.go || RET=$?
GOARCH=arm64 go build -o /dev/null ./cmd/wrapper/main.go || RET=$?
GOOS=windows go build -o /dev/null ./cmd/wrapper/main.go || RET=$?
GOOS=windows GOARCH=386 go build -o /dev/null ./cmd/wrapper/main.go || RET=$?

# Manager is only built for amd64 linux
go build -o /dev/null ./cmd/manager/main.go

for suite in test_suites/*; do
  [[ -d $suite ]] || continue
  go test -C $suite -c -o /dev/null -tags cit || RET=$?
  GOARCH=arm64 go test -C $suite -c -o /dev/null -tags cit || RET=$?
  GOOS=windows go test -C $suite -c -o /dev/null -tags cit || RET=$?
  GOOS=windows GOARCH=386 go test -C $suite -c -o /dev/null -tags cit || RET=$?
done

exit $RET
