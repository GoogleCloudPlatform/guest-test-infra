// Imports.
local arle = import '../templates/arle.libsonnet';
local common = import '../templates/common.libsonnet';
local daisy = import '../templates/daisy.libsonnet';
local gcp_secret_manager = import '../templates/gcp-secret-manager.libsonnet';

local envs = ['testing', 'staging', 'prod'];
local underscore(input) = std.strReplace(input, '-', '_');

// Templates.
local imgbuildjob = {
  local job = self,

  image:: error 'must set image in imgbuildjob',
  workflow:: error 'must set workflow in imgbuildjob',
  iso_secret:: error 'must set iso_secret in imgbuildjob',
  updates_secret:: error 'must set updates_secret in imgbuildjob',

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
      task: 'generate-build-id',
      file: 'guest-test-infra/concourse/tasks/generate-build-id.yaml',
      vars: { prefix: job.image },
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
      task: 'daisy-build',
      config: daisy.daisyimagetask {
        gcs_url: '((.:gcs-url))',
        workflow: job.workflow,
        vars+: [
          'cloudsdk=((.:windows-cloud-sdk))',
          'dotnet48=((.:windows-gcs-dotnet48))',
          'media=((.:windows-iso))',
          'pwsh=((.:windows-gcs-pwsh))',
          'updates=((.:windows-updates))',
          'google_cloud_repo=stable',
        ],
      },
    },
  ],
};

local sqlimgbuildjob = {
  local job = self,

  image:: error 'must set image in sqlimgbuildjob',
  base_image:: error 'must set base_image in sqlimgbuildjob',
  workflow:: error 'must set workflow in sqlimgbuildjob',
  sql_version:: error 'must set sql_version in sqlbuildjob',

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
      get: '%s-gcs' % job.base_image,
      params: { skip_download: 'true' },
      passed: ['publish-to-testing-' + job.base_image],
      trigger: true,
    },
    {
      task: 'generate-build-id',
      file: 'guest-test-infra/concourse/tasks/generate-build-id.yaml',
      vars: { prefix: job.image },
    },
    {
      put: job.image + '-gcs',
      params: { file: 'build-id-dir/%s*' % job.image },
    },
    {
      load_var: 'gcs-url',
      file: '%s-gcs/url' % job.image,
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
      task: 'get-secret-sql-server-media',
      config: gcp_secret_manager.getsecrettask { secret_name: job.sql_version },
    },
    {
      load_var: 'sql-server-media',
      file: 'gcp-secret-manager/' + job.sql_version,
    },
    {
      task: 'get-secret-windows-gcs-ssms-exe',
      config: gcp_secret_manager.getsecrettask { secret_name: 'windows_gcs_ssms_exe' },
    },
    {
      load_var: 'windows-gcs-ssms-exe',
      file: 'gcp-secret-manager/windows_gcs_ssms_exe',
    },
    {
      task: 'daisy-build',
      config: daisy.daisyimagetask {
        gcs_url: '((.:gcs-url))',
        workflow: job.workflow,
        vars+: [
          'source_image_project=bct-prod-images',
          'sql_server_media=((.:sql-server-media))',
          'ssms_exe=((.:windows-gcs-ssms-exe))',
          'timeout=4h',
        ],
      },
    },
  ],
};

local containerimgbuildjob = {
  local job = self,

  image:: error 'must set image in containerimgbuildjob',
  base_image:: error 'must set base_image in containerimgbuildjob',
  workflow:: error 'must set workflow in containerimgbuildjob',

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
  plan: [
    {
      get: '%s-gcs' % job.base_image,
      params: { skip_download: 'true' },
      passed: ['publish-to-testing-' + job.base_image],
      trigger: true,
    },
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
      task: 'generate-build-id',
      file: 'guest-test-infra/concourse/tasks/generate-build-id.yaml',
      vars: { prefix: job.image },
    },
    {
      put: '%s-gcs' % job.image,
      params: { file: 'build-id-dir/%s*' % job.image },
    },
    {
      load_var: 'gcs-url',
      file: '%s-gcs/url' % job.image,
    },
    {
      task: 'daisy-build',
      config: daisy.daisyimagetask {
        gcs_url: '((.:gcs-url))',
        workflow: job.workflow,
        vars+: [
          'source_image_project=bct-prod-images',
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

  // Start of job.
  name: 'publish-to-%s-%s' % [job.env, job.image],
  plan: [
    { get: 'guest-test-infra' },
    { get: 'compute-image-tools' },
    {
      get: '%s-gcs' % job.image,
      params: { skip_download: 'true' },
      passed: [
        // build -> testing -> staging -> prod
        if job.env == 'testing' then
          'build-' + job.image
        else if job.env == 'staging' then
          'publish-to-testing-' + job.image
        else if job.env == 'prod' then
          'publish-to-staging-' + job.image,
      ],
      // Auto-publish to testing after build.
      trigger: if job.env == 'testing' then true else false,
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
    // Different publish step in prod
    if job.env == 'prod' then
      {
        task: 'arle-publish-' + job.image,
        config: arle.arlepublishtask {
          gcs_image_path: job.gcs,
          source_version: 'v((.:source-version))',
          publish_version: '((.:publish-version))',
          wf: job.workflow,
          image_name: job.image,
        },
      }
    else
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
  ],
};

local ImgBuildJob(image, iso_secret, updates_secret) = imgbuildjob {
  image: image,
  iso_secret: iso_secret,
  updates_secret: updates_secret,

  workflow: 'windows/%s-uefi.wf.json' % image,
};

local SQLImgBuildJob(image, base_image, sql_version) = sqlimgbuildjob {
  image: image,
  base_image: base_image,
  sql_version: sql_version,

  workflow: 'sqlserver/%s.wf.json' % image,
};

local ContainerImgBuildJob(image, base_image, workflow) = containerimgbuildjob {
  image: image,
  base_image: base_image,
  workflow: workflow,
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
    'build-' + image
    for image in images
  ] + [
    'publish-to-%s-%s' % [env, image]
    for env in envs
    for image in images
  ],
};

// Start of output.
{
  local windows_2004_images = [
    'windows-server-2004-dc-core',
  ],
  local windows_2012_images = [
    'windows-server-2012r2-dc',
    'windows-server-2012r2-dc-core',
  ],
  local windows_2016_images = [
    'windows-server-2016-dc',
    'windows-server-2016-dc-core',
  ],
  local windows_2019_images = [
    'windows-server-2019-dc',
    'windows-server-2019-dc-core',
  ],
  local windows_20h2_images = [
    'windows-server-20h2-dc-core',
  ],
  local windows_2022_images = [
    'windows-server-2022-dc',
    'windows-server-2022-dc-core',
  ],
  local sql_2012_images = [
    'sql-2012-enterprise-windows-2012-r2-dc',
    'sql-2012-standard-windows-2012-r2-dc',
    'sql-2012-web-windows-2012-r2-dc',
  ],
  local sql_2014_images = [
    'sql-2014-enterprise-windows-2012-r2-dc',
    'sql-2014-enterprise-windows-2016-dc',
    'sql-2014-standard-windows-2012-r2-dc',
    'sql-2014-web-windows-2012-r2-dc',
  ],
  local sql_2016_images = [
    'sql-2016-enterprise-windows-2012-r2-dc',
    'sql-2016-enterprise-windows-2016-dc',
    'sql-2016-enterprise-windows-2019-dc',
    'sql-2016-standard-windows-2012-r2-dc',
    'sql-2016-standard-windows-2016-dc',
    'sql-2016-standard-windows-2019-dc',
    'sql-2016-web-windows-2012-r2-dc',
    'sql-2016-web-windows-2016-dc',
    'sql-2016-web-windows-2019-dc',
  ],
  local sql_2017_images = [
    'sql-2017-enterprise-windows-2016-dc',
    'sql-2017-enterprise-windows-2019-dc',
    'sql-2017-enterprise-windows-2022-dc',
    'sql-2017-express-windows-2012-r2-dc',
    'sql-2017-express-windows-2016-dc',
    'sql-2017-express-windows-2019-dc',
    'sql-2017-standard-windows-2016-dc',
    'sql-2017-standard-windows-2019-dc',
    'sql-2017-standard-windows-2022-dc',
    'sql-2017-web-windows-2016-dc',
    'sql-2017-web-windows-2019-dc',
    'sql-2017-web-windows-2022-dc',
  ],
  local sql_2019_images = [
    'sql-2019-enterprise-windows-2019-dc',
    'sql-2019-enterprise-windows-2022-dc',
    'sql-2019-standard-windows-2019-dc',
    'sql-2019-standard-windows-2022-dc',
    'sql-2019-web-windows-2019-dc',
    'sql-2019-web-windows-2022-dc',
  ],
  local container_images = [
    'windows-server-2019-dc-for-containers',
    'windows-server-2019-dc-core-for-containers',
  ],

  local windows_images = windows_2004_images + windows_2012_images + windows_2016_images + windows_2019_images
                         + windows_20h2_images + windows_2022_images,
  local sql_images = sql_2012_images + sql_2014_images + sql_2016_images + sql_2017_images + sql_2019_images,

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
               common.GcsImgResource(image, 'windows-uefi')
               for image in windows_images + container_images
             ] +
             [
               common.GcsImgResource(image, 'sqlserver-uefi')
               for image in sql_images
             ],
  jobs: [
          // Windows builds

          ImgBuildJob('windows-server-2022-dc', 'win2022-64', 'windows_gcs_updates_server2022'),
          ImgBuildJob('windows-server-2022-dc-core', 'win2022-64', 'windows_gcs_updates_server2022'),
          ImgBuildJob('windows-server-20h2-dc-core', 'winserver-20h2-64', 'windows_gcs_updates_sac20h2'),
          ImgBuildJob('windows-server-2004-dc-core', 'winserver-2004-64', 'windows_gcs_updates_sac2004'),
          ImgBuildJob('windows-server-2019-dc', 'win2019-64', 'windows_gcs_updates_server2019'),
          ImgBuildJob('windows-server-2019-dc-core', 'win2019-64', 'windows_gcs_updates_server2019'),
          ImgBuildJob('windows-server-2016-dc', 'win2016-64', 'windows_gcs_updates_server2016'),
          ImgBuildJob('windows-server-2016-dc-core', 'win2016-64', 'windows_gcs_updates_server2016'),
          ImgBuildJob('windows-server-2012r2-dc', 'win2012-r2-64', 'windows_gcs_updates_server2012r2'),
          ImgBuildJob('windows-server-2012r2-dc-core', 'win2012-r2-64', 'windows_gcs_updates_server2012r2'),

          // SQL derivative builds

          SQLImgBuildJob('sql-2012-enterprise-windows-2012-r2-dc', 'windows-server-2012r2-dc', 'sql-2012-enterprise'),
          SQLImgBuildJob('sql-2012-standard-windows-2012-r2-dc', 'windows-server-2012r2-dc', 'sql-2012-standard'),
          SQLImgBuildJob('sql-2012-web-windows-2012-r2-dc', 'windows-server-2012r2-dc', 'sql-2012-web'),

          SQLImgBuildJob('sql-2014-enterprise-windows-2012-r2-dc', 'windows-server-2012r2-dc', 'sql-2014-enterprise'),
          SQLImgBuildJob('sql-2014-enterprise-windows-2016-dc', 'windows-server-2016-dc', 'sql-2014-enterprise'),
          SQLImgBuildJob('sql-2014-standard-windows-2012-r2-dc', 'windows-server-2012r2-dc', 'sql-2014-standard'),
          SQLImgBuildJob('sql-2014-web-windows-2012-r2-dc', 'windows-server-2012r2-dc', 'sql-2014-web'),

          SQLImgBuildJob('sql-2016-enterprise-windows-2012-r2-dc', 'windows-server-2012r2-dc', 'sql-2016-enterprise'),
          SQLImgBuildJob('sql-2016-enterprise-windows-2016-dc', 'windows-server-2016-dc', 'sql-2016-enterprise'),
          SQLImgBuildJob('sql-2016-enterprise-windows-2019-dc', 'windows-server-2019-dc', 'sql-2016-enterprise'),
          SQLImgBuildJob('sql-2016-standard-windows-2012-r2-dc', 'windows-server-2012r2-dc', 'sql-2016-standard'),
          SQLImgBuildJob('sql-2016-standard-windows-2016-dc', 'windows-server-2016-dc', 'sql-2016-standard'),
          SQLImgBuildJob('sql-2016-standard-windows-2019-dc', 'windows-server-2019-dc', 'sql-2016-standard'),
          SQLImgBuildJob('sql-2016-web-windows-2012-r2-dc', 'windows-server-2012r2-dc', 'sql-2016-web'),
          SQLImgBuildJob('sql-2016-web-windows-2016-dc', 'windows-server-2016-dc', 'sql-2016-web'),
          SQLImgBuildJob('sql-2016-web-windows-2019-dc', 'windows-server-2019-dc', 'sql-2016-web'),

          SQLImgBuildJob('sql-2017-enterprise-windows-2016-dc', 'windows-server-2016-dc', 'sql-2017-enterprise'),
          SQLImgBuildJob('sql-2017-enterprise-windows-2019-dc', 'windows-server-2019-dc', 'sql-2017-enterprise'),
          SQLImgBuildJob('sql-2017-enterprise-windows-2022-dc', 'windows-server-2022-dc', 'sql-2017-enterprise'),
          SQLImgBuildJob('sql-2017-express-windows-2012-r2-dc', 'windows-server-2012r2-dc', 'sql-2017-express'),
          SQLImgBuildJob('sql-2017-express-windows-2016-dc', 'windows-server-2016-dc', 'sql-2017-express'),
          SQLImgBuildJob('sql-2017-express-windows-2019-dc', 'windows-server-2019-dc', 'sql-2017-express'),
          SQLImgBuildJob('sql-2017-standard-windows-2016-dc', 'windows-server-2016-dc', 'sql-2017-standard'),
          SQLImgBuildJob('sql-2017-standard-windows-2019-dc', 'windows-server-2019-dc', 'sql-2017-standard'),
          SQLImgBuildJob('sql-2017-standard-windows-2022-dc', 'windows-server-2022-dc', 'sql-2017-standard'),
          SQLImgBuildJob('sql-2017-web-windows-2016-dc', 'windows-server-2016-dc', 'sql-2017-web'),
          SQLImgBuildJob('sql-2017-web-windows-2019-dc', 'windows-server-2019-dc', 'sql-2017-web'),
          SQLImgBuildJob('sql-2017-web-windows-2022-dc', 'windows-server-2022-dc', 'sql-2017-web'),

          SQLImgBuildJob('sql-2019-enterprise-windows-2019-dc', 'windows-server-2019-dc', 'sql-2019-enterprise'),
          SQLImgBuildJob('sql-2019-enterprise-windows-2022-dc', 'windows-server-2022-dc', 'sql-2019-enterprise'),
          SQLImgBuildJob('sql-2019-standard-windows-2019-dc', 'windows-server-2019-dc', 'sql-2019-standard'),
          SQLImgBuildJob('sql-2019-standard-windows-2022-dc', 'windows-server-2022-dc', 'sql-2019-standard'),
          SQLImgBuildJob('sql-2019-web-windows-2019-dc', 'windows-server-2019-dc', 'sql-2019-web'),
          SQLImgBuildJob('sql-2019-web-windows-2022-dc', 'windows-server-2022-dc', 'sql-2019-web'),

          // Container derivative builds

          ContainerImgBuildJob('windows-server-2019-dc-for-containers',
                               'windows-server-2019-dc',
                               // TODO: Broken naming scheme between image and workflow
                               'windows_container/windows-2019-for-containers-uefi.wf.json'),
          ContainerImgBuildJob('windows-server-2019-dc-core-for-containers',
                               'windows-server-2019-dc-core',
                               'windows_container/windows-2019-core-for-containers-uefi.wf.json'),

        ] +

        // Publish jobs

        [
          ImgPublishJob(image, env, 'windows', 'windows-uefi')
          for image in windows_images
          for env in envs
        ] +
        [
          ImgPublishJob(image, env, 'sqlserver', 'sqlserver-uefi')
          for image in sql_images
          for env in envs
        ] +
        [
          ImgPublishJob(image, env, 'windows_container', 'windows-uefi')
          for image in container_images
          for env in envs
        ],

  groups: [
    ImgGroup('windows-2004', windows_2004_images),
    ImgGroup('windows-2012', windows_2012_images),
    ImgGroup('windows-2016', windows_2016_images),
    ImgGroup('windows-2019', windows_2019_images),
    ImgGroup('windows-2022', windows_2022_images),
    ImgGroup('windows-20h2', windows_20h2_images),
    ImgGroup('sql-2012', sql_2012_images),
    ImgGroup('sql-2014', sql_2014_images),
    ImgGroup('sql-2016', sql_2016_images),
    ImgGroup('sql-2017', sql_2017_images),
    ImgGroup('sql-2019', sql_2019_images),
    ImgGroup('container-2019', container_images),
  ],
}
