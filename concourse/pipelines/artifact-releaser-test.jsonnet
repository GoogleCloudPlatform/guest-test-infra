// Imports.
local arle = import '../templates/arle.libsonnet';
local common = import '../templates/common.libsonnet';

// Get the repository given the build.
local get_repo(build) = if std.startsWith(build, 'deb') then 'guest-arle-autopush-trusty'
else if std.startsWith(build, 'el') then 'guest-arle-autopush-el7-x86_64'
else 'guest-arle-autopush';

// Get the universe given the build.
local get_universe(build) = if std.startsWith(build, 'deb') then 'cloud-apt'
else if std.startsWith(build, 'el') then 'cloud-yum'
else 'cloud-yuck';

// Change '-' to '_', mainly used for images
local underscore(input) = std.strReplace(input, '-', '_');


local upload_arle_autopush_staging = {
  local tl = self,

  package:: error 'must set package in upload_arle_autopush_staging',
  builds:: error 'must set builds in upload_arle_autopush_staging',
  file_endings:: error 'must set file_endings in upload_arle_autopush_staging',

  gcs_dir:: tl.package,
  gcs_pkg_name:: error 'must set gcs_pkg_name in upload_arle_autopush_staging',
  gcs_filename:: if std.endsWith(tl.gcs_pkg_name, '.') then tl.gcs_pkg_name else tl.gcs_pkg_name + '_',

  name: 'upload-arle-autopush-staging-%s' % tl.package,
  plan: [
    {
      in_parallel: {
        steps: [
                 { get: 'guest-test-infra' },
                 { get: 'compute-image-tools' },
               ] +
               [
                 {
                   get: '%s-%s-gcs' % [tl.package, build],
                   trigger: true,
                   params: { skip_download: true },
                 }
                 for build in tl.builds
               ],
      },
    },
    { task: 'generate-timestamp', file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml' },
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
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
          arle.packagepublishtask {
            task: 'upload-arle-autopush-staging-%s-%s' % [tl.package, tl.builds[i]],
            topic: 'projects/artifact-releaser-autopush/topics/gcs-guest-package-upload-autopush',
            package_paths: '{"bucket":"%s","object":"%s/%s((.:package-version-%s)).00%s"}' % [
              common.prod_package_bucket,
              tl.gcs_dir,
              tl.gcs_filename,
              tl.builds[i],
              tl.file_endings[i],
            ],
            repo: get_repo(tl.builds[i]),
            universe: get_universe(tl.builds[i]),
          }
          for i in std.range(0, std.length(tl.builds) - 1)
        ],
      },
    },
  ],
  on_success: {
    task: 'publish-success-metric',
    config: common.publishresulttask {
      pipeline: 'artifact-releaser-test',
      job: 'upload-arle-autopush-staging-%s' % tl.package,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'publish-failure-metric',
    config: common.publishresulttask {
      pipeline: 'artifact-releaser-test',
      job: 'upload-arle-autopush-staging-%s' % tl.package,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local promote_arle_autopush_stable = {
  local tl = self,

  repos:: ['guest-arle-autopush-trusty', 'guest-arle-autopush-el7-x86_64', 'guest-arle-autopush'],
  universes:: ['cloud-apt', 'cloud-yum', 'cloud-yuck'],
  env:: 'stable',
  topic:: 'projects/artifact-releaser-autopush/topics/gcp-guest-package-upload-autopush',

  name: 'promote-arle-autopush-stable',
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
            config: {
              platform: 'linux',
              image_resource: {
                type: 'registry-image',
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
                  '{"type": "insertPackage", "request": {"universe": "%s", "repo": "%s", "environment": "%s"}}' % [
                    tl.universes[i],
                    tl.repos[i],
                    tl.env,
                  ],
                ],
              },
            },
          }
          for i in std.range(0, std.length(tl.repos) - 1)
        ],
  on_success: {
    task: 'publish-success-metric',
    config: common.publishresulttask {
      pipeline: 'artifact-releaser-test',
      job: 'promote-arle-autopush-stable',
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'publish-failure-metric',
    config: common.publishresulttask {
      pipeline: 'artifact-releaser-test',
      job: 'promote-arle-autopush-stable',
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local arle_publish_images_autopush = {
  local tl = self,
  image:: error 'must set image in arle_publish_images_autopush',

  // Even though 'autopush' is not a specifically defined environment, given the current definitions
  // of the publish workflows, any environment that is not 'testing' or 'prod' will make the publish
  // project the running project, in this case 'artifact-releaser-autopush'.
  env:: 'autopush',

  wf_dir:: error 'must set wf_dir in arle_publish_images_autopush',
  workflow:: '%s/%s.publish.json' % [self.workflow_dir, underscore(self.image)],

  gcs_dir:: error 'must set gcs_dir in arle-publish-images-autopush',
  gcs_bucket:: common.prod_bucket,
  gcs:: 'gs://%s/%s' % [self.gcs_bucket, self.gcs_dir],

  passed:: 'build-' + tl.image,

  name: 'arle-publish-images-autopush-%s' % tl.image,
  plan: [
    { get: 'guest-test-infra' },
    {
      get: '%s-gcs' % tl.image,
      trigger: true,
      params: { skip_download: true },
    },
    { task: 'generate-timestamp', file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml' },
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    { load_var: 'source-version', file: '%s-gcs/version' % tl.image },
    { task: 'generate-version', file: 'guest-test-infra/concourse/tasks/generate-version.yaml' },
    { load_var: 'publish-version', file: 'publish-version/version' },
    {
      task: 'publish-autopush-%s' % tl.image,
      config: arle.gcepublishtask {
        source_gcs_path: tl.gcs,
        source_version: 'v((.:source-version))',
        publish_version: '((.:publish-version))',
        wf: tl.workflow,
        environment: tl.env,
      },
    },
  ],
  on_success: {
    task: 'publish-success-metric',
    config: common.publishresulttask {
      pipeline: 'artifact-releaser-test',
      job: 'arle-publish-image-autopush-%s' % tl.image,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'publish-failure-metric',
    config: common.publishresulttask {
      pipeline: 'artifact-releaser-test',
      job: 'arle-publish-image-autopush-%s' % tl.image,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

// Image group definition.
local imggroup = {
  local tl = self,

  images:: error 'must set images in imggroup',

  name: error 'must set name in imggroup',
  jobs: [
    'arle-publish-images-autopush-' + image
    for image in tl.images
  ],
};

// Package group definition
local pkggroup = {
  local tl = self,

  packages:: error 'must set packages in pkggroup',
  builds:: error 'must set builds in pkggroup',

  name: error 'must set name in pkggroup',
  jobs: [
    'upload-arle-autopush-staging-%s' % package
    for package in tl.packages
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

  // Package builds and file endings.
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

  // Start of output.
  resource_types: [
    {
      name: 'gcs',
      type: 'registry-image',
      source: { repository: 'frodenas/gcs-resource' },
    },
  ],
  // All resources.
  resources: [
               common.GitResource('guest-test-infra'),
               common.GitResource('compute-image-tools'),
             ] +
             [
               // Guest Agent resources.
               common.gcspkgresource {
                 package: 'guest-agent',
                 build: guest_agent_builds[i],
                 regexp: 'guest-agent/google-guest-agent_([0-9]+).00' + guest_agent_file_endings[i],
               }
               for i in std.range(0, std.length(guest_agent_builds) - 1)
             ] +
             [
               // OSLogin resources.
               common.gcspkgresource {
                 package: 'oslogin',
                 build: oslogin_builds[i],
                 regexp: 'oslogin/google-compute-engine-oslogin_([0-9]+).00' + oslogin_file_endings[i],
               }
               for i in std.range(0, std.length(oslogin_builds) - 1)
             ] +
             [
               // OSConfig resources.
               common.gcspkgresource {
                 package: 'osconfig',
                 build: osconfig_builds[i],
                 regexp: 'osconfig/google-osconfig-agent-([0-9]+).00' + osconfig_file_endings[i],
               }
               for i in std.range(0, std.length(osconfig_builds) - 1)
             ] +
             [
               // Guest Diskexpand resources.
               common.gcspkgresource {
                 package: 'gce-disk-expand',
                 build: guest_diskexpand_builds[i],
                 regexp: 'gce-disk-expand/gce-disk-expand-([0-9]+).00' + guest_diskexpand_file_endings[i],
               }
               for i in std.range(0, std.length(guest_diskexpand_builds) - 1)
             ] +
             [
               // Guest Config resources.
               common.gcspkgresource {
                 package: 'google-compute-engine',
                 build: guest_config_builds[i],
                 regexp: 'google-compute-engine/google-compute-engine_([0-9]+).00' + guest_config_file_endings[i],
               }
               for i in std.range(0, std.length(guest_config_builds) - 1)
             ] +
             [
               // Artifact Registry Yum Plugin resources (dnf).
               common.gcspkgresource {
                 package: 'yum-plugin-artifact-registry',
                 build: yum_plugin_dnf_builds[i],
                 regexp: 'yum-plugin-artifact-registry/dnf-plugin-artifact-registry_([0-9]+).00' + yum_plugin_dnf_file_endings[i],
               }
               for i in std.range(0, std.length(yum_plugin_dnf_builds) - 1)
             ] +
             [
               // Artifact Registry Apt Transport resources.
               common.gcspkgresource {
                 package: 'apt-transport-artifact-registry',
                 build: apt_transport_builds[i],
                 regexp: 'apt-transport-artifact-registry/apt-transport-artifact-registry_([0-9]+).00' + apt_transport_file_endings[i],
               }
               for i in std.range(0, std.length(apt_transport_builds) - 1)
             ] +
             [
               // Compute Image Windows resources.
               common.gcspkgresource {
                 package: compute_image_windows_packages[i],
                 build: 'goo',
                 regexp: 'compute-image-windows/%s([0-9]+).00.0@1.goo' % compute_image_windows_gcs_names[i],
               }
               for i in std.range(0, std.length(compute_image_windows_packages) - 1)
             ] +
             [
               // Diagnostics
               common.gcspkgresource {
                 package: 'diagnostics',
                 build: 'goo',
                 regexp: 'compute-image-tools/google-compute-engine-diagnostics.x86_64.([0-9]+).00.0@0.goo',
               },
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
          upload_arle_autopush_staging {
            package: 'oslogin',
            builds: oslogin_builds,
            gcs_pkg_name: 'google-compute-engine-oslogin',
            file_endings: oslogin_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'osconfig',
            builds: osconfig_builds,
            gcs_pkg_name: 'google-osconfig-agent',
            file_endings: osconfig_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'gce-disk-expand',
            builds: guest_diskexpand_builds,
            gcs_pkg_name: 'gcs-disk-expand',
            file_endings: guest_diskexpand_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'google-compute-engine',
            builds: guest_config_builds,
            gcs_pkg_name: 'google-compute-engine',
            file_endings: guest_config_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'yum-plugin-artifact-registry',
            builds: yum_plugin_dnf_builds,
            gcs_pkg_name: 'dnf-plugin-artifact-registry',
            file_endings: yum_plugin_dnf_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'apt-transport-artifact-registry',
            builds: apt_transport_builds,
            gcs_pkg_name: 'apt-transport-artifact-registry',
            file_endings: apt_transport_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'diagnostics',
            gcs_dir: 'compute-image-tools',
            gcs_pkg_name: 'google-compute-engine-diagnostics.x86_64.',
            builds: ['goo'],
            file_endings: ['.00.0@1.goo'],
          },
        ] +
        [
          // Compute Image Windows packages are set up differently, so need to create the jobs differently.
          upload_arle_autopush_staging {
            package: compute_image_windows_packages[i],
            gcs_dir: 'compute-image-windows',
            gcs_pkg_name: compute_image_windows_gcs_names[i],
            builds: ['goo'],
            file_endings: ['.00.0@1.goo'],
          }
          for i in std.range(0, std.length(compute_image_windows_packages) - 1)
        ] +
        [
          // Promote all packages in every repo.
          promote_arle_autopush_stable,
        ] +
        [
          // Debian publish jobs.
          arle_publish_images_autopush {
            image: image,
            gcs_dir: 'debian',
            workflow_dir: 'debian',
          }
          for image in debian
        ] +
        [
          arle_publish_images_autopush {
            image: image,
            gcs_dir: 'rhel',
            workflow_dir: 'enterprise_linux',
          }
          for image in rhel
        ] + [
          arle_publish_images_autopush {
            image: image,
            gcs_dir: 'centos',
            workflow_dir: 'enterprise_linux',
          }
          for image in centos
        ] +
        [
          arle_publish_images_autopush {
            image: image,
            gcs_dir: 'almalinux',
            workflow_dir: 'enterprise_linux',
          }
          for image in almalinux
        ] +
        [
          arle_publish_images_autopush {
            image: image,
            gcs_dir: 'rocky-linux',
            workflow_dir: 'enterprise_linux',
          }
          for image in rocky_linux
        ],
  groups: [
    // Image groups
    imggroup { name: 'debian', images: debian },
    imggroup { name: 'rhel', images: rhel },
    imggroup { name: 'centos', images: centos },
    imggroup { name: 'almalinux', images: almalinux },
    imggroup { name: 'rocky-linux', images: rocky_linux },

    // Package groups
    pkggroup { name: 'guest-agent', packages: ['guest-agent'], builds: guest_agent_builds },
    pkggroup { name: 'oslogin', packages: ['oslogin'], builds: oslogin_builds },
    pkggroup { name: 'osconfig', packages: ['osconfig'], builds: osconfig_builds },
    pkggroup { name: 'guest-diskexpand', packages: ['gce-disk-expand'], builds: guest_diskexpand_builds },
    pkggroup { name: 'guest-configs', packages: ['google-compute-engine'], builds: guest_config_builds },
    pkggroup { name: 'yum-plugin-artifact-registry', packages: ['yum-plugin-artifact-registry'], builds: yum_plugin_dnf_builds },
    pkggroup { name: 'apt-transport-artifact-registry', packages: ['apt-transport-artifact-registry'], builds: apt_transport_builds },
    pkggroup { name: 'diagnostics', packages: ['diagnostics'], builds: ['goo'] },
    pkggroup { name: 'compute-image-windows', packages: compute_image_windows_packages, builds: ['goo'] },
    {
      name: 'promote-autopush-stable',
      jobs: ['promote-arle-autopush-stable'],
    },
  ],
}
