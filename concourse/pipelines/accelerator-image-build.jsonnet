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

local imgbuildjob = {
  local tl = self,

  image:: error 'must set image in imgbuildjob',
  image_prefix:: self.image,
  workflow_dir:: error 'must set workflow_dir in imgbuildjob',
  workflow:: '%s/%s.wf.json' % [tl.workflow_dir, underscore(tl.image)],
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

local elimgbuildjob = imgbuildjob {
  local tl = self,

  workflow_dir: 'enterprise_linux',
  sbom_util_secret_name:: 'sbom-util-secret',
  isopath:: trim_strings(tl.image, ['-nvidia-latest', '-nvidia-570']),

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

  // Publish to testing after build, if within the defined test window.
  passed:: if tl.env == 'testing' then
    'build-' + tl.image
  else if tl.env == 'prod' then
    'publish-to-testing-' + tl.image,
  
  citfilter:: common.default_linux_image_build_cit_filter,
  cit_extra_args:: [],
  cit_project:: common.default_cit_project,
  cit_test_projects:: common.default_cit_test_projects,

  // Rather than modifying the default CIT invocation above, it's also possible to specify a extra CIT invocations.
  // The images field will be overriden with the image under test.
  extra_test_tasks:: [],

   runtests:: if tl.env == 'testing' then true
   else false,

  // Start of job.
  name: 'publish-to-%s-%s' % [tl.env, tl.image],
  plan: [
          {
            get: 'time-' + tl.image,
            trigger: if tl.env == 'testing' then true
            else false,
          },
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
        // Invoke ARLE in prod
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
  local rocky_linux_accelerator_images = [
    'rocky-linux-8-optimized-gcp-nvidia-570',
    'rocky-linux-9-optimized-gcp-nvidia-570',
    'rocky-linux-8-optimized-gcp-nvidia-latest',
    'rocky-linux-9-optimized-gcp-nvidia-latest',
  ],

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
                 source: { interval: '24h', start: '10:00 PM', stop: '10:30 PM', location: 'America/Los_Angeles', days: ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday'], initial_version: true },
               },
               // Stagger the publish tasks by 1 hour so that A3/A4 tests cannot overlap causing capacity issues.
               {
                 name: 'time-rocky-linux-8-optimized-gcp-nvidia-570',
                 type: 'time',
                 source: { start: '11:30 PM', stop: '12:00 AM', location: 'America/Los_Angeles', days: ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday'], initial_version: true },
               },
               {
                 name: 'time-rocky-linux-8-optimized-gcp-nvidia-latest',
                 type: 'time',
                 source: { start: '1:00 AM', stop: '1:30 AM', location: 'America/Los_Angeles', days: ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday'], initial_version: true },
               },
               {
                 name: 'time-rocky-linux-9-optimized-gcp-nvidia-570',
                 type: 'time',
                 source: { start: '2:30 AM', stop: '3:00 AM', location: 'America/Los_Angeles', days: ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday'], initial_version: true },
               },
               {
                 name: 'time-rocky-linux-9-optimized-gcp-nvidia-latest',
                 type: 'time',
                 source: { start: '4:00 AM', stop: '4:30 AM', location: 'America/Los_Angeles', days: ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday'], initial_version: true },
               },
               common.gitresource { name: 'compute-image-tools' },
               common.gitresource { name: 'guest-test-infra' },
             ] +
             [common.gcsimgresource { image: image, gcs_dir: 'rocky-linux' } for image in rocky_linux_accelerator_images ] +
             [common.gcssbomresource { image: image, sbom_destination: 'rocky-linux' } for image in rocky_linux_accelerator_images] +
             [common.gcsshasumresource { image: image, shasum_destination: 'rocky-linux' } for image in rocky_linux_accelerator_images],
  jobs: [
          // EL build jobs
          elimgbuildjob { image: image }
          for image in rocky_linux_accelerator_images
        ] +
        [
          // Accelerator publish jobs to testing
          imgpublishjob {
            image: 'rocky-linux-8-optimized-gcp-nvidia-570',
            env: 'testing',
            gcs_dir: 'rocky-linux',
            workflow_dir: 'enterprise_linux',
            # Add accelerator tests
            extra_test_tasks: [
              common.imagetesttask {
                task: 'accelerator-tests-a3u',
                filter: '^(acceleratorrdma|acceleratorconfig)$',
                project: 'compute-image-test-pool-001',
                test_projects: 'compute-image-test-pool-001',
                extra_args:: [ '-parallel_count=1', '-compute_endpoint_override=https://www.googleapis.com/compute/alpha/', '-use_reservations=true', '-reservation_urls=nvidia-h200-8mx2qd0luip8o', '-x86_shape=a3-ultragpu-8g', '-zone=europe-west1-b', '-accelerator_type=nvidia-h200-141gb' ],
              },
              common.imagetesttask {
                task: 'accelerator-tests-a4',
                filter: '^(acceleratorrdma|acceleratorconfig)$',
                project: 'compute-image-test-pool-001',
                test_projects: 'compute-image-test-pool-001',
                extra_args:: [ '-parallel_count=1', '-compute_endpoint_override=https://www.googleapis.com/compute/alpha/', '-use_reservations=true', '-reservation_urls=nvidia-b200-9r888mvujoxrz', '-x86_shape=a4-highgpu-8g', '-zone=us-central1-b', '-accelerator_type=nvidia-b200' ],
              },
            ],
          },
          imgpublishjob {
            image: 'rocky-linux-9-optimized-gcp-nvidia-570',
            env: 'testing',
            gcs_dir: 'rocky-linux',
            workflow_dir: 'enterprise_linux',
            # Add accelerator tests
            extra_test_tasks: [
              common.imagetesttask {
                task: 'accelerator-tests-a3u',
                filter: '^(acceleratorrdma|acceleratorconfig)$',
                project: 'compute-image-test-pool-001',
                test_projects: 'compute-image-test-pool-001',
                extra_args:: [ '-parallel_count=1', '-compute_endpoint_override=https://www.googleapis.com/compute/alpha/', '-use_reservations=true', '-reservation_urls=nvidia-h200-8mx2qd0luip8o', '-x86_shape=a3-ultragpu-8g', '-zone=europe-west1-b', '-accelerator_type=nvidia-h200-141gb' ],
              },
              common.imagetesttask {
                task: 'accelerator-tests-a4',
                filter: '^(acceleratorrdma|acceleratorconfig)$',
                project: 'compute-image-test-pool-001',
                test_projects: 'compute-image-test-pool-001',
                extra_args:: [ '-parallel_count=1', '-compute_endpoint_override=https://www.googleapis.com/compute/alpha/', '-use_reservations=true', '-reservation_urls=nvidia-b200-9r888mvujoxrz', '-x86_shape=a4-highgpu-8g', '-zone=us-central1-b', '-accelerator_type=nvidia-b200' ],
              },
            ],
          },
          imgpublishjob {
            image: 'rocky-linux-8-optimized-gcp-nvidia-latest',
            env: 'testing',
            gcs_dir: 'rocky-linux',
            workflow_dir: 'enterprise_linux',
            # Add accelerator tests
            extra_test_tasks: [
              common.imagetesttask {
                task: 'accelerator-tests-a3u',
                filter: '^(acceleratorrdma|acceleratorconfig)$',
                project: 'compute-image-test-pool-001',
                test_projects: 'compute-image-test-pool-001',
                extra_args:: [ '-parallel_count=1', '-compute_endpoint_override=https://www.googleapis.com/compute/alpha/', '-use_reservations=true', '-reservation_urls=nvidia-h200-8mx2qd0luip8o', '-x86_shape=a3-ultragpu-8g', '-zone=europe-west1-b', '-accelerator_type=nvidia-h200-141gb' ],
              },
              common.imagetesttask {
                task: 'accelerator-tests-a4',
                filter: '^(acceleratorrdma|acceleratorconfig)$',
                project: 'compute-image-test-pool-001',
                test_projects: 'compute-image-test-pool-001',
                extra_args:: [ '-parallel_count=1', '-compute_endpoint_override=https://www.googleapis.com/compute/alpha/', '-use_reservations=true', '-reservation_urls=nvidia-b200-9r888mvujoxrz', '-x86_shape=a4-highgpu-8g', '-zone=us-central1-b', '-accelerator_type=nvidia-b200' ],
              },
            ],
          },
          imgpublishjob {
            image: 'rocky-linux-9-optimized-gcp-nvidia-latest',
            env: 'testing',
            gcs_dir: 'rocky-linux',
            workflow_dir: 'enterprise_linux',
            # Add accelerator tests
            extra_test_tasks: [
              common.imagetesttask {
                task: 'accelerator-tests-a3u',
                filter: '^(acceleratorrdma|acceleratorconfig)$',
                project: 'compute-image-test-pool-001',
                test_projects: 'compute-image-test-pool-001',
                extra_args:: [ '-parallel_count=1', '-compute_endpoint_override=https://www.googleapis.com/compute/alpha/', '-use_reservations=true', '-reservation_urls=nvidia-h200-8mx2qd0luip8o', '-x86_shape=a3-ultragpu-8g', '-zone=europe-west1-b', '-accelerator_type=nvidia-h200-141gb' ],
              },
              common.imagetesttask {
                task: 'accelerator-tests-a4',
                filter: '^(acceleratorrdma|acceleratorconfig)$',
                project: 'compute-image-test-pool-001',
                test_projects: 'compute-image-test-pool-001',
                extra_args:: [ '-parallel_count=1', '-compute_endpoint_override=https://www.googleapis.com/compute/alpha/', '-use_reservations=true', '-reservation_urls=nvidia-b200-9r888mvujoxrz', '-x86_shape=a4-highgpu-8g', '-zone=us-central1-b', '-accelerator_type=nvidia-b200' ],
              },
            ],
          }
        ] +
        [
          // Accelerator publish jobs to prod
          imgpublishjob {
            image: 'rocky-linux-8-optimized-gcp-nvidia-570',
            env: 'prod',
            gcs_dir: 'rocky-linux',
            workflow_dir: 'enterprise_linux',
          },
          imgpublishjob {
            image: 'rocky-linux-9-optimized-gcp-nvidia-570',
            env: 'prod',
            gcs_dir: 'rocky-linux',
            workflow_dir: 'enterprise_linux',
          },
          imgpublishjob {
            image: 'rocky-linux-8-optimized-gcp-nvidia-latest',
            env: 'prod',
            gcs_dir: 'rocky-linux',
            workflow_dir: 'enterprise_linux',
          },
          imgpublishjob {
            image: 'rocky-linux-9-optimized-gcp-nvidia-latest',
            env: 'prod',
            gcs_dir: 'rocky-linux',
            workflow_dir: 'enterprise_linux',
          }
        ],
  groups: [
    imggroup { name: 'rocky-linux', images: rocky_linux_accelerator_images},
  ],
}
