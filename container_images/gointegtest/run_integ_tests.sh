#!/bin/bash

URL="http://metadata/computeMetadata/v1/instance/"
OUTS_PATH=$(curl -f -H Metadata-Flavor:Google ${URL}/attributes/daisy-outs-path)
SRC_PATH=$(curl -f -H Metadata-Flavor:Google ${URL}/attributes/daisy-sources-path)
REPO_OWNER=$(curl -f -H Metadata-Flavor:Google ${URL}/attributes/repo-owner)
REPO_NAME=$(curl -f -H Metadata-Flavor:Google ${URL}/attributes/repo-name)
GIT_REF=$(curl -f -H Metadata-Flavor:Google ${URL}/attributes/git-ref)
IMAGE=$(curl -f -H Metadata-Flavor:Google ${URL}/image)

export CGO_ENABLED=0

if [[ -e /etc/SUSEConnect ]]; then
  wget https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-311.0.0-linux-x86_64.tar.gz
  tar xf *.tar.gz
  export PATH=$PATH:${PWD}/google-cloud-sdk/bin
  zypper install -y python36
  export CLOUDSDK_PYTHON=/usr/bin/python3.6
fi

gsutil cp "${SRC_PATH}/common.sh" ./
. common.sh

if [[ -e /etc/debian_version ]]; then
  export DEBIAN_FRONTEND=noninteractive
  try_command apt-get -y update
  try_command apt-get install -y --no-install-{suggests,recommends} git-core
elif [[ -e /etc/SUSEConnect ]]; then
  zypper install -y git
else
  VERSION_ID=6
  if [[ -f /etc/os-release ]]; then
    eval $(grep VERSION_ID /etc/os-release)
    VERSION_ID=${VERSION_ID:0:1}
  fi

  GIT="git"
  if [[ ${VERSION_ID} =~ 6|7 ]]; then
      try_command yum install -y "https://repo.ius.io/ius-release-el${VERSION_ID}.rpm"
      rpm --import /etc/pki/rpm-gpg/RPM-GPG-KEY-IUS-${VERSION_ID}
      GIT="git224"
  fi

  try_command yum install -y $GIT
fi

install_go
git_checkout "$REPO_OWNER" "$REPO_NAME" "$GIT_REF"

$GO get -d -t ./...
$GO test -tags integration -v ./... > /go-test.txt

platform=$(echo $IMAGE|sed -e 's/.*\///' -e 's/-v.*//')
gsutil cp /go-test.txt ${OUTS_PATH}/go-test-${platform}.txt
if [[ $? -eq 0 ]]; then
  build_success
else
  build_fail
fi
