# Copyright 2017 Google Inc. All Rights Reserved.
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
FROM golang:bullseye

RUN apt-get update && apt-get -y install bash ca-certificates curl libssl-dev wget \
protobuf-compiler protoc-gen-go && rm -rf /var/cache/apt/archives

RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Go test junit xml output
RUN go install github.com/jstemmer/go-junit-report@latest

# Copy this Dockerfile for debugging.
COPY Dockerfile Dockerfile
COPY main.sh main.sh
ENTRYPOINT ["./main.sh"]
