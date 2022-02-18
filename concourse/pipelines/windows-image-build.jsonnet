// Imports.
local common = import '../templates/common.libsonnet';
local daisy = import '../templates/daisy.libsonnet';


// Templates.
local imgbuildjob = {
  local job = self,

  image:: error 'must set image in whatever',
  workflow:: error 'must set workflow in whatever',
  iso_secret:: error 'must set iso_secret in whatever',
  updates_secret:: error 'must set updates_secret in whatever',

  // Start of job.
  name: 'build-' + job.image,
  on_failure: {
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    task: 'failure',
    vars: {
      job: job.name,
      pipeline: 'windows-image-build',
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_success: {
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    task: 'success',
    vars: {
      job: job.name,
      pipeline: 'windows-image-build',
      result_state: 'success',
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
      task: 'get-credential',
      file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
    },
    {
      task: 'get-secret-iso',
      file: 'guest-test-infra/concourse/tasks/gcloud-get-secret.yaml',
      vars: { secret_name: job.iso_secret },
    },
    {
      load_var: 'windows-iso',
      file: 'gcp-secret-manager/' + job.iso_secret,
    },
    {
      task: 'get-secret-updates',
      file: 'guest-test-infra/concourse/tasks/gcloud-get-secret.yaml',
      vars: { secret_name: job.updates_secret },
    },
    {
      load_var: 'windows-updates',
      file: 'gcp-secret-manager/' + job.updates_secret,
    },
    {
      file: 'guest-test-infra/concourse/tasks/gcloud-get-secret.yaml',
      task: 'get-secret-pwsh',
      vars: { secret_name: 'windows_gcs_pwsh' },
    },
    // We download and then load each of the 3 following secrets; they could be merged.
    {
      file: 'gcp-secret-manager/windows_gcs_pwsh',
      load_var: 'windows-gcs-pwsh',
    },
    {
      file: 'guest-test-infra/concourse/tasks/gcloud-get-secret.yaml',
      task: 'get-secret-cloud-sdk',
      vars: { secret_name: 'windows_gcs_cloud_sdk' },
    },
    {
      file: 'gcp-secret-manager/windows_gcs_cloud_sdk',
      load_var: 'windows-cloud-sdk',
    },
    {
      file: 'guest-test-infra/concourse/tasks/gcloud-get-secret.yaml',
      task: 'get-secret-dotnet48',
      vars: { secret_name: 'windows_gcs_dotnet48' },
    },
    {
      file: 'gcp-secret-manager/windows_gcs_dotnet48',
      load_var: 'windows-gcs-dotnet48',
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

  image:: error 'must set image in sqlbuildjob',
  base_image:: error 'must set base_image in sqlbuildjob',
  passed:: error 'must set passed in sqlbuildjob',
  workflow:: error 'must set workflow in sqlbuildjob',
  media_secret:: error 'must set media_secret in sqlbuildjob',

  // Start of job.
  name: 'build-' + job.image,
  on_failure: {
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    task: 'failure',
    vars: {
      job: job.name,
      pipeline: 'windows-image-build',
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_success: {
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    task: 'success',
    vars: {
      job: job.name,
      pipeline: 'windows-image-build',
      result_state: 'success',
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
      passed: [job.passed],
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
      file: 'guest-test-infra/concourse/tasks/gcloud-get-secret.yaml',
      vars: { secret_name: job.media_secret },
    },
    {
      load_var: 'sql-server-media',
      file: 'gcp-secret-manager/' + job.media_secret,
    },
    {
      task: 'get-secret-windows-gcs-ssms-exe',
      file: 'guest-test-infra/concourse/tasks/gcloud-get-secret.yaml',
      vars: { secret_name: 'windows_gcs_ssms_exe' },
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

  image:: error 'must set image in coreimgbuild',
  base_image:: error 'must set base_image in coreimgbuild',
  passed:: error 'must set passed in coreimgbuild',
  workflow:: error 'must set workflow in corebuildjob',

  // Start of job.
  name: 'build-' + job.image,
  on_failure: {
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    task: 'failure',
    vars: {
      job: job.name,
      pipeline: 'windows-image-build',
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_success: {
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    task: 'success',
    vars: {
      job: job.name,
      pipeline: 'windows-image-build',
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  plan: [
    {
      get: job.base_image,
      params: { skip_download: 'true' },
      passed: [job.passed],
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

local ImgBuildJob(image, workflow, iso_secret, updates_secret) = imgbuildjob {
  image: image,
  workflow: workflow,
  iso_secret: iso_secret,
  updates_secret: updates_secret,
};

local SQLImgBuildJob(image, base_image, workflow, passed, media_secret) = sqlimgbuildjob {
  image: image,
  base_image: base_image,
  workflow: workflow,
  passed: passed,
  media_secret: media_secret,
};

local ContainerImgBuildJob(image, base_image, workflow, passed) = containerimgbuildjob {
  image: image,
  base_image: base_image,
  workflow: workflow,
  passed: passed,
};

// Start of output.
{
  resource_types: [
    {
      name: 'gcs',
      source: {
        repository: 'frodenas/gcs-resource',
      },
      type: 'registry-image',
    },
  ],
  resources: [
    common.GitResource('compute-image-tools'),
    common.GitResource('guest-test-infra'),
    common.GcsImgResource('windows-server-20h2-dc-core', 'windows-uefi/'),
    common.GcsImgResource('windows-server-2004-dc-core', 'windows-uefi/'),
    common.GcsImgResource('windows-server-2022-dc', 'windows-uefi/'),
    common.GcsImgResource('windows-server-2022-dc-core', 'windows-uefi/'),
    common.GcsImgResource('windows-server-2019-dc', 'windows-uefi/'),
    common.GcsImgResource('windows-server-2019-dc-core', 'windows-uefi/'),
    common.GcsImgResource('windows-server-2016-dc', 'windows-uefi/'),
    common.GcsImgResource('windows-server-2016-dc-core', 'windows-uefi/'),
    common.GcsImgResource('windows-server-2012-r2-dc', 'windows-uefi/'),
    common.GcsImgResource('windows-server-2012-r2-dc-core', 'windows-uefi/'),
    common.GcsImgResource('sql-2012-enterprise-windows-2012-r2-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2012-standard-windows-2012-r2-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2012-web-windows-2012-r2-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2014-enterprise-windows-2012-r2-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2014-enterprise-windows-2016-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2014-standard-windows-2012-r2-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2014-web-windows-2012-r2-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2016-enterprise-windows-2012-r2-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2016-enterprise-windows-2016-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2016-enterprise-windows-2019-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2016-standard-windows-2012-r2-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2016-standard-windows-2016-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2016-standard-windows-2019-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2016-web-windows-2012-r2-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2016-web-windows-2016-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2016-web-windows-2019-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2017-enterprise-windows-2016-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2017-enterprise-windows-2019-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2017-express-windows-2012-r2-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2017-express-windows-2016-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2017-express-windows-2019-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2017-standard-windows-2016-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2017-standard-windows-2019-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2017-web-windows-2016-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2017-web-windows-2019-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2019-enterprise-windows-2019-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2019-standard-windows-2019-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2019-web-windows-2019-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2017-enterprise-windows-2022-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2017-standard-windows-2022-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2017-web-windows-2022-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2019-enterprise-windows-2022-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2019-standard-windows-2022-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('sql-2019-web-windows-2022-dc', 'sqlserver-uefi/'),
    common.GcsImgResource('windows-server-2019-dc-core-for-containers', 'windows_uefi/'),
    common.GcsImgResource('windows-server-2019-dc-for-containers', 'windows_uefi/'),
  ],
  jobs: [
    ImgBuildJob('windows-server-2022-dc',
                'windows/windows-server-2022-dc-uefi.wf.json',
                'win2022-64',
                'windows_gcs_updates_server2022'),
    ImgBuildJob('windows-server-2022-dc-core',
                'windows/windows-server-2022-dc-core-uefi.wf.json',
                'win2022-64',
                'windows_gcs_updates_server2022'),
    ImgBuildJob('windows-server-20h2-dc-core',
                'windows/windows-server-20h2-dc-core-uefi.wf.json',
                'winserver-20h2-64',
                'windows_gcs_updates_sac20h2'),
    ImgBuildJob('windows-server-2004-dc-core',
                'windows/windows-server-2004-dc-core-uefi.wf.json',
                'winserver-2004-64',
                'windows_gcs_updates_sac2004'),
    ImgBuildJob('windows-server-2019-dc',
                'windows/windows-server-2019-dc-uefi.wf.json',
                'win2019-64',
                'windows_gcs_updates_server2019'),
    ImgBuildJob('windows-server-2019-dc-core',
                'windows/windows-server-2019-dc-core-uefi.wf.json',
                'win2019-64',
                'windows_gcs_updates_server2019'),
    ImgBuildJob('windows-server-2016-dc',
                'windows/windows-server-2016-dc-uefi.wf.json',
                'win2016-64',
                'windows_gcs_updates_server2016'),
    ImgBuildJob('windows-server-2016-dc-core',
                'windows/windows-server-2016-dc-core-uefi.wf.json',
                'win2016-64',
                'windows_gcs_updates_server2016'),
    ImgBuildJob('windows-server-2012-r2-dc',
                'windows/windows-server-2012r2-dc-uefi.wf.json',
                'win2012-r2-64',
                'windows_gcs_updates_server2012r2'),
    ImgBuildJob('windows-server-2012-r2-dc-core',
                'windows/windows-server-2012r2-dc-core-uefi.wf.json',
                'win2012-r2-64',
                'windows_gcs_updates_server2012r2'),
    SQLImgBuildJob('sql-2012-enterprise-windows-2012-r2-dc',
                   'windows-server-2012-r2-dc',
                   'sqlserver/sql-2012-enterprise-windows-2012-r2-dc.wf.json',
                   'publish-to-testing-2012r2',
                   'sql-2012-enterprise'),
    SQLImgBuildJob('sql-2012-enterprise-windows-2012-r2-dc',
                   'windows-2012r2-gcs',
                   'sqlserver/sql-2012-enterprise-windows-2012-r2-dc.wf.json',
                   'publish-to-testing-2012r2',
                   'sql-2012-enterprise'),
    SQLImgBuildJob('sql-2012-standard-windows-2012-r2-dc',
                   'windows-2012r2-gcs',
                   'sqlserver/sql-2012-standard-windows-2012-r2-dc.wf.json',
                   'publish-to-testing-2012r2',
                   'sql-2012-standard'),
    SQLImgBuildJob('sql-2012-web-windows-2012-r2-dc',
                   'windows-2012r2-gcs',
                   'sqlserver/sql-2012-web-windows-2012-r2-dc.wf.json',
                   'publish-to-testing-2012r2',
                   'sql-2012-web'),
    SQLImgBuildJob('sql-2014-enterprise-windows-2012-r2-dc',
                   'windows-2012r2-gcs',
                   'sqlserver/sql-2014-enterprise-windows-2012-r2-dc.wf.json',
                   'publish-to-testing-2012r2',
                   'sql-2014-enterprise'),
    SQLImgBuildJob('sql-2014-enterprise-windows-2016-dc',
                   'windows-2016-gcs',
                   'sqlserver/sql-2014-enterprise-windows-2016-dc.wf.json',
                   'publish-to-testing-2016',
                   'sql-2014-enterprise'),
    SQLImgBuildJob('sql-2014-standard-windows-2012-r2-dc',
                   'windows-2012r2-gcs',
                   'sqlserver/sql-2014-standard-windows-2012-r2-dc.wf.json',
                   'publish-to-testing-2012r2',
                   'sql-2014-standard'),
    SQLImgBuildJob('sql-2014-web-windows-2012-r2-dc',
                   'windows-2012r2-gcs',
                   'sqlserver/sql-2014-web-windows-2012-r2-dc.wf.json',
                   'publish-to-testing-2012r2',
                   'sql-2014-web'),
    SQLImgBuildJob('sql-2016-enterprise-windows-2012-r2-dc',
                   'windows-2012r2-gcs',
                   'sqlserver/sql-2016-enterprise-windows-2012-r2-dc.wf.json',
                   'publish-to-testing-2012r2',
                   'sql-2016-enterprise'),
    SQLImgBuildJob('sql-2016-enterprise-windows-2016-dc',
                   'windows-2016-gcs',
                   'sqlserver/sql-2016-enterprise-windows-2016-dc.wf.json',
                   'publish-to-testing-2016',
                   'sql-2016-enterprise'),
    SQLImgBuildJob('sql-2016-enterprise-windows-2019-dc',
                   'windows-2019-gcs',
                   'sqlserver/sql-2016-enterprise-windows-2019-dc.wf.json',
                   'publish-to-testing-2019',
                   'sql-2016-enterprise'),
    SQLImgBuildJob('sql-2016-standard-windows-2012-r2-dc',
                   'windows-2012r2-gcs',
                   'sqlserver/sql-2016-standard-windows-2012-r2-dc.wf.json',
                   'publish-to-testing-2012r2',
                   'sql-2016-standard'),
    SQLImgBuildJob('sql-2016-standard-windows-2016-dc',
                   'windows-2016-gcs',
                   'sqlserver/sql-2016-standard-windows-2016-dc.wf.json',
                   'publish-to-testing-2016',
                   'sql-2016-standard'),
    SQLImgBuildJob('sql-2016-standard-windows-2019-dc',
                   'windows-2019-gcs',
                   'sqlserver/sql-2016-standard-windows-2019-dc.wf.json',
                   'publish-to-testing-2019',
                   'sql-2016-standard'),
    SQLImgBuildJob('sql-2016-web-windows-2012-r2-dc',
                   'windows-2012r2-gcs',
                   'sqlserver/sql-2016-web-windows-2012-r2-dc.wf.json',
                   'publish-to-testing-2012r2',
                   'sql-2016-web'),
    SQLImgBuildJob('sql-2016-web-windows-2016-dc',
                   'windows-2016-gcs',
                   'sqlserver/sql-2016-web-windows-2016-dc.wf.json',
                   'publish-to-testing-2016',
                   'sql-2016-web'),
    SQLImgBuildJob('sql-2016-web-windows-2019-dc',
                   'windows-2019-gcs',
                   'sqlserver/sql-2016-web-windows-2019-dc.wf.json',
                   'publish-to-testing-2019',
                   'sql-2016-web'),
    SQLImgBuildJob('sql-2017-enterprise-windows-2016-dc',
                   'windows-2016-gcs',
                   'sqlserver/sql-2017-enterprise-windows-2016-dc.wf.json',
                   'publish-to-testing-2016',
                   'sql-2017-enterprise'),
    SQLImgBuildJob('sql-2017-enterprise-windows-2019-dc',
                   'windows-2019-gcs',
                   'sqlserver/sql-2017-enterprise-windows-2019-dc.wf.json',
                   'publish-to-testing-2019',
                   'sql-2017-enterprise'),
    SQLImgBuildJob('sql-2017-express-windows-2012-r2-dc',
                   'windows-2012r2-gcs',
                   'sqlserver/sql-2017-express-windows-2012-r2-dc.wf.json',
                   'publish-to-testing-2012r2',
                   'sql-2017-express'),
    SQLImgBuildJob('sql-2017-express-windows-2016-dc',
                   'windows-2016-gcs',
                   'sqlserver/sql-2017-express-windows-2016-dc.wf.json',
                   'publish-to-testing-2016',
                   'sql-2017-express'),
    SQLImgBuildJob('sql-2017-express-windows-2019-dc',
                   'windows-2019-gcs',
                   'sqlserver/sql-2017-express-windows-2019-dc.wf.json',
                   'publish-to-testing-2019',
                   'sql-2017-express'),
    SQLImgBuildJob('sql-2017-standard-windows-2016-dc',
                   'windows-2016-gcs',
                   'sqlserver/sql-2017-standard-windows-2016-dc.wf.json',
                   'publish-to-testing-2016',
                   'sql-2017-standard'),
    SQLImgBuildJob('sql-2017-standard-windows-2019-dc',
                   'windows-2019-gcs',
                   'sqlserver/sql-2017-standard-windows-2019-dc.wf.json',
                   'publish-to-testing-2019',
                   'sql-2017-standard'),
    SQLImgBuildJob('sql-2017-web-windows-2016-dc',
                   'windows-2016-gcs',
                   'sqlserver/sql-2017-web-windows-2016-dc.wf.json',
                   'publish-to-testing-2016',
                   'sql-2017-web'),
    SQLImgBuildJob('sql-2017-web-windows-2019-dc',
                   'windows-2019-gcs',
                   'sqlserver/sql-2017-web-windows-2019-dc.wf.json',
                   'publish-to-testing-2019',
                   'sql-2017-web'),
    SQLImgBuildJob('sql-2019-enterprise-windows-2019-dc',
                   'windows-2019-gcs',
                   'sqlserver/sql-2019-enterprise-windows-2019-dc.wf.json',
                   'publish-to-testing-2019',
                   'sql-2019-enterprise'),
    SQLImgBuildJob('sql-2019-standard-windows-2019-dc',
                   'windows-2019-gcs',
                   'sqlserver/sql-2019-standard-windows-2019-dc.wf.json',
                   'publish-to-testing-2019',
                   'sql-2019-standard'),
    SQLImgBuildJob('sql-2019-web-windows-2019-dc',
                   'windows-2019-gcs',
                   'sqlserver/sql-2019-web-windows-2019-dc.wf.json',
                   'publish-to-testing-2019',
                   'sql-2019-web'),
    SQLImgBuildJob('sql-2019-enterprise-windows-2022-dc',
                   'windows-2022-gcs',
                   'sqlserver/sql-2019-enterprise-windows-2022-dc.wf.json',
                   'publish-to-testing-2022',
                   'sql-2019-enterprise'),
    SQLImgBuildJob('sql-2019-standard-windows-2022-dc',
                   'windows-2022-gcs',
                   'sqlserver/sql-2019-standard-windows-2022-dc.wf.json',
                   'publish-to-testing-2022',
                   'sql-2019-standard'),
    SQLImgBuildJob('sql-2019-web-windows-2022-dc',
                   'windows-2022-gcs',
                   'sqlserver/sql-2019-web-windows-2022-dc.wf.json',
                   'publish-to-testing-2022',
                   'sql-2019-web'),
    SQLImgBuildJob('sql-2017-enterprise-windows-2022-dc',
                   'windows-2022-gcs',
                   'sqlserver/sql-2017-enterprise-windows-2022-dc.wf.json',
                   'publish-to-testing-2022',
                   'sql-2017-enterprise'),
    SQLImgBuildJob('sql-2017-standard-windows-2022-dc',
                   'windows-2022-gcs',
                   'sqlserver/sql-2017-standard-windows-2022-dc.wf.json',
                   'publish-to-testing-2022',
                   'sql-2017-standard'),
    SQLImgBuildJob('sql-2017-web-windows-2022-dc',
                   'windows-2022-gcs',
                   'sqlserver/sql-2017-web-windows-2022-dc.wf.json',
                   'publish-to-testing-2022',
                   'sql-2017-web'),
    ContainerImgBuildJob('windows-server-2019-dc-core-for-containers',
                         'windows-2019-core-gcs',
                         'windows_container/windows-2019-core-for-containers-uefi.wf.json',
                         'publish-to-testing-2019-core'),
    ContainerImgBuildJob('windows-server-2019-dc-for-containers',
                         'windows-2019-gcs',
                         'windows_container/windows-2019-for-containers-uefi.wf.json',
                         'publish-to-testing-2019'),
    {
      name: 'publish-to-testing-2022',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'windows-2022-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-windows-2022',
          ],
          trigger: true,
        },
        {
          file: 'windows-2022-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2022',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2022-dc-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-2022-core',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'windows-2022-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-windows-2022-core',
          ],
          trigger: true,
        },
        {
          file: 'windows-2022-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2022-core',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2022-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-20h2',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-testing-20h2',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-testing-20h2',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-20h2-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-windows-20h2-core',
          ],
          trigger: true,
        },
        {
          file: 'windows-20h2-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-20h2-core',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-20h2-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-2004',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-testing-2004',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-testing-2004',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2004-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-windows-2004-core',
          ],
          trigger: true,
        },
        {
          file: 'windows-2004-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2004-core',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2004-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-2019',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-testing-2019',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-testing-2019',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2019-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-windows-2019',
          ],
          trigger: true,
        },
        {
          file: 'windows-2019-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2019',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2019-dc-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-2019-core',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-testing-2019-core',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-testing-2019-core',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2019-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-windows-2019-core',
          ],
          trigger: true,
        },
        {
          file: 'windows-2019-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2019-core',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2019-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-2016',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-testing-2016',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-testing-2016',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2016-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-windows-2016',
          ],
          trigger: true,
        },
        {
          file: 'windows-2016-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2016',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2016-dc-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-2016-core',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-testing-2016-core',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-testing-2016-core',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2016-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-windows-2016-core',
          ],
          trigger: true,
        },
        {
          file: 'windows-2016-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2016-core',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2016-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-2012r2',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-testing-2012r2',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-testing-2012r2',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2012r2-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-windows-2012r2',
          ],
          trigger: true,
        },
        {
          file: 'windows-2012r2-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2012r2',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2012r2-dc-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-2012r2-core',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-testing-2012r2-core',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-testing-2012r2-core',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2012r2-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-windows-2012r2-core',
          ],
          trigger: true,
        },
        {
          file: 'windows-2012r2-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2012r2-core',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2012r2-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2012-enterprise-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2012-enterprise-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2012-enterprise-windows-2012-r2-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2012-enterprise-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2012-enterprise-windows-2012-r2-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2012-enterprise-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2012-standard-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2012-standard-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2012-standard-windows-2012-r2-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2012-standard-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2012-standard-windows-2012-r2-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2012-standard-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2012-web-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2012-web-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2012-web-windows-2012-r2-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2012-web-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2012-web-windows-2012-r2-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2012-web-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2014-enterprise-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2014-enterprise-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2014-enterprise-windows-2012-r2-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2014-enterprise-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2014-enterprise-windows-2012-r2-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2014-enterprise-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2014-enterprise-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2014-enterprise-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2014-enterprise-windows-2016-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2014-enterprise-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2014-enterprise-windows-2016-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2014-enterprise-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2014-standard-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2014-standard-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2014-standard-windows-2012-r2-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2014-standard-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2014-standard-windows-2012-r2-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2014-standard-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2014-web-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2014-web-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2014-web-windows-2012-r2-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2014-web-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2014-web-windows-2012-r2-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2014-web-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2016-enterprise-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-enterprise-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2016-enterprise-windows-2012-r2-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2016-enterprise-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-enterprise-windows-2012-r2-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-enterprise-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2016-enterprise-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-enterprise-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2016-enterprise-windows-2016-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2016-enterprise-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-enterprise-windows-2016-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-enterprise-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2016-enterprise-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-enterprise-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2016-enterprise-windows-2019-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2016-enterprise-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-enterprise-windows-2019-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-enterprise-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2016-standard-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-standard-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2016-standard-windows-2012-r2-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2016-standard-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-standard-windows-2012-r2-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-standard-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2016-standard-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-standard-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2016-standard-windows-2016-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2016-standard-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-standard-windows-2016-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-standard-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2016-standard-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-standard-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2016-standard-windows-2019-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2016-standard-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-standard-windows-2019-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-standard-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2016-web-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-web-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2016-web-windows-2012-r2-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2016-web-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-web-windows-2012-r2-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-web-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2016-web-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-web-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2016-web-windows-2016-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2016-web-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-web-windows-2016-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-web-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2016-web-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-web-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2016-web-windows-2019-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2016-web-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-web-windows-2019-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-web-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2017-enterprise-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-enterprise-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2017-enterprise-windows-2016-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2017-enterprise-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-enterprise-windows-2016-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-enterprise-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2017-enterprise-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-enterprise-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2017-enterprise-windows-2019-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2017-enterprise-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-enterprise-windows-2019-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-enterprise-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2017-express-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-express-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2017-express-windows-2012-r2-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2017-express-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-express-windows-2012-r2-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-express-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2017-express-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-express-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2017-express-windows-2016-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2017-express-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-express-windows-2016-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-express-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2017-express-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-express-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2017-express-windows-2019-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2017-express-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-express-windows-2019-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-express-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2017-standard-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-standard-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2017-standard-windows-2016-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2017-standard-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-standard-windows-2016-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-standard-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2017-standard-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-standard-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2017-standard-windows-2019-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2017-standard-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-standard-windows-2019-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-standard-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2017-web-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-web-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2017-web-windows-2016-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2017-web-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-web-windows-2016-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-web-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2017-web-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-web-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2017-web-windows-2019-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2017-web-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-web-windows-2019-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-web-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2019-enterprise-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2019-enterprise-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2019-enterprise-windows-2019-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2019-enterprise-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2019-enterprise-windows-2019-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2019-enterprise-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2019-standard-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2019-standard-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2019-standard-windows-2019-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2019-standard-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2019-standard-windows-2019-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2019-standard-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2019-web-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2019-web-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2019-web-windows-2019-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2019-web-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2019-web-windows-2019-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2019-web-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2019-enterprise-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2019-enterprise-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2019-enterprise-windows-2022-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2019-enterprise-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2019-enterprise-windows-2022-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2019-enterprise-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2019-standard-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2019-standard-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2019-standard-windows-2022-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2019-standard-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2019-standard-windows-2022-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2019-standard-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2019-web-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2019-web-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2019-web-windows-2022-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2019-web-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2019-web-windows-2022-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2019-web-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2017-enterprise-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-enterprise-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2017-enterprise-windows-2022-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2017-enterprise-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-enterprise-windows-2022-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-enterprise-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2017-standard-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-standard-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2017-standard-windows-2022-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2017-standard-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-standard-windows-2022-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-standard-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-sql-2017-web-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-web-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-sql-2017-web-windows-2022-dc',
          ],
          trigger: true,
        },
        {
          file: 'sql-2017-web-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-web-windows-2022-dc',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-web-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-2019-core-for-containers',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'windows-2019-core-for-containers-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-windows-2019-core-for-containers',
          ],
          trigger: true,
        },
        {
          file: 'windows-2019-core-for-containers-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-2019-core-for-containers',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows_uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows_container/windows-2019-core-for-containers-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-testing-2019-for-containers',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'windows-2019-for-containers-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'build-windows-2019-for-containers',
          ],
          trigger: true,
        },
        {
          file: 'windows-2019-for-containers-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-2019-for-containers',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows_uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows_container/windows-2019-for-containers-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-2022',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'windows-2022-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-2022',
          ],
          trigger: false,
        },
        {
          file: 'windows-2022-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2022',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2022-dc-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-2022-core',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'windows-2022-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-2022-core',
          ],
          trigger: false,
        },
        {
          file: 'windows-2022-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2022-core',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2022-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-20h2',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-staging-20h2',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-staging-20h2',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-20h2-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-20h2',
          ],
          trigger: false,
        },
        {
          file: 'windows-20h2-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-20h2-core',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-20h2-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-2004',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-staging-2004',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-staging-2004',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2004-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-2004',
          ],
          trigger: false,
        },
        {
          file: 'windows-2004-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2004-core',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2004-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-2019',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-staging-2019',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-staging-2019',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2019-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-2019',
          ],
          trigger: false,
        },
        {
          file: 'windows-2019-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2019',
          vars: {
            environment: 'test',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2019-dc-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-2019-core',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-staging-2019-core',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-staging-2019-core',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2019-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-2019-core',
          ],
          trigger: false,
        },
        {
          file: 'windows-2019-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2019-core',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2019-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-2016',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-staging-2016',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-staging-2016',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2016-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-2016',
          ],
          trigger: false,
        },
        {
          file: 'windows-2016-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2016',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2016-dc-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-2016-core',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-staging-2016-core',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-staging-2016-core',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2016-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-2016-core',
          ],
          trigger: false,
        },
        {
          file: 'windows-2016-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2016-core',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2016-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-2012r2',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-staging-2012r2',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-staging-2012r2',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2012r2-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-2012r2',
          ],
          trigger: false,
        },
        {
          file: 'windows-2012r2-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2012r2',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2012r2-dc-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-2012r2-core',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-staging-2012r2-core',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-staging-2012r2-core',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2012r2-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-2012r2-core',
          ],
          trigger: false,
        },
        {
          file: 'windows-2012r2-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-windows-2012r2-core',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows/windows-server-2012r2-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2012-enterprise-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2012-enterprise-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2012-enterprise-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2012-enterprise-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2012-enterprise-windows-2012-r2-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2012-enterprise-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2012-standard-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2012-standard-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2012-standard-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2012-standard-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2012-standard-windows-2012-r2-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2012-standard-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2012-web-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2012-web-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2012-web-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2012-web-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2012-web-windows-2012-r2-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2012-web-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2014-enterprise-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2014-enterprise-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2014-enterprise-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2014-enterprise-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2014-enterprise-windows-2012-r2-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2014-enterprise-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2014-enterprise-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2014-enterprise-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2014-enterprise-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2014-enterprise-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2014-enterprise-windows-2016-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2014-enterprise-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2014-standard-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2014-standard-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2014-standard-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2014-standard-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2014-standard-windows-2012-r2-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2014-standard-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2014-web-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2014-web-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2014-web-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2014-web-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2014-web-windows-2012-r2-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2014-web-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2016-enterprise-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-enterprise-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2016-enterprise-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-enterprise-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-enterprise-windows-2012-r2-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-enterprise-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2016-enterprise-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-enterprise-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2016-enterprise-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-enterprise-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-enterprise-windows-2016-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-enterprise-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2016-enterprise-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-enterprise-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2016-enterprise-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-enterprise-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-enterprise-windows-2019-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-enterprise-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2016-standard-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-standard-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2016-standard-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-standard-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-standard-windows-2012-r2-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-standard-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2016-standard-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-standard-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2016-standard-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-standard-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-standard-windows-2016-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-standard-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2016-standard-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-standard-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2016-standard-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-standard-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-standard-windows-2019-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-standard-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2016-web-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-web-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2016-web-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-web-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-web-windows-2012-r2-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-web-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2016-web-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-web-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2016-web-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-web-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-web-windows-2016-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-web-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2016-web-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2016-web-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2016-web-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-web-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2016-web-windows-2019-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2016-web-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2017-enterprise-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-enterprise-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2017-enterprise-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-enterprise-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-enterprise-windows-2016-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-enterprise-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2017-enterprise-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-enterprise-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2017-enterprise-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-enterprise-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-enterprise-windows-2019-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-enterprise-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2017-express-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-express-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2017-express-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-express-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-express-windows-2012-r2-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-express-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2017-express-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-express-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2017-express-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-express-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-express-windows-2016-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-express-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2017-express-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-express-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2017-express-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-express-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-express-windows-2019-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-express-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2017-standard-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-standard-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2017-standard-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-standard-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-standard-windows-2016-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-standard-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2017-standard-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-standard-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2017-standard-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-standard-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-standard-windows-2019-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-standard-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2017-web-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-web-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2017-web-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-web-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-web-windows-2016-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-web-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2017-web-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-web-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2017-web-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-web-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-web-windows-2019-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-web-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2019-enterprise-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2019-enterprise-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2019-enterprise-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2019-enterprise-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2019-enterprise-windows-2019-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2019-enterprise-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2019-standard-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2019-standard-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2019-standard-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2019-standard-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2019-standard-windows-2019-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2019-standard-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2019-web-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2019-web-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2019-web-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2019-web-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2019-web-windows-2019-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2019-web-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2019-enterprise-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2019-enterprise-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2019-enterprise-windows-2022-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2019-enterprise-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2019-enterprise-windows-2022-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2019-enterprise-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2019-standard-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2019-standard-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2019-standard-windows-2022-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2019-standard-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2019-standard-windows-2022-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2019-standard-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2019-web-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2019-web-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2019-web-windows-2022-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2019-web-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2019-web-windows-2022-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2019-web-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2017-enterprise-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-enterprise-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2017-enterprise-windows-2022-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-enterprise-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-enterprise-windows-2022-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-enterprise-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2017-standard-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-standard-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2017-standard-windows-2022-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-standard-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-standard-windows-2022-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-standard-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-sql-2017-web-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'sql-2017-web-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-sql-2017-web-windows-2022-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-web-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-sql-2017-web-windows-2022-dc',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            source_version: 'v((.:source-version))',
            wf: 'sqlserver/sql-2017-web-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-2019-core-for-containers',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'windows-2019-core-for-containers-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-2019-core-for-containers',
          ],
          trigger: false,
        },
        {
          file: 'windows-2019-core-for-containers-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-2019-core-for-containers',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows_uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows_container/windows-2019-core-for-containers-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-staging-2019-for-containers',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          get: 'windows-2019-for-containers-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-testing-2019-for-containers',
          ],
          trigger: false,
        },
        {
          file: 'windows-2019-for-containers-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
          task: 'publish-2019-for-containers',
          vars: {
            environment: 'staging',
            publish_version: '((.:publish-version))',
            source_gcs_path: 'gs://artifact-releaser-prod-rtp/windows_uefi',
            source_version: 'v((.:source-version))',
            wf: 'windows_container/windows-2019-for-containers-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-2022',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'windows-2022-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-2022',
          ],
          trigger: false,
        },
        {
          file: 'windows-2022-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-windows-2022',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            image_name: 'windows-2022',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'windows/windows-server-2022-dc-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-2022-core',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'windows-2022-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-2022-core',
          ],
          trigger: false,
        },
        {
          file: 'windows-2022-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-windows-2022-core',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            image_name: 'windows-2022-core',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'windows/windows-server-2022-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-20h2',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-prod-20h2',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-prod-20h2',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-20h2-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-20h2',
          ],
          trigger: false,
        },
        {
          file: 'windows-20h2-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-windows-20h2-core',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            image_name: 'windows-20h2-core',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'windows/windows-server-20h2-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-2004',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-prod-2004',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-prod-2004',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2004-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-2004',
          ],
          trigger: false,
        },
        {
          file: 'windows-2004-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-windows-2004-core',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            image_name: 'windows-2004-core',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'windows/windows-server-2004-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-2019',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-prod-2019',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-prod-2019',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2019-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-2019',
          ],
          trigger: false,
        },
        {
          file: 'windows-2019-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-windows-2019',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            image_name: 'windows-2019',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'windows/windows-server-2019-dc-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-2019-core',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-prod-2019-core',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-prod-2019-core',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2019-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-2019-core',
          ],
          trigger: false,
        },
        {
          file: 'windows-2019-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-windows-2019-core',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            image_name: 'windows-2019-core',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'windows/windows-server-2019-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-2016',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-prod-2016',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-prod-2016',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2016-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-2016',
          ],
          trigger: false,
        },
        {
          file: 'windows-2016-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-windows-2016',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            image_name: 'windows-2016',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'windows/windows-server-2016-dc-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-2016-core',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-prod-2016-core',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-prod-2016-core',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2016-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-2016-core',
          ],
          trigger: false,
        },
        {
          file: 'windows-2016-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-windows-2016-core',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            image_name: 'windows-2016-core',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'windows/windows-server-2016-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-2012r2',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-prod-2012r2',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-prod-2012r2',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2012r2-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-2012r2',
          ],
          trigger: false,
        },
        {
          file: 'windows-2012r2-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-windows-2012r2',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            image_name: 'windows-2012r2',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'windows/windows-server-2012r2-dc-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-2012r2-core',
      on_failure: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'failure',
        vars: {
          job: 'publish-to-prod-2012r2-core',
          pipeline: 'windows-image-build',
          result_state: 'failure',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      on_success: {
        file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
        task: 'success',
        vars: {
          job: 'publish-to-prod-2012r2-core',
          pipeline: 'windows-image-build',
          result_state: 'success',
          start_timestamp: '((.:start-timestamp-ms))',
        },
      },
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          task: 'generate-timestamp',
        },
        {
          file: 'timestamp/timestamp-ms',
          load_var: 'start-timestamp-ms',
        },
        {
          get: 'windows-2012r2-core-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-2012r2-core',
          ],
          trigger: false,
        },
        {
          file: 'windows-2012r2-core-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-windows-2012r2-core',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/windows-uefi',
            image_name: 'windows-2012r2-core',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'windows/windows-server-2012r2-dc-core-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2012-enterprise-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2012-enterprise-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2012-enterprise-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2012-enterprise-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2012-enterprise-windows-2012-r2-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2012-enterprise-windows-2012-r2-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2012-enterprise-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2012-standard-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2012-standard-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2012-standard-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2012-standard-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2012-standard-windows-2012-r2-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2012-standard-windows-2012-r2-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2012-standard-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2012-web-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2012-web-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2012-web-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2012-web-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2012-web-windows-2012-r2-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2012-web-windows-2012-r2-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2012-web-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2014-enterprise-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2014-enterprise-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2014-enterprise-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2014-enterprise-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2014-enterprise-windows-2012-r2-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2014-enterprise-windows-2012-r2-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2014-enterprise-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2014-enterprise-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2014-enterprise-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2014-enterprise-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2014-enterprise-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2014-enterprise-windows-2016-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2014-enterprise-windows-2016-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2014-enterprise-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2014-standard-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2014-standard-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2014-standard-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2014-standard-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2014-standard-windows-2012-r2-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2014-standard-windows-2012-r2-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2014-standard-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2014-web-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2014-web-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2014-web-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2014-web-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2014-web-windows-2012-r2-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2014-web-windows-2012-r2-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2014-web-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2016-enterprise-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2016-enterprise-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2016-enterprise-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-enterprise-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2016-enterprise-windows-2012-r2-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2016-enterprise-windows-2012-r2-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2016-enterprise-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2016-enterprise-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2016-enterprise-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2016-enterprise-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-enterprise-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2016-enterprise-windows-2016-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2016-enterprise-windows-2016-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2016-enterprise-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2016-enterprise-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2016-enterprise-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2016-enterprise-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-enterprise-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2016-enterprise-windows-2019-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2016-enterprise-windows-2019-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2016-enterprise-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2016-standard-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2016-standard-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2016-standard-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-standard-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2016-standard-windows-2012-r2-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2016-standard-windows-2012-r2-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2016-standard-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2016-standard-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2016-standard-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2016-standard-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-standard-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2016-standard-windows-2016-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2016-standard-windows-2016-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2016-standard-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2016-standard-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2016-standard-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2016-standard-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-standard-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2016-standard-windows-2019-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2016-standard-windows-2019-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2016-standard-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2016-web-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2016-web-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2016-web-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-web-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2016-web-windows-2012-r2-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2016-web-windows-2012-r2-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2016-web-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2016-web-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2016-web-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2016-web-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-web-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2016-web-windows-2016-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2016-web-windows-2016-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2016-web-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2016-web-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2016-web-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2016-web-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2016-web-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2016-web-windows-2019-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2016-web-windows-2019-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2016-web-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2017-enterprise-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2017-enterprise-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2017-enterprise-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-enterprise-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2017-enterprise-windows-2016-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2017-enterprise-windows-2016-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2017-enterprise-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2017-enterprise-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2017-enterprise-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2017-enterprise-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-enterprise-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2017-enterprise-windows-2019-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2017-enterprise-windows-2019-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2017-enterprise-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2017-express-windows-2012-r2-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2017-express-windows-2012-r2-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2017-express-windows-2012-r2-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-express-windows-2012-r2-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2017-express-windows-2012-r2-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2017-express-windows-2012-r2-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2017-express-windows-2012-r2-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2017-express-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2017-express-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2017-express-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-express-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2017-express-windows-2016-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2017-express-windows-2016-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2017-express-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2017-express-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2017-express-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2017-express-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-express-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2017-express-windows-2019-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2017-express-windows-2019-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2017-express-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2017-standard-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2017-standard-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2017-standard-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-standard-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2017-standard-windows-2016-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2017-standard-windows-2016-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2017-standard-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2017-standard-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2017-standard-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2017-standard-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-standard-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2017-standard-windows-2019-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2017-standard-windows-2019-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2017-standard-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2017-web-windows-2016-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2017-web-windows-2016-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2017-web-windows-2016-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-web-windows-2016-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2017-web-windows-2016-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2017-web-windows-2016-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2017-web-windows-2016-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2017-web-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2017-web-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2017-web-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-web-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2017-web-windows-2019-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2017-web-windows-2019-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2017-web-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2019-enterprise-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2019-enterprise-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2019-enterprise-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2019-enterprise-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2019-enterprise-windows-2019-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2019-enterprise-windows-2019-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2019-enterprise-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2019-standard-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2019-standard-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2019-standard-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2019-standard-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2019-standard-windows-2019-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2019-standard-windows-2019-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2019-standard-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2019-web-windows-2019-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2019-web-windows-2019-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2019-web-windows-2019-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2019-web-windows-2019-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2019-web-windows-2019-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2019-web-windows-2019-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2019-web-windows-2019-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2019-enterprise-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2019-enterprise-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2019-enterprise-windows-2022-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2019-enterprise-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2019-enterprise-windows-2022-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2019-enterprise-windows-2022-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2019-enterprise-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2019-standard-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2019-standard-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2019-standard-windows-2022-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2019-standard-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2019-standard-windows-2022-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2019-standard-windows-2022-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2019-standard-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2019-web-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2019-web-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2019-web-windows-2022-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2019-web-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2019-web-windows-2022-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2019-web-windows-2022-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2019-web-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2017-enterprise-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2017-enterprise-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2017-enterprise-windows-2022-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-enterprise-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2017-enterprise-windows-2022-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2017-enterprise-windows-2022-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2017-enterprise-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2017-standard-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2017-standard-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2017-standard-windows-2022-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-standard-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2017-standard-windows-2022-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2017-standard-windows-2022-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2017-standard-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-sql-2017-web-windows-2022-dc',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'sql-2017-web-windows-2022-dc-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-sql-2017-web-windows-2022-dc',
          ],
          trigger: false,
        },
        {
          file: 'sql-2017-web-windows-2022-dc-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-sql-2017-web-windows-2022-dc',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/sqlserver-uefi',
            image_name: 'sql-2017-web-windows-2022-dc',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'sqlserver/sql-2017-web-windows-2022-dc-uefi.publish',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-windows-2019-core-for-containers',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'windows-2019-core-for-containers-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-2019-core-for-containers',
          ],
          trigger: false,
        },
        {
          file: 'windows-2019-core-for-containers-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-2019-core-for-containers',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/windows_uefi',
            image_name: '2019-core-for-containers',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'windows_container/windows-2019-core-for-containers-uefi.publish.json',
          },
        },
      ],
    },
    {
      name: 'publish-to-prod-windows-2019-for-containers',
      plan: [
        {
          get: 'guest-test-infra',
        },
        {
          get: 'compute-image-tools',
        },
        {
          get: 'windows-2019-for-containers-gcs',
          params: {
            skip_download: 'true',
          },
          passed: [
            'publish-to-staging-2019-for-containers',
          ],
          trigger: false,
        },
        {
          file: 'windows-2019-for-containers-gcs/version',
          load_var: 'source-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
          task: 'get-credential',
        },
        {
          file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          task: 'generate-version',
        },
        {
          file: 'publish-version/version',
          load_var: 'publish-version',
        },
        {
          file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
          task: 'publish-2019-for-containers',
          vars: {
            gcs_image_path: 'gs://artifact-releaser-prod-rtp/windows_uefi',
            image_name: '2019-for-containers',
            publish_version: '((.:publish-version))',
            release_notes: '',
            source_version: 'v((.:source-version))',
            topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
            wf: 'windows_container/windows-2019-for-containers-uefi.publish.json',
          },
        },
      ],
    },
  ],
  groups: [
    {
      jobs: [
        'build-windows-2004-core',
        'publish-to-prod-2004',
        'publish-to-staging-2004',
        'publish-to-testing-2004',
      ],
      name: 'windows-2004',
    },
    {
      jobs: [
        'build-windows-2012r2',
        'build-windows-2012r2-core',
        'publish-to-testing-2012r2',
        'publish-to-testing-2012r2-core',
        'publish-to-staging-2012r2',
        'publish-to-staging-2012r2-core',
        'publish-to-prod-2012r2',
        'publish-to-prod-2012r2-core',
        'build-sql-2012-enterprise-windows-2012-r2-dc',
        'build-sql-2012-standard-windows-2012-r2-dc',
        'build-sql-2012-web-windows-2012-r2-dc',
        'build-sql-2014-enterprise-windows-2012-r2-dc',
        'build-sql-2014-standard-windows-2012-r2-dc',
        'build-sql-2014-web-windows-2012-r2-dc',
        'build-sql-2016-enterprise-windows-2012-r2-dc',
        'build-sql-2016-standard-windows-2012-r2-dc',
        'build-sql-2016-web-windows-2012-r2-dc',
        'build-sql-2017-express-windows-2012-r2-dc',
        'publish-to-prod-sql-2012-enterprise-windows-2012-r2-dc',
        'publish-to-prod-sql-2012-standard-windows-2012-r2-dc',
        'publish-to-prod-sql-2012-web-windows-2012-r2-dc',
        'publish-to-prod-sql-2014-enterprise-windows-2012-r2-dc',
        'publish-to-prod-sql-2014-standard-windows-2012-r2-dc',
        'publish-to-prod-sql-2014-web-windows-2012-r2-dc',
        'publish-to-prod-sql-2016-enterprise-windows-2012-r2-dc',
        'publish-to-prod-sql-2016-standard-windows-2012-r2-dc',
        'publish-to-prod-sql-2016-web-windows-2012-r2-dc',
        'publish-to-prod-sql-2017-express-windows-2012-r2-dc',
        'publish-to-staging-sql-2012-enterprise-windows-2012-r2-dc',
        'publish-to-staging-sql-2012-standard-windows-2012-r2-dc',
        'publish-to-staging-sql-2012-web-windows-2012-r2-dc',
        'publish-to-staging-sql-2014-enterprise-windows-2012-r2-dc',
        'publish-to-staging-sql-2014-standard-windows-2012-r2-dc',
        'publish-to-staging-sql-2014-web-windows-2012-r2-dc',
        'publish-to-staging-sql-2016-enterprise-windows-2012-r2-dc',
        'publish-to-staging-sql-2016-standard-windows-2012-r2-dc',
        'publish-to-staging-sql-2016-web-windows-2012-r2-dc',
        'publish-to-staging-sql-2017-express-windows-2012-r2-dc',
        'publish-to-testing-sql-2012-enterprise-windows-2012-r2-dc',
        'publish-to-testing-sql-2012-standard-windows-2012-r2-dc',
        'publish-to-testing-sql-2012-web-windows-2012-r2-dc',
        'publish-to-testing-sql-2014-enterprise-windows-2012-r2-dc',
        'publish-to-testing-sql-2014-standard-windows-2012-r2-dc',
        'publish-to-testing-sql-2014-web-windows-2012-r2-dc',
        'publish-to-testing-sql-2016-enterprise-windows-2012-r2-dc',
        'publish-to-testing-sql-2016-standard-windows-2012-r2-dc',
        'publish-to-testing-sql-2016-web-windows-2012-r2-dc',
        'publish-to-testing-sql-2017-express-windows-2012-r2-dc',
      ],
      name: 'windows-2012r2',
    },
    {
      jobs: [
        'build-windows-2016',
        'build-windows-2016-core',
        'publish-to-testing-2016',
        'publish-to-testing-2016-core',
        'publish-to-staging-2016',
        'publish-to-staging-2016-core',
        'publish-to-prod-2016',
        'publish-to-prod-2016-core',
        'build-sql-2014-enterprise-windows-2016-dc',
        'build-sql-2016-enterprise-windows-2016-dc',
        'build-sql-2016-standard-windows-2016-dc',
        'build-sql-2016-web-windows-2016-dc',
        'build-sql-2017-enterprise-windows-2016-dc',
        'build-sql-2017-express-windows-2016-dc',
        'build-sql-2017-standard-windows-2016-dc',
        'build-sql-2017-web-windows-2016-dc',
        'publish-to-testing-sql-2014-enterprise-windows-2016-dc',
        'publish-to-testing-sql-2016-enterprise-windows-2016-dc',
        'publish-to-testing-sql-2016-standard-windows-2016-dc',
        'publish-to-testing-sql-2016-web-windows-2016-dc',
        'publish-to-testing-sql-2017-enterprise-windows-2016-dc',
        'publish-to-testing-sql-2017-express-windows-2016-dc',
        'publish-to-testing-sql-2017-standard-windows-2016-dc',
        'publish-to-testing-sql-2017-web-windows-2016-dc',
        'publish-to-staging-sql-2014-enterprise-windows-2016-dc',
        'publish-to-staging-sql-2016-enterprise-windows-2016-dc',
        'publish-to-staging-sql-2016-standard-windows-2016-dc',
        'publish-to-staging-sql-2016-web-windows-2016-dc',
        'publish-to-staging-sql-2017-enterprise-windows-2016-dc',
        'publish-to-staging-sql-2017-express-windows-2016-dc',
        'publish-to-staging-sql-2017-standard-windows-2016-dc',
        'publish-to-staging-sql-2017-web-windows-2016-dc',
        'publish-to-prod-sql-2014-enterprise-windows-2016-dc',
        'publish-to-prod-sql-2016-enterprise-windows-2016-dc',
        'publish-to-prod-sql-2016-standard-windows-2016-dc',
        'publish-to-prod-sql-2016-web-windows-2016-dc',
        'publish-to-prod-sql-2017-enterprise-windows-2016-dc',
        'publish-to-prod-sql-2017-express-windows-2016-dc',
        'publish-to-prod-sql-2017-standard-windows-2016-dc',
        'publish-to-prod-sql-2017-web-windows-2016-dc',
      ],
      name: 'windows-2016',
    },
    {
      jobs: [
        'build-windows-2019',
        'build-windows-2019-core',
        'publish-to-testing-2019',
        'publish-to-testing-2019-core',
        'publish-to-staging-2019',
        'publish-to-staging-2019-core',
        'publish-to-prod-2019',
        'publish-to-prod-2019-core',
        'build-sql-2016-enterprise-windows-2019-dc',
        'build-sql-2016-standard-windows-2019-dc',
        'build-sql-2016-web-windows-2019-dc',
        'build-sql-2017-enterprise-windows-2019-dc',
        'build-sql-2017-express-windows-2019-dc',
        'build-sql-2017-standard-windows-2019-dc',
        'build-sql-2017-web-windows-2019-dc',
        'build-sql-2019-enterprise-windows-2019-dc',
        'build-sql-2019-standard-windows-2019-dc',
        'build-sql-2019-web-windows-2019-dc',
        'publish-to-testing-sql-2016-enterprise-windows-2019-dc',
        'publish-to-testing-sql-2016-standard-windows-2019-dc',
        'publish-to-testing-sql-2016-web-windows-2019-dc',
        'publish-to-testing-sql-2017-enterprise-windows-2019-dc',
        'publish-to-testing-sql-2017-express-windows-2019-dc',
        'publish-to-testing-sql-2017-standard-windows-2019-dc',
        'publish-to-testing-sql-2017-web-windows-2019-dc',
        'publish-to-testing-sql-2019-enterprise-windows-2019-dc',
        'publish-to-testing-sql-2019-standard-windows-2019-dc',
        'publish-to-testing-sql-2019-web-windows-2019-dc',
        'publish-to-staging-sql-2016-enterprise-windows-2019-dc',
        'publish-to-staging-sql-2016-standard-windows-2019-dc',
        'publish-to-staging-sql-2016-web-windows-2019-dc',
        'publish-to-staging-sql-2017-enterprise-windows-2019-dc',
        'publish-to-staging-sql-2017-express-windows-2019-dc',
        'publish-to-staging-sql-2017-standard-windows-2019-dc',
        'publish-to-staging-sql-2017-web-windows-2019-dc',
        'publish-to-staging-sql-2019-enterprise-windows-2019-dc',
        'publish-to-staging-sql-2019-standard-windows-2019-dc',
        'publish-to-staging-sql-2019-web-windows-2019-dc',
        'publish-to-prod-sql-2016-enterprise-windows-2019-dc',
        'publish-to-prod-sql-2016-standard-windows-2019-dc',
        'publish-to-prod-sql-2016-web-windows-2019-dc',
        'publish-to-prod-sql-2017-enterprise-windows-2019-dc',
        'publish-to-prod-sql-2017-express-windows-2019-dc',
        'publish-to-prod-sql-2017-standard-windows-2019-dc',
        'publish-to-prod-sql-2017-web-windows-2019-dc',
        'publish-to-prod-sql-2019-enterprise-windows-2019-dc',
        'publish-to-prod-sql-2019-standard-windows-2019-dc',
        'publish-to-prod-sql-2019-web-windows-2019-dc',
        'build-windows-2019-core-for-containers',
        'build-windows-2019-for-containers',
        'publish-to-prod-windows-2019-core-for-containers',
        'publish-to-prod-windows-2019-for-containers',
        'publish-to-staging-2019-core-for-containers',
        'publish-to-staging-2019-for-containers',
        'publish-to-testing-2019-core-for-containers',
        'publish-to-testing-2019-for-containers',
      ],
      name: 'windows-2019',
    },
    {
      jobs: [
        'build-windows-2022',
        'build-windows-2022-core',
        'publish-to-testing-2022',
        'publish-to-testing-2022-core',
        'publish-to-staging-2022',
        'publish-to-staging-2022-core',
        'publish-to-prod-2022',
        'publish-to-prod-2022-core',
        'build-sql-2019-enterprise-windows-2022-dc',
        'build-sql-2019-standard-windows-2022-dc',
        'build-sql-2019-web-windows-2022-dc',
        'publish-to-prod-sql-2019-enterprise-windows-2022-dc',
        'publish-to-prod-sql-2019-standard-windows-2022-dc',
        'publish-to-prod-sql-2019-web-windows-2022-dc',
        'publish-to-staging-sql-2019-enterprise-windows-2022-dc',
        'publish-to-staging-sql-2019-standard-windows-2022-dc',
        'publish-to-staging-sql-2019-web-windows-2022-dc',
        'publish-to-testing-sql-2019-enterprise-windows-2022-dc',
        'publish-to-testing-sql-2019-standard-windows-2022-dc',
        'publish-to-testing-sql-2019-web-windows-2022-dc',
        'build-sql-2017-enterprise-windows-2022-dc',
        'build-sql-2017-standard-windows-2022-dc',
        'build-sql-2017-web-windows-2022-dc',
        'publish-to-prod-sql-2017-enterprise-windows-2022-dc',
        'publish-to-prod-sql-2017-standard-windows-2022-dc',
        'publish-to-prod-sql-2017-web-windows-2022-dc',
        'publish-to-staging-sql-2017-enterprise-windows-2022-dc',
        'publish-to-staging-sql-2017-standard-windows-2022-dc',
        'publish-to-staging-sql-2017-web-windows-2022-dc',
        'publish-to-testing-sql-2017-enterprise-windows-2022-dc',
        'publish-to-testing-sql-2017-standard-windows-2022-dc',
        'publish-to-testing-sql-2017-web-windows-2022-dc',
      ],
      name: 'windows-2022',
    },
    {
      jobs: [
        'build-windows-20h2-core',
        'publish-to-testing-20h2',
        'publish-to-staging-20h2',
        'publish-to-prod-20h2',
      ],
      name: 'windows-20h2',
    },
    {
      jobs: [
        'build-sql-2012-enterprise-windows-2012-r2-dc',
        'build-sql-2012-standard-windows-2012-r2-dc',
        'build-sql-2012-web-windows-2012-r2-dc',
        'publish-to-testing-sql-2012-enterprise-windows-2012-r2-dc',
        'publish-to-testing-sql-2012-standard-windows-2012-r2-dc',
        'publish-to-testing-sql-2012-web-windows-2012-r2-dc',
        'publish-to-staging-sql-2012-enterprise-windows-2012-r2-dc',
        'publish-to-staging-sql-2012-standard-windows-2012-r2-dc',
        'publish-to-staging-sql-2012-web-windows-2012-r2-dc',
        'publish-to-prod-sql-2012-enterprise-windows-2012-r2-dc',
        'publish-to-prod-sql-2012-standard-windows-2012-r2-dc',
        'publish-to-prod-sql-2012-web-windows-2012-r2-dc',
      ],
      name: 'sql-2012',
    },
    {
      jobs: [
        'build-sql-2014-enterprise-windows-2012-r2-dc',
        'build-sql-2014-enterprise-windows-2016-dc',
        'build-sql-2014-standard-windows-2012-r2-dc',
        'build-sql-2014-web-windows-2012-r2-dc',
        'publish-to-testing-sql-2014-enterprise-windows-2012-r2-dc',
        'publish-to-testing-sql-2014-enterprise-windows-2016-dc',
        'publish-to-testing-sql-2014-standard-windows-2012-r2-dc',
        'publish-to-testing-sql-2014-web-windows-2012-r2-dc',
        'publish-to-staging-sql-2014-enterprise-windows-2012-r2-dc',
        'publish-to-staging-sql-2014-enterprise-windows-2016-dc',
        'publish-to-staging-sql-2014-standard-windows-2012-r2-dc',
        'publish-to-staging-sql-2014-web-windows-2012-r2-dc',
        'publish-to-prod-sql-2014-enterprise-windows-2012-r2-dc',
        'publish-to-prod-sql-2014-enterprise-windows-2016-dc',
        'publish-to-prod-sql-2014-standard-windows-2012-r2-dc',
        'publish-to-prod-sql-2014-web-windows-2012-r2-dc',
      ],
      name: 'sql-2014',
    },
    {
      jobs: [
        'build-sql-2016-enterprise-windows-2012-r2-dc',
        'build-sql-2016-enterprise-windows-2016-dc',
        'build-sql-2016-enterprise-windows-2019-dc',
        'build-sql-2016-standard-windows-2012-r2-dc',
        'build-sql-2016-standard-windows-2016-dc',
        'build-sql-2016-standard-windows-2019-dc',
        'build-sql-2016-web-windows-2012-r2-dc',
        'build-sql-2016-web-windows-2016-dc',
        'build-sql-2016-web-windows-2019-dc',
        'publish-to-testing-sql-2016-enterprise-windows-2012-r2-dc',
        'publish-to-testing-sql-2016-enterprise-windows-2016-dc',
        'publish-to-testing-sql-2016-enterprise-windows-2019-dc',
        'publish-to-testing-sql-2016-standard-windows-2012-r2-dc',
        'publish-to-testing-sql-2016-standard-windows-2016-dc',
        'publish-to-testing-sql-2016-standard-windows-2019-dc',
        'publish-to-testing-sql-2016-web-windows-2012-r2-dc',
        'publish-to-testing-sql-2016-web-windows-2016-dc',
        'publish-to-testing-sql-2016-web-windows-2019-dc',
        'publish-to-staging-sql-2016-enterprise-windows-2012-r2-dc',
        'publish-to-staging-sql-2016-enterprise-windows-2016-dc',
        'publish-to-staging-sql-2016-enterprise-windows-2019-dc',
        'publish-to-staging-sql-2016-standard-windows-2012-r2-dc',
        'publish-to-staging-sql-2016-standard-windows-2016-dc',
        'publish-to-staging-sql-2016-standard-windows-2019-dc',
        'publish-to-staging-sql-2016-web-windows-2012-r2-dc',
        'publish-to-staging-sql-2016-web-windows-2016-dc',
        'publish-to-staging-sql-2016-web-windows-2019-dc',
        'publish-to-prod-sql-2016-enterprise-windows-2012-r2-dc',
        'publish-to-prod-sql-2016-enterprise-windows-2016-dc',
        'publish-to-prod-sql-2016-enterprise-windows-2019-dc',
        'publish-to-prod-sql-2016-standard-windows-2012-r2-dc',
        'publish-to-prod-sql-2016-standard-windows-2016-dc',
        'publish-to-prod-sql-2016-standard-windows-2019-dc',
        'publish-to-prod-sql-2016-web-windows-2012-r2-dc',
        'publish-to-prod-sql-2016-web-windows-2016-dc',
        'publish-to-prod-sql-2016-web-windows-2019-dc',
      ],
      name: 'sql-2016',
    },
    {
      jobs: [
        'build-sql-2017-enterprise-windows-2016-dc',
        'build-sql-2017-enterprise-windows-2019-dc',
        'build-sql-2017-enterprise-windows-2022-dc',
        'build-sql-2017-express-windows-2012-r2-dc',
        'build-sql-2017-express-windows-2016-dc',
        'build-sql-2017-express-windows-2019-dc',
        'build-sql-2017-standard-windows-2016-dc',
        'build-sql-2017-standard-windows-2019-dc',
        'build-sql-2017-standard-windows-2022-dc',
        'build-sql-2017-web-windows-2016-dc',
        'build-sql-2017-web-windows-2019-dc',
        'build-sql-2017-web-windows-2022-dc',
        'publish-to-prod-sql-2017-enterprise-windows-2016-dc',
        'publish-to-prod-sql-2017-enterprise-windows-2019-dc',
        'publish-to-prod-sql-2017-enterprise-windows-2022-dc',
        'publish-to-prod-sql-2017-express-windows-2012-r2-dc',
        'publish-to-prod-sql-2017-express-windows-2016-dc',
        'publish-to-prod-sql-2017-express-windows-2019-dc',
        'publish-to-prod-sql-2017-standard-windows-2016-dc',
        'publish-to-prod-sql-2017-standard-windows-2019-dc',
        'publish-to-prod-sql-2017-standard-windows-2022-dc',
        'publish-to-prod-sql-2017-web-windows-2016-dc',
        'publish-to-prod-sql-2017-web-windows-2019-dc',
        'publish-to-prod-sql-2017-web-windows-2022-dc',
        'publish-to-staging-sql-2017-enterprise-windows-2016-dc',
        'publish-to-staging-sql-2017-enterprise-windows-2019-dc',
        'publish-to-staging-sql-2017-enterprise-windows-2022-dc',
        'publish-to-staging-sql-2017-express-windows-2012-r2-dc',
        'publish-to-staging-sql-2017-express-windows-2016-dc',
        'publish-to-staging-sql-2017-express-windows-2019-dc',
        'publish-to-staging-sql-2017-standard-windows-2016-dc',
        'publish-to-staging-sql-2017-standard-windows-2019-dc',
        'publish-to-staging-sql-2017-standard-windows-2022-dc',
        'publish-to-staging-sql-2017-web-windows-2016-dc',
        'publish-to-staging-sql-2017-web-windows-2019-dc',
        'publish-to-staging-sql-2017-web-windows-2022-dc',
        'publish-to-testing-sql-2017-enterprise-windows-2016-dc',
        'publish-to-testing-sql-2017-enterprise-windows-2019-dc',
        'publish-to-testing-sql-2017-enterprise-windows-2022-dc',
        'publish-to-testing-sql-2017-express-windows-2012-r2-dc',
        'publish-to-testing-sql-2017-express-windows-2016-dc',
        'publish-to-testing-sql-2017-express-windows-2019-dc',
        'publish-to-testing-sql-2017-standard-windows-2016-dc',
        'publish-to-testing-sql-2017-standard-windows-2019-dc',
        'publish-to-testing-sql-2017-standard-windows-2022-dc',
        'publish-to-testing-sql-2017-web-windows-2016-dc',
        'publish-to-testing-sql-2017-web-windows-2019-dc',
        'publish-to-testing-sql-2017-web-windows-2022-dc',
      ],
      name: 'sql-2017',
    },
    {
      jobs: [
        'build-sql-2019-enterprise-windows-2019-dc',
        'build-sql-2019-standard-windows-2019-dc',
        'build-sql-2019-web-windows-2019-dc',
        'publish-to-prod-sql-2019-enterprise-windows-2019-dc',
        'publish-to-prod-sql-2019-standard-windows-2019-dc',
        'publish-to-prod-sql-2019-web-windows-2019-dc',
        'publish-to-staging-sql-2019-enterprise-windows-2019-dc',
        'publish-to-staging-sql-2019-standard-windows-2019-dc',
        'publish-to-staging-sql-2019-web-windows-2019-dc',
        'publish-to-testing-sql-2019-enterprise-windows-2019-dc',
        'publish-to-testing-sql-2019-standard-windows-2019-dc',
        'publish-to-testing-sql-2019-web-windows-2019-dc',
        'build-sql-2019-enterprise-windows-2022-dc',
        'build-sql-2019-standard-windows-2022-dc',
        'build-sql-2019-web-windows-2022-dc',
        'publish-to-prod-sql-2019-enterprise-windows-2022-dc',
        'publish-to-prod-sql-2019-standard-windows-2022-dc',
        'publish-to-prod-sql-2019-web-windows-2022-dc',
        'publish-to-staging-sql-2019-enterprise-windows-2022-dc',
        'publish-to-staging-sql-2019-standard-windows-2022-dc',
        'publish-to-staging-sql-2019-web-windows-2022-dc',
        'publish-to-testing-sql-2019-enterprise-windows-2022-dc',
        'publish-to-testing-sql-2019-standard-windows-2022-dc',
        'publish-to-testing-sql-2019-web-windows-2022-dc',
      ],
      name: 'sql-2019',
    },
    {
      jobs: [
        'build-windows-2019-core-for-containers',
        'build-windows-2019-for-containers',
        'publish-to-prod-windows-2019-core-for-containers',
        'publish-to-prod-windows-2019-for-containers',
        'publish-to-staging-2019-core-for-containers',
        'publish-to-staging-2019-for-containers',
        'publish-to-testing-2019-core-for-containers',
        'publish-to-testing-2019-for-containers',
      ],
      name: 'container-2019',
    },
  ],
}
