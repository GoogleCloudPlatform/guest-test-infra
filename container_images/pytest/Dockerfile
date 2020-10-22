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

# Assembles multiple versions of Python into a single container.

FROM python:3.5.9-slim-buster
FROM python:3.6.11-slim-buster
FROM python:3.7.8-slim-buster

FROM python:3.8.5-slim-buster
RUN pip3 install tox==3.20.1

COPY --from=0 /usr/local/lib/python3.5/ /usr/local/lib/python3.5/
COPY --from=0 /usr/local/lib/lib*3.5* /usr/local/lib/
COPY --from=0 /usr/local/bin/python3.5 /usr/local/bin/
COPY --from=0 /usr/local/include/python3.5m/ /usr/local/include/python3.5m/

COPY --from=1 /usr/local/lib/python3.6/ /usr/local/lib/python3.6/
COPY --from=1 /usr/local/lib/lib*3.6* /usr/local/lib/
COPY --from=1 /usr/local/bin/python3.6 /usr/local/bin/
COPY --from=1 /usr/local/include/python3.6m/ /usr/local/include/python3.6m/

COPY --from=2 /usr/local/lib/python3.7/ /usr/local/lib/python3.7/
COPY --from=2 /usr/local/lib/lib*3.7* /usr/local/lib/
COPY --from=2 /usr/local/bin/python3.7 /usr/local/bin/
COPY --from=2 /usr/local/include/python3.7m/ /usr/local/include/python3.7m/

RUN ldconfig

WORKDIR /
COPY Dockerfile Dockerfile
COPY main.py main.py
ENTRYPOINT ["/main.py"]
