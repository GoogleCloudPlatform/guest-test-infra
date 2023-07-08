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

// Package versions are preceded by '_' in debian builds, '-' in EL builds, and '.' in Windows builds.
local get_filename(filename, build) = if build == 'goo' then filename + '.'
else if std.startsWith(build, 'deb') then filename + '_'
else filename + '-';

// Get builds given the index, or index 0 if there is only one build.
local get_build(builds, index) = if std.length(builds) > 1 then builds[index] else builds[0];

// Change '-' to '_', mainly used for images.
local underscore(input) = std.strReplace(input, '-', '_');

// Publish task results
local publishresulttask = {
  local tl = self,

  result:: error 'must set result in publishresulttask',
  job:: error 'must set job in publishresulttask',

  task: tl.result,
  config: {
    platform: 'linux',
    image_resource: {
      type: 'registry-image',
      source: { repository: 'gcr.io/gcp-guest/concourse-metrics' },
    },
    run: {
      path: '/publish-job-result',
      args: [
        '--project-id=gcp-guest',
        '--zone=us-west1-a',
        '--pipeline=artifact-releaser-test',
        '--job=' + tl.job,
        '--task=publish-job-result',
        '--result-state=' + tl.result,
        '--start-timestamp=((.:start-timestamp-ms))',
        '--metric-path=concourse/job/duration',
      ],
    },
  },
};

local upload_arle_autopush_staging = {
  local tl = self,

  package:: error 'must set package in upload_arle_autopush_staging',

  builds:: error 'must set builds in upload_arle_autopush_staging',
  file_endings:: error 'must set file_endings in upload_arle_autopush_staging',

  gcs_dir:: tl.package,
  gcs_pkg_names:: error 'must set gcs_pkg_names in upload_arle_autopush_staging',

  name: 'upload-arle-autopush-staging-%s' % tl.package,
  plan: [
    {
      in_parallel: {
        steps: [
          { get: 'guest-test-infra' },
          { get: 'compute-image-tools' },
          { get: 'time-interval', trigger: true },
          { get: '%s-tag' % tl.package, trigger: true },
        ],
      },
    },
    { task: 'generate-timestamp', file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml' },
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    { load_var: 'package-version', file: '%s-tag/version' % tl.package },
    {
      in_parallel: {
        steps: [
          arle.packagepublishtask {
            task: 'upload-arle-autopush-staging-%s-%s' % [tl.package, get_build(tl.builds, i)],
            topic: 'projects/artifact-releaser-autopush/topics/gcp-guest-package-upload-autopush',
            package_paths: '{"bucket":"%s","object":"%s/%s((.:package-version))%s"}' % [
              common.prod_package_bucket,
              tl.gcs_dir,
              get_filename(
                if std.length(tl.gcs_pkg_names) > 1 then tl.gcs_pkg_names[i] else tl.gcs_pkg_names[0],
                get_build(tl.builds, i)
              ),
              tl.file_endings[i],
            ],
            repo: get_repo(get_build(tl.builds, i)),
            universe: get_universe(get_build(tl.builds, i)),
          }
          // file_endings is the common denominator, so we loop through that instead.
          for i in std.range(0, std.length(tl.file_endings) - 1)
        ],
      },
    },
  ],
  on_success: publishresulttask {
    result: 'success',
    job: tl.name,
  },
  on_failure: publishresulttask {
    result: 'failure',
    job: tl.name,
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
          { get: 'time-interval', trigger: true },
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
  on_success: publishresulttask {
    result: 'success',
    job: tl.name,
  },
  on_failure: publishresulttask {
    result: 'failure',
    job: tl.name,
  },
};

local arle_publish_images_autopush = {
  local tl = self,
  image:: error 'must set image in arle_publish_images_autopush',

  workflow_dir:: error 'must set workflow_dir in arle_publish_images_autopush',
  workflow:: '%s/%s.publish.json' % [self.workflow_dir, underscore(self.image)],

  gcs_dir:: error 'must set gcs_dir in arle-publish-images-autopush',
  gcs_bucket:: common.prod_bucket,
  gcs:: 'gs://%s/%s' % [self.gcs_bucket, self.gcs_dir],

  name: 'arle-publish-images-autopush-%s' % tl.image,
  plan: [
    {
      in_parallel: {
        steps: [
          { get: 'guest-test-infra' },
          { get: 'compute-image-tools' },
          { get: 'time-interval', trigger: true },
          {
            get: '%s-gcs' % tl.image,
            trigger: true,
            params: { skip_download: 'true' },
          },
        ],
      },
    },
    { task: 'generate-timestamp', file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml' },
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    { load_var: 'source-version', file: '%s-gcs/version' % tl.image },
    { task: 'generate-version', file: 'guest-test-infra/concourse/tasks/generate-version.yaml' },
    { load_var: 'publish-version', file: 'publish-version/version' },
    {
      task: 'publish-autopush-%s' % tl.image,
      config: arle.arlepublishtask {
        topic: 'projects/artifact-releaser-autopush/topics/gcp-guest-image-release-autopush',
        image_name: tl.image,
        gcs_image_path: tl.gcs,
        source_version: 'v((.:source-version))',
        publish_version: '((.:publish-version))',
        wf: tl.workflow,
      },
    },
  ],
  on_success: publishresulttask {
    result: 'success',
    job: tl.name,
  },
  on_failure: publishresulttask {
    result: 'failure',
    job: tl.name,
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

// Package group definition.
local pkggroup = {
  local tl = self,

  packages:: [tl.name],

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
    '-g1.el7.x86_64.rpm',
    '-g1.el8.x86_64.rpm',
    '-g1.el8.aarch64.rpm',
    '-g1.el9.x86_64.rpm',
    '-g1.el9.aarch64.rpm',
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
    '-g1.el7.x86_64.rpm',
    '-g1.el8.x86_64.rpm',
    '-g1.el8.aarch64.rpm',
    '-g1.el9.x86_64.rpm',
    '-g1.el9.aarch64.rpm',
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
    '-g1.el8.x86_64.rpm',
    '-g1.el8.aarch64.rpm',
    '-g1.el9.x86_64.rpm',
    '-g1.el9.aarch64.rpm',
  ],
  local apt_transport_builds = ['deb10', 'deb11-arm64'],
  local apt_transport_file_endings = [
    '-g1_amd64.deb',
    '-g1_arm64.deb',
  ],
  local compute_image_windows_packages = ['certgen', 'auto-updater', 'powershell', 'sysprep', 'ssh'],
  local compute_image_windows_gcs_names = [
    'certgen.x86_64.x86_64',
    'google-compute-engine-auto-updater.noarch',
    'google-compute-engine-powershell.noarch',
    'google-compute-engine-sysprep.noarch',
    'google-compute-engine-ssh.x86_64',
  ],
  local compute_image_windows_file_endings = [
    '.0@1.goo',
    '@1.goo',
    '@1.goo',
    '.0@1.goo',
    '@1.goo',
  ],

  // List of all packages.
  local packages = [
    'guest-agent',
    'guest-oslogin',
    'osconfig',
    'guest-diskexpand',
    'guest-configs',
    'artifact-registry-yum-plugin',
    'artifact-registry-apt-transport',
    'compute-image-windows',
    'compute-image-tools',
  ],

  // Start of output.
  resource_types: [
    {
      name: 'gcs',
      type: 'registry-image',
      source: { repository: 'frodenas/gcs-resource' },
    },
    {
      name: 'cron-resource',
      type: 'docker-image',
      source: { repository: 'cftoolsmiths/cron-resource' },
    },
  ],
  resources: [
               common.GitResource('guest-test-infra'),
               common.GitResource('compute-image-tools'),

               // Time resource.
               {
                 name: 'time-interval',
                 type: 'cron-resource',
                 icon: 'clock-outline',
                 source: {
                   // Every day at midnight.
                   expression: '0 0 * * *',
                   fire_immediately: true,
                 },
               },
             ] +
             [
               // Package resources.
               {
                 name: '%s-tag' % package,
                 type: 'github-release',
                 source: {
                   owner: 'GoogleCloudPlatform',
                   repository: package,
                   access_token: '((github-token.token))',
                   tag_filter: '([0-9]+.*)',
                 },
               }
               for package in packages
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

  jobs: [
          upload_arle_autopush_staging {
            package: 'guest-agent',
            builds: guest_agent_builds,
            gcs_pkg_names: ['google-guest-agent'],
            file_endings: guest_agent_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'guest-oslogin',
            gcs_dir: 'oslogin',
            builds: oslogin_builds,
            gcs_pkg_names: ['google-compute-engine-oslogin'],
            file_endings: oslogin_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'osconfig',
            builds: osconfig_builds,
            gcs_pkg_names: ['google-osconfig-agent'],
            file_endings: osconfig_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'guest-diskexpand',
            gcs_dir: 'gce-disk-expand',
            builds: guest_diskexpand_builds,
            gcs_pkg_names: ['gce-disk-expand'],
            file_endings: guest_diskexpand_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'guest-configs',
            gcs_dir: 'google-compute-engine',
            builds: guest_config_builds,
            gcs_pkg_names: ['google-compute-engine'],
            file_endings: guest_config_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'artifact-registry-yum-plugin',
            gcs_dir: 'yum-plugin-artifact-registry',
            builds: yum_plugin_dnf_builds,
            gcs_pkg_names: ['dnf-plugin-artifact-registry'],
            file_endings: yum_plugin_dnf_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'artifact-registry-apt-transport',
            gcs_dir: 'apt-transport-artifact-registry',
            builds: apt_transport_builds,
            gcs_pkg_names: ['apt-transport-artifact-registry'],
            file_endings: apt_transport_file_endings,
          },
          upload_arle_autopush_staging {
            package: 'compute-image-tools',
            gcs_pkg_names: ['google-compute-engine-diagnostics.x86_64'],
            builds: ['goo'],
            file_endings: ['.0@0.goo'],
          },
          upload_arle_autopush_staging {
            package: 'compute-image-windows',
            gcs_pkg_names: compute_image_windows_gcs_names,
            builds: ['goo'],
            file_endings: compute_image_windows_file_endings,
          },
        ] +
        [
          // Promote all packages in every repo.
          promote_arle_autopush_stable,
        ] +
        [
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
    pkggroup { name: 'guest-agent' },
    pkggroup { name: 'guest-oslogin' },
    pkggroup { name: 'osconfig' },
    pkggroup { name: 'guest-diskexpand' },
    pkggroup { name: 'guest-configs' },
    pkggroup { name: 'artifact-registry-yum-plugin' },
    pkggroup { name: 'artifact-registry-apt-transport' },
    pkggroup { name: 'compute-image-tools' },
    pkggroup { name: 'compute-image-windows' },

    // Other groups.
    {
      name: 'promote-autopush-stable',
      jobs: ['promote-arle-autopush-stable'],
    },
  ],
}
