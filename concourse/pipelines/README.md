# Concourse pipelines

This directory contains pipeline definitions for our [Concourse] CI/CD system.
Pipelines may be in Concourse YAML syntax, or templated using [jsonnet].

[Concourse]: https://concourse-ci.org
[jsonnet]: https://jsonnet.org

## JSONnet templating

Originally, we separated reusable tasks into task YAML files which were then
referenced in the pipeline config YAML files. This is the only option for
reusability provided by Concourse natively. However, these config YAMLs were
limited, not supporting optional arguments, and making the resulting pipeline
definition difficult to read. As a result we migrated to JSONnet templates,
where the resulting pipeline has the entire configuration included. It also
resulted in a 10x reduction in config file length.

Within templates, we follow some basic patterns. The main repeatable elements in
Concourse Pipelines are Jobs and Tasks. We generall try to use templates
directly rather than functions, and write as much directly as possible (i.e. not
try to maximize the amount that is templated).

## Pipelines

### artifact-releaser-test.yaml

Used to perform testing of an internal artifact releasing system. It principally
involves gcloud commands to publish PubSub messages.

### container-build.jsonnet

Builds container images using [Kaniko], and publishes them to private & public
[Container Registry] registries.

[Kaniko]: https:/github.com/GoogleContainerTools/kaniko

### debian-worker-image-build.yaml

Creates 'derivative' Debian image builds using Daisy with tooling pre-installed
for use as a worker image in other jobs and workflows.

### guest-package-build.jsonnet

Compiles and packages various software using our package build Daisy workflows,
also controls testing with CIT and releasing via gcloud PubSub messages to an
internal artifact releasing system.

### linux-image-build.jsonnet

Builds the [Public Images] using Daisy, also controls testing with [CIT] and
releasing via either `gce_image_publish` or via gcloud PubSub messages to an
internal artifact releasing system.

[Public Images]: https://cloud.google.com/compute/docs/images#os-compute-support
[CIT]: https://github.com/GoogleCloudPlatform/guest-test-infra/tree/master/imagetest

### partner-image-validations.jsonnet

Performs automatic tests (via CIT) of Public Images produced by third parties,
e.g.  Ubuntu images built and published by Canonical.

### pipeline-set-pipeline.yaml

A pipeline used by Concourse to update other pipelines from this git repository.

### rhui-release.jsonnet

A pipeline for performing releases to the Red Hat Update Infrastructure ([RHUI])
clusters managed for RHEL customers on Google Cloud.

[RHUI]: https://access.redhat.com/documentation/en-us/red_hat_update_infrastructure/4

### windows-image-build.jsonnet

Builds the Windows [Public Images] using Daisy, also controls testing with CIT
and releasing via either `gce_image_publish` or via gcloud PubSub messages to an
internal artifact releasing system.
