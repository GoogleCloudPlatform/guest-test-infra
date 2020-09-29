# Copyright 2020 Google Inc. All Rights Reserved.
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
FROM gcr.io/compute-image-tools/daisy:latest
FROM golang

RUN apt-get update && apt-get -y install bash ca-certificates curl libssl-dev wget && \
    rm -rf /var/cache/apt/archives

# Go test junit xml output
RUN go get -u github.com/jstemmer/go-junit-report

COPY --from=0 /daisy daisy
# Daisy image copies this from gcr.io/distroless/base, we copy here.
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy this Dockerfile for debugging.
COPY Dockerfile Dockerfile
COPY main.sh main.sh
COPY run_integ_tests.sh run_integ_tests.sh
COPY integ-test-all.json integ-test-all.json

ENTRYPOINT ["./main.sh"]
