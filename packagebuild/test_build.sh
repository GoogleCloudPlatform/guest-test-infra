#!/bin/bash

# Produce a single package build with dummy version.

DEFAULT_TYPE='deb9'
DEFAULT_PROJECT='gcp-guest'
DEFAULT_ZONE='us-west1-a'
DEFAULT_OWNER='GoogleCloudPlatform'
DEFAULT_GIT_REF='master'
DEFAULT_GCS_PATH='${SCRATCHPATH}/packages'

[[ -z $TYPE ]] && read -p "Build type [$DEFAULT_TYPE]: " TYPE
[[ -z $PROJECT ]] && read -p "Build project [$DEFAULT_PROJECT]: " PROJECT
[[ -z $ZONE ]] && read -p "Build zone [$DEFAULT_ZONE]: " ZONE
[[ -z $OWNER ]] && read -p "Repo owner or org [$DEFAULT_OWNER]: " OWNER
[[ -z $GIT_REF ]] && read -p "Ref [$DEFAULT_GIT_REF]: " GIT_REF
[[ -z $GCS_PATH ]] && read -p "GCS Path to upload to [$DEFAULT_GCS_PATH]: " GIT_REF
[[ -z $REPO ]] && read -p "Repo name: " REPO

[[ $TYPE == "" ]] && TYPE=$DEFAULT_TYPE
[[ $PROJECT == "" ]] && PROJECT=$DEFAULT_PROJECT
[[ $ZONE == "" ]] && ZONE=$DEFAULT_ZONE
[[ $OWNER == "" ]] && OWNER=$DEFAULT_OWNER
[[ $GIT_REF == "" ]] && GIT_REF=$DEFAULT_GIT_REF
[[ $GCS_PATH == "" ]] && GCS_PATH=$DEFAULT_GCS_PATH

WF="workflows/build_${TYPE}.wf.json"

if [[ ! -f "$WF" ]]; then
  echo "Unknown build type $TYPE"
  exit 1
fi

daisy \
  -project $PROJECT \
  -zone $ZONE \
  -var:gcs_path=$GCS_PATH \
  -var:repo_owner=$OWNER \
  -var:repo_name=$REPO \
  -var:git_ref=$GIT_REF \
  -var:version=1dummy \
  "$WF"
