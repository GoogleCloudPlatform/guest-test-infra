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

PROJECT=gcp-guest
ZONE=us-central1-c
DAISY_OUTS_BKT="$PROJECT-daisy-bkt"

echo "Pulling imports"
go get -d -t ./...

echo "Running daisy workflow for package build"

/daisy -project ${PROJECT} -zone us-central1-c -var:gcs_path=gs://${PKG_GCS_OUT_DIR} ./packaging/build_packages.wf.json

# copy daisy logs and artifacts to artifacts folder for prow
gsutil cp gs://${PKG_GCS_OUT_DIR} ${ARTIFACTS}/
gsutil cp gs://${DAISY_OUTS_BKT} ${ARTIFACTS}/