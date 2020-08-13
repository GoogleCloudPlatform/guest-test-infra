#!/usr/bin/env python3
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
"""Execute pytest within a virtual environment."""

import configparser
import os
import re
import shutil
import sys
import tempfile
import typing

import toml
import tox

# Dependencies to install in addition to user's requested dependencies.
_common_test_deps = ["pytest==6.0.1"]

_per_interpreter_command = [
  # Don't invoke the `pytest` binary directly, as it circumvents
  # the virtual environment.
  "python",
  "-m",
  "pytest",
  # Write test result to stdout out, except if test passes.
  "-ra",
  # Ensure junit report uses xunit1, which is the syntax used
  # by spyglass.
  # https://github.com/GoogleCloudPlatform/testgrid/blob/master/metadata/junit/junit.go
  "--override-ini=junit_family=xunit1",
  # Within the junit report, add a namespace for the environment,
  # to ensure that tests from different interpreters aren't
  # considered duplicates.
  "--junit-prefix={envname}",
  # Write a new junit xml file for each interpreter. The filename pattern
  # is defined in our Prow instance's configuration:
  #  https://github.com/GoogleCloudPlatform/oss-test-infra/blob/cc1f0cf1ffecc0e3b75664b0d264abc6165276d1/prow/oss/config.yaml#L98
  "--junit-xml={env:ARTIFACTS}/junit-{envname}.xml",
]


def setup_execution_root(pyproject_toml_file: str) -> str:
  """Create execution directory and copy project's code to it.

  Tox creates in-directory files, and its execution fails when the
  source tree is mounted read-only.

  Args:
    pyproject_toml_file: Absolute path to the project config file. The
      containing directory will be recursively copied to the execution root.

  Returns:
    Absolute path to execution root.
  """
  assert (os.path.isabs(pyproject_toml_file) and
          os.path.exists(pyproject_toml_file))

  source_root = os.path.dirname(pyproject_toml_file)
  exec_root = tempfile.mkdtemp()
  shutil.copytree(source_root, exec_root, dirs_exist_ok=True)
  return exec_root


def to_tox_version(py_version: str):
  """Convert `{major}.{minor}` to `py{major}{minor}`"""
  if re.fullmatch(r"\d.\d{1,2}", py_version):
    major, minor = py_version.split(".")
    return "py{major}{minor}".format(major=major, minor=minor)
  else:
    sys.exit("Invalid version number: " + py_version)


def write_tox_ini():
  """Read project configuration from pyproject.toml, and write a new tox.ini
  file.

  The tox.ini file is generated to ensure that tests are run consistently,
  and that test reports are written to the correct location.
  """
  assert os.path.exists("pyproject.toml")

  with open("pyproject.toml") as f:
    project_cfg = toml.load(f)
    test_cfg = project_cfg.get("tool", {}).get("gcp-guest-pytest", {})

  # Which interpreters to enable.
  envlist = [to_tox_version(env) for env in test_cfg.get("envlist", [])]

  # Test-specific dependencies to install.
  test_deps = test_cfg.get("test-deps", [])

  if not envlist:
    sys.exit("pyproject.toml must contain a section [tool.gcp-guest-pytest] "
             'with a key "envlist" and at least one interpreter.')

  config = configparser.ConfigParser()
  config["tox"] = {
    "envlist": ", ".join(envlist),
  }

  config["testenv"] = {
    "envlogdir": "{env:ARTIFACTS}/tox/{envname}",
    "deps": "\n\t".join(_common_test_deps + test_deps),
    "commands": " ".join(_per_interpreter_command),
  }

  if os.path.exists("tox.ini"):
    print("Ignoring existing tox.ini file.")
    os.remove("tox.ini")

  with open("tox.ini", "w") as f:
    config.write(f)


def save_configs(dst: str):
  print("Saving config files to " + dst)
  if not os.path.exists(dst):
    os.mkdir(dst)
  for fname in ["pyproject.toml", "tox.ini"]:
    shutil.copy2(fname, dst)


def validate_args(args: typing.List[str]):
  if (len(args) > 1 and args[1].endswith("pyproject.toml") and
      os.path.exists(args[1])):
    toml_file = os.path.abspath(args[1])
  else:
    sys.exit("First argument must be path to pyproject.toml")
  if "ARTIFACTS" in os.environ and os.path.exists(os.environ["ARTIFACTS"]):
    artifact_dir = os.path.abspath(os.environ["ARTIFACTS"])
    # Protect against a relative artifacts directory.
    os.environ["ARTIFACTS"] = artifact_dir
  else:
    sys.exit("$ARTIFACTS must be set to a directory that exists")
  return artifact_dir, toml_file


def main():
  print("args: %s" % sys.argv)
  print("$ARTIFACTS: %s" % os.environ.get("ARTIFACTS"))
  artifact_dir, toml_file = validate_args(sys.argv)

  execution_root = setup_execution_root(toml_file)
  os.chdir(execution_root)
  print("Executing tests in " + execution_root)

  write_tox_ini()
  save_configs(os.path.join(artifact_dir, "tox"))
  result_code = tox.cmdline(["-v"])
  sys.exit(result_code)


if __name__ == "__main__":
  main()
