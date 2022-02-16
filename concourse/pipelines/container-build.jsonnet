// Imports.
local common = import '../templates/common.libsonnet';

local buildcontainerimgtask = {
  local task = self,

  dockerfile:: 'Dockerfile',
  input:: 'guest-test-infra',
  context:: error 'must set context in buildcontainerimgtask',
  destination:: error 'must set destination in buildcontainerimgtask',

  platform: 'linux',
  image_resource: {
    type: 'registry-image',
    source: {
      repository: 'gcr.io/kaniko-project/executor',
      tag: 'v1.2.0',
    },
  },
  inputs: [
    { name: task.input },
  ],
  run: {
    path: 'executor',
    args: [
      '--dockerfile=' + task.dockerfile,
      '--context=' + task.context,
      '--destination=' + task.destination,
    ],
  },
};

// Start of output.
{
  resources: [
    common.GitResource('guest-test-infra'),
  ],
  jobs: [
    {
      name: 'build-cit-container',
      plan: [
        {
          get: 'guest-test-infra',
          trigger: true,
        },
        {
          task: 'build-image',
          config: buildcontainerimgtask {
            dockerfile: 'guest-test-infra/imagetest/Dockerfile',
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
          task: 'build-image',
          config: buildcontainerimgtask {
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
          task: 'build-image',
          config: buildcontainerimgtask {
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
          task: 'build-image',
          config: buildcontainerimgtask {
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
          task: 'build-image',
          config: buildcontainerimgtask {
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
          task: 'build-image',
          config: buildcontainerimgtask {
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
          config: buildcontainerimgtask {
            dockerfile: 'guest-test-infra/container_images/concourse-metrics/Dockerfile',
            context: 'guest-test-infra',
            destination: 'gcr.io/gcp-guest/concourse-metrics:latest',
          },
          task: 'build-image',
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
          task: 'build-image',
          config: buildcontainerimgtask {
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
          task: 'build-image',
          config: buildcontainerimgtask {
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
          task: 'build-image',
          config: buildcontainerimgtask {
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
          task: 'build-image',
          config: buildcontainerimgtask {
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
          task: 'build-image',
          config: buildcontainerimgtask {
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
          task: 'build-image',
          config: buildcontainerimgtask {
            dockerfile: 'dockerfiles/alpine/Dockerfile',
            context: 'guest-test-infra/container_images/registry-image-forked',
            destination: 'gcr.io/compute-image-tools/registry-image-forked:latest',
          },
        },
      ],
      serial_groups: ['serial'],
    },
  ],
}
