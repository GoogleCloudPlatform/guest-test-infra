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

FROM golang:1.17-alpine as builder

WORKDIR /build
COPY . .
RUN mkdir /out

ENV CGO_ENABLED 0
ENV GOOS linux
ENV GO111MODULE on

RUN go mod download
RUN go build -o /out/wrapper.amd64 ./cmd/wrapper/main.go
RUN GOARCH=arm64 go build -o /out/wrapper.arm64 ./cmd/wrapper/main.go
# Setting the windows file names to respect 8.3 file naming.
RUN GOOS=windows GOARCH=amd64 go build -o /out/wrapp64.exe ./cmd/wrapper/main.go
RUN GOOS=windows GOARCH=386 go build -o /out/wrapp32.exe ./cmd/wrapper/main.go
RUN go build -o /out/manager ./cmd/manager/main.go
RUN cd test_suites; for suite in *; do \
  [[ -d $suite ]] || continue; \
  cd $suite; \
  go test -c -tags cit || exit 1; \
  ./"${suite}.test" -test.list '.*' > /out/"${suite}_tests.txt" || exit 1; \
  mv "${suite}.test" "/out/${suite}.amd64.test" || exit 1; \
  GOARCH=arm64 go test -c -tags cit || exit 1; \
  mv "${suite}.test" "/out/${suite}.arm64.test" || exit 1; \
  GOOS=windows GOARCH=amd64 go test -c -tags cit || exit 1; \
  if [ -f "${suite}.test.exe" ]; then \
    mv "${suite}.test.exe" "/out/${suite}64.exe" || exit 1; \
  fi; \
  GOOS=windows GOARCH=386 go test -c -tags cit || exit 1; \
  if [ -f "${suite}.test.exe" ]; then \
    mv "${suite}.test.exe" "/out/${suite}32.exe" || exit 1; \
  fi; \
  cd ..; \
  done

FROM alpine:edge
RUN apk add --no-cache openssh-keygen
COPY --from=builder /out/* /

ENTRYPOINT ["/manager"]
