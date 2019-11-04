#!/bin/bash

function usage() {
  echo "Usage: DISTROS=x,y,z $0"
  exit 1
}

function genwf() {
  config='{
  "Name": "build-packages",
  "Vars": {
    "gcs_path": {
      "Value": "${SCRATCHPATH}/packages",
      "Description": "GCS path for built packages e.g. gs://my-bucket/packages"
    },
    "repo_owner": {
      "Value": "GoogleCloudPlatform",
      "Description": "GitHub repo owner or organization"
    },
    "repo_name": {
      "Description": "Github repo name",
      "Required": true
    },
    "git_ref": {
      "Value": "master",
      "Description": "Branch to build from"
    }
  },
  "Steps": {'
  for distro in ${DISTROS//,/ }; do
    if [[ "$config" =~ IncludeWorkflow ]]; then
      config="${config},"
    fi
    config="${config}\n"'    "'"$distro"'": {
      "IncludeWorkflow": {
        "Path": "./workflows/build_'"$distro"'.wf.json",
        "Vars": {
          "gcs_path": "${gcs_path}",
          "repo_owner": "${repo_owner}",
          "repo_name": "${repo_name}",
          "git_ref": "${git_ref}"
        }
      }
    }'
  done
  config="$config"'
  }
}'
  echo -e "$config"
}

[[ -z "$DISTROS" ]] && usage
genwf > ${WF:="temp.wf.json"}
daisy -project liamh-testing -zone us-west1-b -var:repo_name=osconfig $WF
rm $WF
