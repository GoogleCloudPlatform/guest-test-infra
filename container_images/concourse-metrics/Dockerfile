# Copyright 2022 Google LLC
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
FROM golang:alpine

RUN apk add --no-cache git

WORKDIR /

# this needs to be run from root of googlecloudplatform/guest-test-infra
COPY . /
RUN CGO_ENABLED=0 go build -o /publish-job-result ./container_images/concourse-metrics/cmd/publish-job-result/main.go
RUN CGO_ENABLED=0 go build -o /publish-coverage ./container_images/concourse-metrics/cmd/publish-coverage/main.go
RUN CGO_ENABLED=0 go build -o /publish-time-since ./container_images/concourse-metrics/cmd/publish-time-since/main.go
RUN apk --update add ca-certificates

RUN chmod +x /publish-job-result

FROM scratch

COPY --from=0 publish-job-result /publish-job-result
COPY --from=0 publish-coverage /publish-coverage
COPY --from=0 publish-time-since /publish-time-since

COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# for debugging
COPY container_images/concourse-metrics/Dockerfile /
