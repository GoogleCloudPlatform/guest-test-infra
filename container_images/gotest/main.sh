#!/bin/bash
# Copyright 2019 Google Inc. All Rights Reserved.
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

set -x

BUILD_DIR=$1
if [[ -n $BUILD_DIR ]]; then
  cd $BUILD_DIR || exit 1
fi
shift

echo "Pulling Linux imports..."
go get -d -t ./... || exit 1
echo "Pulling Windows imports..."
GOOS=windows go get -d -t ./... || exit 1

mkdir /tmp/tests/
mkdir /tmp/coverage/

# Run go test for all directories in each module
for dir in $(find $(pwd) -name go.mod | xargs dirname); do
  TESTARGS=("-C" "$dir" "-coverprofile=/tmp/coverage.out")
  # Verbose output from registry-image-forked causes go-junit-report to output
  # invalid utf-8 for some reason.
  [[ $(basename $dir) == "registry-image-forked" ]] || TESTARGS+=("-v")
  # Skip subdirectory testing in CIT because we use those for test suites
  [[ $(basename $dir) == "imagetest" ]] || TESTARGS+=("./...")

  rm -f /tmp/coverage.out
  go test "${TESTARGS[@]}" > "/tmp/tests/$(basename $dir)"
  R=$?

  if [[ $R -ne 0 ]]; then
	RET=$R
    echo "go test ${TESTARGS[@]} returned ${RET}"
  fi

  # Concatenate test ouput into artifacts
  cat "/tmp/tests/$(basename $dir)" >> ${ARTIFACTS}/go-test.txt

  # Generate coverage percentage for this module
  go tool -C $dir cover -func=/tmp/coverage.out | grep ^total | awk '{print $NF}' | cut -d'.' -f1 > "/tmp/coverage/$(basename $dir)"
done


# $ARTIFACTS is provided by prow decoration containers
# Generate a junit report for all modules
cat /tmp/tests/* | go-junit-report > ${ARTIFACTS}/junit.xml

# Average module coverage and record individual values in artifacts
nummodules="$(ls /tmp/coverage/ | wc -l)"
sum=0
for mod in $(ls /tmp/coverage/); do
  modcov=$(cat /tmp/coverage/$mod)
  echo "$mod $modcov" >> ${ARTIFACTS}/module-coverage.txt
  sum=$(($sum + $modcov))
done
echo $(($sum / $nummodules)) > ${ARTIFACTS}/coverage.txt

echo Done
exit "$RET"
