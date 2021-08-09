# Copyright 2021 Google LLC
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

FROM golang:alpine as builder

WORKDIR /build
COPY . .
RUN mkdir /out

ENV CGO_ENABLED 0
ENV GOOS linux
ENV GO111MODULE on

RUN go mod download
RUN go build -o /out/wrapper ./imagetest/cmd/wrapper/main.go
RUN go build -o /out/manager ./imagetest/cmd/manager/main.go
RUN cd imagetest/test_suites; for suite in *; do \
  cd $suite; go test -c -tags cit || exit 1; \
  ./"${suite}.test" -test.list '.*' > /out/"${suite}_tests.txt" || exit 1; \
  mv "${suite}.test" /out || exit 1; cd ..; \
  done

FROM alpine:edge
RUN apk add --no-cache openssh-keygen
COPY --from=builder /out/* /

ENTRYPOINT ["/manager"]
