#!/bin/bash
# Copyright 2020 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -ex

# Change to the directory of this script.
cd "$(dirname "$(readlink -f "$0")")"

pushd ..
docker build . -t pytest
popd

rm -rf artifacts && mkdir artifacts

docker run \
  `# The first mount contains the repository that's being tested. In this example, we're` \
  `# assuming that *this* is the root of the repository. The root is important since ` \
  `# in-repo dependencies are expressed as an absolute path from the root of the repository.` \
  --volume "$(pwd):/project:ro" \
  --workdir /project \
  `# After the test runs, check the "artifacts" directory for test results.` \
  --volume "$(pwd)/artifacts:/artifacts" \
  --env ARTIFACTS=/artifacts \
  `# The argument to "pytest" is the path to the Python package that will be tested.` \
  pytest src/application
