// Imports.
local arle = import '../templates/arle.libsonnet';
local common = import '../templates/common.libsonnet';
local daisy = import '../templates/daisy.libsonnet';
local gcp_secret_manager = import '../templates/gcp-secret-manager.libsonnet';

local client_envs = ['testing', 'staging', 'internal'];
local server_envs = ['testing', 'staging', 'internal', 'prod'];
local sql_envs = ['testing', 'staging', 'prod'];
local prerelease_envs = ['testing', 'staging'];
local windows_install_media_envs = ['testing', 'prod'];
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
  ssms_version:: error 'must set ssms_version in sqlbuildjob',

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
      task: 'get-secret-ssms-version',
      config: gcp_secret_manager.getsecrettask { secret_name: job.ssms_version },
    },
    {
      load_var: 'ssms-version',
      file: 'gcp-secret-manager/' + job.ssms_version,
    },
    {
      task: 'daisy-build',
      config: daisy.daisyimagetask {
        gcs_url: '((.:gcs-url))',
        workflow: job.workflow,
        vars+: [
          'source_image_project=bct-prod-images',
          'sql_server_media=((.:sql-server-media))',
          'ssms_exe=((.:ssms-version))',
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

local windowsinstallmediaimgbuildjob = {
  local job = self,

  image:: error 'must set image in windowsinstallmediaimgbuildjob',
  workflow:: error 'must set workflow in windowsinstallmediaimgbuildjob',

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
      put: '%s-gcs' % job.image,
      params: { file: 'build-id-dir/%s*' % job.image },
    },
    {
      load_var: 'gcs-url',
      file: '%s-gcs/url' % job.image,
    },
    {
      task: 'get-secret-iso-path-2022',
      config: gcp_secret_manager.getsecrettask { secret_name: 'win2022-64' },
    },
    {
      load_var: 'iso_path_2022',
      file: 'gcp-secret-manager/win2022-64',
    },
    {
      task: 'get-secret-iso-path-2019',
      config: gcp_secret_manager.getsecrettask { secret_name: 'win2019-64' },
    },
    {
      load_var: 'iso_path_2019',
      file: 'gcp-secret-manager/win2019-64',
    },
    {
      task: 'get-secret-iso-path-2016',
      config: gcp_secret_manager.getsecrettask { secret_name: 'win2016-64' },
    },
    {
      load_var: 'iso_path_2016',
      file: 'gcp-secret-manager/win2016-64',
    },
    {
      task: 'get-secret-iso-path-2012r2',
      config: gcp_secret_manager.getsecrettask { secret_name: 'win2012-r2-64' },
    },
    {
      load_var: 'iso_path_2012r2',
      file: 'gcp-secret-manager/win2012-r2-64',
    },
    {
       task: 'get-secret-updates-path-2022',
       config: gcp_secret_manager.getsecrettask { secret_name: 'windows_gcs_updates_server2022' },
     },
     {
       load_var: 'updates_path_2022',
       file: 'gcp-secret-manager/windows_gcs_updates_server2022',
     },
    {
       task: 'get-secret-updates-path-2019',
       config: gcp_secret_manager.getsecrettask { secret_name: 'windows_gcs_updates_server2019' },
     },
     {
       load_var: 'updates_path_2019',
       file: 'gcp-secret-manager/windows_gcs_updates_server2019',
     },
    {
       task: 'get-secret-updates-path-2016',
       config: gcp_secret_manager.getsecrettask { secret_name: 'windows_gcs_updates_server2016' },
     },
     {
       load_var: 'updates_path_2016',
       file: 'gcp-secret-manager/windows_gcs_updates_server2016',
     },
    {
       task: 'get-secret-updates-path-2012r2',
       config: gcp_secret_manager.getsecrettask { secret_name: 'windows_gcs_updates_server2012r2' },
     },
     {
       load_var: 'updates_path_2012r2',
       file: 'gcp-secret-manager/windows_gcs_updates_server2012r2',
     },
     {
      task: 'daisy-build',
      config: daisy.daisywindowsinstallmediatask {
        workflow: job.workflow,
        gcs_url: '((.:gcs-url))',
        iso_path_2022: '((.:iso_path_2022))',
        iso_path_2019: '((.:iso_path_2019))',
        iso_path_2016: '((.:iso_path_2016))',
        iso_path_2012r2: '((.:iso_path_2012r2))',
        updates_path_2022: '((.:updates_path_2022))',
        updates_path_2019: '((.:updates_path_2019))',
        updates_path_2016: '((.:updates_path_2016))',
        updates_path_2012r2: '((.:updates_path_2012r2))',
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
  passed:: error 'must set passed in imgpublishjob',

  // Start of job.
  name: 'publish-to-%s-%s' % [job.env, job.image],
  plan: [
    { get: 'guest-test-infra' },
    { get: 'compute-image-tools' },
    {
      get: '%s-gcs' % job.image,
      params: { skip_download: 'true' },
      passed: [
        job.passed,
      ],
      // Builds are automatically pushed to testing. Triggering staging will automatically progress to prod and internal.
      trigger: if job.env == 'staging' then false else true,
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

local SQLImgBuildJob(image, base_image, sql_version, ssms_version) = sqlimgbuildjob {
  image: image,
  base_image: base_image,
  sql_version: sql_version,
  ssms_version: ssms_version,

  workflow: 'sqlserver/%s.wf.json' % image,
};

local ContainerImgBuildJob(image, base_image, workflow) = containerimgbuildjob {
  image: image,
  base_image: base_image,
  workflow: workflow,
};

local WindowsInstallMediaImgBuildJob(image) = windowsinstallmediaimgbuildjob {
  image: image,
  workflow: 'windows/%s.wf.json' % image,
};

local ImgPublishJob(image, env, workflow_dir, gcs_dir) = imgpublishjob {
  image: image,
  env: env,
  gcs_dir: gcs_dir,
  // build -> testing -> staging -> prod -> internal
  passed:: if env == 'testing' then
             'build-' + image
           else if env == 'staging' then
             'publish-to-testing-' + image
           else if env == 'prod' then
             'publish-to-staging-' + image
           else if env == 'internal' then
             'publish-to-prod-' + image,

  workflow: '%s/%s' % [workflow_dir, image + '-uefi.publish.json'],
};

local MediaImgPublishJob(image, env, workflow_dir, gcs_dir) = imgpublishjob {
  image: image,
  env: env,
  gcs_dir: gcs_dir,
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
  local windows_81_images = [
    'windows-81-ent-x64',
  ],
  local windows_10_images = [
    'windows-10-21h2-ent-x64',
  ],
  local windows_11_images = [
    'windows-11-21h2-ent-x64',
  ],
  local windows_2012_images = [
    'windows-server-2012-r2-dc',
    'windows-server-2012-r2-dc-core',
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
  local sql_2022_images = [
    'sql-2022-preview-windows-2022-dc',
  ],
  local container_images = [
    'windows-server-2019-dc-for-containers',
    'windows-server-2019-dc-core-for-containers',
  ],
  local windows_install_media_images = [
    'windows-install-media',
  ],

  local windows_client_images = windows_81_images + windows_10_images + windows_11_images,
  local windows_server_images = windows_2012_images + windows_2016_images + windows_2019_images
                         + windows_20h2_images + windows_2022_images,
  local sql_images = sql_2014_images + sql_2016_images + sql_2017_images + sql_2019_images,
  local prerelease_images = sql_2022_images,

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
               for image in windows_client_images + windows_server_images + container_images
             ] +
             [
               common.GcsImgResource(image, 'sqlserver-uefi')
               for image in sql_images
             ] +
             //ImgResource for SQL Preview build. Will be rolled into sql_images on formal release.
             [
               common.GcsImgResource('sql-2022-preview-windows-2022-dc', 'sqlserver-uefi')
             ] +
             [
               common.GcsImgResource(image, 'windows-install-media')
               for image in windows_install_media_images
             ],
  jobs: [
          // Windows builds

          ImgBuildJob('windows-81-ent-x64', 'win81-64', 'windows_gcs_updates_client81-64'),
          ImgBuildJob('windows-10-21h2-ent-x64', 'win10-21h2-64', 'windows_gcs_updates_client10-21h2-64'),
          ImgBuildJob('windows-11-21h2-ent-x64', 'win11-21h2-64', 'windows_gcs_updates_client11-21h2-64'),
          ImgBuildJob('windows-server-2022-dc', 'win2022-64', 'windows_gcs_updates_server2022'),
          ImgBuildJob('windows-server-2022-dc-core', 'win2022-64', 'windows_gcs_updates_server2022'),
          ImgBuildJob('windows-server-20h2-dc-core', 'winserver-20h2-64', 'windows_gcs_updates_sac20h2'),
          ImgBuildJob('windows-server-2019-dc', 'win2019-64', 'windows_gcs_updates_server2019'),
          ImgBuildJob('windows-server-2019-dc-core', 'win2019-64', 'windows_gcs_updates_server2019'),
          ImgBuildJob('windows-server-2016-dc', 'win2016-64', 'windows_gcs_updates_server2016'),
          ImgBuildJob('windows-server-2016-dc-core', 'win2016-64', 'windows_gcs_updates_server2016'),
          ImgBuildJob('windows-server-2012-r2-dc', 'win2012-r2-64', 'windows_gcs_updates_server2012r2'),
          ImgBuildJob('windows-server-2012-r2-dc-core', 'win2012-r2-64', 'windows_gcs_updates_server2012r2'),

          // SQL derivative builds

          SQLImgBuildJob('sql-2014-enterprise-windows-2012-r2-dc', 'windows-server-2012-r2-dc', 'sql-2014-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2014-enterprise-windows-2016-dc', 'windows-server-2016-dc', 'sql-2014-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2014-standard-windows-2012-r2-dc', 'windows-server-2012-r2-dc', 'sql-2014-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2014-web-windows-2012-r2-dc', 'windows-server-2012-r2-dc', 'sql-2014-web', 'windows_gcs_ssms_exe'),

          SQLImgBuildJob('sql-2016-enterprise-windows-2012-r2-dc', 'windows-server-2012-r2-dc', 'sql-2016-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-enterprise-windows-2016-dc', 'windows-server-2016-dc', 'sql-2016-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-enterprise-windows-2019-dc', 'windows-server-2019-dc', 'sql-2016-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-standard-windows-2012-r2-dc', 'windows-server-2012-r2-dc', 'sql-2016-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-standard-windows-2016-dc', 'windows-server-2016-dc', 'sql-2016-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-standard-windows-2019-dc', 'windows-server-2019-dc', 'sql-2016-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-web-windows-2012-r2-dc', 'windows-server-2012-r2-dc', 'sql-2016-web', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-web-windows-2016-dc', 'windows-server-2016-dc', 'sql-2016-web', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-web-windows-2019-dc', 'windows-server-2019-dc', 'sql-2016-web', 'windows_gcs_ssms_exe'),

          SQLImgBuildJob('sql-2017-enterprise-windows-2016-dc', 'windows-server-2016-dc', 'sql-2017-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-enterprise-windows-2019-dc', 'windows-server-2019-dc', 'sql-2017-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-enterprise-windows-2022-dc', 'windows-server-2022-dc', 'sql-2017-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-express-windows-2012-r2-dc', 'windows-server-2012-r2-dc', 'sql-2017-express', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-express-windows-2016-dc', 'windows-server-2016-dc', 'sql-2017-express', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-express-windows-2019-dc', 'windows-server-2019-dc', 'sql-2017-express', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-standard-windows-2016-dc', 'windows-server-2016-dc', 'sql-2017-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-standard-windows-2019-dc', 'windows-server-2019-dc', 'sql-2017-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-standard-windows-2022-dc', 'windows-server-2022-dc', 'sql-2017-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-web-windows-2016-dc', 'windows-server-2016-dc', 'sql-2017-web', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-web-windows-2019-dc', 'windows-server-2019-dc', 'sql-2017-web', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-web-windows-2022-dc', 'windows-server-2022-dc', 'sql-2017-web', 'windows_gcs_ssms_exe'),

          SQLImgBuildJob('sql-2019-enterprise-windows-2019-dc', 'windows-server-2019-dc', 'sql-2019-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2019-enterprise-windows-2022-dc', 'windows-server-2022-dc', 'sql-2019-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2019-standard-windows-2019-dc', 'windows-server-2019-dc', 'sql-2019-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2019-standard-windows-2022-dc', 'windows-server-2022-dc', 'sql-2019-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2019-web-windows-2019-dc', 'windows-server-2019-dc', 'sql-2019-web', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2019-web-windows-2022-dc', 'windows-server-2022-dc', 'sql-2019-web', 'windows_gcs_ssms_exe'),

          SQLImgBuildJob('sql-2022-preview-windows-2022-dc', 'windows-server-2022-dc', 'sql-2022-preview', 'windows_gcs_ssms19_exe'),

          // Container derivative builds

          ContainerImgBuildJob('windows-server-2019-dc-for-containers',
                               'windows-server-2019-dc',
                               // TODO: Broken naming scheme between image and workflow
                               'windows_container/windows-2019-for-containers-uefi.wf.json'),
          ContainerImgBuildJob('windows-server-2019-dc-core-for-containers',
                               'windows-server-2019-dc-core',
                               'windows_container/windows-2019-core-for-containers-uefi.wf.json'),

          // Windows install media builds

          WindowsInstallMediaImgBuildJob('windows-install-media'),
        ] +

        // Publish jobs

        // Windows client has 2 jobs to account for skipping of prod environment. This avoids needing to
        // rewrite the rest of the passed logic. TODO: Mod logic such that only 1 ImgPublishJob is needed

        [
          ImgPublishJob(image, env, 'windows', 'windows-uefi')
          for image in windows_client_images
          for env in ['testing', 'staging']
        ] +
        [
          ImgPublishJob(image, 'internal', 'windows', 'windows-uefi') {passed:'publish-to-staging-' + image}
          for image in windows_client_images
        ] +
        [
          ImgPublishJob(image, env, 'windows', 'windows-uefi')
          for image in windows_server_images
          for env in server_envs
        ] +
        [
          ImgPublishJob(image, env, 'sqlserver', 'sqlserver-uefi')
          for image in sql_images
          for env in sql_envs
        ] +
        //Publish job for SQL Preview build. Will be rolled into sql_images on formal release.
        [
          ImgPublishJob(image, env, 'sqlserver', 'sqlserver-uefi')
          for image in prerelease_images
          for env in prerelease_envs
        ] +
        [
          ImgPublishJob(image, env, 'windows_container', 'windows-uefi')
          for image in container_images
          for env in server_envs
        ] +
        [
          MediaImgPublishJob(image, env, 'windows', 'windows-install-media')
          for image in windows_install_media_images
          for env in windows_install_media_envs
        ],

  groups: [
    ImgGroup('windows-81', windows_81_images, client_envs),
    ImgGroup('windows-10', windows_10_images, client_envs),
    ImgGroup('windows-11', windows_11_images, client_envs),
    ImgGroup('windows-2012', windows_2012_images, server_envs),
    ImgGroup('windows-2016', windows_2016_images, server_envs),
    ImgGroup('windows-2019', windows_2019_images, server_envs),
    ImgGroup('windows-2022', windows_2022_images, server_envs),
    ImgGroup('windows-20h2', windows_20h2_images, server_envs),
    ImgGroup('sql-2014', sql_2014_images, sql_envs),
    ImgGroup('sql-2016', sql_2016_images, sql_envs),
    ImgGroup('sql-2017', sql_2017_images, sql_envs),
    ImgGroup('sql-2019', sql_2019_images, sql_envs),
    ImgGroup('container-2019', container_images, server_envs),
    ImgGroup('windows-install-media', windows_install_media_images, windows_install_media_envs),
    ImgGroup('pre-release', prerelease_images, prerelease_envs),
  ],
}
