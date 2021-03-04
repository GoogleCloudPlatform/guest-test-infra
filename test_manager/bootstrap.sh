# Copyright 2021 Google Inc. All Rights Reserved.
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

GetMetadataAttribute() {
    attribute="$1"
    url="http://metadata.google.internal/computeMetadata/v1/instance/attributes/$attribute"
    attribute_value=$(curl -H "Metadata-Flavor: Google" -X GET "$url")
}

echo "This is Startup Script"

# Get token
metadata_url="http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
response=$(curl -sS -H "Metadata-Flavor: Google" -X GET "$metadata_url")
access_token=$(echo $response | cut -d '"' -f 4)
token_type=$(echo $response | grep -o '"token_type":".*"' | cut -d '"' -f 4)
token="$token_type $access_token"

# Get bucket and bucket_dir
GetMetadataAttribute "files_gcs_dir" && files_gcs_dir="$attribute_value"
GetMetadataAttribute "test_binary_path" && test_binary_path="$attribute_value"

path_stripped=$(echo $files_gcs_dir | sed -r 's#^gs://##')
IFS='/' read -r bucket bucket_dir <<EOF
$path_stripped
EOF

# Download test wrapper
echo "Download E2E test wrapper"
WORK_DIR="/e2etest" && mkdir -p $WORK_DIR
storage_url="https://storage.googleapis.com/$bucket/$bucket_dir"
curl -sS -H "Metadata-Flavor: Google" -H "Authorization: $token" -X GET -o $WORK_DIR/test_wrapper.sh "$storage_url/test_wrapper.sh"

# Execute test wrapper
test_wrapper_script="$WORK_DIR/test_wrapper.sh"
chmod +x "$test_wrapper_script"
"$test_wrapper_script" $test_binary_path $token $storage_url $WORK_DIR

echo "E2ESuccess"