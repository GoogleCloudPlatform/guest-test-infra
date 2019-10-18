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

# Check this out in GOPATH since go package handling requires it to be here.
#REPO_PATH=${GOPATH}/src/github.com/${REPO_OWNER}/${REPO_NAME}
#mkdir -p ${REPO_PATH}
#git clone https://github.com/${REPO_OWNER}/${REPO_NAME} ${REPO_PATH}
#cd ${REPO_PATH}
#
## Pull PR if this is a PR.
#if [ ! -z "${PULL_NUMBER}" ]; then
#  git fetch origin pull/${PULL_NUMBER}/head:${PULL_NUMBER}
#  git checkout ${PULL_NUMBER}
#fi

[[ -d /tmp/gobuild ]] && rm -rf /tmp/gobuild
mkdir /tmp/gobuild

go version

echo 'Pulling Linux imports...'
go get -d ./...
echo 'Pulling Windows imports...'
GOOS=windows go get -d ./...

GOBUILD_OUT=0
#cd /

echo "Building for Linux"
go build -o /tmp/gobuild ./...
RET=$?
if [[ $RET -ne 0 ]]; then
  GOBUILD_OUT=$RET
  echo "'go build' exited with ${GOBUILD_OUT}"
fi
rm /tmp/gobuild

echo "Building for Windows"
GOOS=windows go build -o /tmp/gobuild ./...
RET=$?
if [[ $RET -ne 0 ]]; then
  GOBUILD_OUT=$RET
  echo "'GOOS=windows go build' exited with ${GOBUILD_OUT}"
fi

sync
echo Done
exit $GOBUILD_OUT
