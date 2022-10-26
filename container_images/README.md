# Docker container images

The source for Docker container images used in our CI/CD infrastructure. Each
folder represents one image. These are built by kaniko jobs in our Concourse
`container-build` pipeline and generally published on gcr.io container registry,
but are not made publicly available.

We use these in [Prow] jobs and [Concourse] pipelines. Concourse pipelines are
located in the [concourse](../concourse/) directory. Prow jobs are configured in
the [oss-test-infra] repository.

[Prow]: https://github.com/kubernetes/test-infra/tree/master/prow
[Concourse]: https://concourse-ci.org
[oss-test-infra]: https://github.com/GoogleCloudPlatform/oss-test-infra/tree/master/prow/prowjobs/GoogleCloudPlatform/gcp-guest/

## Images

A brief description of each container image is listed below.

### build-essential

This is a clone of a Debian container with the 'build-essential' package
preinstalled. Usage: ??

### cleanerupper

Contains the 'cleanerupper' binary, which will remove GCE and VM Manager
resources older than a certain age. Used to help 'clean up' test projects in
case of uncaught failures.

### cli-tools-module-tests

A derivative of the 'gotest' image with gcsfuse included. Usage: ??

### concourse-metrics

A container image with several metric-publisher binaries included.

* publish-coverage - produce a lines of coverage metric
* publish-job-result - publish a success or fail metric for a Concourse job, as
  well as duration
* publish-time-since - publish time since last release for a package

Used in Concourse pipelines.

### daisy-builder

The daisy binary, the packagebuild workflows, and a shell script for use in
Prow packagebuild presubmit jobs.

### flake8

The flake8 Python linter, used for Prow flake8 presubmit jobs.

### fly-validate-pipelines

The `fly` CLI tool, the `jsonnet` CLI tool (from jsonnet-go, below) and a shell
script for use in Prow validate-pipeline presubmit jobs.

### gce-img-resource

A Concourse [`Resource Type`] that tracks GCE Images by name or family. Used in
Concourse pipelines.

[`Resource Type`]: https://concourse-ci.org/implementing-resource-types.html

### gobuild

The go runtime and a shell script for use in Prow gobuild presubmit jobs.

### gocheck

The go runtime, golint, and a shell script for use in Prow gocheck presubmit
jobs.

### gointegtest

The daisy binary, a go test output to jUnit formatter, and a shell script for
use in Prow gointegtest jobs. These run a daisy workflow that creates VMs from
various OS images, checks out the code test and runs unit tests tagged 'integ'.

### gotest

The go runtime and a shell script for use in Prow gotest presubmit jobs.

### jsonnet-go

The `jsonnet` CLI tool. Used in fly-validate-pipelines, above.

### pytest

The `pytest` tool for multiple versions of Python. Additional details in the
[README](pytest/README.md). Usage: ??

### registry-image-forked

A fork of [concourse/registry-image-resource] that supports GCE authentication.
Used in Concourse pipelines to track private images such as those built in this
repository.

[concourse/registry-image-resource]: https://github.com/concourse/registry-image-resource

### selinux-tools

SELinux tools for building SELinux modules. Currently broken, see issue: blah

### validate-integtest

Tools and script to validate the output of the gointegtest workflow. No longer
used.
