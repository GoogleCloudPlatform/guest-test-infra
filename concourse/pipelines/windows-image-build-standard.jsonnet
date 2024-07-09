// Imports.
local arle = import '../templates/arle.libsonnet';
local common = import '../templates/common.libsonnet';
local daisy = import '../templates/daisy.libsonnet';
local gcp_secret_manager = import '../templates/gcp-secret-manager.libsonnet';

local server_envs = ['testing', 'internal', 'prod'];
local sql_envs = ['testing', 'prod'];
local underscore(input) = std.strReplace(input, '-', '_');

// Templates.
local imagetesttask = common.imagetesttask {
  exclude: '(oslogin)|(storageperf)|(networkperf)',
  extra_args: [ '-x86_shape=n1-standard-4', '-shapevalidation_test_filter=^(([A-Z][0-3])|(N4))' ],
};

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

local windowsinstallmediaimgbuildjob = {
  local job = self,

  image:: error 'must set image in windowsinstallmediaimgbuildjob',
  workflow:: error 'must set workflow in windowsinstallmediaimgbuildjob',

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
      vars: { prefix: job.image, id: '((.:id))' },
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
      task: 'daisy-build',
      config: daisy.daisywindowsinstallmediatask {
        workflow: job.workflow,
        gcs_url: '((.:gcs-url))',
        iso_path_2022: '((.:iso_path_2022))',
        iso_path_2019: '((.:iso_path_2019))',
        iso_path_2016: '((.:iso_path_2016))',
        iso_path_2012r2: '((.:iso_path_2012r2',
        updates_path_2022: '((.:updates_path_2022))',
        updates_path_2019: '((.:updates_path_2019))',
        updates_path_2016: '((.:updates_path_2016))',
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

  runtests:: if job.env == 'testing' && (std.length(std.findSubstr("server", job.image)) > 0 || std.length(std.findSubstr("sql", job.image)) > 0) then true
    else false,

  // Publish can proceed if build passes.
  passed:: if job.env == 'testing' then
    'build-' + job.image
  else
    'publish-to-testing-' + job.image,

  // Builds are automatically pushed to testing.
  trigger:: if job.env == 'testing' then true
    else if job.env == 'internal' then true
    else if job.env == 'client' then true
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
 ] +
  (if job.env == 'prod' then
  [
    // Different publish step in prod
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
  ])
  +
  (if job.runtests then
  [
      {
        task: 'image-test-' + job.image,
        config: imagetesttask {
          images: 'projects/bct-prod-images/global/images/%s-((.:publish-version))' % job.image,
        },
        attempts: 3,
    },
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

local SQLImgBuildJob(image, base_image, sql_version, ssms_version) = sqlimgbuildjob {
  image: image,
  base_image: base_image,
  sql_version: sql_version,
  ssms_version: ssms_version,

  workflow: 'sqlserver/%s.wf.json' % image,
};

local WindowsInstallMediaImgBuildJob(image) = windowsinstallmediaimgbuildjob {
  image: image,
  workflow: 'windows/%s.wf.json' % image,
};

local ImgPublishJob(image, env, workflow_dir, gcs_dir) = imgpublishjob {
  image: image,
  env: env,
  gcs_dir: gcs_dir,
  // build -> testing -> prod/client -> internal
  passed:: if env == 'testing' then
             'build-' + image
           else if env == 'prod' then
             'publish-to-testing-' + image
           else if env == 'internal' then
             'publish-to-prod-' + image
           else if env == 'client' then
             'publish-to-testing-' + image,

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
  local windows_2016_images = [
    'windows-server-2016',
    'windows-server-2016-core',
  ],
  local windows_2019_images = [
    'windows-server-2019',
    'windows-server-2019-core',
  ],
  local windows_2022_images = [
    'windows-server-2022',
    'windows-server-2022-core',
  ],
  local windows_2025_images = [
    'windows-server-2025',
    'windows-server-2025-core',
  ],
  local sql_2016_images = [
    'sql-2016-enterprise-windows-2016',
    'sql-2016-enterprise-windows-2019',
    'sql-2016-standard-windows-2016',
    'sql-2016-standard-windows-2019',
    'sql-2016-web-windows-2016',
    'sql-2016-web-windows-2019',
  ],
  local sql_2017_images = [
    'sql-2017-enterprise-windows-2016',
    'sql-2017-enterprise-windows-2019',
    'sql-2017-enterprise-windows-2022',
    'sql-2017-express-windows-2016',
    'sql-2017-express-windows-2019',
    'sql-2017-standard-windows-2016',
    'sql-2017-standard-windows-2019',
    'sql-2017-standard-windows-2022',
    'sql-2017-web-windows-2016',
    'sql-2017-web-windows-2019',
    'sql-2017-web-windows-2022',
  ],
  local sql_2019_images = [
    'sql-2019-enterprise-windows-2019',
    'sql-2019-enterprise-windows-2022',
    'sql-2019-standard-windows-2019',
    'sql-2019-standard-windows-2022',
    'sql-2019-web-windows-2019',
    'sql-2019-web-windows-2022',
  ],
  local sql_2022_images = [
    'sql-2022-enterprise-windows-2019',
    'sql-2022-enterprise-windows-2022',
    'sql-2022-standard-windows-2019',
    'sql-2022-standard-windows-2022',
    'sql-2022-web-windows-2019',
    'sql-2022-web-windows-2022',
  ],

  local windows_server_images = windows_2016_images + windows_2019_images
                              + windows_2022_images + windows_2025_images,
  local sql_images = sql_2016_images + sql_2017_images + sql_2019_images + sql_2022_images,

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
               for image in windows_server_images
             ] +
             [
               common.GcsImgResource(image, 'sqlserver-uefi')
               for image in sql_images
             ],
  jobs: [
          // Windows builds

          ImgBuildJob('windows-server-2025', 'win2025-64', 'windows_gcs_updates_server2025'),
          ImgBuildJob('windows-server-2025-core', 'win2025-64', 'windows_gcs_updates_server2025'),
          ImgBuildJob('windows-server-2022', 'win2022-64', 'windows_gcs_updates_server2022'),
          ImgBuildJob('windows-server-2022-core', 'win2022-64', 'windows_gcs_updates_server2022'),
          ImgBuildJob('windows-server-2019', 'win2019-64', 'windows_gcs_updates_server2019'),
          ImgBuildJob('windows-server-2019-core', 'win2019-64', 'windows_gcs_updates_server2019'),
          ImgBuildJob('windows-server-2016', 'win2016-64', 'windows_gcs_updates_server2016'),
          ImgBuildJob('windows-server-2016-core', 'win2016-64', 'windows_gcs_updates_server2016'),

          // SQL derivative builds

          SQLImgBuildJob('sql-2016-enterprise-windows-2016', 'windows-server-2016', 'sql-2016-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-enterprise-windows-2019', 'windows-server-2019', 'sql-2016-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-standard-windows-2016', 'windows-server-2016', 'sql-2016-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-standard-windows-2019', 'windows-server-2019', 'sql-2016-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-web-windows-2016', 'windows-server-2016', 'sql-2016-web', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2016-web-windows-2019', 'windows-server-2019', 'sql-2016-web', 'windows_gcs_ssms_exe'),

          SQLImgBuildJob('sql-2017-enterprise-windows-2016', 'windows-server-2016', 'sql-2017-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-enterprise-windows-2019', 'windows-server-2019', 'sql-2017-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-enterprise-windows-2022', 'windows-server-2022', 'sql-2017-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-express-windows-2016', 'windows-server-2016', 'sql-2017-express', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-express-windows-2019', 'windows-server-2019', 'sql-2017-express', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-standard-windows-2016', 'windows-server-2016', 'sql-2017-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-standard-windows-2019', 'windows-server-2019', 'sql-2017-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-standard-windows-2022', 'windows-server-2022', 'sql-2017-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-web-windows-2016', 'windows-server-2016', 'sql-2017-web', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-web-windows-2019', 'windows-server-2019', 'sql-2017-web', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2017-web-windows-2022', 'windows-server-2022', 'sql-2017-web', 'windows_gcs_ssms_exe'),
          
          SQLImgBuildJob('sql-2019-enterprise-windows-2019', 'windows-server-2019', 'sql-2019-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2019-enterprise-windows-2022', 'windows-server-2022', 'sql-2019-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2019-standard-windows-2019', 'windows-server-2019', 'sql-2019-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2019-standard-windows-2022', 'windows-server-2022', 'sql-2019-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2019-web-windows-2019', 'windows-server-2019', 'sql-2019-web', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2019-web-windows-2022', 'windows-server-2022', 'sql-2019-web', 'windows_gcs_ssms_exe'),

          SQLImgBuildJob('sql-2022-enterprise-windows-2019', 'windows-server-2019', 'sql-2022-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2022-enterprise-windows-2022', 'windows-server-2022', 'sql-2022-enterprise', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2022-standard-windows-2019', 'windows-server-2019', 'sql-2022-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2022-standard-windows-2022', 'windows-server-2022', 'sql-2022-standard', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2022-web-windows-2019', 'windows-server-2019', 'sql-2022-web', 'windows_gcs_ssms_exe'),
          SQLImgBuildJob('sql-2022-web-windows-2022', 'windows-server-2022', 'sql-2022-web', 'windows_gcs_ssms_exe'),
        ] +

        // Publish jobs
        [
          ImgPublishJob(image, env, 'windows', 'windows-uefi')
          for image in windows_server_images
          for env in server_envs
        ] +
        [
          ImgPublishJob(image, env, 'sqlserver', 'sqlserver-uefi')
          for image in sql_images
          for env in sql_envs
        ],

  groups: [
    ImgGroup('windows-2016', windows_2016_images, server_envs),
    ImgGroup('windows-2019', windows_2019_images, server_envs),
    ImgGroup('windows-2022', windows_2022_images, server_envs),
    ImgGroup('windows-2025', windows_2025_images, server_envs),
    ImgGroup('sql-2016', sql_2016_images, sql_envs),
    ImgGroup('sql-2017', sql_2017_images, sql_envs),
    ImgGroup('sql-2019', sql_2019_images, sql_envs),
    ImgGroup('sql-2022', sql_2022_images, sql_envs),
  ],
}
