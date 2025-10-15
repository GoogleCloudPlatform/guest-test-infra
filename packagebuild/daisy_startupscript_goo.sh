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

echo "Scanning for goospec files and processing packages..."
GCS_BUCKET="gs://gce-image-build-resources"
# Create a base directory for all legacy content
mkdir -p ./legacy_bin

# Loop through each .goospec file in the packaging/googet directory
for spec in *.goospec; do
  name=$(basename "${spec}")
  pkg_name=${name%.*}

  echo "--- Processing package: ${pkg_name} ---"
  # gs://gce-image-build-resources/windows/{pkg_name}/

  gcs_pkg_root="windows/${pkg_name}/"
  gcs_full_path="${GCS_BUCKET}/${gcs_pkg_root}"

  # Local directory to download content into
  local_pkg_legacy_root="./legacy_bin/${pkg_name}/"

  echo "  Checking for legacy content for ${pkg_name} at: ${gcs_full_path}"
  # Check if the GCS prefix exists and has contents by listing.
  if gcloud storage ls "${gcs_full_path}" > /dev/null 2>&1; then
    echo "  Found legacy content. Syncing recursively to ${local_pkg_legacy_root}"
    mkdir -p "${local_pkg_legacy_root}"
    # Copy the contents of the GCS prefix.
    try_command gcloud storage cp --recursive "${gcs_full_path}*" "${local_pkg_legacy_root}"
  else
    echo "  No legacy content found at ${gcs_full_path}. Skipping sync."
  fi
  
  echo "  Building package: ${pkg_name}"
  goopack -var:version="$VERSION" "$spec"
  generate_and_push_sbom ./ "${spec}" "${pkg_name}" "${VERSION}"
done

echo "Finished building all packages."

gsutil cp -n *.goo "$GCS_PATH/"

echo "Cleaning up temporary legacy binary directory..."
if [ -d "./legacy_bin" ]; then
  rm -rf ./legacy_bin/
  echo "Removed ./legacy_bin/"
fi

build_success "Built `ls *.goo|xargs`"
