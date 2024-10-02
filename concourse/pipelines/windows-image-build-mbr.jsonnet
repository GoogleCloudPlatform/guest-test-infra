// Imports.
THIS WILL FAIL
local arle = import '../templates/arle.libsonnet';
local common = import '../templates/common.libsonnet';
local daisy = import '../templates/daisy.libsonnet';
local gcp_secret_manager = import '../templates/gcp-secret-manager.libsonnet';

local envs = ['testing'];
local underscore(input) = std.strReplace(input, '-', '_');

local imagetesttask = common.imagetesttask {
  filter: '^(cvm|livemigrate|suspendresume|loadbalancer|guestagent|hostnamevalidation|imageboot|licensevalidation|network|security|hotattach|lssd|disk|shapevalidation|packageupgrade|packagevalidation|ssh|winrm|metadata|sql|windowscontainers)$',
  extra_args: [ '-x86_shape=n1-standard-4', '-shapevalidation_test_filter=^(([A-Z][0-3])|(N4))' ],
};

// Templates.
local imgbuildjob = {
  local job = self,

  image:: error 'must set image in imgbuildjob',
  workflow:: error 'must set workflow in imgbuildjob',
  iso_secret:: error 'must set iso_secret in imgbuildjob',
  updates_secret:: error 'must set updates_secret in imgbuildjob',

  // Start of job.
  name: 'build-' + job.image,
  plan: [
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
  gcs_bucket:: common.prod_bucket,
  topic:: common.prod_topic,
  image_prefix:: self.image,

  // Publish can proceed if build passes.
  passed:: if job.env == 'testing' then
    'build-' + job.image
  else
    'publish-to-testing-' + job.image,

  // Builds are automatically pushed to testing.
  trigger:: if job.env == 'testing' then true
    else false,

  runtests:: if job.env == 'testing' && (std.length(std.findSubstr("server", job.image)) > 0 || std.length(std.findSubstr("sql", job.image)) > 0) then true
    else false,

  // Start of job.
  name: 'publish-to-%s-%s' % [job.env, job.image],
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
    },
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
  ] +
  if job.runtests then
    [
      {
        task: 'image-test-' + job.image,
        config: imagetesttask {
          images: 'projects/bct-prod-images/global/images/%s-((.:publish-version))' % job.image_prefix,
        },
        attempts: 3,
      },
    ]
  else [],
};

local ImgBuildJob(image, iso_secret, updates_secret) = imgbuildjob {
  image: image,
  iso_secret: iso_secret,
  updates_secret: updates_secret,
  workflow: 'windows/%s.wf.json' % image,
};

local ImgPublishJob(image, env, workflow_dir, gcs_dir) = imgpublishjob {
  image: image,
  env: env,
  gcs_dir: gcs_dir,
  passed:: 'build-' + image,
  workflow: '%s/%s' % [workflow_dir, image + '.publish.json'],
};

local ImgGroup(name, images, environments) = {
  name: name,
  jobs: [
    'build-' + image
    for image in images
  ] +
  [
    'publish-to-%s-%s' % [env, image]
    for env in environments
    for image in images
  ],
};

// Start of output.
{
  local windows_2016_images = [
    'windows-server-2016-dc-bios',
  ],
  local windows_2019_images = [
    'windows-server-2019-dc-bios',
  ],
  local windows_2022_images = [
    'windows-server-2022-dc-bios',
  ],

  local images = windows_2016_images + windows_2019_images
               + windows_2022_images,

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
               common.GitResource('compute-image-tools'),
               common.GitResource('guest-test-infra'),
             ] +
             [
               common.GcsImgResource(image, 'windows-bios')
               for image in images
             ] +
             [
               common.GcsSbomResource(image, 'windows-server-bios')
               for image in images
             ],
  jobs: [
          ImgBuildJob('windows-server-2022-dc-bios', 'win2022-64', 'windows_gcs_updates_server2022'),
          ImgBuildJob('windows-server-2019-dc-bios', 'win2019-64', 'windows_gcs_updates_server2019'),
          ImgBuildJob('windows-server-2016-dc-bios', 'win2016-64', 'windows_gcs_updates_server2016'),
        ] +
        [
          ImgPublishJob(image, env, 'windows', 'windows-bios')
          for image in images
          for env in envs
        ],

  groups: [
    ImgGroup('windows-2016-bios', windows_2016_images, envs),
    ImgGroup('windows-2019-bios', windows_2019_images, envs),
    ImgGroup('windows-2022-bios', windows_2022_images, envs),
  ],
}
