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
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import typing

import toml
import tox

# Dependencies to install in addition to user's requested dependencies.
_common_test_deps = ["pytest==6.0.1"]

# Command that's passed to tox.
#
# This is passed through 'format', so use double braces to bypass
# the formatter's replacement (they'll be converted to a single brace).
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
  "--junit-prefix={{envname}}",
  # Write a new junit xml file for each interpreter. The filename pattern
  # is defined in our Prow instance's configuration:
  #  https://github.com/GoogleCloudPlatform/oss-test-infra/blob/cc1f0cf1ffecc0e3b75664b0d264abc6165276d1/prow/oss/config.yaml#L98
  "--junit-xml={artifact_dir}/junit_{{envname}}.xml",
]


class TestConfig:
  def __init__(self, repo_root: Path, package_root: Path,
               envlist: typing.Iterable[str],
               pip_deps: typing.Iterable[str],
               local_deps: typing.Iterable[str]):
    """ The user's test specifications.

    Args:
      repo_root: Filesystem path to the repo's root directory.
      package_root: Filesystem path to the Python package's root directory.
      envlist: Interpreters to run tests with.
      pip_deps: List of dependencies from Pip
      local_deps: List of dependencies installable from this repo.

    """
    self.repo_root = repo_root
    self.package_root = package_root
    self.envlist = envlist
    self.pip_deps = pip_deps
    self.local_deps = local_deps


def setup_execution_root(cfg: TestConfig) -> Path:
  """Create execution directory and copy project's code to it.

  Tox creates in-directory files, and its execution fails when the
  source tree is mounted read-only.

  Args:
    package_root: Absolute path to the Python package's root directory.

  Returns:
    Absolute path to execution root.
  """
  assert cfg.package_root.exists(), \
      f"Expected {cfg.package_root} to exist and to be an absolute path."


  exec_root = Path(tempfile.mkdtemp())
  shutil.copytree(cfg.package_root, exec_root, dirs_exist_ok=True)

  if cfg.local_deps:
    dep_dir = exec_root / "deps"
    dep_dir.mkdir()
    for dep in cfg.local_deps:
      shutil.copytree(cfg.repo_root.absolute() / dep, dep_dir / dep)
  return Path(exec_root)


def to_tox_version(py_version: str):
  """Convert `{major}.{minor}` to `py{major}{minor}`"""
  if re.fullmatch(r"\d.\d{1,2}", py_version):
    major, minor = py_version.split(".")
    return "py{major}{minor}".format(major=major, minor=minor)
  else:
    raise ValueError("Invalid version number: " + py_version)


def read_tox_ini(repo_root: Path, package_root: Path) -> TestConfig:
  """Read pyproject.toml and write a new tox.ini file.

  The tox.ini file is generated to ensure that tests are run consistently,
  and that test reports are written to the correct location.
  """
  cfg = package_root / "pyproject.toml"
  assert cfg.exists(), "Expected pyproject.toml to exist."

  with cfg.open() as f:
    project_cfg = toml.load(f)

  # Read the [tool.gcp-guest-pytest] section of pyproject.toml.
  test_cfg = project_cfg.get("tool", {}).get("gcp-guest-pytest", {})

  # Which interpreters to enable.
  envlist = [to_tox_version(env) for env in test_cfg.get("envlist", [])]

  if not envlist:
    raise ValueError(
      "pyproject.toml must contain a section [tool.gcp-guest-pytest] "
      "with a key `envlist` and at least one interpreter.")

  pip_deps, local_deps = [], []
  for dep in test_cfg.get("test-deps", []):
    if dep.startswith("//"):
      local = dep[2:]
      if not (repo_root / local).exists():
        raise ValueError("Dependency {} not found".format(dep))
      local_deps.append(local)
    else:
      pip_deps.append(dep)

  return TestConfig(
      repo_root=repo_root,
      package_root=package_root,
      envlist=envlist,
      pip_deps=pip_deps,
      local_deps=local_deps
  )


def write_tox_ini(cfg: TestConfig, artifact_dir: Path, execution_root: Path):
  """Write the test config to a new tox.ini file.

  The tox.ini file is generated to ensure that tests are run consistently,
  and that test reports are written to the correct location.
  """

  config = configparser.ConfigParser()
  config["tox"] = {
    "envlist": ", ".join(cfg.envlist),
  }

  local_deps = []
  dep_dir = execution_root.absolute() / "deps"
  for d in cfg.local_deps:
    local_deps.append((dep_dir / d).as_posix())

  config["testenv"] = {
    "envlogdir":
      "{artifact_dir}/tox/{{envname}}".format(artifact_dir=artifact_dir),
    "deps":
      "\n\t".join(_common_test_deps + cfg.pip_deps + local_deps),
    "commands":
      " ".join(_per_interpreter_command).format(artifact_dir=artifact_dir),
  }

  if os.path.exists("tox.ini"):
    print("Removing existing tox.ini file.")
    os.remove("tox.ini")

  with open("tox.ini", "w") as f:
    config.write(f)


def archive_configs(dst: Path):
  if dst.exists():
    assert dst.is_dir(), f"Expected {dst} to be a directory."
  else:
    dst.mkdir()
  print("Saving config files to ", dst)
  for fname in ["pyproject.toml", "tox.ini"]:
    shutil.copy2(fname, dst)


def validate_args(args: typing.List[str]) -> typing.Tuple[Path, Path]:
  if (len(args) > 1
      and os.path.isdir(args[1])
      and os.path.isfile(os.path.join(args[1], "pyproject.toml"))):
    package_root = os.path.abspath(args[1])
  else:
    raise ValueError("First argument must be path to python package")
  if "ARTIFACTS" in os.environ and os.path.exists(os.environ["ARTIFACTS"]):
    artifact_dir = os.path.abspath(os.environ["ARTIFACTS"])
  else:
    raise ValueError("$ARTIFACTS must point to a directory that exists")
  return Path(artifact_dir), Path(package_root)


def main():
  print("args:", sys.argv)
  print("$ARTIFACTS:", os.environ.get("ARTIFACTS"))
  artifact_dir, package_root = validate_args(sys.argv)
  cfg = read_tox_ini(Path.cwd(), package_root)
  # Create a new execution area, since we'll be writing files.
  execution_root = setup_execution_root(cfg)
  os.chdir(execution_root)
  subprocess.run(["ls", "-lah", "deps"])
  print("Executing tests in", execution_root)

  write_tox_ini(cfg, artifact_dir, Path())
  archive_configs(artifact_dir / "tox")
  result_code = tox.cmdline(["-v"])
  sys.exit(result_code)


if __name__ == "__main__":
  main()
