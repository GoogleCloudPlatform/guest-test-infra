// Imports.
local common = import '../templates/common.libsonnet';

// Get the repository given the build.
local get_repo(build) = if std.startsWith(build, 'deb') then 'guest-arle-autopush-trusty'
  else if std.startsWith(build, 'el') then 'guest-arle-autopush-el7-x86_64'
  else 'guest-arle-autopush';

// Get the universe given the build.
local get_universe(build) = if std.startsWith(build, 'deb') then 'cloud-apt'
  else if std.startsWith(build, 'el') then 'cloud-yum'
  else 'cloud-yuck';

local upload-arle-autopush-staging-task = {
  local tl = self,
  
  package:: error 'must set package in upload-arle-autopush-staging-task',
  build:: error 'must set build in upload-arle-autopush-staging-task',
  gcs_pkg_name:: error 'must set gcs_pkg_name in upload-arle-autopush-staging-task',
  file_ending:: error 'must set file_ending in upload-arle-autopush-staging-task',

  task: 'upload-arle-autopush-staging-%s-%s' % [tl.package, tl.build],
  params: {
    TOPIC: 'projects/artifact-releaser-autopush/topics/gcp-guest-package-upload-autopush',
    TYPE: 'uploadToStaging',
  },
  file: 'guest-test-infra/concourse/tasks/gcloud-package-operation.yaml',
  vars: {
    package_paths: '{"bucket": "%s", "object": "%s/%s_((.:package-version))%s"}' % [
        common.prod_package_bucket,
        tl.package,
        tl.gcs_pkg_name,
        tl.file_ending, 
      ],
    repo: get_repo(tl.build),
    universe: get_universe(tl.build),
  },
};

local upload-arle-autopush-staging = {
  local tl = self

  package:: error 'must set package in upload-arle-autopush-staging',
  builds:: error 'must set build in upload-arle-autopush-staging',
  gcs_pkg_name:: error 'must set gcs_pkg_name in upload-arle-autopush-staging',
  file_endings:: error 'must set file_endings in upload-arle-autopush-staging',

  if std.length(builds) != std.length(file_endings) 
    then error 'file_endings and builds must be of same length',

  plan: [
    { get: 'guest-test-infra' },
    { get: 'compute-image-tools' },
    { 
      get: '%s-%s-gcs' % [tl.package, tl.build],
      trigger: true,
      params: { skip_download: true },
    },
    { 
      task: 'generate-timestamp',
      file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
    },
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    { load_var: 'package-version', file: '%s-%s-gcs/version' % [tl.package, tl.build] },
    {
      in_parallel: {
        steps: [
          upload-arle-autopush-staging-task {
            package: tl.package,
            build: tl.builds[i],
            gcs_dir: tl.gcs_pkg_name,
            file_ending: tl.file_endings[i],
          }
          for i in std.range(0, std.length(builds) - 1)
        ],
      },
    },
  ],
  on_success: common.publishresulttask {
    pipeline: 'artifact-releaser-test',
    job: 'upload-arle-autopush-staging-%s-%s' % [tl.package, tl.build],
    result_state: 'success',
    start_timestamp: '((.:start-timestamp-ms))',
  },
  on_failure: common.publishresulttask {
    pipeline: 'artifact-releaser-test',
    job: 'upload-arle-autopush-staging-%s-%s' % [tl.package, tl.build],
    result_state: 'success',
    start_timestamp: '((.:start-timestamp-ms))',
  },
};

local promote-arle-autopush-stable = {
  local tl = self,
  
  package:: error 'must set package in promote-arle-autopush-stable',
  build:: error 'muset set build in promote-arle-autopush-stable',
  passed:: 'upload-arle-autopush-staging-%s-%s' % [tl.package, tl.build],
  repo:: get_repo(tl.build),
  universe:: get_universe(tl.build),

  plan: [
    { get: 'guest-test-infra' },
    { get: 'compute-image-tools' },
    {
      task: 'generate-timestamp',
      file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml'
    },
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    {
      in_parallel: {
        steps: [
          on_success: {
            task: 'publish-success-metric',
            config: common.publishresulttask {
              pipeline: 'artifact-releaser-test',
              job: 'promote-arle-autopush-stable-%s' % tl.package,
              result_state: 'success',
              start_timestamp: '((.:start-timestamp-ms))',
            },
          },
          on_failure: {
            task: 'publish-failure-metric',
            config: common.publishresulttask {
              pipeline: 'artifact-releaser-test',
              job: 'promote-arle-autopush-stable-%s' % tl.package,
              result_state: 'failure',
              start_timestamp: '((.:start-timestamp-ms))',
            },
          },
          {
            file: 'guest-test-infra/concourse/tasks/gcloud-promote-package.yaml',
            params: {
              TOPIC: 'projects/artifact-releaser-autopush/topics/gcp-guest-package-promote-autopush',
            },
            vars: {
              environment: 'stable',
              repo: get_repo(tl.build),
              universe: get_universe(tl.build),
            },
          },
        }
      ]
    },
  ],
};

local arle-publish-images-autopush = {
  local tl = self,
  image:: error 'must set image in arle-publish-images-autopush',
  gcs_path:: error 'must set gcs_path in arle-publish-images-autopush',
  wf:: error 'must set wf in arle-publish-images-autopush',
  
  plan: [
    { get: 'guest-test-infra' },
    { get: '%s-gcs' % tl.image },
    { task: 'generate-timestamp', file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml'},
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    { load_var: 'source_version', file: '%s-gcs/version' % tl.image },
    { task: 'generate-version', file: 'guest-test-infra/concourse/tasks/generate-version.yaml' },
    { load_var: 'publish-version', file: 'publish-version/version' },
    {
      task: 'publish-%s' % tl.image,
      file: 'guest-test-infra/concourse/tasks/gcloud-publish-image.yaml',
      vars: {
        topic: 'projects/artifact-releaser-autopush/topics/gcp-guest-image-release-autopush',
        image_name: tl.image,
        gcs_image_path: 'gs://artifact-releaser-autopush-rtp/%s' % tl.gcs_path,
        wf: tl.wf,
        publish_version: '((.:publish-version))',
        source_version: '((.:source-version))',
        release_notes: 'Disregard this release. %s test release.' % tl.image,
      },
    },
  ],
};

{
  // Image names.
  local almalinux = ['almalinux-8', 'almalinux-9'],
  local debian = ['debian-10', 'debian-11', 'debian-11-arm64', 'debian-12', 'debian-12-arm64'],
  local centos = ['centos-7', 'centos-stream-8', 'centos-stream-9'],
  local rhel = [
    'rhel-7',
    'rhel-7-byos',
    'rhel-8',
    'rhel-8-byos',
    'rhel-9',
    'rhel-9-arm64',
    'rhel-9-byos',
  ],
  local rocky_linux = [
    'rocky-linux-8',
    'rocky-linux-8-optimized-gcp',
    'rocky-linux-8-optimized-gcp-arm64',
    'rocky-linux-9',
    'rocky-linux-9-arm64',
    'rocky-linux-9-optimized-gcp',
    'rocky-linux-9-optimized-gcp-arm64',
  ],
  
  // All resources.
  resources: [
    // Guest Agent resources.
    common.gcspkgresource { package: 'guest-agent', build: 'deb10', regexp: 'guest-agent/google-guest-agent_([0-9]+).00-g1_amd64.deb'},
    common.gcspkgresource { package: 'guest-agent', build: 'deb11-arm64', regexp: 'guest-agent/google-guest-agent_([0-9]+).00-g1_arm64.deb' },
    common.gcspkgresource { package: 'guest-agent', build: 'el7', regexp: 'guest-agent/google-guest-agent-([0-9]+).00-el7.x86_64.rpm'},
    common.gcspkgresource { package: 'guest-agent', build: 'el8', regexp: 'guest-agent/google-guest-agent-([0-9]+).00-el8.x86_64.rpm' },
    common.gcspkgresource { package: 'guest-agent', build: 'el8-arch64', regexp: 'guest-agent/google-guest-agent-([0-9]+).00-el8.aarch64.rpm' },
    common.gcspkgresource { package: 'guest-agent', build: 'el9', regexp: 'guest-agent/google-guest-agent-([0-9]+).00-el9.x86_64.rpm' },
    common.gcspkgresource { package: 'guest-agent', build: 'el9-arch64', regexp: 'guest-agent/google-guest-agent-([0-9]+).00-el9.aarch64.rpm' },
  ] +
  [
    // OSLogin resources.
    common.gcspkgresource { package: 'oslogin', build: 'deb10', regexp: 'oslogin/google-compute-engine-oslogin_([0-9]+).00-g1+deb10_amd64.deb' },
    common.gcspkgresource { package: 'oslogin', build: 'deb11', regexp: 'oslogin/google-compute-engine-oslogin_([0-9]+).00+deb11_amd64.deb' },
    common.gcspkgresource { package: 'oslogin', build: 'deb11-arm64', regexp: 'oslogin/google-compute-engine-oslogin_([0-9]+).00-g1+deb11_arm64.deb' },
    common.gcspkgresource { package: 'oslogin', build: 'deb12', regexp: 'oslogin/google-compute-engine-oslogin_([0-9]+).00-g1+deb12_amd64.deb' },
    common.gcspkgresource { package: 'oslogin', build: 'deb12-arm64', regexp: 'oslogin/google-compute-engine-oslogin_([0-9]+).00-g1+deb12_arm64.deb' },
    common.gcspkgresource { package: 'oslogin', build: 'el7', regexp: 'oslogin/google-compute-engine-oslogin_([0-9]+).00-g1.el7.x86_64.rpm' },
    common.gcspkgresource { package: 'oslogin', build: 'el8', regexp: 'oslogin/google-compute-engine-oslogin_([0-9]+).00-g1.el8.x86_64.rpm' },
    common.gcspkgresource { package: 'oslogin', build: 'el8-arch64', regexp: 'oslogin/google-compute-engine-oslogin_([0-9]+).00-g1.el8.aarch64.rpm' },
    common.gcspkgresource { package: 'oslogin', build: 'el9', regexp: 'oslogin/google-compute-engine-oslogin_([0-9]+).00-g1.el9.x86_64.rpm' },
    common.gcspkgresource { package: 'oslogin', build: 'el9-arch64', regexp: 'oslogin/google-compute-engine-oslogin_([0-9]+).00-g1.el9.aarch64.rpm' },
  ] +
  [
    // OSConfig resources.
    common.gcspkgresource { package: 'osconfig', build: 'deb10', regexp: 'osconfig/google-osconfig-agent-([0-9]+).00-g1_amd64.deb' },
    common.gcspkgresource { package: 'osconfig', build: 'deb11-arm64', regexp: 'osconfig/google-osconfig-agent-([0-9]+).00-g1_arm64.deb' },
    common.gcspkgresource { package: 'osconfig', build: 'el7', regexp: 'osconfig/google-osconfig-agent-([0-9]+).00-g1.el7.x86_64.rpm' },
    common.gcspkgresource { package: 'osconfig', build: 'el8', regexp: 'osconfig/google-osconfig-agent-([0-9]+).00-g1.el8.x86_64.rpm' },
    common.gcspkgresource { package: 'osconfig', build: 'el8-arch64', regexp: 'osconfig/google-osconfig-agent-([0-9]+).00-g1.el8.aarch64.rpm' },
    common.gcspkgresource { package: 'osconfig', build: 'el9', regexp: 'osconfig/google-osconfig-agent-([0-9]+).00-g1.el9.x86_64.rpm' },
    common.gcspkgresource { package: 'osconfig', build: 'el9-arch64', regexp: 'osconfig/google-osconfig-agent-([0-9]+).00-g1.el9.aarch64.rpm' },
    common.gcspkgresource { package: 'osconfig', build: 'goo', regexp: 'osconfig/google-osconfig-agent.x86_64.([0-9]+).00.0+win@1.goo' },
  ] +
  [
    // Guest Diskexpand resources.
    common.gcspkgresource { package: 'gce-disk-expand', build: 'deb10', regexp: 'gce-disk-expand/gce-disk-expand-([0-9]+).00-g1_all.deb' },
    common.gcspkgresource { package: 'gce-disk-expand', build: 'el7', regexp: 'gce-disk-expand/gce-disk-expand-([0-9]+).00-g1.el7.noarch.rpm' },
    common.gcspkgresource { package: 'gce-disk-expand', build: 'el8', regexp: 'gce-disk-expand/gce-disk-expand-([0-9]+).00-g1.el8.noarch.rpm' },
    common.gcspkgresource { package: 'gce-disk-expand', build: 'el8', regexp: 'gce-disk-expand/gce-disk-expand-([0-9]+).00-g1.el9.noarch.rpm' },
  ] +
  [
    // Guest Config resources.
    common.gcspkgresource { package: 'google-compute-engine', build: 'deb10', regexp: 'google-compute-engine/google-compute-engine_([0-9]+).00-g1_all.deb' },
    common.gcspkgresource { package: 'google-compute-engine', build: 'el7', regexp: 'google-compute-engine/google-compute-engine_([0-9]+).00-g1.el7.noarch.rpm' },
    common.gcspkgresource { package: 'google-compute-engine', build: 'el8', regexp: 'google-compute-engine/google-compute-engine_([0-9]+).00-g1.el8.noarch.rpm' },
    common.gcspkgresource { package: 'google-compute-engine', build: 'el9', regexp: 'google-compute-engine/google-compute-engine_([0-9]+).00-g1.el9.noarch.rpm' },
  ] +
  [
    // Artifact Registry Yum Plugin resources.
    common.gcspkgresource { package: 'artifact-registry-yum-plugin', build: 'el7', regexp: 'yum-plugin-artifact-registry/yum-plugin-artifact-registry_([0-9]+).00.el7.x86_64.rpm' },
    common.gcspkgresource { package: 'artifact-registry-yum-plugin', build: 'el8', regexp: 'yum-plugin-artifact-registry/dnf-plugin-artifact-registry_([0-9]+).00.el8.x86_64.rpm' },
    common.gcspkgresource { package: 'artifact-registry-yum-plugin', build: 'el8-arch64', regexp: 'yum-plugin-artifact-registry/dnf-plugin-artifact-registry_([0-9]+).00.el8.aarch64.rpm' },
    common.gcspkgresource { package: 'artifact-registry-yum-plugin', build: 'el9', regexp: 'yum-plugin-artifact-registry/dnf-plugin-artifact-registry_([0-9]+).00.el9.x86_64.rpm' },
    common.gcspkgresource { package: 'artifact-registry-yum-plugin', build: 'el9-arch64', regexp: 'yum-plugin-artifact-registry/dnf-plugin-artfact-registry_([0-9]+).00.el9.aarch64.rpm' },
  ] +
  [
    // Artifact Registry Apt Transport resources.
    common.gcspkgresource { package: 'artifact-registry-apt-transport', build: 'deb10', regexp: 'apt-transport-artifact-registry/apt-transport-artifact-registry_([0-9]+).00-g1_amd64.deb' },
    common.gcspkgresource { package: 'artifact-registry-apt-transport', build: 'deb10-arm64', regexp: 'apt-transport-artifact-registry/apt-transport-artifact-registry_([0-9]+).00-g1_arm64.deb' },
    common.gcspkgresource { package: 'artifact-registry-apt-transport', build: 'deb11-arm64', regexp: 'apt-transport-artifact-registry/apt-transport-artifact-registry_([0-9]+).00-g1_arm64.deb' },
  ] +
  [
    // Compute Image Windows resources.
    common.gcspkgresource { package: 'certgen', regexp: 'compute-image-windows/certgen.x86_64.x86_64.([0-9]+).00.0@1.goo' },
    common.gcspkgresource { package: 'auto-updater', regexp: 'compute-image-windows/google-compute-engine-auto-updater.noarch.([0-9]+).00@1.goo' },
    common.gcspkgresource { package: 'powershell', regexp: 'compute-image-windows/google-compute-engine-powershell.noarch.([0-9]+).00@1.goo' },
    common.gcspkgresource { package: 'sysprep', regexp: 'compute-image-windows/google-compute-engine-sysprep.noarch.([0-9]+).00@1.goo' },
    common.gcspkgresource { package: 'ssh', regexp: 'compute-image-windows/google-compute-engine-ssh.x86_64.([0-9]+).00.0@1.goo' },
  ] +
  [
    // Diagnostics
    common.gcspkgresource { package: 'diagnostics', regexp: 'compute-image-tools/google-compute-engine-diagnostics.x86_64.([0-9]+).00.0@0.goo' },
  ] +
  // Image resources
  [ common.gcsimageresource { image: image, gcs_dir: 'almalinux' } for image in almalinux ] +
  [ common.gcsimageresource { image: image, gcs_dir: 'rocky-linux' } for image in rocky_linux ] +
  [ common.gcsimageresource { 
                              image: image,
                              regexp: 'debian/%s-v([0-9]+).tar.gz' % common.debian_image_prefixes[self.image]
                            } for image in debian_images ] +
  [ common.gcsimageresource { image: image, gcs_dir: 'centos' } for image in centos ] +
  [ common.gcsimageresource { image: image, gcs_dir: 'rhel' } for image in rhel ],
  
  // Run jobs.
  jobs: [
    upload-arle-autopush-staging {
      package: 'guest-agent',
      builds: ['deb10', 'deb11-arm64', 'el7', 'el8', 'el8-arm64', 'el9', 'el9-arm64', 'goo'],
      gcs_pkg_name: 'guest-agent',
      file_endings: [
        '.00-g1_amd64.deb',
        '.00-g1_arm64.deb',
        '.00-g1.el7.x86_64.rpm',
        '.00-g1.el8.x86_64.rpm',
        '.00-g1.el8.aarch64.rpm',
        '.00-g1.el9.x86_64.rpm',
        '.00-g1.el9.aarch64.rpm',
        '.00.0@1.goo',
      ],
    },
  ],
}
