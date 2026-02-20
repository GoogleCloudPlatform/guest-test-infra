// Imports.
local arle = import '../templates/arle.libsonnet';
local common = import '../templates/common.libsonnet';
local daisy = import '../templates/daisy.libsonnet';
local gcp_secret_manager = import '../templates/gcp-secret-manager.libsonnet';
local lego = import '../templates/lego.libsonnet';

// Common
local envs = ['testing', 'prod'];
local underscore(input) = std.strReplace(input, '-', '_');

local trim_strings(s, trim) =
  if std.length(trim) == 0 then
    s
  else
    trim_strings(std.strReplace(s, trim[0], ''), trim[1:]);

local imgbuildtask = daisy.daisyimagetask {
  gcs_url: '((.:gcs-url))',
  sbom_destination: '((.:sbom-destination))',
  shasum_destination: '((.:shasum-destination))',
};

local prepublishtesttask = common.imagetesttask {
  filter: '(shapevalidation)', // TODO enable oslogin
  extra_args: [ '-shapevalidation_test_filter=^(([A-Z][0-3])|(N4))' ],
};

local imgbuildjob = {
  local tl = self,

  use_dynamic_template:: false,

  image:: error 'must set image in imgbuildjob',
  image_prefix:: self.image,
  workflow_dir:: error 'must set workflow_dir in imgbuildjob',
  workflow::
  if tl.use_dynamic_template then
      '%s/rhel_%s_consolidated.wf.json' % [tl.workflow_dir, tl.major_release]
    else
      '%s/%s.wf.json' % [tl.workflow_dir, underscore(tl.image)],
  build_task:: imgbuildtask {
    workflow: tl.workflow,
    vars+: ['google_cloud_repo=stable'],
  },
  extra_tasks:: [],
  daily:: true,
  daily_task:: if self.daily then [
    {
      get: 'daily-time',
      trigger: true,
    },
  ] else [],

  // Start of job
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
      // generate the build id so the tarball and sbom have the same name
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
    // This is the 'put trick'. We don't have the real image tarball to write to GCS here, but we want
    // Concourse to treat this job as producing it. So we write an empty file now, and overwrite it later in
    // the daisy workflow. This also generates the final URL for use in the daisy workflow.
    // This is also done for the sbom file.
    {
      put: tl.image + '-gcs',
      params: {
        // empty file written to GCS e.g. 'build-id-dir/centos-7-v20210107.tar.gz'
        file: 'build-id-dir/%s*' % tl.image_prefix,
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
      task: 'generate-build-id-sbom',
      file: 'guest-test-infra/concourse/tasks/generate-build-id-sbom.yaml',
      vars: { prefix: tl.image_prefix, id: '((.:id))' },
    },
    {
      put: tl.image + '-sbom',
      params: {
        // empty file written to GCS e.g. 'build-id-dir/centos-7-v20210107-1681318938.sbom.json'
        file: 'build-id-dir-sbom/%s*' % tl.image_prefix,
      },
      get_params: {
        skip_download: 'true',
      },
    },
    {
      load_var: 'sbom-destination',
      file: '%s-sbom/url' % tl.image,
    },
    {
      task: 'generate-build-id-shasum',
      file: 'guest-test-infra/concourse/tasks/generate-build-id-shasum.yaml',
      vars: { prefix: tl.image_prefix, id: '((.:id))' },
    },
    {
      put: tl.image + '-shasum',
      params: {
        // empty file written to GCS e.g. 'build-id-dir/centos-7-v20210107-1681318938.txt'
        file: 'build-id-dir-shasum/%s*' % tl.image_prefix,
      },
      get_params: {
        skip_download: 'true',
      },
    },
    {
      load_var: 'shasum-destination',
      file: '%s-shasum/url' % tl.image,
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
    config: common.publishresulttask {
      pipeline: 'linux-image-build',
      job: tl.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'failure',
    config: common.publishresulttask {
      pipeline: 'linux-image-build',
      job: tl.name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local centosimgbuildjob = imgbuildjob {
  local tl = self,

  workflow_dir: 'enterprise_linux',
  sbom_util_secret_name:: 'sbom-util-secret',
  isopath:: trim_strings(tl.image, ['-byos', '-eus', '-lvm', '-sap', '-nvidia-latest', '-nvidia-550']),

  // Add tasks to obtain ISO location and sbom util source
  // Store those in .:iso-secret and .:sbom-util-secret
  extra_tasks: [
    {
      task: 'get-secret-iso',
      config: gcp_secret_manager.getsecrettask { secret_name: tl.isopath },
    },
    {
      load_var: 'iso-secret',
      file: 'gcp-secret-manager/' + tl.isopath,
    },
    {
      task: 'get-secret-sbom-util',
      config: gcp_secret_manager.getsecrettask { secret_name: tl.sbom_util_secret_name },
    },
    {
      load_var: 'sbom-util-secret',
      file: 'gcp-secret-manager/' + tl.sbom_util_secret_name,
    },
  ],

  // Add EL and sbom util args to build task.
  build_task+: { vars+: ['installer_iso=((.:iso-secret))', 'sbom_util_gcs_root=((.:sbom-util-secret))'] },
};

local debianimgbuildjob = imgbuildjob {
  local tl = self,

  workflow_dir: 'debian',
  sbom_util_secret_name:: 'sbom-util-secret',

  // Add tasks to obtain sbom util source
  // Store in .:sbom-util-secret
  extra_tasks: [
    {
      task: 'get-secret-sbom-util',
      config: gcp_secret_manager.getsecrettask { secret_name: tl.sbom_util_secret_name },
    },
    {
      load_var: 'sbom-util-secret',
      file: 'gcp-secret-manager/' + tl.sbom_util_secret_name,
    },
  ],

  // Add sbom util args to build task.
  build_task+: { vars+: ['sbom_util_gcs_root=((.:sbom-util-secret))'] },
};

local rhelimgbuildjob = imgbuildjob {
  local tl = self,

  workflow_dir: 'enterprise_linux',
  sbom_util_secret_name:: 'sbom-util-secret',
  isopath:: trim_strings(tl.image, ['-byos', '-eus', '-lvm', '-sap', '-nvidia-latest', '-nvidia-550']),

  is_arm:: std.member(tl.image, '-arm64'),
  is_byos:: std.member(tl.image, '-byos'),
  is_eus:: std.member(tl.image, '-eus'),
  is_lvm:: std.member(tl.image, '-lvm'),
  is_sap:: std.member(tl.image, '-sap'),
  use_dynamic_template:: true,

  local arch = if tl.is_arm then 'aarch64' else 'x86_64',
  local el_release_components = std.split(trim_strings(tl.isopath, ['-arm64']), '-'),

  disk_name::
    if tl.is_arm then 'disk_export_hyperdisk' else 'disk_export',
  disk_type:: if tl.is_arm then 'hyperdisk-balanced' else 'pd-ssd',
  el_install_disk_size:: if tl.is_lvm then '50' else '20',
  machine_type:: if tl.is_arm then 'c4a-standard-4' else 'e2-standard-4',
  major_release:: el_release_components[1],
  version_lock::
    if std.length(el_release_components) > 2 then
      tl.major_release + '-' + el_release_components[2]
    else '',
  worker_image::
    if tl.is_arm then
      'projects/compute-image-tools/global/images/family/debian-12-worker-arm64'
    else
      'projects/compute-image-tools/global/images/family/debian-12-worker',

  local rhui_package_name_base = 'google-rhui-client-rhel',
  local rhui_package_name_tenth_point_release =
    if tl.is_sap && std.length(el_release_components) > 2  && el_release_components[2] == '10' then '10' else '',
  local rhui_package_name_eus =
    if tl.is_eus then '-eus' else '',
  local rhui_package_name_sap =
    if tl.is_sap then '-sap' else '',

  rhui_package_name:: rhui_package_name_base + tl.major_release + rhui_package_name_tenth_point_release + rhui_package_name_eus + rhui_package_name_sap,

  // Add tasks to obtain ISO location and sbom util source
  // Store those in .:iso-secret and .:sbom-util-secret
  extra_tasks: [
    {
      task: 'get-secret-iso',
      config: gcp_secret_manager.getsecrettask { secret_name: tl.isopath },
    },
    {
      load_var: 'iso-secret',
      file: 'gcp-secret-manager/' + tl.isopath,
    },
    {
      task: 'get-secret-sbom-util',
      config: gcp_secret_manager.getsecrettask { secret_name: tl.sbom_util_secret_name },
    },
    {
      load_var: 'sbom-util-secret',
      file: 'gcp-secret-manager/' + tl.sbom_util_secret_name,
    },
  ],

  // Add EL and sbom util args to build task.
  build_task+: { vars+: [
      'installer_iso=((.:iso-secret))',
      'sbom_util_gcs_root=((.:sbom-util-secret))',
      'el_install_disk_size=' + tl.el_install_disk_size,
      'el_release=' + tl.isopath,
      'disk_name=' + tl.disk_name,
      'disk_type=' + tl.disk_type,
      'machine_type=' + tl.machine_type,
      'worker_image=' + tl.worker_image,
      'is_arm=' + std.toString(tl.is_arm),
      'is_byos=' + std.toString(tl.is_byos),
      'is_lvm=' + std.toString(tl.is_lvm),
      'is_sap=' + std.toString(tl.is_sap),
      'rhui_package_name=' + tl.rhui_package_name,
      'version_lock=' + tl.version_lock,
    ] + (if tl.major_release != '8' then ['is_eus=' + std.toString(tl.is_eus)] else [])
  },
};

local imgpublishjob = {
  local tl = self,

  env:: error 'must set publish env in imgpublishjob',
  workflow:: '%s/%s.publish.json' % [self.workflow_dir, underscore(self.image)],
  workflow_dir:: error 'must set workflow_dir in imgpublishjob',

  image:: error 'must set image in imgpublishjob',
  image_prefix:: self.image,

  gcs:: 'gs://%s/%s' % [self.gcs_bucket, self.gcs_dir],
  gcs_dir:: error 'must set gcs directory in imgpublishjob',
  gcs_bucket:: common.prod_bucket,

  // Publish to testing after build
  passed:: if tl.env == 'testing' then
    'build-' + tl.image
  else if tl.env == 'prod' then
    'publish-to-testing-' + tl.image,

  trigger:: if tl.env == 'testing' then true
  else false,

  citfilter:: common.default_linux_image_build_cit_filter,
  cit_extra_args:: [],
  cit_project:: common.default_cit_project,
  cit_test_projects:: common.default_cit_test_projects,
  oslogin_test_project:: common.default_oslogin_test_project,
  oslogin_cit_filter:: common.default_oslogin_cit_filter,

  // Rather than modifying the default CIT invocation above, it's also possible to specify a extra CIT invocations.
  // The images field will be overriden with the image under test.
  extra_test_tasks:: [],

   runtests:: if tl.env == 'testing' then true
   else false,

  // Start of job.
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
          // all the "get" steps must happen before loading them, otherwise concourse seems to
          // erase all the other "get" steps
          {
            get: tl.image + '-gcs',
            passed: [tl.passed],
            trigger: tl.trigger,
            params: { skip_download: 'true' },
          },
          {
            get: tl.image + '-sbom',
            passed: [tl.passed],
            params: { skip_download: 'true' },
          },
          {
            get: tl.image + '-shasum',
            passed: [tl.passed],
            params: { skip_download: 'true' },
          },
          {
            load_var: 'sbom-destination',
            file: '%s-sbom/url' % tl.image,
          },
          {
            load_var: 'shasum-destination',
            file: '%s-shasum/url' % tl.image,
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
        ] +
        // Run prepublish tests and invoke ARLE in prod
        if tl.env == 'prod' then
        [
          {
            task: 'publish-' + tl.image,
            config: arle.arlepublishtask {
              gcs_image_path: tl.gcs,
              gcs_sbom_path: '((.:sbom-destination))',
              image_sha256_hash_txt: '((.:shasum-destination))',
              source_version: 'v((.:source-version))',
              publish_version: '((.:publish-version))',
              wf: tl.workflow,
              image_name: underscore(tl.image),
            },
          },
        ]
        // Other releases use gce_image_publish directly.
        else
        [
            {
              task: if tl.env == 'testing' then
                'publish-' + tl.image
              else
                'publish-%s-%s' % [tl.env, tl.image],
              config: arle.gcepublishtask {
                source_gcs_path: tl.gcs,
                source_version: 'v((.:source-version))',
                publish_version: '((.:publish-version))',
                wf: tl.workflow,
                environment: if tl.env == 'testing' then 'test' else tl.env,
              },
            },
        ]
		+
        // Run post-publish tests in 'publish-to-testing-' jobs.
        if tl.runtests then
          [
            { 
              in_parallel: {
                fail_fast: true,
                steps: [
                  {
                    task: 'image-test-' + tl.image,
                    config: common.imagetesttask {
                      filter: tl.citfilter,
                      project: tl.cit_project,
                      test_projects: tl.cit_test_projects,
                      images: 'projects/bct-prod-images/global/images/%s-((.:publish-version))' % tl.image_prefix,
                      extra_args:: tl.cit_extra_args,
                    },
                    attempts: 3,
                  },
                  {
                    task: 'oslogin-test-' + tl.image,
                    config: common.imagetesttask {
                      filter: tl.oslogin_cit_filter,
                      project: tl.oslogin_test_project,
                      test_projects: tl.oslogin_test_project,
                      images: 'projects/bct-prod-images/global/images/%s-((.:publish-version))' % tl.image_prefix,
                      extra_args:: tl.cit_extra_args,
                    },
                    attempts: 3,
                  },
                ]
              }
            },
          ] + [
            {
              task: 'extra-image-test-' + tl.image + '-' + testtask.task,
              config: testtask {
                images: 'projects/bct-prod-images/global/images/%s-((.:publish-version))' % tl.image_prefix,
              },
              attempts: 3,
            }
            for testtask in tl.extra_test_tasks
          ]
        else
          [],
  on_success: {
    task: 'success',
    config: common.publishresulttask {
      pipeline: 'linux-image-build',
      job: tl.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'failure',
    config: common.publishresulttask {
      pipeline: 'linux-image-build',
      job: 'publish-to-%s-%s' % [tl.env, tl.image],
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local imggroup = {
  local tl = self,

  images:: error 'must set images in imggroup',
  envs:: envs,

  // Start of group.
  name: error 'must set name in imggroup',
  jobs: [
    'build-' + image
    for image in tl.images
  ] + [
    'publish-to-%s-%s' % [env, image]
    for env in tl.envs
    for image in tl.images
  ],
};

{
  local debian_images = ['debian-11', 'debian-12', 'debian-12-arm64', 'debian-13', 'debian-13-arm64'],
  local centos_images = ['centos-stream-9', 'centos-stream-9-arm64', 'centos-stream-10', 'centos-stream-10-arm64'],
  // Each rhel image group includes PAYG, BYOS, and LVM variation. 
  local rhel_8_base_images = [
    'rhel-8',
    'rhel-8-arm64',
    'rhel-8-byos',
    'rhel-8-byos-arm64',
    'rhel-8-lvm',
    'rhel-8-lvm-arm64',
  ],
  local rhel_8_sap_images = [
    'rhel-8-6-sap',
    'rhel-8-6-sap-byos',
    'rhel-8-8-sap',
    'rhel-8-8-sap-byos',
    'rhel-8-10-sap',
    'rhel-8-10-sap-byos',
  ],
  local rhel_9_base_images = [
    'rhel-9',
    'rhel-9-arm64',
    'rhel-9-lvm',
    'rhel-9-lvm-arm64',
    'rhel-9-byos',
    'rhel-9-byos-arm64',
  ],
  local rhel_9_sap_images = [
    'rhel-9-0-sap',
    'rhel-9-0-sap-byos',
    'rhel-9-2-sap',
    'rhel-9-2-sap-byos',
    'rhel-9-4-sap',
    'rhel-9-4-sap-byos',
    'rhel-9-6-sap',
    'rhel-9-6-sap-byos',
  ],
  local rhel_9_eus_images = [
    'rhel-9-4-eus',
    'rhel-9-4-eus-arm64',
    'rhel-9-4-eus-byos',
    'rhel-9-4-eus-byos-arm64',
    'rhel-9-6-eus',
    'rhel-9-6-eus-arm64',
    'rhel-9-6-eus-byos',
    'rhel-9-6-eus-byos-arm64',
  ],
  local rhel_10_base_images = [
    'rhel-10',
    'rhel-10-arm64',
    'rhel-10-byos',
    'rhel-10-byos-arm64',
    'rhel-10-lvm',
    'rhel-10-lvm-arm64',
  ],
  local rhel_10_eus_images = [
    'rhel-10-0-eus',
    'rhel-10-0-eus-arm64',
    'rhel-10-0-eus-byos',
    'rhel-10-0-eus-byos-arm64',
  ],
  local rhel_images = rhel_8_base_images + rhel_8_sap_images + rhel_9_base_images + rhel_9_sap_images + rhel_9_eus_images + rhel_10_base_images + rhel_10_eus_images,

  // Start of output.
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
               {
                 name: 'daily-time',
                 type: 'time',
                 source: { interval: '24h', start: '8:00 AM', stop: '8:30 AM', location: 'America/Los_Angeles', days: ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday'], initial_version: true },
               },
               common.gitresource { name: 'compute-image-tools' },
               common.gitresource { name: 'guest-test-infra' },
             ] +
             [
               common.gcsimgresource {
                 image: image,
                 regexp: 'debian/%s-v([0-9]+).tar.gz' % common.debian_image_prefixes[self.image],
               }
               for image in debian_images
             ] +
             [common.gcssbomresource {
               image: image,
               image_prefix: common.debian_image_prefixes[image],
               sbom_destination: 'debian',
             } for image in debian_images] +
             [common.gcsshasumresource {
               image: image,
               image_prefix: common.debian_image_prefixes[image],
               shasum_destination: 'debian',
             } for image in debian_images] +
             [common.gcsimgresource { image: image, gcs_dir: 'centos' } for image in centos_images] +
             [common.gcssbomresource { image: image, sbom_destination: 'centos' } for image in centos_images] +
             [common.gcsshasumresource { image: image, shasum_destination: 'centos' } for image in centos_images] +
             [common.gcsimgresource { image: image, gcs_dir: 'rhel' } for image in rhel_images] +
             [common.gcssbomresource { image: image, sbom_destination: 'rhel' } for image in rhel_images] +
             [common.gcsshasumresource { image: image, shasum_destination: 'rhel' } for image in rhel_images],
  jobs: [
          // Centos build jobs
          centosimgbuildjob { image: image }
          for image in centos_images
        ] +
        [
          // Debian build jobs
          debianimgbuildjob {
            image: image,
            image_prefix: common.debian_image_prefixes[image],
          }
          for image in debian_images
        ] +
        [
          // RHEL build jobs
          rhelimgbuildjob { image: image }
          for image in rhel_images
        ] +
        [
          // Debian publish jobs
          imgpublishjob {
            image: image,
            env: env,
            gcs_dir: 'debian',
            workflow_dir: 'debian',

            // Debian tarballs and images use a longer name, but jobs use the shorter name.
            image_prefix: common.debian_image_prefixes[image],
          }
          for env in envs
          for image in debian_images
        ] +
        [
          // RHEL publish jobs
          imgpublishjob {
            image: image,
            env: env,
            gcs_dir: 'rhel',
            workflow_dir: 'enterprise_linux',
          }
          for env in envs
          for image in rhel_images
        ] +
        [
          // CentOS publish jobs
          imgpublishjob {
            image: image,
            env: env,
            gcs_dir: 'centos',
            workflow_dir: 'enterprise_linux',
          }
          for env in envs
          for image in centos_images
        ],
  groups: [
    imggroup { name: 'debian', images: debian_images },
    imggroup {
      name: 'rhel-8-base',
      images: rhel_8_base_images,
    },
    imggroup{
      name: 'rhel-8-sap',
      images: rhel_8_sap_images,
    },
    imggroup{
      name: 'rhel-9-base',
      images: rhel_9_base_images,
    },
    imggroup{
      name: 'rhel-9-sap',
      images: rhel_9_sap_images,
    },
     imggroup{
      name: 'rhel-9-eus',
      images: rhel_9_eus_images,
    },
     imggroup{
      name: 'rhel-10-base',
      images: rhel_10_base_images,
    },
     imggroup{
      name: 'rhel-10-eus',
      images: rhel_10_eus_images,
    },
    imggroup { name: 'centos', images: centos_images },
  ],
}
