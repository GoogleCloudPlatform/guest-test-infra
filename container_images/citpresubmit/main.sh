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

cd imagetest/
RET=0

if [[ $1 == "test" ]]; then
	# Test the testworkflow package and generate code coverage
	go test -v -coverprofile=/tmp/coverage.out . >${ARTIFACTS}/go-test.txt || RET=$?
	go tool cover -func=/tmp/coverage.out | grep ^total | awk '{print $NF}' | cut -d'.' -f1 > ${ARTIFACTS}/coverage.txt
	cat ${ARTIFACTS}/go-test.txt | go-junit-report > ${ARTIFACTS}/junit.xml
fi

if [[ $1 == "build" ]]; then
	# Build all test suites and manager
	mkdir -p /tmp/cit
	./local_build.sh -o /tmp/cit -s "$(ls test_suites | xargs)" || RET=$?

	# Test that the manager can generate a workflow for all built suites on a Linux x86 and arm64 image and a windows image.
	# Currently non-blocking and informational only
	/tmp/cit/manager -print -images projects/debian-cloud/global/images/family/debian-12,projects/debian-cloud/global/images/family/debian-12-arm64,projects/windows-cloud/global/images/family/windows-2022 -project gcp-guest > /dev/null || RET=$?
fi

exit $RET
