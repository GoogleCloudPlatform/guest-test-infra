A prow worker that executes Python tests using [tox](https://tox.readthedocs.io/).
Check the Dockerfile for which Python versions are installed.

### Interface

Your job:
1. Create a [tox configuration file](https://tox.readthedocs.io/en/latest/config.html) in your
repository's test directory.
   * Optionally, write test artifacts to the `ARTIFACTS` directory.
1. Configure prow with the test directory's path as the first positional argument.

Prior to invoking, the runner will:
1. Checkout your repository, mount it as a volume, and set its root as Docker's 
[workdir](https://docs.docker.com/engine/reference/commandline/run/#set-working-directory--w).
2. Mount a volume for artifacts, and inject its path with the `ARTIFACTS` env variable.

### Example

Given a repository called `project` in the user's home directory:

```shell script
~/project
├── bin
├── docs
└── src
    └── py
        └── tests
            └── tox.ini
```

This tox configuration will execute [pytest](https://docs.pytest.org/) for both Python 3.6 and Python 3.8,
and ensure that results are written to the `ARTIFACTS` directory as junit XML files.

```ini
[tox]
envlist = py36, py38

[testenv]
deps =
    pytest

commands = pytest --junit-prefix={envname} \
                  --junit-xml={env:ARTIFACTS}/junit-{envname}.xml
```

To simulate prow's execution, use the following:

```shell script
docker run --volume ~/project:/project \
           --workdir /project \
           --volume /tmp/artifacts:/artifacts \
           --env ARTIFACTS=/artifacts \
           tox src/py/tests
```

The test results are available in the local `/tmp/artifacts` directory:

```shell script
tree /tmp/artifacts
/tmp/artifacts
├── junit-py36.xml
└── junit-py38.xml
```
