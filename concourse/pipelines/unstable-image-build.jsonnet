// Imports
local arle = import '../templates/arle.libsonnet';
local common = import '../templates/common.libsonnet';
local daisy = import '../templates/daisy.libsonnet';

// Helper functions to fetch resources and tasks
local underscore(input) = std.strReplace(input, '-', '_');
local prod_bucket = 'artifact-releaser-prod-rtp';
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
      bucket: prod_bucket,
      regexp: 'debian/%s-v([0-9]+).tar.gz' % common.debian_image_prefixes[self.image],
    }
  else
    common.GcsImgResource(image.name, image.gcs_dir) {
      bucket: prod_bucket,
    }
);

local getshasumresource(image) = (
  if image.os_type == 'debian' then
    common.gcsshasumresource {
      image: image.name,
      image_prefix: common.debian_image_prefixes[image.name],
      shasum_destination: 'debian',
    }
  else
    common.gcsshasumresource {
      image: image.name,
      shasum_destination: image.gcs_dir,
    }
);


// Task template to build with UNSTABLE guest packages
local imgbuildtask = daisy.daisyimagetask {
  google_cloud_repo: 'unstable',
  gcs_url: '((.:gcs-url))',
  shasum_destination: '((.:shasum-destination))',
};

// Start of job
local imgbuildjob(image) = {
  local tl = self,

  image_name:: image.name,
  image_prefix:: if image.os_type == 'debian' then common.debian_image_prefixes[image.name] else image.name,
  workflow_dir:: image.workflow_dir,
  workflow:: '%s/%s.wf.json' % [tl.workflow_dir, underscore(tl.image_prefix)],
  zone:: get_zone(image.name),
  build_task:: imgbuildtask {
    workflow: tl.workflow,
    zone: tl.zone,
  },

  // Start of job
  name: 'build-unstable-%s' % [tl.image_prefix],
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
      vars: { prefix: tl.image_prefix, id: '((.:id))' },
    },
    {
      put: image.name + '-gcs',
      trigger: true,
      params: { file: 'build-id-dir/%s*' % tl.image_prefix },
      get_params: { skip_download: true },
    },
    {
      load_var: 'gcs-url',
      file: '%s-gcs/url' % tl.image_name,
    },
    {
      task: 'generate-build-id-shasum',
      file: 'guest-test-infra/concourse/tasks/generate-build-id-shasum.yaml',
      vars: { prefix: tl.image_prefix, id: '((.:id))' },
    },
    {
      put: tl.image_name + '-shasum',
      params: { file: 'build-id-dir-shasum/%s*' % tl.image_prefix },
      get_params: { skip_download: true },
    },
    {
      load_var: 'shasum-destination',
      file: '%s-shasum/url' % tl.image_name,
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
      pipeline: 'unstable-image-build',
      job: tl.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'build-failure-metric',
    config: common.publishresulttask {
      pipeline: 'unstable-image-build',
      job: tl.name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

// Task template that extends gcepublishtask to target a custom GCP project
local imgpublishtask = arle.gcepublishtask {
  local tl = self,

  target_project:: error 'must set target_project in imgpublishtask',

  run+: {
    args: [
      '-work_project=' + tl.target_project,
    ] + super.args,
  },
};

local imgpublishjob(image) = {
  local tl = self,  // tl = Top Level

  env:: 'test',
  image_name:: image.name,
  image_prefix:: if image.os_type == 'debian' then common.debian_image_prefixes[image.name] else image.name,
  workflow_dir:: if image.os_type == 'debian' then 'debian' else 'enterprise_linux',
  workflow:: if image.os_type != 'windows'
    then '%s/%s.publish.json' % [tl.workflow_dir, underscore(tl.image_prefix)]
    else '%s/%s' % [tl.workflow_dir, tl.image_name + '-uefi.publish.json'],
  gcs_dir:: image.gcs_dir,
  gcs_bucket:: prod_bucket,
  gcs:: 'gs://%s/%s' % [tl.gcs_bucket, tl.gcs_dir],

  // Start of job
  name: 'publish-unstable-%s-%s' % [tl.env, tl.image_name],
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
      get: image.name + '-gcs',
      trigger: true,
      params: { skip_download: true },
    },
    {
      load_var: 'source-version',
      file: image.name + '-gcs/version',
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
      task: 'unstable-gce-image-publish-' + image.name,
      config: imgpublishtask {
        environment: 'test',
        publish_version: 'v((.:source-version))',
        source_gcs_path: tl.gcs,
        source_version: 'v((.:source-version))',
        wf: tl.workflow,
        target_project: target_project,
      },
    },
  ],
  on_success: {
    task: 'publish-success-metric',
    config: common.publishresulttask {
      pipeline: 'unstable-image-build',
      job: tl.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'publish-failure-metric',
    config: common.publishresulttask {
      pipeline: 'unstable-image-build',
      job: tl.name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

// Start of output
{
  // Image Configurations
  local debian_images = [
    { name: name, os_type: 'debian', gcs_dir: 'debian', workflow_dir: 'debian' }
    for name in ['debian-11', 'debian-12', 'debian-12-arm64', 'debian-13', 'debian-13-arm64']
  ],
  local centos_images = [
    { name: name, os_type: 'centos', gcs_dir: 'centos', workflow_dir: 'enterprise-linux' }
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
    getimgresource(img)
    for img in all_images
  ] + [
    getshasumresource(img)
    for img in all_images
  ],
  jobs: [
    imgbuildjob(img)
    for img in all_images
  ] + [
    imgpublishjob(img)
    for img in all_images
  ],
}
