# Compute Engine Guest OS - Test infrastructure

This repository contains tools, configuration and documentation for the CI/CD 
infrastructure used by GCE Guest OS Images team. 

* [concourse](concourse/pipelines/README.md) - contains configuration files for
  our [Concourse] cluster
* [container\_images](container_images/README.md) - container image sources and
  Dockerfiles
* [imagetest](imagetest/README.md) - the cloud image tests framework and test
  suites
* [packagebuild](packagebuild/README.md) - package builder Daisy workflows

[Concourse]: https://concourse-ci.org
