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

echo "Running daisy integration test workflow"
cd /  # Prow will put us in the dir with repo checked out.

GCS_PATH="${GCS_BUCKET}/`date '+%s'`"

if [[ "$JOB_TYPE" == "presubmit" ]]; then
  GIT_REF="pull/${PULL_NUMBER}/head"
else
  GIT_REF="$PULL_BASE_REF"
fi

/daisy -project "$PROJECT" -zone "$ZONE" -var:repo_name="$REPO_NAME" \
  -var:repo_owner="$REPO_OWNER" -var:git_ref="$GIT_REF" \
  -var:gcs_path="$GCS_PATH" integ-test-all.wf.json
RET=$?

gsutil cp "$GCS_PATH"/go-test*.txt ./

for f in go-test*.txt; do
  # $ARTIFACTS is provided by prow decoration containers
  cp "$f" "${ARTIFACTS}/"
  cat "$f" | /go-junit-report > "${ARTIFACTS}/junit_${f%%.txt}.xml"
done

echo Done
exit $RET
