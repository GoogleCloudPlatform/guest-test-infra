#!/bin/sh
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

ValidateReturn() {
  RET=$?
  if [ $RET -ne 0 ]; then
    GOBUILD_OUT=$RET
    echo "'go build or go test -c' exited with ${GOBUILD_OUT}"
  else
    echo "'go build or go test -c' success"
  fi
}

export CGO_ENABLED=0 GOOS=linux
BUILD_DIR=./test_manager
chmod o+x -R $BUILD_DIR
[ -n "$BUILD_DIR" ] && cd $BUILD_DIR
mkdir /out

GOBUILD_OUT=0

echo "Start Building"
for d in ./*; do
  d=`basename $d`
  if [ "${d}" = "test_suites" ];then
    for test_suite in ${d}/*; do
      test_suite=`basename ${test_suite}`
      go test -c ./test_suites/${test_suite} -o /out/${test_suite}.test
      ValidateReturn
    done
  else
    pushd ${d} || continue
    go get && go build -v -o /out/${d}
    ValidateReturn
    popd
  fi
done

echo "All build output:"
ls /out

sync
echo "Finish Building"
exit $GOBUILD_OUT