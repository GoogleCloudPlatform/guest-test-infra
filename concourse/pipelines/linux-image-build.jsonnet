// Imports.
local common = import '../templates/common.libsonnet';
local daisy = import '../templates/daisy.libsonnet';
local gcp_secret_manager = import '../templates/gcp-secret-manager.libsonnet';
local lego = import '../templates/lego.libsonnet';

// Common
local envs = ['testing', 'staging', 'oslogin-staging', 'prod'];
local underscore(input) = std.strReplace(input, '-', '_');

local DebianGcsImgResource(image) = common.gcsimgresource {
  image: image,
  gcs_dir: 'debian/',
  regexp: self.gcs_dir + common.debian_image_prefixes[self.image] + '-v([0-9]+).tar.gz',
};

local GitResource(name) = common.GitResource(name);

local ImgBuildTask(workflow, gcs_url) = daisy.daisyimagetask {
  vars+: [
    'google_cloud_repo=stable',
  ],
  gcs_url: gcs_url,
  workflow: workflow,
};

local ELImgBuildTask(workflow, gcs_url, installer_iso) = daisy.daisyimagetask {
  vars+: [
    'google_cloud_repo=stable',
    'installer_iso=' + installer_iso,
  ],
  gcs_url: gcs_url,
  workflow: workflow,
};

local RHUIImgBuildTask(workflow, gcs_url) = daisy.daisyimagetask {
  vars+: [
    'instance_service_account=rhui-builder@rhel-infra.google.com.iam.gserviceaccount.com',
  ],

  project: 'google.com:rhel-infra',

  gcs_url: gcs_url,
  workflow: workflow,
};

local publishresulttask = {
  local task = self,

  project:: 'gcp-guest',
  zone:: 'us-central1-a',
  pipeline:: 'linux-image-build',
  job:: error 'must set job in publishresulttask',
  result_state:: error 'must set result_state in publishresulttask',
  start_timestamp:: error 'must set start_timestamp in publishresulttask',

  // Start of output.
  platform: 'linux',
  image_resource: {
    type: 'docker-image',
    source: {
      repository: 'gcr.io/gcp-guest/concourse-metrics',
      tag: 'latest',
    },
  },
  run: {
    path: '/publish-job-result',
    args:
      [
        '--project-id=' + task.project,
        '--zone=' + task.zone,
        '--pipeline=' + task.pipeline,
        '--job=' + task.job,
        '--task=publish-job-result',
        '--result-state=' + task.result_state,
        '--start-timestamp=' + task.start_timestamp,
        '--metric-path=concourse/job/duration',
      ],
  },
};

local gcepublishtask = {
  local task = self,

  source_gcs_path:: error 'must set source_gcs_path in gcepublishtask',
  source_version:: error 'must set source_version in gcepublishtask',
  publish_version:: error 'must set publish_version in gcepublishtask',
  environment:: error 'must set environment in gcepublishtask',
  wf:: error 'must set wf in gcepublishtask',

  platform: 'linux',
  image_resource: {
    type: 'docker-image',
    source: {
      repository: 'gcr.io/compute-image-tools/gce_image_publish',
      tag: 'latest',
    },
  },
  inputs: [
    { name: 'compute-image-tools' },
  ],
  run: {
    path: '/gce_image_publish',
    args: [
      '-rollout_rate=0',
      '-skip_confirmation',
      '-replace',
      '-no_root',
      '-source_gcs_path=' + task.source_gcs_path,
      '-source_version=' + task.source_version,
      '-publish_version=' + task.publish_version,
      '-var:environment=' + task.environment,
      './compute-image-tools/daisy_workflows/build-publish/' + task.wf,
    ],
  },
};

local arlepublishtask = {
  local task = self,

  topic:: common.prod_topic,
  image_name:: error 'must set image_name in arlepublishtask',
  gcs_image_path:: error 'must set gcs_image_path in arlepublishtask',
  wf:: error 'must set wf in arlepublishtask',
  source_version:: error 'must set source_version in arlepublishtask',
  publish_version:: error 'must set publish_version in arlepublishtask',

  platform: 'linux',
  image_resource: {
    type: 'docker-image',
    source: {
      repository: 'google/cloud-sdk',
      tag: 'alpine',
    },
  },
  inputs: [
    {
      name: 'compute-image-tools',
    },
  ],
  run: {
    path: 'sh',
    args: [
      '-exc',
      "wf=$(sed 's/\\\"/\\\\\"/g' ./compute-image-tools/daisy_workflows/build-publish/%s | tr -d '\\n')\n" % task.wf +
      'gcloud pubsub topics publish "%s" --message "{\\"type\\": \\"ImagePublish\\", \\"request\\":\n{\\"image_name\\": \\"%s\\", \\"gcs_image_path\\": \\"%s\\", \\"image_publish_template\\": \\"${wf}\\",\n      \\"source_version\\": \\"%s\\", \\"publish_version\\": \\"%s\\", \\"release_notes\\": \\"\\"}}"\n' %
      [task.topic, task.image_name, task.gcs_image_path, task.source_version, task.publish_version],
    ],
  },
};


local imgbuildjob = {
  local tl = self,

  image:: '',
  image_prefix:: self.image,
  workflow:: '',
  build_task:: ImgBuildTask(self.workflow, '((.:gcs-url))'),
  extra_tasks:: [],
  daily:: true,
  daily_task:: if self.daily then [
    {
      get: 'daily-time',
      trigger: true,
    },
  ] else [],

  name: 'build-' + self.image,
  plan: tl.daily_task + [
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
      vars: {
        prefix: tl.image_prefix,
      },
    },
    // This is the 'put trick'. We don't have the real image tarball to write to GCS here, but we want
    // Concourse to treat this job as producing it. So we write an empty file now, and overwrite it later in
    // the daisy workflow. This also generates the final URL for use in the daisy workflow.
    {
      put: tl.image + '-gcs',
      params: {
        // empty file written to GCS e.g. 'build-id-dir/centos-7-v20210107.tar.gz'
        file: 'build-id-dir/%s*' % tl.image,
      },
      get_params: {
        skip_download: 'true',
      },
    },
    {
      load_var: 'gcs-url',
      file: '%s-gcs/url' % tl.image,
    },
    {
      task: 'generate-build-date',
      file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
    },
    {
      load_var: 'build-date',
      file: 'publish-version/version',
    },
  ] + tl.extra_tasks + [
    {
      task: 'daisy-build-' + tl.image,
      config: tl.build_task,
    },
  ],
  on_success: {
    task: 'success',
    config: publishresulttask {
      job: tl.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'failure',
    config: publishresulttask {
      job: tl.name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local DebianImgBuildJob(image, workflow) = imgbuildjob {
  image: image,
  workflow: workflow,
  image_prefix: common.debian_image_prefixes[image],
};

local ELImgBuildJob(image, workflow) = imgbuildjob {
  image: image,
  workflow: workflow,

  local isopath = if std.endsWith(image, '-sap') then
    std.strReplace(image, '-sap', '')
  else if std.endsWith(image, '-byos') then
    std.strReplace(image, '-byos', '')
  else
    image,

  // Override build_task with an EL specific task.
  build_task: ELImgBuildTask(workflow, '((.:gcs-url))', '((iso-paths.%s))' % isopath),
};

local RHUAImgBuildJob(image, workflow) = imgbuildjob {
  image: image,
  workflow: workflow,
  daily: false,

  // Append var to Daisy image build task
  build_task: RHUIImgBuildTask(workflow, '((.:gcs-url))'),
};

local CDSImgBuildJob(image, workflow) = imgbuildjob {
  local acme_server = 'dv.acme-v02.api.pki.goog',
  local acme_email = 'bigcluster-guest-team@google.com',
  local rhui_project = 'google.com:rhel-infra',

  image: image,
  workflow: workflow,
  daily: false,
  extra_tasks: [
    {
      task: 'get-acme-account-json',
      config: gcp_secret_manager.getsecrettask {
        secret_name: 'rhui-acme-account-json',
        project: rhui_project,
        output_path: 'accounts/%s/%s/account.json' % [acme_server, acme_email],
      },
    },
    {
      task: 'get-rhui-tls-key',
      config: gcp_secret_manager.getsecrettask {
        secret_name: 'rhui-tls-key',
        project: rhui_project,
        output_path: 'accounts/%s/%s/%s.key' % [acme_server, acme_email, acme_email],

        // Layer onto the same output as previous task
        inputs+: gcp_secret_manager.getsecrettask.outputs,
      },
    },
    {
      task: 'lego-provision-tls-crt',
      config: lego.legotask {
        domains: ['rhui.googlecloud.com', 'staging-rhui.googlecloud.com'],
        acme_server: acme_server,
        email: acme_email,
        project: rhui_project,
        input: 'gcp-secret-manager',
      },
    },
  ],

  // Append var to Daisy build task
  build_task: RHUIImgBuildTask(workflow, '((.:gcs-url))') {
    inputs: [
      { name: 'gcp-secret-manager' },
    ],
    vars+: [
      'tls_cert_path=../../../../gcp-secret-manager/certificates/rhui.googlecloud.com.crt',
    ],
  },
};

local imgpublishjob = {
  local tl = self,

  env:: error 'must set publish env in template',
  workflow:: self.workflow_dir + underscore(self.image) + '.publish.json',
  workflow_dir:: error 'must set workflow_dir in template',

  image:: error 'must set image in template',
  image_prefix:: self.image,

  gcs:: 'gs://%s%s' % [self.gcs_bucket, self.gcs_dir],
  gcs_dir:: error 'must set gcs directory in template',
  gcs_bucket:: common.prod_bucket,

  // Begin output of Concourse Task definition.
  name: 'publish-to-%s-%s' % [tl.env, tl.image],
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
            get: tl.image + '-gcs',
            passed: [
              if tl.env == 'testing' then
                'build-' + tl.image
              else
                // Everyone else depends on testing. If this changes, we'll parameterize this field.
                'publish-to-testing-' + tl.image,
            ],
            trigger: if tl.env == 'testing' then true else false,
            params: {
              skip_download: 'true',
            },
          },
          {
            load_var: 'source-version',
            file: tl.image + '-gcs/version',
          },
          {
            task: 'generate-version',
            file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
          },
          {
            load_var: 'publish-version',
            file: 'publish-version/version',
          },
          // Prod releases use a different final publish step that invokes ARLE.
          if tl.env == 'prod' then
            {
              task: 'publish-' + tl.image,
              config: arlepublishtask {
                gcs_image_path: tl.gcs,
                source_version: 'v((.:source-version))',
                publish_version: '((.:publish-version))',
                wf: tl.workflow,
                image_name: underscore(tl.image),
              },
            }
          // Other releases use gce_image_publish directly.
          else
            {
              task: if tl.env == 'testing' then
                'publish-' + tl.image
              else
                'publish-%s-%s' % [tl.env, tl.image],
              config: gcepublishtask {
                source_gcs_path: tl.gcs,
                source_version: 'v((.:source-version))',
                publish_version: '((.:publish-version))',
                wf: tl.workflow,
                environment: if tl.env == 'testing' then 'test' else tl.env,
              },
            },
        ] +
        // Run post-publish tests in 'publish-to-testing-' jobs.
        if tl.env == 'testing' then
          [
            {
              task: 'image-test-' + tl.image,
              file: 'guest-test-infra/concourse/tasks/image-test.yaml',
              attempts: 3,
              vars: {
                images: 'projects/bct-prod-images/global/images/%s-((.:publish-version))' % tl.image_prefix,
              },
            },
          ]
        else
          [],
  on_success: {
    task: 'success',
    config: publishresulttask {
      pipeline: 'linux-image-build',
      job: tl.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'failure',
    config: publishresulttask {
      pipeline: 'linux-image-build',
      job: 'publish-to-%s-%s' % [tl.env, tl.image],
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local ImgPublishJob(image, env, gcs_dir, workflow_dir) = imgpublishjob {
  image: image,
  env: env,
  gcs_dir: gcs_dir,
  workflow_dir: workflow_dir,
};

local DebianImgPublishJob(image, env, workflow_dir) = imgpublishjob {
  image: image,
  env: env,
  gcs_dir: '/debian',
  workflow_dir: workflow_dir,

  // Debian tarballs and images use a longer name, but jobs use the shorter name.
  image_prefix: common.debian_image_prefixes[image],
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

{
  local debian_images = ['debian-9', 'debian-10', 'debian-11'],
  local centos_images = ['centos-7', 'centos-stream-8', 'centos-stream-9'],
  local rhel_images = [
    'rhel-7',
    'rhel-7-6-sap',
    'rhel-7-7-sap',
    'rhel-7-9-sap',
    'rhel-7-byos',
    'rhel-8',
    'rhel-8-1-sap',
    'rhel-8-2-sap',
    'rhel-8-4-sap',
    'rhel-8-byos',
  ],

  resource_types: [
    {
      name: 'gcs',
      type: 'registry-image',
      source: { repository: 'frodenas/gcs-resource' },
    },
  ],
  resources: [
               {
                 name: 'daily-time',
                 type: 'time',
                 source: { interval: '24h' },
               },
               common.GitResource('compute-image-tools'),
               common.GitResource('guest-test-infra'),
               common.GcsImgResource('almalinux-8', 'almalinux/'),
               common.GcsImgResource('rocky-linux-8', 'rocky-linux/'),
               common.GcsImgResource('rhua', 'rhui/'),
               common.GcsImgResource('cds', 'rhui/'),
             ] +
             [
               DebianGcsImgResource(image)
               for image in debian_images
             ] +
             [
               common.GcsImgResource(image, 'centos/')
               for image in centos_images
             ] +
             [
               common.GcsImgResource(image, 'rhel/')
               for image in rhel_images
             ],
  jobs: [
          // Image build jobs
          DebianImgBuildJob('debian-9', 'debian/debian_9.wf.json'),
          DebianImgBuildJob('debian-10', 'debian/debian_10.wf.json'),
          DebianImgBuildJob('debian-11', 'debian/debian_11.wf.json'),
          ELImgBuildJob('rhel-7', 'enterprise_linux/rhel_7.wf.json'),
          ELImgBuildJob('rhel-8', 'enterprise_linux/rhel_8.wf.json'),
          ELImgBuildJob('rhel-7-byos', 'enterprise_linux/rhel_7_byos.wf.json'),
          ELImgBuildJob('rhel-8-byos', 'enterprise_linux/rhel_8_byos.wf.json'),
          ELImgBuildJob('rhel-7-6-sap', 'enterprise_linux/rhel_7_6_sap.wf.json'),
          ELImgBuildJob('rhel-7-7-sap', 'enterprise_linux/rhel_7_7_sap.wf.json'),
          ELImgBuildJob('rhel-7-9-sap', 'enterprise_linux/rhel_7_9_sap.wf.json'),
          ELImgBuildJob('rhel-8-1-sap', 'enterprise_linux/rhel_8_1_sap.wf.json'),
          ELImgBuildJob('rhel-8-2-sap', 'enterprise_linux/rhel_8_2_sap.wf.json'),
          ELImgBuildJob('rhel-8-4-sap', 'enterprise_linux/rhel_8_4_sap.wf.json'),
          ELImgBuildJob('centos-7', 'enterprise_linux/centos_7.wf.json'),
          ELImgBuildJob('centos-stream-8', 'enterprise_linux/centos_stream_8.wf.json'),
          ELImgBuildJob('centos-stream-9', 'enterprise_linux/centos_stream_9.wf.json'),
          ELImgBuildJob('almalinux-8', 'enterprise_linux/almalinux_8.wf.json'),
          ELImgBuildJob('rocky-linux-8', 'enterprise_linux/rocky_linux_8.wf.json'),
          RHUAImgBuildJob('rhua', 'rhui/rhua.wf.json'),
          CDSImgBuildJob('cds', 'rhui/cds.wf.json'),
        ] +
        [
          // Debian publish jobs
          DebianImgPublishJob(image, env, 'debian/')
          for env in envs
          for image in debian_images
        ] +
        [
          // RHEL publish jobs
          ImgPublishJob(image, env, '/rhel', 'enterprise_linux/')
          for env in envs
          for image in rhel_images
        ] +
        [
          // CentOS publish jobs
          ImgPublishJob(image, env, '/centos', 'enterprise_linux/')
          for env in envs
          for image in centos_images
        ] +
        [
          ImgPublishJob('almalinux-8', env, '/almalinux', 'enterprise_linux/')
          for env in envs
        ] +
        [
          ImgPublishJob('rocky-linux-8', env, '/rocky-linux', 'enterprise_linux/')
          for env in envs
        ],
  groups: [
    ImgGroup('debian', debian_images),
    ImgGroup('rhel', rhel_images),
    ImgGroup('centos', centos_images),
    ImgGroup('almalinux', ['almalinux-8']),
    ImgGroup('rocky-linux', ['rocky-linux-8']),
    // No publish jobs yet, can't use ImgGroup function.
    { name: 'rhui', jobs: ['build-rhua', 'build-cds'] },
  ],
}
