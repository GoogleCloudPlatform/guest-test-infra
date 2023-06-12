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

local generatetimestamptask = {
  task: 'generate-timestamp',
  config: {
    platform: 'linux',
    image_resource: {
      type: 'registry-image',
      source: { repository: 'bash' },
    },
    outputs: [{ name: 'timestamp' }],
    run: {
      path: '/usr/local/bin/bash',
      args: [
        '-c',
        'timestamp=$((${EPOCHREALTIME/./}/1000)); echo $(($timestamp/1000)) | tee timestamp/timestamp; echo $timestamp | tee timestamp/timestamp-ms',
      ],
    },
  },
};

local upload_arle_autopush_task = {
  local tl = self,

  package_paths:: error 'must set package_paths in upload_arle_autopush_task',
  topic:: 'projects/artifact-releaser-autopush/topics/gcs-guest-package-upload-autopush',
  type:: error 'must set type in upload_arle_autopush_task',
  universe:: error 'must set universe in upload_arle_autopush_task',
  repo:: error 'must set repo in upload_arle_autopush_task',

  task: 'upload_arle_autopush_task',
  config: {
    platform: 'linux',
    image_resource: {
      type: 'image-registry',
      source: { repository: 'google/cloud-sdk', tag: 'alpine' },
    },
    run: {
      path: 'gcloud',
      args: [
        'pubsub',
        'topics',
        'publish',
        tl.topic,
        '--message',
        '{"type:" "%s", "request": {"gcsfiles": ["%s"], "universe": "%s", "repo": "%s"}}' % [
          tl.type,
          tl.package_paths,
          tl.universe,
          tl.repo,
        ],
      ],
    },
  },
};

local upload_arle_autopush_staging = {
  local tl = self,

  package:: error 'must set package in upload_arle_autopush_staging',
  builds:: error 'must set builds in upload_arle_autopush_staging',
  gcs_pkg_name:: error 'must set gcs_pkg_name in upload_arle_autopush_staging',
  file_endings:: error 'must set file_endings in upload_arle_autopush_staging',

  name: 'upload-arle-autopush-staging-%s' % tl.package,
  plan: [
    { get: 'guest-test-infra' },
    { get: 'compute-image-tools' },
    generatetimestamptask,
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    {
      in_parallel: {
        steps: [
          {
            get: '%s-%s-gcs' % [tl.package, build],
            trigger: true,
            params: { skip_download: true },
          }
          for build in tl.builds
        ],
      },
    },
    {
      in_parallel: {
        steps: [
          {
            load_var: 'package-version-%s' % build,
            file: '%s-%s-gcs/version' % [tl.package, build],
          }
          for build in tl.builds
        ],
      },
    },
    {
      in_parallel: {
        steps: [
          upload_arle_autopush_task {
            task: 'upload-arle-autopush-staging-%s-%s' % [tl.package, tl.builds[i]],
            package_paths: '{"bucket": "%s", "object": "%s/%s_((.:package-version)).00%s"}' % [
              common.prod_package_bucket,
              tl.package,
              tl.gcs_pkg_name,
              tl.file_endings[i],
            ],
            type: 'uploadToStaging',
            repo: get_repo(tl.builds[i]),
            universe: get_universe(tl.builds[i]),
          }
          for i in std.range(0, std.length(tl.builds) - 1)
        ],
      },
    },
  ],

  on_success: common.publishresulttask {
    pipeline: 'artifact-releaser-test',
    job: 'upload-arle-autopush-staging-%s' % tl.package,
    result_state: 'success',
    start_timestamp: '((.:start-timestamp-ms))',
  },
  on_failure: common.publishresulttask {
    pipeline: 'artifact-releaser-test',
    job: 'upload-arle-autopush-staging-%s' % tl.package,
    result_state: 'failure',
    start_timestamp: '((.:start-timestamp-ms))',
  },
};

local promote_arle_autopush_stable = {
  local tl = self,

  package:: error 'must set package in promote_arle_autopush_stable',
  passed:: 'upload-arle-autopush-staging-%s' % tl.package,
  repos:: error 'must set repos in promote_arle_autopush_stable',
  universes:: error 'must set universes in promote_arle_autopush_stable',

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
  plan: [
          { get: 'guest-test-infra' },
          { get: 'compute-image-tools' },
          {
            task: 'generate-timestamp',
            file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
          },
          { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
        ] +
        [
          {
            task: 'promote-autopush-stable',
            file: 'guest-test-infra/concourse/tasks/gcloud-promote-package.yaml',
            params: {
              TOPIC: 'projects/artifact-releaser-autopush/topics/gcp-guest-package-promote-autopush',
            },
            vars: {
              environment: 'stable',
              repo: tl.repos[i],
              universe: tl.universes[i],
            },
          }
          for i in std.range(0, std.length(tl.repos) - 1)
        ],
};

local arle_publish_images_autopush = {
  local tl = self,
  image:: error 'must set image in arle_publish_images_autopush',
  gcs_path:: error 'must set gcs_path in arle_publish_images_autopush',
  wf:: error 'must set wf in arle_publish_images_autopush',

  plan: [
    { get: 'guest-test-infra' },
    { get: '%s-gcs' % tl.image },
    { task: 'generate-timestamp', file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml' },
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

  // Package names and builds.
  local guest_agent_builds = ['deb10', 'deb11-arm64', 'el7', 'el8', 'el8-arch64', 'el9', 'el9-arch64'],
  local guest_agent_file_endings = [
    '-g1_amd64.deb',
    '-g1_arm64.deb',
    '-el7.x86_64.rpm',
    '-el8.x86_64.rpm',
    '-el8.aarch64.rpm',
    '-el9.x86_64.rpm',
    '-el9.aarch64.rpm',
  ],
  local oslogin_builds = ['deb10', 'deb11', 'deb11-arm64', 'deb12', 'deb12-arm64', 'el7', 'el8', 'el8-arch64', 'el9', 'el9-arch64'],
  local oslogin_file_endings = [
    '-g1+deb10_amd64.deb',
    '-g1+deb11_amd64.deb',
    '-g1+deb11_arm64.deb',
    '-g1+deb12_amd64.deb',
    '-g1+deb12_arm64.deb',
    '-g1.el7.x86_64.rpm',
    '-g1.el8.x86_64.rpm',
    '-g1.el8.aarch64.rpm',
    '-g1.el9.x86_64.rpm',
    '-g1.el9.aarch64.rpm',
  ],
  local osconfig_builds = ['deb10', 'deb11-arm64', 'el7', 'el8', 'el8-arch64', 'el9', 'el9-arch64'],
  local osconfig_file_endings = [
    '-g1_amd64.deb',
    '-g1_arm64.deb',
    '-el7.x86_64.rpm',
    '-el8.x86_64.rpm',
    '-el8.aarch64.rpm',
    '-el9.x86_64.rpm',
    '-el9.aarch64.rpm',
  ],
  local guest_diskexpand_builds = ['deb10', 'el7', 'el8', 'el9'],
  local guest_diskexpand_file_endings = [
    '-g1_all.deb',
    '-g1.el7.noarch.rpm',
    '-g1.el8.noarch.rpm',
    '-g1.el9.noarch.rpm',
  ],
  local guest_config_builds = ['deb10', 'el7', 'el8', 'el9'],
  local guest_config_file_endings = [
    '-g1_all.deb',
    '-g1.el7.noarch.rpm',
    '-g1.el8.noarch.rpm',
    '-g1.el9.noarch.rpm',
  ],
  local yum_plugin_builds = ['el7'],
  local yum_plugin_file_endings = [
    '.el7.x86_64.rpm',
  ],
  local yum_plugin_dnf_builds = ['el8', 'el8-arch64', 'el9', 'el9-arch64'],
  local yum_plugin_dnf_file_endings = [
    '.el8.x86_64.rpm',
    '.el8.aarch64.rpm',
    '.el9.x86_64.rpm',
    '.el9.aarch64.rpm',
  ],
  local apt_transport_builds = ['deb10', 'deb11-arm64'],
  local apt_transport_file_endings = [
    '-g1_amd64.deb',
    '-g1_arm64.deb',
  ],
  local compute_image_windows_packages = ['certgen', 'auto-updater', 'powershell', 'sysprep', 'ssh'],
  local compute_image_windows_gcs_names = [
    'certgen.x86_64.x86_64.',
    'google-compute-engine-auto-updater.noarch.',
    'google-compute-engine-powershell.noarch.',
    'google-compute-engine-sysprep.noarch.',
    'google-compute-engine-ssh.x86_64.',
  ],

  // All resources.
  resources: [
               // Guest Agent resources.
               common.gcspkgresource {
                 package: 'guest-agent',
                 build: guest_agent_builds[i],
                 regexp: 'guest-agent/google-guest-agent_([0-9]+).00%s' % guest_agent_file_endings[i],
               }
               for i in std.range(0, std.length(guest_agent_builds) - 1)
             ] +
             [
               // OSLogin resources.
               common.gcspkgresource {
                 package: 'oslogin',
                 build: oslogin_builds[i],
                 regexp: 'oslogin/google-compute-engine-oslogin_([0-9]+).00%s' % oslogin_file_endings[i],
               }
               for i in std.range(0, std.length(oslogin_builds) - 1)
             ] +
             [
               // OSConfig resources.
               common.gcspkgresource {
                 package: 'osconfig',
                 build: osconfig_builds[i],
                 regexp: 'osconfig/google-osconfig-agent-([0-9]+).00%s' % osconfig_file_endings[i],
               }
               for i in std.range(0, std.length(osconfig_builds) - 1)
             ] +
             [
               // Guest Diskexpand resources.
               common.gcspkgresource {
                 package: 'gce-disk-expand',
                 build: guest_diskexpand_builds[i],
                 regexp: 'gce-disk-expand/gce-disk-expand-([0-9]+).00%s' % guest_diskexpand_file_endings[i],
               }
               for i in std.range(0, std.length(guest_diskexpand_builds) - 1)
             ] +
             [
               // Guest Config resources.
               common.gcspkgresource {
                 package: 'google-compute-engine',
                 build: guest_config_builds[i],
                 regexp: 'google-compute-engine/google-compute-engine_([0-9]+).00%s' % guest_config_file_endings[i],
               }
               for i in std.range(0, std.length(guest_config_builds) - 1)
             ] +
             [
               // Artifact Registry Yum Plugin resources.
               common.gcspkgresource {
                 package: 'yum-plugin-artifact-registry',
                 build: yum_plugin_builds[i],
                 regexp: 'yum-plugin-artifact-registry/yum-plugin-artifact-registry_([0-9]+).00%s' % yum_plugin_file_endings[i],
               }
               for i in std.range(0, std.length(yum_plugin_builds) - 1)
             ] +
             [
               // Artifact Registry Yum Plugin resources (dnf).
               common.gcspkgresource {
                 package: 'artifact-registry-yum-plugin',
                 build: yum_plugin_dnf_builds[i],
                 regexp: 'yum-plugin-artifact-registry/dnf-plugin-artifact-registry_([0-9]+).00%s' % yum_plugin_dnf_file_endings[i],
               }
               for i in std.range(0, std.length(yum_plugin_dnf_builds) - 1)
             ] +
             [
               // Artifact Registry Apt Transport resources.
               common.gcspkgresource {
                 package: 'apt-transport-artifact-registry',
                 build: apt_transport_builds[i],
                 regexp: 'apt-transport-artifact-registry/apt-transport-artifact-registry_([0-9]+).00%s' % apt_transport_file_endings[i],
               }
               for i in std.range(0, std.length(apt_transport_builds) - 1)
             ] +
             [
               // Compute Image Windows resources.
               common.gcspkgresource {
                 package: compute_image_windows_packages[i],
                 regexp: 'compute-image-windows/%s([0-9]+).00.0@1.goo' % compute_image_windows_gcs_names[i],
               }
               for i in std.range(0, std.length(compute_image_windows_packages) - 1)
             ] +
             [
               // Diagnostics
               common.gcspkgresource { package: 'diagnostics', regexp: 'compute-image-tools/google-compute-engine-diagnostics.x86_64.([0-9]+).00.0@0.goo' },
             ] +
             // Image resources
             [common.gcsimgresource { image: image, gcs_dir: 'almalinux' } for image in almalinux] +
             [common.gcsimgresource { image: image, gcs_dir: 'rocky-linux' } for image in rocky_linux] +
             [common.gcsimgresource {
               image: image,
               regexp: 'debian/%s-v([0-9]+).tar.gz' % common.debian_image_prefixes[self.image],
             } for image in debian] +
             [common.gcsimgresource { image: image, gcs_dir: 'centos' } for image in centos] +
             [common.gcsimgresource { image: image, gcs_dir: 'rhel' } for image in rhel],

  // Run jobs.
  jobs: [
    upload_arle_autopush_staging {
      package: 'guest-agent',
      builds: guest_agent_builds,
      gcs_pkg_name: 'google-guest-agent',
      file_endings: guest_agent_file_endings,
    },
  ],
}
