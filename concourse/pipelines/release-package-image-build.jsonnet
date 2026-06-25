// Imports
local arle = import '../templates/arle.libsonnet';
local common = import '../templates/common.libsonnet';
local daisy = import '../templates/daisy.libsonnet';

// Helper functions to fetch resources and tasks
local underscore(input) = std.strReplace(input, '-', '_');
local target_project = 'gce-unstable-pkg-test-images';

local build_zones = ['us-central1-b', 'europe-west1-b', 'europe-west4-b', 'asia-east1-a'];
local arm_build_zones = ['us-central1-b', 'europe-west4-a', 'europe-west4-b'];
local string_hash(s) = std.foldl(function(acc, c) acc + std.codepoint(c), std.stringChars(s), 0);
local get_zone(image) =
  local zones = if std.member(image, '-arm64') then arm_build_zones else build_zones;
  zones[std.mod(string_hash(image), std.length(zones))];

local getimgresource(image) = (
  if image.os_type == 'debian' then
    common.gcsimgresource {
      image: image.name,
      bucket: common.prod_bucket,
      regexp: 'debian/%s-v([0-9]+).tar.gz' % common.debian_image_prefixes[self.image],
    }
  else
    common.GcsImgResource(image.name, image.gcs_dir) {
      bucket: common.prod_bucket,
    }
);
local getunstableimgresource(image) = (
  if image.os_type == 'debian' then
    common.gcsimgresource {
      image: image.name,
      name: '%s-unstable-gcs' % [self.image],
      bucket: common.prod_bucket,
      regexp: 'debian-unstable/%s-v([0-9]+).tar.gz' % common.debian_image_prefixes[self.image],
    }
  else
    common.GcsImgResource(image.name, image.gcs_dir + '-unstable') {
      name: '%s-unstable-gcs' % [self.image],
      bucket: common.prod_bucket,
    }
);

// Task template to build with UNSTABLE guest packages
local imgbuildtask = daisy.daisyimagetask {
  gcs_url: '((.:gcs-url))',
};

// Start of job
local imgbuildjob = {
  local tl = self,

  image_name:: self.image.name,
  image_prefix:: if self.image.os_type == 'debian' then common.debian_image_prefixes[self.image.name] else self.image.name,
  workflow_dir:: self.image.workflow_dir,
  workflow:: if self.image.workflow_dir == 'windows' then '%s/%s' % [tl.workflow_dir, tl.image_name + '-uefi.wf.json']
    else if self.image.workflow_dir == 'sqlserver' then '%s/%s.wf.json' % [tl.workflow_dir, tl.image_name]
    else '%s/%s.wf.json' % [tl.workflow_dir, underscore(tl.image_prefix)],
  zone:: get_zone(tl.image_name),
  build_task:: imgbuildtask {
    workflow: tl.workflow,
    zone: tl.zone,
    vars+: ['google_cloud_repo=unstable'],
  },

  // Start of job
  name: 'build-release-package-testing-%s' % tl.image_name,
  plan: [
    { get: 'compute-image-tools' },
    { get: 'guest-test-infra' },
    {
      get: '%s-gcs' % [tl.image_name],
      trigger: true,
      params: { skip_download: 'true' },
    },
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
      vars: { prefix: tl.image_prefix, id: '((.:id))' },
    },
    {
      put: '%s-unstable-gcs' % [tl.image_name],
      params: { file: 'build-id-dir/%s*' % tl.image_prefix },
      get_params: { skip_download: 'true' },
    },
    {
      load_var: 'gcs-url',
      file: '%s-unstable-gcs/url' % tl.image_name,
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
      task: 'daisy-build-' + tl.image_name,
      config: tl.build_task,
    },
  ],
  on_success: {
    task: 'build-success-metric',
    config: common.publishresulttask {
      pipeline: 'release-package-image-build',
      job: tl.image_name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'build-failure-metric',
    config: common.publishresulttask {
      pipeline: 'release-package-image-build',
      job: tl.image_name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local imgpublishjob = {
  local tl = self,  // tl = Top Level

  env:: 'package',
  image_name:: self.image.name,
  image_prefix:: if self.image.os_type == 'debian' then common.debian_image_prefixes[self.image.name] else self.image.name,
  workflow_dir:: self.image.workflow_dir,
  workflow:: if self.image.os_type != 'windows'
    then '%s/%s.publish.json' % [tl.workflow_dir, underscore(tl.image_prefix)]
    else '%s/%s' % [tl.workflow_dir, tl.image_name + '-uefi.publish.json'],
  gcs:: 'gs://%s/%s' % [tl.gcs_bucket, tl.gcs_dir],
  gcs_dir:: self.image.gcs_dir,
  gcs_bucket:: common.prod_bucket,

  // Start of job
  name: 'publish-to-release-package-testing-%s-%s' % [tl.env, tl.image_name],
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
      get: '%s-unstable-gcs' % [tl.image_name],
      passed: ['build-release-package-testing-%s' % tl.image_name],
      trigger: true,
      params: { skip_download: 'true' },
    },
    {
      load_var: 'source-version',
      file: tl.image_name + '-unstable-gcs/version',
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
      task: 'publish-release-package-testing-' + tl.image_name,
      config: arle.gcepublishtask {
        environment: tl.env,
        publish_version: 'v((.:publish-version))',
        source_gcs_path: tl.gcs,
        source_version: 'v((.:source-version))',
        wf: tl.workflow,
      },
    },
  ],
  on_success: {
    task: 'publish-success-metric',
    config: common.publishresulttask {
      pipeline: 'release-package-testing-image-build',
      job: tl.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'publish-failure-metric',
    config: common.publishresulttask {
      pipeline: 'release-package-testing-image-build',
      job: tl.name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local imggroup = {
    local tl = self,

    images:: error 'must set images in imggroup',
    env:: 'package',

    name: error 'must set name in imggroup',
    jobs: [
      'build-release-package-testing-' + image
      for image in tl.images
    ] + [
      'publish-to-release-package-testing-%s-%s' % [tl.env, image]
      for image in tl.images
    ],
  };

// Start of output
{
  // Image Configurations
  local debian_images = [
    { name: name, os_type: 'debian', gcs_dir: 'debian', workflow_dir: 'debian' }
    for name in ['debian-11', 'debian-12', 'debian-12-arm64', 'debian-13', 'debian-13-arm64']
  ],
  local centos_images = [
    { name: name, os_type: 'centos', gcs_dir: 'centos', workflow_dir: 'enterprise_linux' }
    for name in ['centos-stream-9', 'centos-stream-9-arm64', 'centos-stream-10', 'centos-stream-10-arm64']
  ],
  local rhel_8_base_image_names = [
    'rhel-8',
    'rhel-8-arm64',
    'rhel-8-byos',
    'rhel-8-byos-arm64',
    'rhel-8-lvm',
    'rhel-8-lvm-byos',
    'rhel-8-lvm-arm64',
    'rhel-8-lvm-byos-arm64',
  ],
  local rhel_8_sap_image_names = [
    'rhel-8-8-sap',
    'rhel-8-8-sap-byos',
    'rhel-8-10-sap',
    'rhel-8-10-sap-byos',
  ],
  local rhel_9_base_image_names = [
    'rhel-9',
    'rhel-9-arm64',
    'rhel-9-lvm',
    'rhel-9-lvm-byos',
    'rhel-9-lvm-arm64',
    'rhel-9-lvm-byos-arm64',
    'rhel-9-byos',
    'rhel-9-byos-arm64',
  ],
  local rhel_9_sap_image_names = [
    'rhel-9-2-sap',
    'rhel-9-2-sap-byos',
    'rhel-9-4-sap',
    'rhel-9-4-sap-byos',
    'rhel-9-6-sap',
    'rhel-9-6-sap-byos',
    'rhel-9-8-sap',
    'rhel-9-8-sap-byos',
  ],
  local rhel_9_eus_image_names = [
    'rhel-9-6-eus',
    'rhel-9-6-eus-arm64',
    'rhel-9-6-eus-byos',
    'rhel-9-6-eus-byos-arm64',
    'rhel-9-8-eus',
    'rhel-9-8-eus-arm64',
    'rhel-9-8-eus-byos',
    'rhel-9-8-eus-byos-arm64',
  ],
  local rhel_9_eus_lvm_image_names = [
    'rhel-9-6-eus-lvm',
    'rhel-9-6-eus-lvm-byos',
    'rhel-9-6-eus-lvm-arm64',
    'rhel-9-6-eus-lvm-byos-arm64',
    'rhel-9-8-eus-lvm',
    'rhel-9-8-eus-lvm-arm64',
    'rhel-9-8-eus-lvm-byos',
    'rhel-9-8-eus-lvm-byos-arm64',
  ],
  local rhel_10_base_image_names = [
    'rhel-10',
    'rhel-10-arm64',
    'rhel-10-byos',
    'rhel-10-byos-arm64',
    'rhel-10-lvm',
    'rhel-10-lvm-byos',
    'rhel-10-lvm-arm64',
    'rhel-10-lvm-byos-arm64',
  ],
  local rhel_10_sap_image_names = [
    'rhel-10-0-sap',
    'rhel-10-0-sap-byos',
    'rhel-10-2-sap',
    'rhel-10-2-sap-byos',
  ],
  local rhel_10_eus_image_names = [
    'rhel-10-0-eus',
    'rhel-10-0-eus-arm64',
    'rhel-10-0-eus-byos',
    'rhel-10-0-eus-byos-arm64',
    'rhel-10-2-eus',
    'rhel-10-2-eus-arm64',
    'rhel-10-2-eus-byos',
    'rhel-10-2-eus-byos-arm64',
    'rhel-10-2-eus-gvnic-baremetal',
    'rhel-10-2-eus-gvnic-baremetal-byos',
  ],
  local rhel_10_eus_lvm_image_names = [
    'rhel-10-0-eus-lvm',
    'rhel-10-0-eus-lvm-byos',
    'rhel-10-0-eus-lvm-arm64',
    'rhel-10-0-eus-lvm-byos-arm64',
    'rhel-10-2-eus-lvm',
    'rhel-10-2-eus-lvm-arm64',
    'rhel-10-2-eus-lvm-byos',
    'rhel-10-2-eus-lvm-byos-arm64',
    'rhel-10-2-eus-lvm-gvnic-baremetal',
    'rhel-10-2-eus-lvm-gvnic-baremetal-byos',
  ],
  local rhel_image_names = rhel_8_base_image_names + rhel_8_sap_image_names + rhel_9_base_image_names + rhel_9_sap_image_names + rhel_9_eus_image_names + rhel_9_eus_lvm_image_names + rhel_10_base_image_names + rhel_10_sap_image_names + rhel_10_eus_image_names + rhel_10_eus_lvm_image_names,
  local rhel_images = [
    { name: name, os_type: 'rhel', gcs_dir: 'rhel', workflow_dir: 'enterprise_linux' }
    for name in rhel_image_names
  ],
  local windows_image_names = [
    'windows-11-23h2-ent-x64', // EOL after Nov 10, 2026
    'windows-11-24h2-ent-x64', // EOL after Oct 12, 2027
    'windows-11-25h2-ent-x64',
    'windows-server-2016-dc',
    'windows-server-2016-dc-core',
    'windows-server-2019-dc',
    'windows-server-2019-dc-core',
    'windows-server-2022-dc',
    'windows-server-2022-dc-core',
    'windows-server-2025-dc',
    'windows-server-2025-dc-core',
  ],
  local windows_images = [
    { name: name, os_type: 'windows', gcs_dir: 'windows-uefi', workflow_dir: 'windows' }
    for name in windows_image_names
  ],
  local sql_image_names = [
    'sql-2016-enterprise-windows-2016-dc',
    'sql-2016-enterprise-windows-2019-dc',
    'sql-2016-standard-windows-2016-dc',
    'sql-2016-standard-windows-2019-dc',
    'sql-2016-web-windows-2016-dc',
    'sql-2016-web-windows-2019-dc',
    'sql-2017-enterprise-windows-2016-dc',
    'sql-2017-enterprise-windows-2019-dc',
    'sql-2017-enterprise-windows-2022-dc',
    'sql-2017-enterprise-windows-2025-dc',
    'sql-2017-express-windows-2016-dc',
    'sql-2017-express-windows-2019-dc',
    'sql-2017-standard-windows-2016-dc',
    'sql-2017-standard-windows-2019-dc',
    'sql-2017-standard-windows-2022-dc',
    'sql-2017-standard-windows-2025-dc',
    'sql-2017-web-windows-2016-dc',
    'sql-2017-web-windows-2019-dc',
    'sql-2017-web-windows-2022-dc',
    'sql-2017-web-windows-2025-dc',
    'sql-2019-enterprise-windows-2019-dc',
    'sql-2019-enterprise-windows-2022-dc',
    'sql-2019-enterprise-windows-2025-dc',
    'sql-2019-standard-windows-2019-dc',
    'sql-2019-standard-windows-2022-dc',
    'sql-2019-standard-windows-2025-dc',
    'sql-2019-web-windows-2019-dc',
    'sql-2019-web-windows-2022-dc',
    'sql-2019-web-windows-2025-dc',
    'sql-2022-enterprise-windows-2019-dc',
    'sql-2022-enterprise-windows-2022-dc',
    'sql-2022-enterprise-windows-2025-dc',
    'sql-2022-standard-windows-2019-dc',
    'sql-2022-standard-windows-2022-dc',
    'sql-2022-standard-windows-2025-dc',
    'sql-2022-web-windows-2019-dc',
    'sql-2022-web-windows-2022-dc',
    'sql-2022-web-windows-2025-dc',
    'sql-2025-enterprise-windows-2025-dc',
    'sql-2025-enterprise-windows-2022-dc',
    'sql-2025-enterprise-windows-2019-dc',
    'sql-2025-standard-windows-2025-dc',
    'sql-2025-standard-windows-2022-dc',
    'sql-2025-standard-windows-2019-dc',
  ],
  local sql_images = [
    { name: name, os_type: 'windows', gcs_dir: 'sqlserver-uefi', workflow_dir: 'sqlserver' }
    for name in sql_image_names
  ],
  
  local all_images = debian_images + centos_images + rhel_images + windows_images + sql_images,

  resource_types: [
    {
      name: 'gcs',
      type: 'registry-image',
      source: { repository: 'frodenas/gcs-resource' },
    },
    {
      name: 'registry-image-forked',
      type: 'registry-image',
      source: { repository: 'gcr.io/compute-image-tools/registry-image-forked' },
    },
  ],
  resources: [
    common.gitresource { name: 'compute-image-tools' },
    common.gitresource { name: 'guest-test-infra' },
  ] + [
    // Tracking public releases
    getimgresource(img)
    for img in all_images
  ] + [
    // Tracking unstable releases
    getunstableimgresource(img)
    for img in all_images
  ],
  jobs: [
    imgbuildjob {
      image:: image,
    }
    for image in all_images
  ] + [
    imgpublishjob {
      image:: image,
    }
    for image in all_images
  ],
  groups: [
    imggroup { name: 'centos', images: [img.name for img in centos_images] },
    imggroup { name: 'debian', images: [img.name for img in debian_images] },
    imggroup { name: 'rhel-8-base', images: rhel_8_base_image_names },
    imggroup { name: 'rhel-8-sap', images: rhel_8_sap_image_names },
    imggroup { name: 'rhel-9-base', images: rhel_9_base_image_names },
    imggroup { name: 'rhel-9-sap', images: rhel_9_sap_image_names },
    imggroup { name: 'rhel-9-eus', images: rhel_9_eus_image_names },
    imggroup { name: 'rhel-9-eus-lvm', images: rhel_9_eus_lvm_image_names },
    imggroup { name: 'rhel-10-base', images: rhel_10_base_image_names },
    imggroup { name: 'rhel-10-sap', images: rhel_10_sap_image_names },
    imggroup { name: 'rhel-10-eus', images: rhel_10_eus_image_names },
    imggroup { name: 'rhel-10-eus-lvm', images: rhel_10_eus_lvm_image_names },
    imggroup { name: 'windows', images: windows_image_names },
    imggroup { name: 'windows-sql', images: sql_image_names },
  ]
}
