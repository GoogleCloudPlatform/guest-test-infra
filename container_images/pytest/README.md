A prow worker that executes Python tests using [pytest](https://docs.pytest.org/en/stable/).
Executes tests in a virtual environment, and supports multiple invocations
with different Python interpreters.

## Interface

Your job:
1. Create a configuration file in your repository's test directory.
1. Configure prow with the configuration file's path as the first positional argument.

## Configuration file

### Basic example

Create the file `pyproject.toml` adjacent to the Python package's `setup.py` file.

```toml
# Config section for this prow worker.
[tool.gcp-guest-pytest]

# At least one environment needs to be specified.
envlist = ["3.8"]

# Optionally include test dependencies. May omit if not required.
test-deps = [
    "pyyaml == 5.3.1",
]
```

To run this locally:

```shell script
docker run --volume ~/git/project:/project:ro --workdir /project \
           --volume /tmp/artifacts:/artifacts --env ARTIFACTS=/artifacts \
           gcr.io/gcp-guest/pytest src/py/pyproject.toml
```

* **~/git/project** is the root of the repository that's being tested. 
* **src/py/pyproject.toml** is the relative path to the pyproject.toml file.
* **/tmp/artifacts** is a directory to write test reports.

### Details

Adjacent to `setup.py`, create a `pyproject.yaml` file, and include 
the `[tool.gcp-guest-pytest]` section.  The following keys are supported:

#### envlist

List of interpreter versions to execute tests with. At least one interpreter is required;
the job will fail if this key is missing or empty.

Encode the version as `<major>.<minor>`.

```toml
envlist = [
  "3.5",
  "3.6",
  "3.7",
]
```

See the Dockerfile for currently-installed versions.

#### test-deps (optional)

Dependencies to install prior to installing your package. Two types of dependencies are supported,
"pip package" dependencies and "in-repo" depedencies.

To install test-only dependencies from PyPI, use
[requirement specifiers](https://pip.pypa.io/en/stable/reference/pip_install/#requirement-specifiers):

```toml
test-deps = [
    "pytest",
    "urllib3 == 1.2",
    "six >= 1.12",
    "botocore ~= 1"
]
```


To install a dependency from the current repo, write the dependency's path **relative to the root
of the repository**, starting with two slashes. This example shows how `application` would depend
on `sums`:

```
# directory layout

repo
└── src
    ├── application
    │   ├── main.py
    │   ├── main_test.py
    │   ├── pyproject.toml
    │   └── setup.py
    └── sums
        ├── ints.py
        └── setup.py

# src/application/pyproject.toml

test-deps = [
    "//src/sums",
]
```
