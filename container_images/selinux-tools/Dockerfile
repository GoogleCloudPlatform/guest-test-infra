# Copyright 2018 Google Inc. All Rights Reserved.
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

FROM alpine

COPY build_semodule_utils.sh /build_semodule_utils.sh
RUN apk add --no-cache bash
RUN bash /build_semodule_utils.sh

FROM alpine

RUN apk add --no-cache bash file make
RUN apk add --no-cache -X http://dl-cdn.alpinelinux.org/alpine/edge/testing policycoreutils checkpolicy

# Copy the semodule_package binary which isn't available from APK.
COPY --from=0 /usr/bin/semodule_package /usr/bin/semodule_package

# Copy this Dockerfile for debugging.
COPY Dockerfile /Dockerfile
WORKDIR /
# This image has no entrypoint. It only provides an env with these tools.
