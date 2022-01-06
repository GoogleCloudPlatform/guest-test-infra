// These can be extracted to a utils or common templates file.
local stages = ['test', 'staging', 'oslogin-staging', 'prod'];

local GcsResource(name, regexp) = {
  name: name + '-gcs',
  type: 'gcs',
  source: {
    bucket: 'artifact-releaser-prod-rtp',
    json_key: '((gcs-key.credential))\n',
    regexp: regexp,
  },
};

local GitResource(name, branch='master') = {
  name: name,
  type: 'git',
  source: {
    uri: 'https://github.com/GoogleCloudPlatform/' + name + '.git',
    branch: branch,
  },
};

local ImgBuild(name, workflow) = {
  local isopath = if std.endsWith(name, '-sap') then
    std.strReplace(name, '-sap', '')
  else if std.endsWith(name, '-byos') then
    std.strReplace(name, '-byos', '')
  else
    name,

  name: 'build-' + name,
  plan: [
    {
      get: 'daily-time',
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
      vars: {
        prefix: name + if name == 'debian-9' then
          '-stretch'
        else if name == 'debian-10' then
          '-buster'
        else if name == 'debian-11' then
          '-bullseye'
        else
          '',
      },
    },
    // This is the 'put trick'. We don't have the real image tarball to write to GCS here, but we want
    // Concourse to treat this job as producing it. So we write an empty file, and overwrite it in the daisy
    // workflow. This also generates the final URL for use in the daisy workflow.
    {
      put: name + '-gcs',
      params: {
        file: 'build-id-dir/' + name + '*',
      },
      get_params: {
        skip_download: 'true',
      },
    },
    {
      load_var: 'gcs-url',
      file: name + '-gcs/url',
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
      task: 'daisy-build-' + name,
      file: if std.startsWith(name, 'debian') then
        'guest-test-infra/concourse/tasks/daisy-build-images-debian.yaml'
      else
        'guest-test-infra/concourse/tasks/daisy-build-images.yaml',
      vars: {
        wf: workflow,
        gcs_url: '((.:gcs-url))',
        // A null key is omitted, so this is a shorthand for optional fields.
        [if std.startsWith(name, 'debian') then null else 'iso']: '((iso-paths.' + isopath + '))',
        google_cloud_repo: 'stable',
        build_date: '((.:build-date))',
      },
    },
  ],
  on_success: {
    task: 'success',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'linux-image-build',
      job: 'build-' + name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'failure',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'linux-image-build',
      job: 'build-' + name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local ImgPublish(name, stage, workflow, gcs) = {
  local stagename = if stage == 'test' then stage + 'ing' else stage,

  name: 'publish-to-' + stagename + '-' + name,
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
      get: name + '-gcs',
      passed: ['build-' + name],
      trigger: true,
      params: {
        skip_download: 'true',
      },
    },
    {
      load_var: 'source-version',
      file: name + '-gcs/version',
    },
    {
      task: 'get-credential',
      file: 'guest-test-infra/concourse/tasks/get-credential.yaml',
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
      task: 'publish-' + name,
      file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
      vars: {
        source_gcs_path: gcs,
        source_version: 'v((.:source-version))',
        publish_version: '((.:publish-version))',
        wf: workflow,
        environment: stage,
      },
    },
    (
      if stage == 'test' then
        {
          task: 'image-test-' + name,
          file: 'guest-test-infra/concourse/tasks/image-test.yaml',
          attempts: 3,
          vars: {
            images: 'projects/bct-prod-images/global/images/' + name + '-((.:publish-version))',
          },
        }
      else
        {}
    ),
  ],
  on_success: {
    task: 'success',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'linux-image-build',
      job: 'publish-to-testing-' + name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'failure',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'linux-image-build',
      job: 'publish-to-testing-' + name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local ImgGroup(name, images) = {

  name: name,
  jobs: [
    'build-' + image
    for image in images
  ] + [
    if stage == 'test' then
      'publish-to-' + 'testing' + '-' + image
    else
      'publish-to-' + stage + '-' + image
    for stage in stages
    for image in images
  ],
};

// Begin output
{
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
    GitResource('compute-image-tools'),
    GitResource('guest-test-infra'),
    GcsResource('almalinux-8', 'almalinux/almalinux-8-v([0-9]+).tar.gz'),
    GcsResource('centos-7', 'centos/centos-7-v([0-9]+).tar.gz'),
    GcsResource('centos-8', 'centos/centos-8-v([0-9]+).tar.gz'),
    GcsResource('centos-stream-8', 'centos/centos-stream-8-v([0-9]+).tar.gz'),
    GcsResource('rhel-7', 'rhel/rhel-7-v([0-9]+).tar.gz'),
    GcsResource('rhel-8', 'rhel/rhel-8-v([0-9]+).tar.gz'),
    GcsResource('rhel-7-byos', 'rhel/rhel-7-byos-v([0-9]+).tar.gz'),
    GcsResource('rhel-8-byos', 'rhel/rhel-8-byos-v([0-9]+).tar.gz'),
    GcsResource('rhel-7-6-sap', 'rhel/rhel-7-6-sap-v([0-9]+).tar.gz'),
    GcsResource('rhel-7-7-sap', 'rhel/rhel-7-7-sap-v([0-9]+).tar.gz'),
    GcsResource('rhel-7-9-sap', 'rhel/rhel-7-9-sap-v([0-9]+).tar.gz'),
    GcsResource('rhel-8-1-sap', 'rhel/rhel-8-1-sap-v([0-9]+).tar.gz'),
    GcsResource('rhel-8-2-sap', 'rhel/rhel-8-2-sap-v([0-9]+).tar.gz'),
    GcsResource('rhel-8-4-sap', 'rhel/rhel-8-4-sap-v([0-9]+).tar.gz'),
    GcsResource('rocky-linux-8', 'rocky-linux/rocky-linux-8-v([0-9]+).tar.gz'),
    GcsResource('debian-9', 'debian/debian-9-stretch-v([0-9]+).tar.gz'),
    GcsResource('debian-10', 'debian/debian-10-buster-v([0-9]+).tar.gz'),
    GcsResource('debian-11', 'debian/debian-11-bullseye-v([0-9]+).tar.gz'),
  ],
  jobs: [
    ImgBuild('almalinux-8', 'enterprise_linux/almalinux_8.wf.json'),
    ImgBuild('centos-7', 'enterprise_linux/centos_7.wf.json'),
    ImgBuild('centos-8', 'enterprise_linux/centos_8.wf.json'),
    ImgBuild('centos-stream-8', 'enterprise_linux/centos_stream_8.wf.json'),
    ImgBuild('rhel-7', 'enterprise_linux/rhel_7.wf.json'),
    ImgBuild('rhel-8', 'enterprise_linux/rhel_8.wf.json'),
    ImgBuild('rhel-7-byos', 'enterprise_linux/rhel_7_byos.wf.json'),
    ImgBuild('rhel-8-byos', 'enterprise_linux/rhel_8_byos.wf.json'),
    ImgBuild('rhel-7-6-sap', 'enterprise_linux/rhel_7_6_sap.wf.json'),
    ImgBuild('rhel-7-7-sap', 'enterprise_linux/rhel_7_7_sap.wf.json'),
    ImgBuild('rhel-7-9-sap', 'enterprise_linux/rhel_7_9_sap.wf.json'),
    ImgBuild('rhel-8-1-sap', 'enterprise_linux/rhel_8_1_sap.wf.json'),
    ImgBuild('rhel-8-2-sap', 'enterprise_linux/rhel_8_2_sap.wf.json'),
    ImgBuild('rhel-8-4-sap', 'enterprise_linux/rhel_8_4_sap.wf.json'),
    ImgBuild('rocky-linux-8', 'enterprise_linux/rocky_linux_8.wf.json'),
    ImgBuild('debian-9', 'debian/debian_9.wf.json'),
    ImgBuild('debian-10', 'debian/debian_10.wf.json'),
    ImgBuild('debian-11', 'debian/debian_11.wf.json'),
  ] + [
    ImgPublish('almalinux-8',
               stage,
               'enterprise_linux/almalinux_8.publish.json',
               'gs://artifact-releaser-prod-rtp/almalinux')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('centos-7',
               stage,
               'gs://artifact-releaser-prod-rtp/centos',
               'enterprise_linux/centos_7.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('centos-8',
               stage,
               'gs://artifact-releaser-prod-rtp/centos',
               'enterprise_linux/centos_8.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('centos-stream-8',
               stage,
               'gs://artifact-releaser-prod-rtp/centos',
               'enterprise_linux/centos_stream_8.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('rhel-7',
               stage,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_7.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('rhel-8',
               stage,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_8.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('rhel-7-byos',
               stage,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_7_byos.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('rhel-8-byos',
               stage,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_8_byos.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('rhel-7-6-sap',
               stage,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_7_6_sap.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('rhel-7-7-sap',
               stage,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_7_7_sap.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('rhel-7-9-sap',
               stage,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_7_9_sap.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('rhel-8-1-sap',
               stage,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_8_1_sap.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('rhel-8-2-sap',
               stage,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_8_2_sap.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('rhel-8-4-sap',
               stage,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_8_4_sap.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('rocky-linux-8',
               stage,
               'gs://artifact-releaser-prod-rtp/rocky-linux',
               'enterprise_linux/rocky_linux_8.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('debian-9',
               stage,
               'gs://artifact-releaser-prod-rtp/debian',
               'debian/debian_9.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('debian-10',
               stage,
               'gs://artifact-releaser-prod-rtp/debian',
               'debian/debian_10.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ] + [
    ImgPublish('debian-11',
               stage,
               'gs://artifact-releaser-prod-rtp/debian',
               'debian/debian_11.publish.json')
    for stage in ['test', 'staging', 'oslogin-staging', 'prod']
  ],
  groups: [
    ImgGroup('debian', ['debian-9', 'debian-10', 'debian-11']),
    ImgGroup('centos', ['centos-7', 'centos-8', 'centos-stream-8']),
    ImgGroup('almalinux', ['almalinux-8']),
    ImgGroup('rocky-linux', ['rocky-linux-8']),
    ImgGroup('rhel', [
      'rhel-7',
      'rhel-8',
      'rhel-7-byos',
      'rhel-8-byos',
      'rhel-7-6-sap',
      'rhel-7-7-sap',
      'rhel-7-9-sap',
      'rhel-8-1-sap',
      'rhel-8-2-sap',
      'rhel-8-4-sap',
    ]),
  ],
}
