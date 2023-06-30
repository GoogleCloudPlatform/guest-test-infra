#!/bin/bash
# Copyright 2020 Google Inc. All Rights Reserved.
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

echo "Validate Integration Test Result"

gcloud auth activate-service-account --key-file="$GOOGLE_APPLICATION_CREDENTIALS"
gsutil cp "$GCS_PATH"/go-test*.txt ./

RET=0

# Convert txt report to xml
for f in go-test*.txt; do
  if grep -qc "FAIL" "$f"; then
    RET=1
  fi
  # remove prefix go-test and suffix .txt
  platform=${f%.txt}
  platform=${platform#go-test-}
  echo $platform

  cat "$f"
  # convert txt to xml
  cat "$f" | grep -v "github" | /go-junit-report  -package-name ${PACKAGE_NAME}-${platform} > "./junit_${f%%.txt}.xml"
done

# Convert xml report to html
echo "Merge Test Result"
/usr/local/lib/node_modules/junit-merge/bin/junit-merge ./junit_*.xml -o junit_all_distros.xml
/junit2html ./junit_all_distros.xml ./junit_all_distros.html

gsutil cp ./junit_*.* "$GCS_PATH"/

# Upload test report to GCS
echo "Test Result Report"
echo $GCS_URL/junit_all_distros.html
echo Done

exit $RET
