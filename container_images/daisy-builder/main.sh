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
GCS_OUTPUT_BUCKET="$4"  # Destination for artifacts

function generate_new_version() {
  VERSION_GENERATE_CMD="/versiongenerator --token-file-path=${GITHUB_ACCESS_TOKEN} --org=${REPO_OWNER} --repo=${REPO_NAME}"
  local VERSION=$(${VERSION_GENERATE_CMD})

  if [[ $? -ne 0 ]]; then
    echo -e "could not generate version: $VERSION_GENERATE_OUT"
  fi

  echo "$VERSION"
  return 0
}

# Sets service account used for daisy and gsutil commands below. Will use
# default service account for VM or k8s node if not set.
if [[ -n $GOOGLE_APPLICATION_CREDENTIALS ]]; then
  gcloud auth activate-service-account --key-file=$GOOGLE_APPLICATION_CREDENTIALS
fi

echo "Running daisy workflow for package build"

## REPO_OWNER and PULL_NUMBER are set by prow
DAISY_CMD="/daisy -project ${PROJECT} -zone ${ZONE}"
DAISY_VARS="base_repo=${REPO_OWNER}"

## only add pull reference in case of presubmit jobs
if [[ "$JOB_TYPE" == "presubmit" ]]; then
  DAISY_VARS+=",pull_ref=pull/${PULL_NUMBER}/head"
fi

## generate buildID
if [[ "$JOB_TYPE" == "postsubmit" ]]; then
  VERSION=$(generate_new_version)
  DAISY_VARS+=",version=${VERSION}"
fi

DAISY_CMD+=" -variables ${DAISY_VARS} ${WORKFLOW_FILE}"

echo "running daisy command..."
echo $DAISY_CMD
$DAISY_CMD 2>err | tee out 
if [[ ${PIPESTATUS[0]} -ne 0 ]]; then
  echo "error running daisy: stderr: $(<err)"
  exit 1
fi

pattern="https://console.cloud.google.com/storage/browser/"
DAISY_BUCKET="gs://$(sed -En "s|(^.*)$pattern||p" out)"

# copy daisy logs and artifacts to artifacts folder for prow
# $ARTIFACTS is set by prow
if [[ -n $ARTIFACTS ]]; then
  echo "copying daisy outputs from $DAISY_BUCKET to prow artifacts dir"
  gsutil cp "${DAISY_BUCKET}/outs/*" ${ARTIFACTS}/
fi

# If invoked as periodic, postsubmit, or manually, upload the results.
if [[ "$JOB_TYPE" != "presubmit" ]]; then
  gsutil cp "${DAISY_BUCKET}/outs/*" $GCS_OUTPUT_BUCKET
fi