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

URL="http://metadata/computeMetadata/v1/instance/attributes"
GCS_PATH=$(curl -f -H Metadata-Flavor:Google ${URL}/daisy-outs-path)
SRC_PATH=$(curl -f -H Metadata-Flavor:Google ${URL}/daisy-sources-path)
REPO_OWNER=$(curl -f -H Metadata-Flavor:Google ${URL}/repo-owner)
REPO_NAME=$(curl -f -H Metadata-Flavor:Google ${URL}/repo-name)
GIT_REF=$(curl -f -H Metadata-Flavor:Google ${URL}/git-ref)
EXTRA_REPO=$(curl -f -H Metadata-Flavor:Google ${URL}/extra-repo)
EXTRA_REPO_OWNER=$(curl -f -H Metadata-Flavor:Google ${URL}/extra-repo-owner)
EXTRA_GIT_REF=$(curl -f -H Metadata-Flavor:Google ${URL}/extra-git-ref)
BUILD_DIR=$(curl -f -H Metadata-Flavor:Google ${URL}/build-dir)
VERSION=$(curl -f -H Metadata-Flavor:Google ${URL}/version)
SBOM_UTIL_GCS_ROOT=$(curl -f -H Metadata-Flavor:Google ${URL}/sbom-util-gcs-root)
LKG_GCS_PATH=$(curl -f -H Metadata-Flavor:Google $URL/lkg_gcs_path)
SPEC_NAME=$(curl -f -H Metadata-Flavor:Google $URL/spec_name)

echo "Started build..."

gsutil cp "${SRC_PATH}/common.sh" ./
. common.sh

deploy_sbomutil

# disable the backports repo for debian-10
sed -i 's/^.*debian buster-backports main.*$//g' /etc/apt/sources.list

try_command apt-get -y update
try_command apt-get install -y --no-install-{suggests,recommends} git-core
try_command apt-get install -y make unzip

# We always install go, needed for goopack.
echo "Installing go"
install_go

# Install goopack.
GO111MODULE=on $GO install -v github.com/google/googet/v2/goopack@latest

# Install grpc proto compiler. 
install_protoc
$GO install -v google.golang.org/protobuf/cmd/protoc-gen-go@latest
$GO install -v google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

ORIG_DIR=$(pwd)
if [[ -n "$EXTRA_REPO" && -n "$EXTRA_GIT_REF" ]]; then
  CURRENT_EXTRA_REPO_OWNER="$REPO_OWNER"
  if [[ -n "$EXTRA_REPO_OWNER" ]]; then
    CURRENT_EXTRA_REPO_OWNER="$EXTRA_REPO_OWNER"
  fi
  echo "Pulling extra repo: $EXTRA_REPO from owner $CURRENT_EXTRA_REPO_OWNER with reference: $EXTRA_GIT_REF"
  git_checkout "$CURRENT_EXTRA_REPO_OWNER" "$EXTRA_REPO" "$EXTRA_GIT_REF"  
  # set extra repo owner if different from default GoogleCloudPlatform
  # git_checkout clones the repo and cd's into it. Make sure we are back in
  # original build directory.
  cd "$ORIG_DIR"
fi

git_checkout "$REPO_OWNER" "$REPO_NAME" "$GIT_REF"
if [[ -n "$BUILD_DIR" ]]; then
    cd "$BUILD_DIR"
fi

if find . -type f -iname '*.go' >/dev/null; then
  echo "Installing go dependencies"
  $GO mod download
fi

echo "Building package(s)"
if [[ -n "${SPEC_NAME}" ]]; then
  SPEC_FILE="${SPEC_NAME}.goospec"
  if [[ -n "${LKG_GCS_PATH}" ]]; then
    echo "Copying LKG binaries from ${LKG_GCS_PATH} to ./legacy_bin/${SPEC_NAME}/"
    mkdir -p "./legacy_bin/${SPEC_NAME}/"
    try_command gcloud storage cp --recursive "${LKG_GCS_PATH}/*" "./legacy_bin/${SPEC_NAME}/"
  fi
  goopack -var:version="$VERSION" "${SPEC_FILE}"
  generate_and_push_sbom ./ "${SPEC_FILE}" "${SPEC_NAME}" "${VERSION}"
else
  for spec in packaging/googet/*.goospec; do
    goopack -var:version="$VERSION" "$spec"
    name=$(basename "${spec}")
    pref=${name%.*}
    generate_and_push_sbom ./ "${spec}" "${pref}" "${VERSION}"
  done
fi
echo "Finished building all packages."

echo "Copying $(ls *.goo | xargs) to ${GCS_PATH}/"
gsutil cp -n *.goo "$GCS_PATH/"

echo "Cleaning up temporary legacy binary directory..."
if [ -d "./legacy_bin" ]; then
  rm -rf ./legacy_bin/
  echo "Removed ./legacy_bin/"
fi

build_success "Built `ls *.goo|xargs`"
