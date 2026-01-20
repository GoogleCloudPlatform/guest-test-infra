#!/bin/bash

set -x

URL="http://metadata/computeMetadata/v1/instance/"
OUTS_PATH=$(curl -f -H Metadata-Flavor:Google ${URL}/attributes/daisy-outs-path)
SRC_PATH=$(curl -f -H Metadata-Flavor:Google ${URL}/attributes/daisy-sources-path)
REPO_OWNER=$(curl -f -H Metadata-Flavor:Google ${URL}/attributes/repo-owner)
REPO_NAME=$(curl -f -H Metadata-Flavor:Google ${URL}/attributes/repo-name)
GIT_REF=$(curl -f -H Metadata-Flavor:Google ${URL}/attributes/git-ref)
IMAGE=$(curl -f -H Metadata-Flavor:Google ${URL}/image)

export CGO_ENABLED=0

# SuSE images do not have the cloud SDK, needed for gcloud storage
if [[ -e /etc/SUSEConnect ]]; then
  wget https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-437.0.1-linux-x86_64.tar.gz
  tar xf *.tar.gz
  export PATH=$PATH:${PWD}/google-cloud-sdk/bin
  zypper install -y python36
  export CLOUDSDK_PYTHON=/usr/bin/python3.6
fi

gcloud storage cp "${SRC_PATH}/common.sh" ./
. common.sh

install_go

curl -Ss -L -o git.tar.gz "https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/tarball/${GIT_REF:-HEAD}"
mkdir git && cd git
tar --strip-components=1 -xvf ../git.tar.gz

$GO get -d -t ./...
$GO test -tags integration -v ./... > /go-test.txt || :

platform=$(echo $IMAGE|sed -e 's/.*\///' -e 's/-v.*//')
gcloud storage cp /go-test.txt ${OUTS_PATH}/go-test-${platform}.txt
if [[ $? -eq 0 ]]; then
  build_success
else
  build_fail
fi
