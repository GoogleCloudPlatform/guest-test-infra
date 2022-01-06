// These can be extracted to a utils or common templates file.
local envs = ['test', 'staging', 'oslogin-staging', 'prod'];

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

// An image build runs the daisy image build workflow, producing an artifact in GCS that we track as a
// resource between jobs.
local ImgBuild(name, workflow) = {
  local isopath = if std.endsWith(name, '-sap') then
    std.strReplace(name, '-sap', '')
  else if std.endsWith(name, '-byos') then
    std.strReplace(name, '-byos', '')
  else
    name,

  local imagename = if name == 'debian-9' then
    'debian-9-stretch'
  else if name == 'debian-10' then
    'debian-10-buster'
  else if name == 'debian-11' then
    'debian-11-bullseye'
  else name,

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
      // Produces build-id-dir output
      task: 'generate-build-id',
      file: 'guest-test-infra/concourse/tasks/generate-build-id.yaml',
      vars: {
        prefix: imagename,
      },
    },
    // This is the 'put trick'. We don't have the real image tarball to write to GCS here, but we want
    // Concourse to treat this job as producing it. So we write an empty file, and overwrite it in the daisy
    // workflow. This also generates the final URL for use in the daisy workflow.
    {
      put: name + '-gcs',
      params: {
        // e.g. 'build-id-dir/centos-7*'
        // this matches the file we created above. It needs to be a file for the 'put' step.
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
      // Today these are slightly differing tasks due to different default arguments. EL builds need an ISO,
      // but Debian builds don't. TODO: replace this with JSONnet templating.
      file: if std.startsWith(name, 'debian') then
        'guest-test-infra/concourse/tasks/daisy-build-images-debian.yaml'
      else
        'guest-test-infra/concourse/tasks/daisy-build-images.yaml',
      vars: {
        wf: workflow,
        gcs_url: '((.:gcs-url))',
        // The 'iso' field is omitted in Debian builds.
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

// Image publish involves either invoking gce_image_publish or sending a PubSub message to ARLE.
local ImgPublish(name, env, gcs, workflow) = {
  // Called 'testing' in names but 'test' in arguments to publish tools. TODO: standardize this.
  local envname = if env == 'test' then 'testing' else env,

  // Debian GCS and images use a longer name, but jobs use the shorter name.
  local imagename = if name == 'debian-9' then
    'debian-9-stretch'
  else if name == 'debian-10' then
    'debian-10-buster'
  else if name == 'debian-11' then
    'debian-11-bullseye'
  else name,

  name: 'publish-to-' + envname + '-' + name,
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
            passed: [
              if env == 'test' then
                'build-' + name
              else
                'publish-to-testing-' + name,
            ],
            trigger: if env == 'test' then true else false,
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
        ] + [(
          // Prod releases use a different final publish step that invokes ARLE.
          if env == 'prod' then
            {
              task: 'publish-' + name,
              file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
              vars: {
                gcs_image_path: gcs,
                source_version: 'v((.:source-version))',
                publish_version: '((.:publish-version))',
                wf: workflow,
                release_notes: '',
                image_name: std.strReplace(name, '-', '_'),  // For whatever reason, we use underscores.
                topic: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
              },
            }
          // Other releases use gce_image_publish directly.
          else
            {
              task: if env == 'test' then
                'publish-' + name
              else
                'publish-' + envname + '-' + name,
              file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
              vars: {
                source_gcs_path: gcs,
                source_version: 'v((.:source-version))',
                publish_version: '((.:publish-version))',
                wf: workflow,
                environment: env,
              },
            }
        )] +
        (
          // Run post-publish tests in 'publish-to-testing-' jobs.
          if env == 'test' then
            [
              {
                task: 'image-test-' + name,
                file: 'guest-test-infra/concourse/tasks/image-test.yaml',
                attempts: 3,
                vars: {
                  images: 'projects/bct-prod-images/global/images/' + imagename + '-((.:publish-version))',
                },
              },
            ]
          else
            []
        ),
  on_success: {
    task: 'success',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'linux-image-build',
      job: 'publish-to-' + envname + '-' + name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'failure',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'linux-image-build',
      job: 'publish-to-' + envname + '-' + name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

// Image Group creates a Concourse grouping for the permutation of images and environments.
local ImgGroup(name, images) = {
  name: name,
  jobs: [
    'build-' + image
    for image in images
  ] + [
    if env == 'test' then
      'publish-to-' + 'testing' + '-' + image
    else
      'publish-to-' + env + '-' + image
    for env in envs
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
               env,
               'gs://artifact-releaser-prod-rtp/almalinux',
               'enterprise_linux/almalinux_8.publish.json')
    for env in envs
  ] + [
    ImgPublish('centos-7',
               env,
               'gs://artifact-releaser-prod-rtp/centos',
               'enterprise_linux/centos_7.publish.json')
    for env in envs
  ] + [
    ImgPublish('centos-8',
               env,
               'gs://artifact-releaser-prod-rtp/centos',
               'enterprise_linux/centos_8.publish.json')
    for env in envs
  ] + [
    ImgPublish('centos-stream-8',
               env,
               'gs://artifact-releaser-prod-rtp/centos',
               'enterprise_linux/centos_stream_8.publish.json')
    for env in envs
  ] + [
    ImgPublish('rhel-7',
               env,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_7.publish.json')
    for env in envs
  ] + [
    ImgPublish('rhel-8',
               env,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_8.publish.json')
    for env in envs
  ] + [
    ImgPublish('rhel-7-byos',
               env,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_7_byos.publish.json')
    for env in envs
  ] + [
    ImgPublish('rhel-8-byos',
               env,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_8_byos.publish.json')
    for env in envs
  ] + [
    ImgPublish('rhel-7-6-sap',
               env,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_7_6_sap.publish.json')
    for env in envs
  ] + [
    ImgPublish('rhel-7-7-sap',
               env,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_7_7_sap.publish.json')
    for env in envs
  ] + [
    ImgPublish('rhel-7-9-sap',
               env,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_7_9_sap.publish.json')
    for env in envs
  ] + [
    ImgPublish('rhel-8-1-sap',
               env,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_8_1_sap.publish.json')
    for env in envs
  ] + [
    ImgPublish('rhel-8-2-sap',
               env,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_8_2_sap.publish.json')
    for env in envs
  ] + [
    ImgPublish('rhel-8-4-sap',
               env,
               'gs://artifact-releaser-prod-rtp/rhel',
               'enterprise_linux/rhel_8_4_sap.publish.json')
    for env in envs
  ] + [
    ImgPublish('rocky-linux-8',
               env,
               'gs://artifact-releaser-prod-rtp/rocky-linux',
               'enterprise_linux/rocky_linux_8.publish.json')
    for env in envs
  ] + [
    ImgPublish('debian-9',
               env,
               'gs://artifact-releaser-prod-rtp/debian',
               'debian/debian_9.publish.json')
    for env in envs
  ] + [
    ImgPublish('debian-10',
               env,
               'gs://artifact-releaser-prod-rtp/debian',
               'debian/debian_10.publish.json')
    for env in envs
  ] + [
    ImgPublish('debian-11',
               env,
               'gs://artifact-releaser-prod-rtp/debian',
               'debian/debian_11.publish.json')
    for env in envs
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
