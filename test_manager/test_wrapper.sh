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

echo "This is test wrapper which invoke E2E test binary"

IFS=',' read -r -a test_binary <<< "$1"
token=$2
storage_url=$3
WORK_DIR=$4

# Download and execute test binary
for binary in "${test_binary[@]}"
do
  curl -s -S -H "Metadata-Flavor: Google" -H "Authorization: $token" -X GET "$storage_url/$binary" o "$WORK_DIR"
  chmod +x $WORK_DIR/$binary
  "$WORK_DIR/$binary"
done

# TODO upload test result
echo "Upload E2E test result"

