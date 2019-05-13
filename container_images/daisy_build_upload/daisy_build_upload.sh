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

# Takes following inputs
## project($1): gcp project where daisy will spin up vms
## region($2): gcp region where daisy will spin up vms
## workflow($3): dasiy workflow file
## final_output_path($4): gcs bucket path, where the final artifacts need to be uploaded


set -e

echo "Running daisy workflow for package build"

## repo_owner, repo_name and pill_base_ref is set by prow
DAISY_CMD="/daisy -project $1 -zone $2 -var:base_repo=$REPO_OWNER"

## only add pull reference in case of presubmit jobs
if [ "$JOB_TYPE" = "presubmit" ]; then
  DAISY_CMD+=" -var:pull_ref=pull/$PULL_NUMBER/head:pr-$PULL_NUMBER"
fi

DAISY_CMD+=" $3"

if ! out=$(DAISY_CMD); then
  echo "error running daisy..." && exit 1
fi

pattern="https://console.cloud.google.com/storage/browser/"
bucket_name=$(echo "$out"| sed -En "s|(^.*)$pattern| |p")

# copy daisy logs and artifacts to artifacts folder for prow
# $ARTIFACTS is set by prow
DAISY_BUCKET="gs://$bucket_name"
echo "copying daisy outputs from $DAISY_BUCKET"
gsutil cp DAISY_BUCKET ${ARTIFACTS}/

if [ "$JOB_TYPE" = "postsubmit" ]; then
  echo ""
  gsutil cp ${DAISY_OUTS_PATH} $4
fi
