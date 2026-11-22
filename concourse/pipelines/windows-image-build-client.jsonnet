// Imports.
local arle = import '../templates/arle.libsonnet';
local common = import '../templates/common.libsonnet';
local daisy = import '../templates/daisy.libsonnet';
local gcp_secret_manager = import '../templates/gcp-secret-manager.libsonnet';

local client_envs = ['testing', 'internal', 'client'];
local prerelease_envs = ['testing'];
local underscore(input) = std.strReplace(input, '-', '_');

// Templates.
local imagetestn1 = common.imagetesttask {
  filter: '^(cvm|livemigrate|suspendresume|loadbalancer|guestagent|hostnamevalidation|imageboot|licensevalidation|network|security|hotattach|lssd|disk|shapevalidation|packageupgrade|packagevalidation|ssh|winrm|metadata|sql|windowscontainers)$',
  extra_args: [ '-x86_shape=n1-standard-4', '-timeout=60m', '-shapevalidation_test_filter=^(([A-Z][0-3])|(N4))' ],
};

local imagetestc3 = common.imagetesttask {
  filter: '^(livemigrate|suspendresume|imageboot|network|hotattach|lssd|disk)$',
  extra_args: [ '-x86_shape=c3-standard-4', '-timeout=60m' ],
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
  name: 'build-' + job.image,
  on_success: {
    task: 'publish-success-metric',
    config: common.publishresulttask {
      pipeline: 'windows-image-build',
      job: job.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'publish-failure-metric',
    config: common.publishresulttask {
      pipeline: 'windows-image-build',
      job: job.name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
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
      vars: { prefix: job.image, id: '((.:id))'},
    },
    {
      put: job.image + '-gcs',
      params: { file: 'build-id-dir/%s*' % job.image },
      get_params: { skip_download: 'true' },
    },
    {
      load_var: 'gcs-url',
      file: '%s-gcs/url' % job.image,
    },
    {
      task: 'generate-build-id-sbom',
      file: 'guest-test-infra/concourse/tasks/generate-build-id-sbom.yaml',
      vars: { prefix: job.image, id: '((.:id))'},
    },
    {
      put: job.image + '-sbom',
      params: {
        file: 'build-id-dir-sbom/%s*' % job.image,
      },
      get_params: {
        skip_download: 'true',
      },
    },
    {
      load_var: 'sbom-destination',
      file: '%s-sbom/url' % job.image,
    },
    {
      task: 'generate-build-id-shasum',
      file: 'guest-test-infra/concourse/tasks/generate-build-id-shasum.yaml',
      vars: { prefix: job.image, id: '((.:id))'},
    },
    {
      put: job.image + '-shasum',
      params: {
        file: 'build-id-dir-shasum/%s*' % job.image,
      },
      get_params: {
        skip_download: 'true',
      },
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
    // We download and then load each of the 3 following secrets; they could be merged.
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
      task: 'daisy-build',
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
          'google_cloud_repo=stable',
          'sbom_util_gcs_root=((.:sbom-util-secret))',
        ],
      },
    },
  ],
};

local imgpublishjob = {
  local job = self,

  image:: error 'must set image in imgpublishjob',
  env:: error 'must set publish env in imgpublishjob',
  workflow:: error 'must set workflow in imgpublishjob',
  gcs_dir:: error 'must set gcs_dir in imgpublishjob',
  gcs:: 'gs://%s/%s' % [self.gcs_bucket, self.gcs_dir],
  gcs_shasum:: 'gs://%s/%s' % [self.gcs_sbom_bucket, self.gcs_dir],
  gcs_bucket:: common.prod_bucket,
  gcs_sbom_bucket:: common.sbom_bucket,
  generate_shasum:: true,
  topic:: common.prod_topic,

  // Publish can proceed if build passes.
  passed:: if job.env == 'testing' then
    'build-' + job.image
  else
    'publish-to-testing-' + job.image,

  // Builds are automatically pushed to testing.
  trigger:: if job.env == 'testing' then true
    else if job.env == 'internal' then true
    else if job.env == 'byol' then true
    else if job.env == 'client' then true
    else false,

  // Run tests on server and sql images
  runtests:: if job.env == 'testing' && (std.length(std.findSubstr("server", job.image)) > 0 || std.length(std.findSubstr("sql", job.image)) > 0) then true
    else false,

  // Start of job.
  name: 'publish-to-%s-%s' % [job.env, job.image],
  on_success: {
    task: 'publish-success-metric',
    config: common.publishresulttask {
      pipeline: 'windows-image-build',
      job: job.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'publish-failure-metric',
    config: common.publishresulttask {
      pipeline: 'windows-image-build',
      job: job.name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
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
      get: '%s-gcs' % job.image,
      params: { skip_download: 'true' },
      passed: [job.passed],
      trigger: job.trigger,
    },
    {
      load_var: 'source-version',
      file: '%s-gcs/version' % job.image,
    },
    {
      task: 'generate-version',
      file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
    },
    {
      load_var: 'publish-version',
      file: 'publish-version/version',
    }, ] +
  (if job.generate_shasum then
  [
    {
      get: '%s-shasum' % job.image,
      params: { skip_download: 'true' },
      passed: [job.passed],
      trigger: job.trigger,
    },
    {
      load_var: 'sha256-txt',
      file: '%s-shasum/url' % job.image,
    },
  ]
  else
  []) +
  (if job.env == 'prod' then
  [
    {
      task: 'arle-publish-' + job.image,
      config: arle.arlepublishtask {
        gcs_image_path: job.gcs,
        image_sha256_hash_txt: if job.generate_shasum then '((.:sha256-txt))' else '',
        source_version: 'v((.:source-version))',
        publish_version: '((.:publish-version))',
        wf: job.workflow,
        image_name: job.image,
      },
    },
  ]
  else
  [
    {
      task: 'gce-image-publish-' + job.image,
      config: arle.gcepublishtask {
        source_gcs_path: job.gcs,
        source_version: 'v((.:source-version))',
        publish_version: '((.:publish-version))',
        wf: job.workflow,
        environment: if job.env == 'testing' then 'test' else job.env,
      },
    },
  ]) +
  (if job.runtests then
  [
    {
      task: 'image-test-n1-' + job.image,
      config: imagetestn1 {
        images: 'projects/bct-prod-images/global/images/%s-((.:publish-version))' % job.image,
      },
      attempts: 3,
    },
    {
      task: 'image-test-c3-' + job.image,
      config: imagetestc3 {
        images: 'projects/bct-prod-images/global/images/%s-((.:publish-version))' % job.image,
      },
      attempts: 3,
    }
  ]
  else
  []),
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
  // build -> testing -> prod -> internal & byol
  // build -> testing -> internal & client
  passed:: if env == 'testing' then
             'build-' + image
           else if env == 'prod' then
             'publish-to-testing-' + image
           else if env == 'internal' then
             'publish-to-prod-' + image
           else if env == 'byol' then
             'publish-to-prod-' + image
           else if env == 'client' then
             'publish-to-testing-' + image,

  workflow: '%s/%s' % [workflow_dir, image + '-uefi.publish.json'],
};

local MediaImgPublishJob(image, env, workflow_dir, gcs_dir) = imgpublishjob {
  image: image,
  env: env,
  gcs_dir: gcs_dir,
  generate_shasum: false,
  // build -> testing -> prod
  passed:: if env == 'testing' then
             'build-' + image
           else if env == 'prod' then
             'publish-to-testing-' + image,

  workflow: '%s/%s' % [workflow_dir, image + '.publish.json'],
};

local ImgGroup(name, images, environments) = {
  name: name,
  jobs: [
    'build-' + image
    for image in images
  ] + [
    'publish-to-%s-%s' % [env, image]
    for env in environments
    for image in images
  ],
};

// Start of output.
{
  local windows_10_images = [
    'windows-10-21h2-ent-x64',
    'windows-10-22h2-ent-x64',
  ],
  local windows_11_images = [
    'windows-11-21h2-ent-x64',
    'windows-11-22h2-ent-x64',
    'windows-11-23h2-ent-x64',
    'windows-11-24h2-ent-x64',
  ],

  local windows_client_images = windows_10_images + windows_11_images,

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
               for image in windows_client_images
             ] +
             [
               common.GcsSbomResource(image, 'windows-client')
               for image in windows_client_images
             ] +
             [
               common.GcsShasumResource(image, 'windows-client')
               for image in windows_client_images
             ],
  jobs: [
          // Windows builds

          ImgBuildJob('windows-10-21h2-ent-x64', 'win10-21h2-64', 'windows_gcs_updates_client10-21h2-64'),
          ImgBuildJob('windows-10-22h2-ent-x64', 'win10-22h2-64', 'windows_gcs_updates_client10-22h2-64'),
          ImgBuildJob('windows-11-21h2-ent-x64', 'win11-21h2-64', 'windows_gcs_updates_client11-21h2-64'),
          ImgBuildJob('windows-11-22h2-ent-x64', 'win11-22h2-64', 'windows_gcs_updates_client11-22h2-64'),
          ImgBuildJob('windows-11-23h2-ent-x64', 'win11-23h2-64', 'windows_gcs_updates_client11-23h2-64'),
          ImgBuildJob('windows-11-24h2-ent-x64', 'win11-24h2-64', 'windows_gcs_updates_client11-24h2-64'),
        ] +

        // Publish jobs

        // Windows client has 2 jobs to account for skipping of prod environment. This avoids needing to
        // rewrite the rest of the passed logic. TODO: Mod logic such that only 1 ImgPublishJob is needed

        [
          ImgPublishJob(image, env, 'windows', 'windows-uefi')
          for image in windows_client_images
          for env in ['testing', 'client']
        ] +
        [
          ImgPublishJob(image, 'internal', 'windows', 'windows-uefi') {passed:'publish-to-testing-' + image}
          for image in windows_client_images
        ],

  groups: [
    ImgGroup('windows-10', windows_10_images, client_envs),
    ImgGroup('windows-11', windows_11_images, client_envs),
  ],
}
