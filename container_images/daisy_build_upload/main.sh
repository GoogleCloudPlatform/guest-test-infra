#!/bin/bash
# Copyright 2019 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

PROJECT="$1"
ZONE="$2"
WORKFLOW_FILE="$3"      # Workflow to run
GCS_OUTPUT_BUCKET="$4"  # Destination for artifacts (used in postsubmit only).

echo "Running daisy workflow for package build"

## REPO_OWNER and PULL_NUMBER are set by prow
DAISY_CMD="/daisy -project ${PROJECT} -zone ${ZONE}"
DAISY_VARS="base_repo=${REPO_OWNER}"

## only add pull reference in case of presubmit jobs
if [[ "$JOB_TYPE" == "presubmit" ]]; then
  DAISY_VARS+=",pull_ref=pull/${PULL_NUMBER}/head"
fi

DAISY_CMD+=" -variables ${DAISY_VARS} ${WORKFLOW_FILE}"

if ! out="$($DAISY_CMD 2>&1)"; then
  echo "error running daisy: ${out}"
  exit 1
fi

pattern="https://console.cloud.google.com/storage/browser/"
DAISY_BUCKET="gs://$(echo "$out"| sed -En "s|(^.*)$pattern| |p")"

# copy daisy logs and artifacts to artifacts folder for prow
# $ARTIFACTS is set by prow
echo "copying daisy outputs from $DAISY_BUCKET"
gsutil cp "${DAISY_BUCKET}/*" ${ARTIFACTS}/

if [[ "$JOB_TYPE" == "postsubmit" ]]; then
  gsutil cp "${DAISY_OUTS_PATH}/*" $GCS_OUTPUT_BUCKET
fi
