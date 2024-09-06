// Imports.
local common = import '../templates/common.libsonnet';

local buildcontainerimgtask = {
  local task = self,

  dockerfile:: 'Dockerfile',
  input:: error 'must set input in buildcontainerimgtask',
  context:: error 'must set context in buildcontainerimgtask',
  destination:: error 'must set destination in buildcontainerimgtask',
  commit_sha:: error 'must set commit_sha in buildcontainerimgtask',

  platform: 'linux',
  image_resource: {
    type: 'registry-image',
    source: {
      repository: 'gcr.io/kaniko-project/executor',
      tag: 'latest',
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
      '--destination=%s:latest' % task.destination,
      '--destination=%s:%s' % [task.destination, task.commit_sha],
      '--force',
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
  privileged:: false,
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
            load_var: '%s-commit-sha' % job.image,
            file: '%s/.git/ref' % job.input,
          },
          {
            task: 'build-image',
	    privileged: job.privileged,
            config: buildcontainerimgtask {
              commit_sha: '((.:%s-commit-sha))' % job.image,
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
  destination: '%s/%s' % [self.repo, image],
  context: 'guest-test-infra/container_images/' + image,
};

// Start of output.
{
  local daisy_architectures = ['linux', 'windows', 'darwin'],
  resource_types: [{
    name: 'registry-image-forked',
    type: 'registry-image',
    source: { repository: 'gcr.io/compute-image-tools/registry-image-forked' },
  }],
  resources: [
    {
      name: 'cloud-image-tests',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/cloud-image-tests.git',
        branch: 'main',
      },
    },
    common.GitResource('guest-test-infra'),
    common.GitResource('compute-image-tools'),
    {
      name: 'compute-image-tools-trigger',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/compute-image-tools.git',
        branch: 'master',
        paths: ['daisy_workflows/**'],
      },
    },
    common.GitResource('compute-daisy'),
  ],
  jobs: [
    BuildContainerImage('build-essential'),
    BuildContainerImage('flake8'),
    BuildContainerImage('gobuild'),
    BuildContainerImage('gocheck'),
    BuildContainerImage('cleanerupper'),
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
      privileged: true,
    },

    // These build from the root of the repo.
    BuildContainerImage('concourse-metrics') {
      context: 'guest-test-infra',
      dockerfile: 'guest-test-infra/container_images/concourse-metrics/Dockerfile',
    },
    BuildContainerImage('daisy-builder') {
      context: 'guest-test-infra',
      dockerfile: 'container_images/daisy-builder/Dockerfile',
    },
    BuildContainerImage('gce-img-resource') {
      context: 'guest-test-infra',
      dockerfile: 'guest-test-infra/container_images/gce-img-resource/Dockerfile',
    },

    // TODO: this is built like daisy, with multi-platform binaries in GCS. Currently being built by CB in
    // compute-image-tools project.
    //    buildcontainerimgjob {
    //      destination: 'gcr.io/compute-image-tools/gce_image_publish',
    //      dockerfile: 'gce_image_publish.Dockerfile',
    //      image: 'gce_image_publish',
    //      input: 'compute-image-tools',
    //    },

    // Builds outside g-t-i repo.
    buildcontainerimgjob {
      context: 'compute-image-tools',
      destination: 'gcr.io/compute-image-tools-test/gce-windows-upgrade-tests',
      dockerfile: 'compute-image-tools/gce_windows_upgrade_tests.Dockerfile',
      image: 'gce_windows_upgrade_tests',
      input: 'compute-image-tools',
    },
    buildcontainerimgjob {
      context: 'cloud-image-tests',
      destination: 'gcr.io/compute-image-tools/cloud-image-tests',
      dockerfile: 'Dockerfile',
      input: 'cloud-image-tests',
      image: 'cloud-image-tests',
    },
    buildcontainerimgjob {
      context: 'compute-daisy',
      destination: 'gcr.io/compute-image-tools-test/test-runner',
      dockerfile: 'compute-daisy/daisy_test_runner.Dockerfile',
      image: 'daisy-test-runner',
      input: 'compute-daisy',
    },
    buildcontainerimgjob {
      context: 'compute-daisy',
      destination: 'gcr.io/compute-image-tools/daisy',
      image: 'daisy',
      input: 'compute-daisy',
      passed: 'build-daisy-test-runner',
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
                source: { repository: 'golang', tag: 'bullseye' },
              },
              inputs: [{ name: 'compute-daisy', path: '.' }],
              outputs: [{ name: arch }],
              params: { GOOS: arch, CGO_ENABLED: 0 },
              run: {
                path: 'go',
                dir: 'cli',
                args: ['build', '-o=../%s/daisy' % arch],
              },
            },
          }
          for arch in daisy_architectures
        ],
      plan+: [
        {
          task: 'run-daisy-integ-tests',
          config: {
            inputs: [{ name: 'compute-daisy' }],
            params: {
              // Force the test runner to use application default credentials,
              // which are available through the k8s metadata server.
              GOOGLE_APPLICATION_CREDENTIALS: '',
            },
            platform: 'linux',
            image_resource: {
              type: 'registry-image-forked',
              source: {
                repository: 'gcr.io/compute-image-tools-test/test-runner',
                tag: '((.:daisy-commit-sha))',
                google_auth: true,
              },
            },
            run: {
              path: '/daisy_test_runner',
              args: [
                '-projects=compute-image-test-pool-001',
                '-zone=us-central1-c',
                'compute-daisy/daisy_integration_tests/daisy_e2e.test.gotmpl',
              ],
            },
          },
        },
        // Run a workflow in the staged container.
        {
          task: 'test-daisy-container',
          config: {
            inputs: [{ name: 'compute-daisy' }],
            platform: 'linux',
            image_resource: {
              type: 'registry-image',
              source: {
                repository: 'gcr.io/compute-image-tools/daisy',
                tag: '((.:daisy-commit-sha))',
              },
            },
            run: {
              path: '/daisy',
              args: [
                '-project=compute-image-test-pool-001',
                '-zone=us-central1-c',
                'compute-daisy/daisy_integration_tests/can_retrieve_sources.wf.json',
              ],
            },
          },
        },
        // Put three binaries using gsutil.
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
                'mv windows/daisy windows/daisy.exe;' +
                'for f in darwin/daisy linux/daisy windows/daisy.exe; do' +
                '  for t in latest release; do' +
                '    gsutil cp $f gs://compute-image-tools/$t/$f;' +
                '    gsutil acl ch -u AllUsers:R gs://compute-image-tools/$t/$f;' +
                '  done;' +
                'done',
              ],
            },
          },
        },
        //  Add release tag to the staged container.
        {
          task: 'tag-image',
          config: {
            platform: 'linux',
            image_resource: {
              type: 'registry-image',
              source: { repository: 'google/cloud-sdk', tag: 'alpine' },
            },
            run: {
              path: 'sh',
              args: [
                '-exc',
                'gcloud container images add-tag --quiet' +
                '  gcr.io/compute-image-tools/daisy:((.:daisy-commit-sha))' +
                '  gcr.io/compute-image-tools/daisy:release',
              ],
            },
          },
        },
      ],
    },
  ],
}
