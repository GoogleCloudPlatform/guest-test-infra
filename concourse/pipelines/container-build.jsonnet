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
  context:: self.input,
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
          { get: resource, trigger: true }
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

// Function for our builds in guest-test-infra/container_images
local BuildContainerImage(image) = buildcontainerimgjob {
  repo:: 'gcr.io/gcp-guest',

  image: image,
  destination: '%s/%s:latest' % [self.repo, image],
  context: 'guest-test-infra/container_images/' + image,
};

// Start of output.
{
  local daisy_architectures = ['linux', 'windows', 'darwin'],

  resources: [
    common.GitResource('guest-test-infra'),
    common.GitResource('compute-image-tools'),
    common.GitResource('compute-image-tools-trigger') {
      source+: { paths: ['daisy_workflows/**'] },
    },
    common.GitResource('compute-daisy'),
  ],
  jobs: [
    BuildContainerImage('build-essential'),
    BuildContainerImage('flake8'),
    BuildContainerImage('gobuild'),
    BuildContainerImage('gocheck'),
    BuildContainerImage('gointegtest'),
    BuildContainerImage('gotest'),
    BuildContainerImage('cli-tools-module-tests') { passed: 'build-gotest' },
    BuildContainerImage('jsonnet-go'),
    BuildContainerImage('fly-validate-pipelines') { passed: 'build-jsonnet-go' },
    BuildContainerImage('pytest'),

    // Non-standard dockerfile location and public image.
    BuildContainerImage('registry-image-forked') {
      dockerfile: 'dockerfiles/alpine/Dockerfile',
      repo: 'gcr.io/compute-image-tools',
    },

    // These build from the root of the repo.
    BuildContainerImage('cloud-image-tests') {
      context: 'guest-test-infra',
      dockerfile: 'guest-test-infra/imagetest/Dockerfile',
      // Public image.
      repo: 'gcr.io/compute-image-tools',
    },
    BuildContainerImage('concourse-metrics') {
      context: 'guest-test-infra',
      dockerfile: 'guest-test-infra/container_images/concourse-metrics/Dockerfile',
    },
    BuildContainerImage('daisy-builder') {
      context: 'guest-test-infra',
      dockerfile: 'container_images/daisy-builder/Dockerfile',
    },

    // Builds outside g-t-i repo.
    buildcontainerimgjob {
      destination: 'gcr.io/compute-image-tools/gce_image_publish:latest',
      dockerfile: 'gce_image_publish.Dockerfile',
      image: 'gce_image_publish',
      input: 'compute-image-tools',
    },
    buildcontainerimgjob {
      context: 'compute-daisy',
      destination: 'gcr.io/compute-image-tools-test/test-runner:latest',
      dockerfile: 'compute-daisy/daisy_test_runner.Dockerfile',
      image: 'daisy-test-runner',
      input: 'compute-daisy',
    },
    buildcontainerimgjob {
      context: 'compute-daisy',
      destination: 'gcr.io/compute-image-tools/daisy:latest',
      image: 'daisy',
      input: 'compute-daisy',

      extra_resources: ['compute-image-tools-trigger'],
      extra_steps:
        [
          // Add daisy workflows to compute-daisy.
          {
            task: 'get-daisy-workflows',
            config: {
              platform: 'linux',
              image_resource: {
                type: 'registry-image',
                source: { repository: 'busybox' },
              },
              inputs: [
                { name: 'compute-daisy' },
                { name: 'compute-image-tools-trigger' },
              ],
              outputs: [
                { name: 'compute-daisy' },
              ],
              run: {
                path: 'sh',
                args: [
                  '-exc',
                  'cp -a compute-image-tools-trigger/daisy_workflows compute-daisy/daisy_workflows',
                ],
              },
            },
          },
        ] +
        //  Build three binaries.
        [
          {
            task: 'build-%s-binary' % arch,
            config: {
              platform: 'linux',
              image_resource: {
                type: 'registry-image',
                source: { repository: 'golang' },
              },
              inputs: [{ name: 'compute-daisy', path: '.' }],
              outputs: [{ name: arch }],
              params: { GOOS: arch },
              run: {
                path: 'go',
                dir: 'cli',
                args: ['build', '-o=../%s/daisy' % arch],
              },
            },
          }
          for arch in daisy_architectures
        ] +
        //  Put three binaries using gsutil.
        [
          {
            task: 'upload-daisy-binaries',
            config: {
              platform: 'linux',
              image_resource: {
                type: 'registry-image',
                source: { repository: 'google/cloud-sdk', tag: 'alpine' },
              },
              inputs: [
                { name: 'windows' },
                { name: 'linux' },
                { name: 'darwin' },
              ],
              run: {
                path: 'sh',
                args: [
                  '-exc',
                  'for f in darwin linux windows; do' +
                  '  gsutil cp $f/daisy gs://compute-image-tools/latest/$f/daisy;' +
                  '  gsutil acl ch -u AllUsers:R gs://compute-image-tools/latest/$f/daisy;' +
                  'done',
                ],
              },
            },
          },
        ],
    },
  ],
}
