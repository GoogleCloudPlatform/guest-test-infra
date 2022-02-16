// Start of output.
{
  jobs: [
    {
      name: 'build-cit-container',
      plan: [
        {
          get: 'guest-test-infra',
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          params: {
            DOCKERFILE: 'guest-test-infra/imagetest/Dockerfile',
          },
          task: 'build-image',
          vars: {
            context: 'guest-test-infra',
            destination: 'gcr.io/gcp-guest/cloud-image-tests:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
    {
      name: 'build-gobuild-container',
      plan: [
        {
          get: 'guest-test-infra',
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          task: 'build-image',
          vars: {
            context: 'guest-test-infra/container_images/gobuild',
            destination: 'gcr.io/gcp-guest/gobuild:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
    {
      name: 'build-gotest-container',
      plan: [
        {
          get: 'guest-test-infra',
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          task: 'build-image',
          vars: {
            context: 'guest-test-infra/container_images/gotest',
            destination: 'gcr.io/gcp-guest/gotest:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
    {
      name: 'build-cli-tools-module-tests-container',
      plan: [
        {
          get: 'guest-test-infra',
          passed: [
            'build-gotest-container',
          ],
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          task: 'build-image',
          vars: {
            context: 'guest-test-infra/container_images/cli-tools-module-tests',
            destination: 'gcr.io/gcp-guest/cli-tools-module-tests:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
    {
      name: 'build-gocheck-container',
      plan: [
        {
          get: 'guest-test-infra',
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          task: 'build-image',
          vars: {
            context: 'guest-test-infra/container_images/gocheck',
            destination: 'gcr.io/gcp-guest/gocheck:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
    {
      name: 'build-build-essential-container',
      plan: [
        {
          get: 'guest-test-infra',
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          task: 'build-image',
          vars: {
            context: 'guest-test-infra/container_images/build-essential',
            destination: 'gcr.io/gcp-guest/build-essential:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
    {
      name: 'build-concourse-metrics-container',
      plan: [
        {
          get: 'guest-test-infra',
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          params: {
            DOCKERFILE: 'guest-test-infra/container_images/concourse-metrics/Dockerfile',
          },
          task: 'build-image',
          vars: {
            context: 'guest-test-infra',
            destination: 'gcr.io/gcp-guest/concourse-metrics:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
    {
      name: 'build-flake8-container',
      plan: [
        {
          get: 'guest-test-infra',
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          task: 'build-image',
          vars: {
            context: 'guest-test-infra/container_images/flake8',
            destination: 'gcr.io/gcp-guest/flake8:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
    {
      name: 'build-gointegtest-container',
      plan: [
        {
          get: 'guest-test-infra',
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          task: 'build-image',
          vars: {
            context: 'guest-test-infra/container_images/gointegtest',
            destination: 'gcr.io/gcp-guest/gointegtest:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
    {
      name: 'build-pytest-container',
      plan: [
        {
          get: 'guest-test-infra',
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          task: 'build-image',
          vars: {
            context: 'guest-test-infra/container_images/pytest',
            destination: 'gcr.io/gcp-guest/pytest:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
    {
      name: 'build-fly-vp-container',
      plan: [
        {
          get: 'guest-test-infra',
          passed: [
            'build-jsonnet-go-container',
          ],
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          task: 'build-image',
          vars: {
            context: 'guest-test-infra/container_images/fly-validate-pipelines',
            destination: 'gcr.io/gcp-guest/fly-validate-pipelines:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
    {
      name: 'build-jsonnet-go-container',
      plan: [
        {
          get: 'guest-test-infra',
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          task: 'build-image',
          vars: {
            context: 'guest-test-infra/container_images/jsonnet-go',
            destination: 'gcr.io/gcp-guest/jsonnet-go:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
    {
      name: 'build-registry-image-forked',
      plan: [
        {
          get: 'guest-test-infra',
          trigger: true,
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/build-container-image.yaml',
          params: {
            DOCKERFILE: 'dockerfiles/alpine/Dockerfile',
          },
          task: 'build-image',
          vars: {
            context: 'guest-test-infra/container_images/registry-image-forked',
            destination: 'gcr.io/compute-image-tools/registry-image-forked:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
  ],
  resources: [
    {
      name: 'guest-test-infra',
      source: {
        branch: 'master',
        uri: 'https://github.com/GoogleCloudPlatform/guest-test-infra.git',
      },
      type: 'git',
    },
  ],
}
