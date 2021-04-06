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
FROM golang as builder

WORKDIR /go/src/
COPY . .
RUN chmod +x ./container_images/test-manager/build.sh
RUN ./container_images/test-manager/build.sh

FROM alpine
COPY --from=builder /out/* /

ENTRYPOINT ["/test_manager"]