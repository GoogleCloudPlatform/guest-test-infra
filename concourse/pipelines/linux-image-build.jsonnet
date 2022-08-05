// Imports.
local arle = import '../templates/arle.libsonnet';
local common = import '../templates/common.libsonnet';
local daisy = import '../templates/daisy.libsonnet';
local gcp_secret_manager = import '../templates/gcp-secret-manager.libsonnet';
local lego = import '../templates/lego.libsonnet';

// Common
local envs = ['testing', 'staging', 'oslogin-staging', 'prod'];
local underscore(input) = std.strReplace(input, '-', '_');

local imgbuildtask = daisy.daisyimagetask {
  gcs_url: '((.:gcs-url))',
};

local rhuiimgbuildtask = imgbuildtask {
  project: 'google.com:rhel-infra',
  vars+: ['instance_service_account=rhui-builder@rhel-infra.google.com.iam.gserviceaccount.com'],

  run+: {
    // Prepend, as the workflow must be the last arg. Daisy is picky.
    args: ['-gcs_path=gs://rhel-infra-daisy-bkt/'] + super.args,
  },
};

local imagetesttask = {
  local task = self,

  images:: error 'must set images in imagetesttask',
  extra_args:: [],

  // Start of task
  platform: 'linux',
  image_resource: {
    type: 'registry-image',
    source: { repository: 'gcr.io/compute-image-tools/cloud-image-tests' },
  },
  run: {
    path: '/manager',
    args: [
      '-project=gcp-guest',
      '-zone=us-central1-a',
      '-test_projects=compute-image-test-pool-002,compute-image-test-pool-003,compute-image-test-pool-004,compute-image-test-pool-005',
      '-exclude=oslogin',
      '-images=' + task.images,
    ] + task.extra_args,
  },
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
      task: 'generate-build-id',
      file: 'guest-test-infra/concourse/tasks/generate-build-id.yaml',
      vars: { prefix: tl.image_prefix },
    },
    // This is the 'put trick'. We don't have the real image tarball to write to GCS here, but we want
    // Concourse to treat this job as producing it. So we write an empty file now, and overwrite it later in
    // the daisy workflow. This also generates the final URL for use in the daisy workflow.
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
  isopath:: std.strReplace(std.strReplace(tl.image, '-byos', ''), '-sap', ''),

  // Add tasks to obtain ISO location and store it in .:iso-secret
  extra_tasks: [
    {
      task: 'get-secret-iso',
      config: gcp_secret_manager.getsecrettask { secret_name: tl.isopath },
    },
    {
      load_var: 'iso-secret',
      file: 'gcp-secret-manager/' + tl.isopath,
    },
  ],

  // Add EL args to build task.
  build_task+: { vars+: ['installer_iso=((.:iso-secret))'] },
};

local cdsimgbuildjob = imgbuildjob {
  local tl = self,

  local acme_server = 'dv.acme-v02.api.pki.goog',
  local acme_email = 'bigcluster-guest-team@google.com',
  local rhui_project = 'google.com:rhel-infra',

  image: 'cds',
  workflow_dir: 'rhui',
  daily: false,

  extra_tasks: [
    {
      task: 'get-acme-account-json',
      config: gcp_secret_manager.getsecrettask {
        secret_name: 'rhui_acme_account_json',
        project: rhui_project,
        output_path: 'accounts/%s/%s/account.json' % [acme_server, acme_email],
      },
    },
    {
      task: 'get-acme-account-key',
      config: gcp_secret_manager.getsecrettask {
        secret_name: 'rhui_acme_account_key',
        project: rhui_project,
        output_path: 'accounts/%s/%s/keys/%s.key' % [acme_server, acme_email, acme_email],

        // Layer onto the same output as previous task
        inputs+: gcp_secret_manager.getsecrettask.outputs,
      },
    },
    {
      task: 'get-rhui-tls-key',
      config: gcp_secret_manager.getsecrettask {
        secret_name: 'rhui_tls_key',
        project: rhui_project,

        // Layer onto the same output as previous task
        inputs+: gcp_secret_manager.getsecrettask.outputs,
      },
    },
    {
      task: 'generate-csr',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: { repository: 'alpine/openssl' },
        },
        inputs: [{ name: 'gcp-secret-manager' }],
        outputs: [{ name: 'rhui-csr' }],
        run: {
          path: 'openssl',
          args: [
            'req',
            '-new',
            '-key=./gcp-secret-manager/rhui_tls_key',
            '-subj=/CN=rhui.googlecloud.com',
            '-out=./rhui-csr/thecsr.pem',
          ],
        },
      },
    },
    {
      task: 'lego-provision-tls-crt',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: { repository: 'goacme/lego' },
        },
        params: { GCE_PROJECT: rhui_project },
        inputs: [
          { name: 'gcp-secret-manager' },
          { name: 'rhui-csr' },
        ],
        outputs: [{ name: 'gcp-secret-manager' }],
        run: {
          path: 'lego',
          args: [
            '--csr=./rhui-csr/thecsr.pem',
            '--email=' + acme_email,
            '--server=https://%s/directory' % acme_server,
            '--accept-tos',
            '--eab',
            '--dns.resolvers=ns-cloud-b2.googledomains.com:53',
            '--dns=gcloud',
            '--path=./gcp-secret-manager/',
            'run',
          ],
        },
      },
    },
  ],

  // Append var to Daisy build task
  build_task: rhuiimgbuildtask {
    workflow: tl.workflow,
    inputs+: [{ name: 'gcp-secret-manager' }],
    vars+: ['tls_cert_path=../../../../gcp-secret-manager/certificates/rhui.googlecloud.com.crt'],
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

  passed:: if tl.env == 'testing' then
    'build-' + tl.image
  else
    'publish-to-testing-' + tl.image,

  trigger:: if tl.env == 'testing' then true else false,

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
          {
            get: tl.image + '-gcs',
            passed: [tl.passed],
            trigger: tl.trigger,
            params: { skip_download: 'true' },
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
          // Prod releases use a different final publish step that invokes ARLE.
          if tl.env == 'prod' then
            {
              task: 'publish-' + tl.image,
              config: arle.arlepublishtask {
                gcs_image_path: tl.gcs,
                source_version: 'v((.:source-version))',
                publish_version: '((.:publish-version))',
                wf: tl.workflow,
                image_name: underscore(tl.image),
              },
            }
          // Other releases use gce_image_publish directly.
          else
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
        ] +
        // Run post-publish tests in 'publish-to-testing-' jobs.
        if tl.env == 'testing' then
          [
            {
              task: 'image-test-' + tl.image,
              config: imagetesttask {
                images: 'projects/bct-prod-images/global/images/%s-((.:publish-version))' % tl.image_prefix,
                // Special case ARM for now.
                extra_args: if
                  std.endsWith(tl.image_prefix, '-arm64')
                then
                  ['-machine_type=t2a-standard-2']
                else [],
              },
              attempts: 3,
            },
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

local saptestjob = {
  local tl = self,

  image:: error 'must set image in saptestjob',

  // Start of job.
  name: 'sap-workload-test-' + self.image,
  plan: [
    {
      get: tl.image + '-gcs',
      passed: ['publish-to-testing-' + tl.image],
      params: { skip_download: 'true' },
    },
    { get: 'guest-test-infra' },
    {
      task: 'generate-timestamp',
      file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
    },
    { load_var: 'id', file: 'timestamp/timestamp-ms' },
    {
      task: 'generate-post-script',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: {
            repository: 'google/cloud-sdk',
            tag: 'alpine',
          },
        },
        inputs: [{ name: 'guest-test-infra' }],
        run: {
          path: 'sh',
          dir: 'guest-test-infra/concourse/scripts',
          args: [
            '-exc',
            |||
              # We want to upload this actual script with the unique id
              sed -i 's/__BUCKET__/gcp-guest-test-outputs/g' sap_post_script.sh
              sed -i 's/__RUNID__/((.:id))/g' sap_post_script.sh
              gsutil cp sap_post_script.sh gs://gcp-guest-test-outputs/workload-tests/sap/((.:id))/sap_post_script.sh
            |||,
          ],
        },
      },
    },
    {
      task: 'terraform-init',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: { repository: 'hashicorp/terraform' },
        },
        inputs: [{ name: 'guest-test-infra' }],
        outputs: [{ name: 'guest-test-infra' }],
        run: {
          path: 'terraform',
          dir: 'guest-test-infra/concourse/scripts',
          args: [
            'init',
            '-upgrade',
          ],
        },
      },
    },
    {
      task: 'terraform-apply',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: { repository: 'hashicorp/terraform' },
        },
        inputs: [{ name: 'guest-test-infra' }],
        outputs: [{ name: 'guest-test-infra' }],
        run: {
          path: 'terraform',
          dir: 'guest-test-infra/concourse/scripts',
          args: [
            'apply',
            '-auto-approve',
            '-var=instance_name=hana-instance-((.:id))',
            '-var=post_deployment_script=gs://gcp-guest-test-outputs/workload-tests/sap/((.:id))/sap_post_script.sh',
            '-var=linux_image=%(image)s-ha' % { image: tl.image },
          ],
        },
      },
    },
    {
      task: 'wait-for-and-check-post-script-results',
      timeout: '30m',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: {
            repository: 'google/cloud-sdk',
            tag: 'alpine',
          },
        },
        run: {
          path: 'sh',
          args: [
            '-exc',
            |||
              until gsutil -q stat gs://gcp-guest-test-outputs/workload-tests/sap/((.:id))/run_result
              do
                echo "Waiting for results..."
                sleep 60
              done

              gsutil cat gs://gcp-guest-test-outputs/workload-tests/sap/((.:id))/run_result | grep -q "SUCCESS"
            |||,
          ],
        },
      },
    },
    {
      task: 'terraform-destroy',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: { repository: 'hashicorp/terraform' },
        },
        inputs: [{ name: 'guest-test-infra' }],
        run: {
          path: 'terraform',
          dir: 'guest-test-infra/concourse/scripts',
          args: [
            'destroy',
            '-auto-approve',
            '-var=instance_name=hana-instance-((.:id))',
          ],
        },
      },
    },
  ],
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
  local almalinux_images = ['almalinux-8', 'almalinux-9'],
  local debian_images = ['debian-10', 'debian-11', 'debian-11-arm64'],
  local centos_images = ['centos-7', 'centos-stream-8', 'centos-stream-9'],
  local rhel_sap_images = [
    'rhel-7-6-sap',
    'rhel-7-7-sap',
    'rhel-7-9-sap',
    'rhel-8-1-sap',
    'rhel-8-2-sap',
    'rhel-8-4-sap',
    'rhel-8-6-sap',
  ],
  local rhel_images = rhel_sap_images + [
    'rhel-7',
    'rhel-7-byos',
    'rhel-8',
    'rhel-8-byos',
    'rhel-9',
    'rhel-9-arm64',
    'rhel-9-byos',
  ],
  local rocky_linux_images = [
    'rocky-linux-8',
    'rocky-linux-8-optimized-gcp',
    'rocky-linux-8-optimized-gcp-arm64',
    'rocky-linux-9',
    'rocky-linux-9-arm64',
    'rocky-linux-9-optimized-gcp',
    'rocky-linux-9-optimized-gcp-arm64',
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
                 source: { interval: '24h' },
               },
               common.gitresource { name: 'compute-image-tools' },
               common.gitresource { name: 'guest-test-infra' },
               common.gcsimgresource { image: 'rhua', gcs_dir: 'rhui' },
               common.gcsimgresource { image: 'cds', gcs_dir: 'rhui' },
             ] +
             [common.gcsimgresource { image: image, gcs_dir: 'almalinux' } for image in almalinux_images] +
             [common.gcsimgresource { image: image, gcs_dir: 'rocky-linux' } for image in rocky_linux_images] +
             [
               common.gcsimgresource {
                 image: image,
                 regexp: 'debian/%s-v([0-9]+).tar.gz' % common.debian_image_prefixes[self.image],
               }
               for image in debian_images
             ] +
             [common.gcsimgresource { image: image, gcs_dir: 'centos' } for image in centos_images] +
             [common.gcsimgresource { image: image, gcs_dir: 'rhel' } for image in rhel_images],
  jobs: [
          // Debian build jobs
          imgbuildjob {
            image: image,
            workflow_dir: 'debian',
            image_prefix: common.debian_image_prefixes[image],
          }
          for image in debian_images
        ] +
        [
          // EL build jobs
          elimgbuildjob { image: image }
          for image in rhel_images + centos_images + almalinux_images + rocky_linux_images
        ] +
        [
          // RHUI build jobs.
          imgbuildjob {
            local tl = self,

            image: 'rhua',
            workflow_dir: 'rhui',
            daily: false,

            // Append var to Daisy image build task
            build_task: rhuiimgbuildtask { workflow: tl.workflow },
          },
          cdsimgbuildjob,
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
          // SAP related test jobs
          saptestjob { image: image }
          for image in rhel_sap_images
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
        ] +
        [
          // AlmaLinux publish jobs
          imgpublishjob {
            image: image,
            env: env,
            gcs_dir: 'almalinux',
            workflow_dir: 'enterprise_linux',
          }
          for env in envs
          for image in almalinux_images
        ] +
        [
          // Rocky Linux publish jobs
          imgpublishjob {
            image: image,
            env: env,
            gcs_dir: 'rocky-linux',
            workflow_dir: 'enterprise_linux',
          }
          for env in envs
          for image in rocky_linux_images
        ] +
        [
          imgpublishjob {
            image: image,
            env: 'testing',
            gcs_dir: 'rhui',
            workflow_dir: 'rhui',
          }
          for image in ['cds', 'rhua']
        ],
  groups: [
    imggroup { name: 'debian', images: debian_images },
    imggroup {
      name: 'rhel',
      images: rhel_images,
      jobs+:
        [
          'sap-workload-test-%s' % [image]
          for image in rhel_sap_images
        ],
    },
    imggroup { name: 'centos', images: centos_images },
    imggroup { name: 'almalinux', images: almalinux_images },
    imggroup { name: 'rocky-linux', images: rocky_linux_images },
    imggroup { name: 'rhui', images: ['rhua', 'cds'], envs: ['testing'] },
  ],
}
