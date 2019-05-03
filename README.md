# Compute Engine Guest OS - Test infrastructure

This repository contains tools, configuration and documentation for the public test infrastructure used by the Google Compute Engine Guest OS team. 

## Docker container images

The source for Docker container images used in our [prow](https://github.com/kubernetes/test-infra/tree/master/prow) jobs is located under the [container_images](container_images) directory. The configuration for those jobs lives in a separate repository for the shared open source prow cluster, our config is [here](https://github.com/GoogleCloudPlatform/oss-test-infra/tree/master/prow/prowjobs/GoogleCloudPlatform/gcp-guest/)

## License

All files in this repository are under the [Apache License, Version 2.0](LICENSE) unless noted otherwise.
