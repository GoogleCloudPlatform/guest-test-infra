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
FROM golang

RUN go get -u github.com/jstemmer/go-junit-report

FROM node
RUN npm install -g junit-merge

FROM python:3.7.8-slim-buster
RUN pip3 install junit2html

FROM gcr.io/google.com/cloudsdktool/cloud-sdk:latest

COPY --from=0 /go/bin/go-junit-report /go-junit-report
COPY --from=1 /usr/local/bin/node /usr/local/bin/node
COPY --from=1 /usr/local/lib/node_modules/ /usr/local/lib/node_modules/
COPY --from=2 /usr/local/bin/junit2html /junit2html
COPY --from=2 /usr/local/bin/python /usr/local/bin/python
COPY --from=2 /usr/local/lib/python3.7/ /usr/local/lib/python3.7/
COPY --from=2 /usr/local/lib/lib*3.7* /usr/local/lib/
COPY --from=2 /usr/local/bin/python3.7 /usr/local/bin/
COPY --from=2 /usr/local/include/python3.7m/ /usr/local/include/python3.7m/


RUN apt-get update && apt-get -y install ca-certificates && \
    rm -rf /var/cache/apt/archives

WORKDIR /
COPY . .
RUN chmod +x /validate-result.sh

ENTRYPOINT ["/validate-result.sh"]
