// Imports.
local common = import '../templates/common.libsonnet';

local buildcontainerimgtask = {
  local task = self,

  dockerfile:: 'Dockerfile',
  input:: error 'must set input in buildcontainerimgtask',
  context:: error 'must set context in buildcontainerimgtask',
  destination:: error 'must set destination in buildcontainerimgtask',

  platform: 'linux',
  image_resource: {
    type: 'registry-image',
    source: {
      repository: 'gcr.io/kaniko-project/executor',
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

local buildcontainerimgjob = {
  local job = self,

  image:: error 'must set image in buildcontainerimgjob',
  destination:: error 'must set destination in buildcontainerimgjob',
  context:: error 'must set context in buildcontainerimgjob',
  dockerfile:: 'Dockerfile',
  input:: 'guest-test-infra',
  passed:: '',
  extra_steps:: [],
  extra_resources:: [],

  // Start of job definition
  name: 'build-' + job.image,
  serial_groups: ['serial'],
  plan: [
          {
            get: job.input,
            trigger: true,
            [if job.passed != '' then 'passed']: [job.passed],
          },
        ] +
        [
          {
            get: resource,
            trigger: true,
          }
          for resource in job.extra_resources
        ] +
        job.extra_steps +
        [

          {
            task: 'build-image',
            config: buildcontainerimgtask {
              destination: job.destination,
              dockerfile: job.dockerfile,
              context: job.context,
              input: job.input,
            },
          },
        ],
};

local BuildContainerImage(image) = buildcontainerimgjob {
  repo:: 'gcr.io/gcp-guest',
  image: image,
  destination: '%s/%s:latest' % [self.repo, image],
  context: 'guest-test-infra/container_images/' + image,
};

// Start of output.
{
  resources: [
    common.GitResource('guest-test-infra'),
    common.GitResource('compute-image-tools') {
      source+: { paths: ['daisy_workflows/**'] },
    },
  ],
  jobs: [
    BuildContainerImage('cloud-image-tests') {
      context: 'guest-test-infra',
      repo: 'gcr.io/compute-image-tools',
      dockerfile: 'guest-test-infra/imagetest/Dockerfile',
    },
    BuildContainerImage('gobuild'),
    BuildContainerImage('gotest'),
    BuildContainerImage('cli-tools-module-tests') {
      passed: 'build-gotest',
    },
    BuildContainerImage('gocheck'),
    BuildContainerImage('concourse-metrics') {
      context: 'guest-test-infra',
      dockerfile: 'guest-test-infra/container_images/concourse-metrics/Dockerfile',
    },
    BuildContainerImage('flake8'),
    BuildContainerImage('gointegtest'),
    BuildContainerImage('pytest'),
    BuildContainerImage('fly-validate-pipelines') {
      passed: 'build-jsonnet-go',
    },
    BuildContainerImage('jsonnet-go'),
    BuildContainerImage('registry-image-forked') {
      repo: 'gcr.io/compute-image-tools',
      dockerfile: 'dockerfiles/alpine/Dockerfile',
    },
    BuildContainerImage('daisy-builder'),
    BuildContainerImage('build-essential'),
    buildcontainerimgjob {
      image: 'daisy',
      destination: 'gcr.io/compute-image-tools/daisy:latest',
      context: 'compute-daisy',
      input: 'compute-daisy',
      // Add an extra step before build to layer in the daisy workflows.
      extra_resources: ['compute-image-tools'],
      extra_steps: [{
        task: 'get-daisy-workflows',
        config: {
          platform: 'linux',
          image_resource: {
            type: 'registry-image',
            source: { repository: 'busybox' },
          },
          inputs: [
            { name: 'compute-daisy' },
            { name: 'compute-image-tools' },
          ],
          outputs: [
            { name: 'compute-daisy' },
          ],
          run: {
            path: 'cp -a compute-image-tools/daisy_workflows compute-daisy/daisy_workflows',
          },
        },
      }],
    },
  ],
}
