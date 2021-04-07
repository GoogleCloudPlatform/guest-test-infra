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
cd test_manager
mkdir /out

echo "Start Building"
go get
cd ./testmanager
go build -v -o /out/test_manager
echo "go build exited with $?"
cd ..
cd ./test_wrapper
go build -v -o /out/test_wrapper
echo "go build exited with $?"
cd ..

cd ./test_suites
for test_suite in *; do
  go test -c ${test_suite} -o /out/${test_suite}.test
  echo "go test -c exited with $?"
done

echo "All build output:"
ls /out

sync
echo "Finish Building"
