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

  // Start of job definition
  name: 'build-' + job.image,
  serial_groups: ['serial'],
  plan: [
    {
      get: job.input,
      trigger: true,
      [if job.passed != '' then 'passed']: [job.passed],
    },
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
  ],
}
