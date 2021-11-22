# Copyright 2021 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
FROM gcr.io/gcp-guest/gotest

# gcsfuse install instructions:
#  https://github.com/GoogleCloudPlatform/gcsfuse/blob/master/docs/installing.md
RUN DEBIAN_FRONTEND=noninteractive apt-get install -q -y qemu-utils gnupg ca-certificates \
  && echo "deb http://packages.cloud.google.com/apt gcsfuse-buster main" > /etc/apt/sources.list.d/gcsfuse.list \
  && curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - \
  && apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -q -y gcsfuse \
  && rm -rf /var/cache/apt/archives
