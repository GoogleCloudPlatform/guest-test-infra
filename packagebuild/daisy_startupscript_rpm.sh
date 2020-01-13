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

# This script is expected to be run on an Enterprise Linux system (RHEL, CentOS)
# on GCE with various flags set in the instance metadata including the git
# repository to clone. The script produces an RPM defined by an RPM spec in the
# packaging/ directory from the cloned repo.

URL="http://metadata/computeMetadata/v1/instance/attributes"
function get_md() {
  curl -f -H Metadata-Flavor:Google "${URL}/${1}"
}
GCS_PATH=$(get_md daisy-outs-path)
SRC_PATH=$(get_md daisy-sources-path)
REPO_OWNER=$(get_md repo-owner)
REPO_NAME=$(get_md repo-name)
GIT_REF=$(get_md git-ref)
VERSION=$(get_md version)
VERSION=${VERSION:-"dummy"}

echo "Started build..."

# common.sh contains functions common to all builds.
gsutil cp "${SRC_PATH}/common.sh" ./
. common.sh

# Install git2 as this is not available in centos 6/7
VERSION_ID=6
if [[ -f /etc/os-release ]]; then
  eval $(grep VERSION_ID /etc/os-release)
  VERSION_ID=${VERSION_ID:0:1}
fi

GIT="git"
if [[ ${VERSION_ID} =~ 6|7 ]]; then
    try_command yum install -y https://rhel${VERSION_ID}.iuscommunity.org/ius-release.rpm
    rpm --import /etc/pki/rpm-gpg/RPM-GPG-KEY-IUS-${VERSION_ID}
    GIT="git2u"
fi

try_command yum install -y $GIT rpmdevtools yum-utils

git_checkout "$REPO_OWNER" "$REPO_NAME" "$GIT_REF"

if grep -q '%{_go}' ./packaging/*.spec; then
  echo "Installing go"
  install_go

  echo "Installing go dependencies"
  $GO mod download
fi

# Make build dirs as needed.
RPMDIR=/usr/src/redhat
for dir in ${RPMDIR}/{SOURCES,SPECS}; do
  [[ -d "$dir" ]] || mkdir -p "$dir"
done

# Find the RPM specs.
for spec in ./packaging/*.spec; do
  spec=$(basename "$spec")
  # If the spec file has elN in it, only add it if N matches VERSION_ID
  if [[ "$spec" =~ \.el[0-9] ]]; then
    [[ "$spec" =~ \.el${VERSION_ID} ]] && SPECS="${SPECS} ${spec}"
  else
    SPECS="${SPECS} ${spec}"
  fi
done

[[ -z "$SPECS" ]] && build_fail "No RPM specs found"

echo "Building package(s)"
for spec in $SPECS; do
  PKGNAME="$(grep Name: "./packaging/${spec}"|cut -d' ' -f2-|tr -d ' ')"
  yum-builddep -y "./packaging/${spec}"

  cp "./packaging/${spec}" "${RPMDIR}/SPECS/"
  tar czvf "${RPMDIR}/SOURCES/${PKGNAME}_${VERSION}.orig.tar.gz" \
    --exclude .git --exclude packaging \
    --transform "s/^\./${PKGNAME}-${VERSION}/" .

  rpmbuild --define "_topdir ${RPMDIR}/" --define "_version ${VERSION}" \
    --define "_go ${GO:-"UNSET"}" --define "_gopath ${GOPATH:-"UNSET"}" \
    -ba "${RPMDIR}/SPECS/${spec}"
done

rpms=$(find ${RPMDIR}/{S,}RPMS -iname "${PKGNAME}*.rpm")
echo "copying ${rpms} to $GCS_PATH/"
gsutil cp -n ${rpms} "$GCS_PATH/"
build_success "Built ${rpms}"
