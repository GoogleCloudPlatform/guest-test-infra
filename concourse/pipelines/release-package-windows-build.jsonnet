// Imports.
local arle = import '../templates/arle.libsonnet';
local common = import '../templates/common.libsonnet';
local daisy = import '../templates/daisy.libsonnet';
local gcp_secret_manager = import '../templates/gcp-secret-manager.libsonnet';

local underscore(input) = std.strReplace(input, '-', '_');

// Templates.
local imagetestn1 = common.imagetesttask {
  zones: ['europe-west1-b', 'europe-west1-c', 'europe-west1-d'],
  filter: '^(cvm|livemigrate|suspendresume|loadbalancer|guestagent|hostnamevalidation|imageboot|licensevalidation|network|security|hotattach|lssd|disk|packagevalidation|ssh|winrm|metadata|sql|mdsmtls|mdsroutes|packagemanager|compatmanager|packageupgrade)$',
  extra_args: ['-x86_shape=n1-standard-4', '-timeout=60m'],
};

local imagetestc3 = common.imagetesttask {
  zones: ['asia-northeast1-b', 'europe-north1-b', 'us-west1-c'],
  filter: '^(livemigrate|suspendresume|imageboot|network|hotattach|lssd|disk)$',
  extra_args: ['-x86_shape=c3-standard-4', '-timeout=60m'],
};

local imgbuildjob = {
  local job = self,

  image:: error 'must set image in imgbuildjob',
  workflow:: error 'must set workflow in imgbuildjob',
  iso_secret:: error 'must set iso_secret in imgbuildjob',
  updates_secret:: error 'must set updates_secret in imgbuildjob',
  daily:: true,
  daily_task:: if self.daily then [
    {
      get: 'daily-time',
      trigger: true,
    },
  ] else [],

  // Start of job.
  name: 'build-release-package-testing-' + job.image,
  plan: job.daily_task + [
    { get: 'compute-image-tools' },
    { get: 'guest-test-infra' },
    {
      task: 'generate-timestamp',
      file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
    },
    {
      load_var: 'start-timestamp-ms',
      file: 'timestamp/timestamp-ms',
    },
    {
      task: 'generate-id',
      file: 'guest-test-infra/concourse/tasks/generate-id.yaml',
    },
    {
      load_var: 'id',
      file: 'generate-id/id',
    },
    {
      task: 'generate-build-id',
      file: 'guest-test-infra/concourse/tasks/generate-build-id.yaml',
      vars: { prefix: job.image, id: '((.:id))' },
    },
    {
      put: job.image + '-unstable-gcs',
      params: { file: 'build-id-dir/%s*' % job.image },
      get_params: { skip_download: 'true' },
    },
    {
      load_var: 'gcs-url',
      file: '%s-unstable-gcs/url' % job.image,
    },
    {
      task: 'generate-build-id-sbom',
      file: 'guest-test-infra/concourse/tasks/generate-build-id-sbom.yaml',
      vars: { prefix: job.image, id: '((.:id))' },
    },
    {
      put: job.image + '-sbom',
      params: { file: 'build-id-dir-sbom/%s*' % job.image },
      get_params: { skip_download: 'true' },
    },
    {
      load_var: 'sbom-destination',
      file: '%s-sbom/url' % job.image,
    },
    {
      task: 'generate-build-id-shasum',
      file: 'guest-test-infra/concourse/tasks/generate-build-id-shasum.yaml',
      vars: { prefix: job.image, id: '((.:id))' },
    },
    {
      put: job.image + '-shasum',
      params: { file: 'build-id-dir-shasum/%s*' % job.image },
      get_params: { skip_download: 'true' },
    },
    {
      load_var: 'sha256-txt',
      file: '%s-shasum/url' % job.image,
    },
    {
      task: 'generate-build-date',
      file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
    },
    {
      load_var: 'build-date',
      file: 'publish-version/version',
    },
    {
      task: 'get-secret-iso',
      config: gcp_secret_manager.getsecrettask { secret_name: job.iso_secret },
    },
    {
      load_var: 'windows-iso',
      file: 'gcp-secret-manager/' + job.iso_secret,
    },
    {
      task: 'get-secret-updates',
      config: gcp_secret_manager.getsecrettask { secret_name: job.updates_secret },
    },
    {
      load_var: 'windows-updates',
      file: 'gcp-secret-manager/' + job.updates_secret,
    },
    {
      task: 'get-secret-pwsh',
      config: gcp_secret_manager.getsecrettask { secret_name: 'windows_gcs_pwsh' },
    },
    {
      load_var: 'windows-gcs-pwsh',
      file: 'gcp-secret-manager/windows_gcs_pwsh',
    },
    {
      task: 'get-secret-cloud-sdk',
      config: gcp_secret_manager.getsecrettask { secret_name: 'windows_gcs_cloud_sdk' },
    },
    {
      load_var: 'windows-cloud-sdk',
      file: 'gcp-secret-manager/windows_gcs_cloud_sdk',
    },
    {
      task: 'get-secret-dotnet48',
      config: gcp_secret_manager.getsecrettask { secret_name: 'windows_gcs_dotnet48' },
    },
    {
      load_var: 'windows-gcs-dotnet48',
      file: 'gcp-secret-manager/windows_gcs_dotnet48',
    },
    {
      task: 'get-secret-sbom-util',
      config: gcp_secret_manager.getsecrettask { secret_name: 'sbom-util-secret' },
    },
    {
      load_var: 'sbom-util-secret',
      file: 'gcp-secret-manager/sbom-util-secret',
    },
    {
      task: 'daisy-build-' + job.image,
      config: daisy.daisyimagetask {
        gcs_url: '((.:gcs-url))',
        sbom_destination: '((.:sbom-destination))',
        shasum_destination: '((.:sha256-txt))',
        workflow: job.workflow,
        vars+: [
          'cloudsdk=((.:windows-cloud-sdk))',
          'dotnet48=((.:windows-gcs-dotnet48))',
          'media=((.:windows-iso))',
          'pwsh=((.:windows-gcs-pwsh))',
          'updates=((.:windows-updates))',
          'google_cloud_repo=unstable',
          'sbom_util_gcs_root=((.:sbom-util-secret))',
        ],
      },
    },
  ],
  on_success: {
    task: 'publish-success-metric',
    config: common.publishresulttask {
      pipeline: 'release-package-windows-build',
      job: job.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'publish-failure-metric',
    config: common.publishresulttask {
      pipeline: 'release-package-windows-build',
      job: job.name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local imgpublishjob = {
  local job = self,

  image:: error 'must set image in imgpublishjob',
  env:: 'package',
  workflow:: error 'must set workflow in imgpublishjob',
  gcs_dir:: error 'must set gcs_dir in imgpublishjob',
  gcs:: 'gs://%s/%s' % [self.gcs_bucket, self.gcs_dir],
  gcs_shasum:: 'gs://%s/%s' % [self.gcs_sbom_bucket, self.gcs_dir],
  gcs_bucket:: common.prod_bucket,
  gcs_sbom_bucket:: common.sbom_bucket,
  generate_shasum:: true,
  topic:: common.prod_topic,

  // Start of job.
  name: 'publish-to-release-package-testing-%s-%s' % [job.env, job.image],
  plan: [
          { get: 'guest-test-infra' },
          { get: 'compute-image-tools' },
          {
            task: 'generate-timestamp',
            file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          },
          {
            load_var: 'start-timestamp-ms',
            file: 'timestamp/timestamp-ms',
          },
          {
            get: '%s-unstable-gcs' % job.image,
            params: { skip_download: 'true' },
            passed: ['build-release-package-testing-%s' % job.image],
            trigger: true,
          },
          {
            load_var: 'source-version',
            file: '%s-unstable-gcs/version' % job.image,
          },
          {
            task: 'generate-version',
            file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          },
          {
            load_var: 'publish-version',
            file: 'publish-version/version',
          },
        ] +
        (
          if job.generate_shasum then
            [
              {
                get: '%s-shasum' % job.image,
                params: { skip_download: 'true' },
                passed: ['build-release-package-testing-%s' % job.image],
                trigger: true,
              },
              {
                load_var: 'sha256-txt',
                file: '%s-shasum/url' % job.image,
              },
            ] else []
        ) +
        [
          {
            task: 'publish-release-package-testing-' + job.image,
            config: arle.gcepublishtask {
              source_gcs_path: job.gcs,
              source_version: 'v((.:source-version))',
              publish_version: '((.:publish-version))',
              wf: job.workflow,
              environment: job.env,
            },
          },
        ],
  on_success: {
    task: 'publish-success-metric',
    config: common.publishresulttask {
      pipeline: 'release-package-windows-build',
      job: job.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'publish-failure-metric',
    config: common.publishresulttask {
      pipeline: 'release-package-windows-build',
      job: job.name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local ImgBuildJob(image, iso_secret, updates_secret) = imgbuildjob {
  image: image,
  iso_secret: iso_secret,
  updates_secret: updates_secret,
  workflow: 'windows/%s-uefi.wf.json' % image,
};

local ImgPublishJob(image, env, workflow_dir, gcs_dir) = imgpublishjob {
  image: image,
  env: env,
  gcs_dir: gcs_dir,
  workflow: '%s/%s' % [workflow_dir, image + '-uefi.publish.json'],
};

local ImgGroup(name, images) = {
  name: name,
  jobs: [
    'build-release-package-testing-' + image
    for image in images
  ] + [
    'publish-to-release-package-testing-%s-%s' % ['package', image]
    for image in images
  ],
};

// Start of output.
{
  local windows_11_images = [
    'windows-11-23h2-ent-x64',  // remove after Nov 10, 2026
    'windows-11-24h2-ent-x64',  // remove after Oct 12, 2027
    'windows-11-25h2-ent-x64',
  ],
  local windows_2016_images = [
    'windows-server-2016-dc',
    'windows-server-2016-dc-core',
  ],
  local windows_2019_images = [
    'windows-server-2019-dc',
    'windows-server-2019-dc-core',
  ],
  local windows_2022_images = [
    'windows-server-2022-dc',
    'windows-server-2022-dc-core',
  ],
  local windows_2025_images = [
    'windows-server-2025-dc',
    'windows-server-2025-dc-core',
  ],

  local windows_client_images = windows_11_images,
  local windows_server_images = windows_2016_images + windows_2019_images + windows_2022_images + windows_2025_images,

  resource_types: [
    {
      name: 'gcs',
      source: { repository: 'frodenas/gcs-resource' },
      type: 'registry-image',
    },
    {
      name: 'registry-image-forked',
      type: 'registry-image',
      source: { repository: 'gcr.io/compute-image-tools/registry-image-forked' },
    },
  ],
  resources: [
               {
                 name: 'daily-time',
                 type: 'time',
                 source: { interval: '24h', start: '10:30 AM', stop: '11:00 AM', location: 'America/Los_Angeles', days: ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday'], initial_version: true },
               },
               common.GitResource('compute-image-tools'),
               common.GitResource('guest-test-infra'),
             ] +
             [
               common.GcsImgResource(image, 'windows-uefi')
               for image in windows_client_images + windows_server_images
             ] +
             [
               common.GcsImgResource(image, 'windows-uefi-unstable') {
                 name: '%s-unstable-gcs' % [self.image],
               }
               for image in windows_client_images + windows_server_images
             ] +
             [
               common.GcsSbomResource(image, 'windows-client')
               for image in windows_client_images
             ] +
             [
               common.GcsSbomResource(image, 'windows-server')
               for image in windows_server_images
             ] +
             [
               common.GcsShasumResource(image, 'windows-client')
               for image in windows_client_images
             ] +
             [
               common.GcsShasumResource(image, 'windows-server')
               for image in windows_server_images
             ],
  jobs: [
          // Windows builds
          ImgBuildJob('windows-11-23h2-ent-x64', 'win11-23h2-64', 'windows_gcs_updates_client11-23h2-64'),
          ImgBuildJob('windows-11-24h2-ent-x64', 'win11-24h2-64', 'windows_gcs_updates_client11-24h2-64'),
          ImgBuildJob('windows-11-25h2-ent-x64', 'win11-25h2-64', 'windows_gcs_updates_client11-25h2-64'),
          ImgBuildJob('windows-server-2025-dc', 'win2025-64', 'windows_gcs_updates_server2025'),
          ImgBuildJob('windows-server-2025-dc-core', 'win2025-64', 'windows_gcs_updates_server2025'),
          ImgBuildJob('windows-server-2022-dc', 'win2022-64', 'windows_gcs_updates_server2022'),
          ImgBuildJob('windows-server-2022-dc-core', 'win2022-64', 'windows_gcs_updates_server2022'),
          ImgBuildJob('windows-server-2019-dc', 'win2019-64', 'windows_gcs_updates_server2019'),
          ImgBuildJob('windows-server-2019-dc-core', 'win2019-64', 'windows_gcs_updates_server2019'),
          ImgBuildJob('windows-server-2016-dc', 'win2016-64', 'windows_gcs_updates_server2016'),
          ImgBuildJob('windows-server-2016-dc-core', 'win2016-64', 'windows_gcs_updates_server2016'),
        ] +
        // Publish jobs
        [
          ImgPublishJob(image, 'package', 'windows', 'windows-uefi')
          for image in windows_server_images
        ],

  groups: [
    ImgGroup('windows-11', windows_11_images),
    ImgGroup('windows-2016', windows_2016_images),
    ImgGroup('windows-2019', windows_2019_images),
    ImgGroup('windows-2022', windows_2022_images),
    ImgGroup('windows-2025', windows_2025_images),
  ],
}
