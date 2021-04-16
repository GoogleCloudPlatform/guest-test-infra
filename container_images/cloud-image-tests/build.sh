#!/bin/bash
# Copyright 2021 Google Inc. All Rights Reserved.
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

set -e

export CGO_ENABLED=0 GOOS=linux

cd imagetest
mkdir /out

echo "Start Building"
go get ./...

pushd cmd/manager
go build -v -o /out/manager
popd

pushd cmd/wrapper
go build -v -o /out/wrapper
popd

cd test_suites
for suite in *; do
  go test -c "$suite" -o "/out/${suite}.test"
done

echo "Build output:"
ls /out

sync
echo "Finished building"
