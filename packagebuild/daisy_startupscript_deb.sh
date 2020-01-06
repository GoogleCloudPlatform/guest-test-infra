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
VERSION=$(curl -f -H Metadata-Flavor:Google ${URL}/version)
VERSION=${VERSION:="1dummy"}

DEBIAN_FRONTEND=noninteractive

echo "Started build..."

gsutil cp "${SRC_PATH}/common.sh" ./
. common.sh

try_command apt-get -y update
try_command apt-get install -y --no-install-{suggests,recommends} git-core \
  debhelper devscripts build-essential equivs

git_checkout "$REPO_OWNER" "$REPO_NAME" "$GIT_REF"

PKGNAME="$(grep "^Package:" ./packaging/debian/control|cut -d' ' -f2-)"

# Install build deps
mk-build-deps -t "apt-get -o Debug::pkgProblemResolver=yes \
  --no-install-recommends --yes" --install packaging/debian/control
dpkg-checkbuilddeps packaging/debian/control

if grep -q '+deb' packaging/debian/changelog; then
  DEB=$(</etc/debian_version)
  DEB="+deb${DEB:0:1}"
fi

if grep -q 'golang' packaging/debian/control; then
  echo "Installing go"
  install_go

  echo "Installing go dependencies"
  $GO mod download
fi

echo "Building package(s)"

# Create build dir.
BUILD_DIR="/tmp/debpackage/"
[[ -d $BUILD_DIR ]] || mkdir $BUILD_DIR

# Create 'upstream' tarball.
TAR="${PKGNAME}_${VERSION}.orig.tar.gz"
tar czvf "${BUILD_DIR}/${TAR}" --exclude .git --exclude packaging \
  --transform "s/^\./${PKGNAME}-${VERSION}/" .

# Extract tarball and build.
tar -C "$BUILD_DIR" -xzvf "${BUILD_DIR}/${TAR}"
PKGDIR="${BUILD_DIR}/${PKGNAME}-${VERSION}"
cp -r packaging/debian "${BUILD_DIR}/${PKGNAME}-${VERSION}/"

cd "${BUILD_DIR}/${PKGNAME}-${VERSION}"

# We generate this to enable auto-versioning.
[[ -f debian/changelog ]] && rm debian/changelog
dch --create -M -v 1:${VERSION}-g1${DEB} --package $PKGNAME -D stable \
  "Debian packaging for ${PKGNAME}"
debuild -e "VERSION=${VERSION}" -us -uc

echo "copying $BUILD_DIR/*.deb to $GCS_PATH"
gsutil cp -n "$BUILD_DIR"/*.deb "$GCS_PATH"
build_success "Built `ls "$BUILD_DIR"/*.deb|xargs`"
