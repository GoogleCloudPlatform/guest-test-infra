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
FROM golang:bookworm

RUN apt-get update && apt-get install -y git curl grep unzip && \
    rm -rf /var/cache/apt/archives

# Install up-to-date protoc and plugins.
RUN VERSION=$(curl -s "https://api.github.com/repos/protocolbuffers/protobuf/releases/latest" | grep -oP '"tag_name": "v\K(.*)(?=")') && \
        curl -LO "https://github.com/protocolbuffers/protobuf/releases/download/v$VERSION/protoc-$VERSION-linux-x86_64.zip" && \
        unzip "protoc-$VERSION-linux-x86_64.zip" -d "/usr/local/"
RUN echo $PATH
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# Install the linter.
RUN go install golang.org/x/lint/golint@latest

# Copy this Dockerfile for debugging.
COPY Dockerfile Dockerfile
COPY main.sh main.sh
ENTRYPOINT ["./main.sh"]
