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
[[ -n $BUILD_DIR ]] && cd $BUILD_DIR

echo 'Pulling Linux imports...'
go get -d -t ./...
echo 'Pulling Windows imports...'
GOOS=windows go get -d -t ./...

# We dont run golint on Windows only code as style often matches win32 api
# style, not golang style
golint -set_exit_status ./...
GOLINT_RET=$?
if [[ $GOLINT_RET -ne 0 ]]; then
  echo "'golint ./...' returned ${GOLINT_RET}"
fi

GOFMT_OUT=$(gofmt -d $(find . -type f -name "*.go") 2>&1)
if [[ -z "${GOFMT_OUT}" ]]; then
  GOFMT_RET=0
else
  GOFMT_RET=1
  echo "'gofmt -d \$(find . -type f -name \"*.go\")' output: ${GOFMT_OUT}"
fi

go vet --structtag=false ./...
GOVET_RET=$?
if [[ $GOVET_RET -ne 0 ]]; then
  echo "'go vet --structtag=false ./...' returned ${GOVET_RET}"
fi

GOOS=windows go vet --structtag=false ./...
RET=$?  # Don't overwrite GOVET_RET in case this is 0 but it was not.
if [[ $RET -ne 0 ]]; then
  GOVET_RET=$RET
  echo "'GOOS=windows go vet --structtag=false ./...' returned ${GOVET_RET}"
fi

echo Done

if [[ ${GOLINT_RET} -ne 0 ]] || [[ ${GOFMT_RET} -ne 0 ]] || [[ ${GOVET_RET} -ne 0 ]]; then
  exit 1
fi

exit 0
