local underscore(input) = std.strReplace(input, '-', '_');

local cos_images = [
  'cos-85-lts',
  'cos-89-lts',
  'cos-93-lts',
  'cos-97-lts',
  'cos-dev',
];
local fedora_images = [
  'fedora-33',
  'fedora-34',
  'fedora-coreos-next',
  'fedora-coreos-stable',
  'fedora-coreos-testing',
];
local freebsd_images = [
  'freebsd-11',
  'freebsd-12',
  'freebsd-13',
];
local suse_images = [
  'opensuse-leap-15',
  'opensuse-leap-15-arm64',
  'sles-12',
  'sles-15',
  'sles-15-arm64',
];
local ubuntu_images = [
  'ubuntu-1804',
  'ubuntu-2004',
  'ubuntu-2204',
  'ubuntu-1804-arm64',
  'ubuntu-2004-arm64',
  'ubuntu-2204-arm64',
  'ubuntu-pro-1604',
  'ubuntu-pro-1804',
  'ubuntu-pro-2004',
  'ubuntu-pro-2204',
];


local exportjob = {
  local tl = self,

  image:: error 'must set image in exportjob',

  // Start of output.
  name: 'export-' + tl.image,
  plan: [
    { get: 'compute-image-tools' },
    { get: 'guest-test-infra' },
    {
      task: 'generate-timestamp',
      file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
    },
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    {
      task: 'generate-build-id',
      file: 'guest-test-infra/concourse/tasks/generate-build-id.yaml',
      vars: { prefix: tl.image },
    },
    {
      put: '%s-gcs' % tl.image,
      params: { file: 'build-id-dir/%s*' % tl.image },
      get_params: { skip_download: 'true' },
    },
    { load_var: 'gcs-url', file: '%s-gcs/url' % tl.image },
    {
      task: 'generate-build-date',
      file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
    },
    { load_var: 'build-date', file: 'publish-version/version' },
    {
      task: 'daisy-export-' + tl.image,
      file: 'guest-test-infra/concourse/tasks/daisy-export-images-partner.yaml',
      vars: {
        wf: 'partner/%s_export.wf.json' % underscore(tl.image),
        gcs_url: '((.:gcs-url))',
        build_date: '((.:build-date))',
      },
    },
  ],
  on_success: {
    task: 'success',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'partner-image-export',
      job: tl.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'failure',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'partner-image-export',
      job: tl.name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local publishjob = {
  local tl = self,

  environment:: error 'must set environment in publishjob',
  gcsdir:: error 'must set gcsdir in publishjob',
  image:: error 'must set image in publishjob',
  displayenv:: if tl.environment == 'oslogin-staging' then 'oslogin' else tl.environment,

  //Start of output.
  name: 'publish-%s-%s' % [
    tl.displayenv,
    tl.image,
  ],
  plan: [
    { get: 'guest-test-infra' },
    { get: 'compute-image-tools' },
    {
      task: 'generate-timestamp',
      file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
    },
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    {
      get: '%s-gcs' % tl.image,
      passed: ['export-' + tl.image],
      trigger: true,
      params: { skip_download: 'true' },
    },
    { load_var: 'source-version', file: '%s-gcs/version' % tl.image },
    {
      task: 'generate-version',
      file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
    },
    { load_var: 'publish-version', file: 'publish-version/version' },
    {
      task: 'publish-%s-%s' % [tl.displayenv, tl.image],
      file: 'guest-test-infra/concourse/tasks/daisy-publish-images.yaml',
      vars: {
        source_gcs_path: 'gs://gce-image-archive/partner/' + tl.gcsdir,
        source_version: 'v((.:source-version))',
        publish_version: '((.:publish-version))',
        wf: 'partner/%s.publish.json' % underscore(tl.image),
        environment: tl.environment,
      },
    },
  ],
  on_success: {
    task: 'success',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'partner-image-export',
      job: tl.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'failure',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'partner-image-export',
      job: tl.name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

{
  resource_types: [
    {
      name: 'gcs',
      type: 'registry-image',
      source: {
        repository: 'frodenas/gcs-resource',
      },
    },
  ],
  resources: [
    {
      name: 'compute-image-tools',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/compute-image-tools.git',
        branch: 'master',
      },
    },
    {
      name: 'guest-test-infra',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/guest-test-infra.git',
        branch: 'master',
      },
    },
  ] + [
    {
      name: '%s-gcs' % image,
      type: 'gcs',
      source: {
        bucket: 'gce-image-archive',
        regexp: 'partner/cos/%s-v([0-9]+).tar.gz' % image,
      },
    }
    for image in cos_images
  ] + [
    {
      name: '%s-gcs' % image,
      type: 'gcs',
      source: {
        bucket: 'gce-image-archive',
        regexp: 'partner/fedora/%s-v([0-9]+).tar.gz' % image,
      },
    }
    for image in fedora_images
  ] + [
    {
      name: '%s-gcs' % image,
      type: 'gcs',
      source: {
        bucket: 'gce-image-archive',
        regexp: 'partner/freebsd/%s-v([0-9]+).tar.gz' % image,
      },
    }
    for image in freebsd_images
  ] + [
    {
      name: '%s-gcs' % image,
      type: 'gcs',
      source: {
        bucket: 'gce-image-archive',
        regexp: 'partner/suse/%s-v([0-9]+).tar.gz' % image,
      },
    }
    for image in suse_images
  ] + [
    {
      name: '%s-gcs' % image,
      type: 'gcs',
      source: {
        bucket: 'gce-image-archive',
        regexp: 'partner/ubuntu/%s-v([0-9]+).tar.gz' % image,
      },
    }
    for image in ubuntu_images
  ],
  jobs: [
    exportjob { image: image }
    for image in cos_images + fedora_images + freebsd_images + suse_images + ubuntu_images
  ] + [
    publishjob { image: image, environment: 'oslogin-staging', gcsdir: 'cos' }
    for image in cos_images
  ] + [
    publishjob { image: image, environment: 'oslogin-staging', gcsdir: 'suse' }
    for image in suse_images
  ] + [
    publishjob { image: image, environment: 'oslogin-staging', gcsdir: 'ubuntu' }
    for image in ubuntu_images
  ] + [
    publishjob { image: image, environment: 'staging', gcsdir: 'cos' }
    for image in cos_images
  ] + [
    publishjob { image: image, environment: 'staging', gcsdir: 'fedora' }
    for image in fedora_images
  ] + [
    publishjob { image: image, environment: 'staging', gcsdir: 'freebsd' }
    for image in freebsd_images
  ] + [
    publishjob { image: image, environment: 'staging', gcsdir: 'suse' }
    for image in suse_images
  ] + [
    publishjob { image: image, environment: 'staging', gcsdir: 'ubuntu' }
    for image in ubuntu_images
  ] + [
  ],
  groups: [
    {
      name: 'cos',
      jobs: [
        'export-cos-85-lts',
        'export-cos-89-lts',
        'export-cos-93-lts',
        'export-cos-97-lts',
        'export-cos-dev',
        'publish-oslogin-cos-85-lts',
        'publish-oslogin-cos-89-lts',
        'publish-oslogin-cos-93-lts',
        'publish-oslogin-cos-97-lts',
        'publish-oslogin-cos-dev',
        'publish-staging-cos-85-lts',
        'publish-staging-cos-89-lts',
        'publish-staging-cos-93-lts',
        'publish-staging-cos-97-lts',
        'publish-staging-cos-dev',
      ],
    },
    {
      name: 'fedora',
      jobs: [
        'export-fedora-33',
        'export-fedora-34',
        'export-fedora-coreos-next',
        'export-fedora-coreos-stable',
        'export-fedora-coreos-testing',
        'publish-staging-fedora-33',
        'publish-staging-fedora-34',
        'publish-staging-fedora-coreos-next',
        'publish-staging-fedora-coreos-stable',
        'publish-staging-fedora-coreos-testing',
      ],
    },
    {
      name: 'freebsd',
      jobs: [
        'export-freebsd-11',
        'export-freebsd-12',
        'export-freebsd-13',
        'publish-staging-freebsd-11',
        'publish-staging-freebsd-12',
        'publish-staging-freebsd-13',
      ],
    },
    {
      name: 'suse',
      jobs: [
        'export-opensuse-leap-15',
        'export-opensuse-leap-15-arm64',
        'export-sles-12',
        'export-sles-15',
        'export-sles-15-arm64',
        'publish-oslogin-opensuse-leap-15',
        'publish-oslogin-opensuse-leap-15-arm64',
        'publish-oslogin-sles-12',
        'publish-oslogin-sles-15',
        'publish-oslogin-sles-15-arm64',
        'publish-staging-opensuse-leap-15',
        'publish-staging-opensuse-leap-15-arm64',
        'publish-staging-sles-12',
        'publish-staging-sles-15',
        'publish-staging-sles-15-arm64',
      ],
    },
    {
      name: 'ubuntu',
      jobs: [
        'export-ubuntu-1804',
        'export-ubuntu-2004',
        'export-ubuntu-2204',
        'export-ubuntu-1804-arm64',
        'export-ubuntu-2004-arm64',
        'export-ubuntu-2204-arm64',
        'export-ubuntu-pro-1604',
        'export-ubuntu-pro-1804',
        'export-ubuntu-pro-2004',
        'export-ubuntu-pro-2204',
        'publish-oslogin-ubuntu-1804',
        'publish-oslogin-ubuntu-2004',
        'publish-oslogin-ubuntu-2204',
        'publish-oslogin-ubuntu-1804-arm64',
        'publish-oslogin-ubuntu-2004-arm64',
        'publish-oslogin-ubuntu-2204-arm64',
        'publish-oslogin-ubuntu-pro-1604',
        'publish-oslogin-ubuntu-pro-1804',
        'publish-oslogin-ubuntu-pro-2004',
        'publish-oslogin-ubuntu-pro-2204',
        'publish-staging-ubuntu-1804',
        'publish-staging-ubuntu-2004',
        'publish-staging-ubuntu-2204',
        'publish-staging-ubuntu-1804-arm64',
        'publish-staging-ubuntu-2004-arm64',
        'publish-staging-ubuntu-2204-arm64',
        'publish-staging-ubuntu-pro-1604',
        'publish-staging-ubuntu-pro-1804',
        'publish-staging-ubuntu-pro-2004',
        'publish-staging-ubuntu-pro-2204',
      ],
    },
  ],
}
