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
FROM golang:1.16
RUN go get -u github.com/jstemmer/go-junit-report

FROM gcr.io/compute-image-tools/daisy:latest
FROM gcr.io/google.com/cloudsdktool/cloud-sdk:latest

COPY --from=0 /go/bin/go-junit-report /go-junit-report
COPY --from=1 /daisy /daisy

RUN apt-get update && apt-get -y install ca-certificates && \
    rm -rf /var/cache/apt/archives

COPY . .

ENTRYPOINT ["./main.sh"]
