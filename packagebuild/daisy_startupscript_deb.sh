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
GOBUILD=$(curl -f -H Metadata-Flavor:Google ${URL}/gobuild)
VERSION=$(curl -f -H Metadata-Flavor:Google ${URL}/version)

DEBIAN_FRONTEND=noninteractive

echo "Started build..."

gsutil cp "${SRC_PATH}/common.sh" ./
. common.sh

try_command apt-get -y update
try_command apt-get install -y --no-install-{suggests,recommends} git-core \
  debhelper devscripts build-essential equivs

git_checkout "$REPO_OWNER" "$REPO_NAME" "$GIT_REF"

# Ensure deps are met
mk-build-deps -t "apt-get -o Debug::pkgProblemResolver=yes \
  --no-install-recommends --yes" --install packaging/debian/control
dpkg-checkbuilddeps packaging/debian/control

if [[ -n "$GOBUILD" ]]; then
  echo "Installing go"
  install_go

  echo "Installing go dependencies"
  go mod download
fi

echo "Building package(s)"
[[ -d $dpkg_working_dir ]] && rm -rf $dpkg_working_dir
mkdir $dpkg_working_dir
tar czvf $dpkg_working_dir/${PKGNAME}_${VERSION}.orig.tar.gz --exclude .git \
  --exclude packaging --transform "s/^\./${PKGNAME}-${VERSION}/" .

working_dir=${PWD}
cd $dpkg_working_dir
tar xzvf ${PKGNAME}_${VERSION}.orig.tar.gz

cd ${PKGNAME}-${VERSION}

cp -r ${working_dir}/packaging/debian ./
cp -r ${working_dir}/*.service ./debian/

debuild -e "VERSION=${VERSION}" -us -uc
