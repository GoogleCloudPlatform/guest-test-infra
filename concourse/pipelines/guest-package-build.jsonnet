local underscore(input) = std.strReplace(input, '-', '_');

local buildpackagejob = {
  local tl = self,

  package:: error 'must set package in buildpackagejob',
  gcs_dir:: tl.package,
  repos:: error 'must set repos in buildpackagejob',
  uploads:: error 'must set uploads in buildpackagejob',
  extra_tasks:: [],

  // Start of output.
  name: 'build-' + tl.package,
  plan: [
    {
      in_parallel: {
        steps: [
          {
            get: tl.package,
            trigger: true,
            params: { skip_download: true },
          },
          { get: 'guest-test-infra' },
        ],
      },
    },
    { load_var: 'commit-sha', file: '%s/.git/ref' % tl.package },
    {
      task: 'generate-timestamp',
      file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
    },
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    {
      task: 'get-github-token',
      file: 'guest-test-infra/concourse/tasks/get-github-token.yaml',
    },
    {
      task: 'generate-package-version',
      file: 'guest-test-infra/concourse/tasks/generate-package-version.yaml',
      input_mapping: { repo: tl.package },
    },
    { load_var: 'package-version', file: 'package-version/version' },
    {
      in_parallel: {
        fail_fast: true,
        steps: [
          {
            task: 'guest-package-build-%s-%s' % [tl.package, repo],
            file: 'guest-test-infra/concourse/tasks/guest-package-build.yaml',
            vars: {
              wf: 'build_%s.wf.json' % underscore(repo),
              'repo-name': tl.package,
              version: '((.:package-version))',
              gcs_path: 'gs://gcp-guest-package-uploads/' + tl.gcs_dir,
              git_ref: '((.:commit-sha))',
            },
          }
          for repo in tl.repos
        ],
      },
    },
  ] + tl.extra_tasks + [
    {
      in_parallel: {
        fail_fast: true,
        steps: tl.uploads,
      },
    },
    {
      put: '%s-tag' % tl.package,
      params: {
        name: 'package-version/version',
        tag: 'package-version/version',
        commitish: '%s/.git/ref' % tl.package,
      },
    },
  ],
  on_success: {
    task: 'success',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'guest-package-build',
      job: 'build-' + tl.package,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'failure',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'guest-package-build',
      job: 'build-' + tl.package,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local packageuploadtask = {
  local tl = self,

  package_paths:: error 'must set package_paths in packageuploadtask',
  repo:: error 'must set rapture_repo in packageuploadtask',
  topic:: 'projects/artifact-releaser-prod/topics/gcp-guest-package-upload-prod',
  type:: 'uploadToStaging',
  universe:: error 'must set universe in packageuploadtask',

  task: 'upload-' + tl.repo,
  file: 'guest-test-infra/concourse/tasks/gcloud-package-operation.yaml',
  vars: {
    package_paths: tl.package_paths,
    universe: tl.universe,
    repo: tl.repo,
  },
  params: { TYPE: tl.type },
};

local promotepackagejob = {
  local tl = self,

  package:: error 'must set package in promotepackagejob',
  promotions:: error 'must set promotions in promotepackagejob',
  dest:: error 'must set dest in promotepackagejob',
  passed:: if tl.dest == 'staging' then
    'build-' + tl.package
  else
    'promote-%s-staging' % tl.package,
  tag:: true,

  // Start of output.
  name: 'promote-%s-%s' % [tl.package, tl.dest],
  plan: [
    {
      get: '%s-tag' % tl.package,
      passed: [tl.passed],
    },
    {
      get: tl.package,
      params: { fetch_tags: true },
    },
    { get: 'guest-test-infra' },
    {
      task: 'generate-timestamp',
      file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
    },
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    {
      task: 'get-last-stable-date',
      file: 'guest-test-infra/concourse/tasks/get-last-stable-tag.yaml',
      input_mapping: { repo: tl.package },
    },
    { load_var: 'last-stable-date', file: 'last-stable-tag/date' },
    { in_parallel: tl.promotions },
  ] + if tl.tag then [
    {
      put: '%s-tag' % tl.package,
      params: {
        name: 'last-stable-tag/stable',
        tag: 'last-stable-tag/stable',
        commitish: '%s-tag/commit_sha' % tl.package,
      },
    },
  ] else [],
  on_success: {
    task: 'success',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'guest-package-build',
      job: tl.name,
      result_state: 'success',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
  on_failure: {
    task: 'failure',
    file: 'guest-test-infra/concourse/tasks/publish-job-result.yaml',
    vars: {
      pipeline: 'guest-package-build',
      job: tl.name,
      result_state: 'failure',
      start_timestamp: '((.:start-timestamp-ms))',
    },
  },
};

local promotepackagestagingtask = {
  local tl = self,

  repo:: error 'must set repo in promotepackagestagingtask',
  universe:: error 'must set universe in promotepackagestagingtask',

  // Start of output.
  task: 'promote-staging-' + tl.repo,
  file: 'guest-test-infra/concourse/tasks/gcloud-package-operation.yaml',
  vars: {
    package_paths: '',  // Unused for this RPC but required by the task.
    repo: tl.repo,
    universe: tl.universe,
  },
  params: { TYPE: 'promoteToStaging' },
};

local promotepackagestabletask = {
  local tl = self,

  repo:: error 'must set repo in promotepackagestabletask',
  universe:: error 'must set universe in promotepackagestabletask',

  // Start of output.
  task: 'promote-stable-' + tl.repo,
  file: 'guest-test-infra/concourse/tasks/gcloud-promote-package.yaml',
  vars: {
    environment: 'stable',
    repo: tl.repo,
    universe: tl.universe,
  },
};

// Start of output
{
  jobs: [
    buildpackagejob {
      package: 'guest-agent',
      repos: ['deb9', 'deb11-arm64', 'el7', 'el8', 'el9', 'goo'],
      // The guest agent has additional testing steps to build derivative images then run CIT against them.
      extra_tasks: [
        {
          task: 'generate-build-id',
          file: 'guest-test-infra/concourse/tasks/generate-build-id.yaml',
          vars: { prefix: '' },
        },
        { load_var: 'build-id', file: 'build-id-dir/build-id' },
        {
          in_parallel: {
            fail_fast: true,
            steps: [
              {
                task: 'build-package-image-debian-9',
                file: 'guest-test-infra/concourse/tasks/daisy-build-package-image.yaml',
                vars: {
                  'source-image': 'projects/debian-cloud/global/images/family/debian-9',
                  'dest-image': 'debian-9-((.:build-id))',
                  'gcs-package-path': 'gs://gcp-guest-package-uploads/guest-agent/google-guest-agent_((.:package-version))-g1_amd64.deb',
                  'machine-type': 'e2-medium',
                  'worker-image': 'projects/compute-image-tools/global/images/family/debian-10-worker',
                },
              },
              {
                task: 'build-package-image-debian-10',
                file: 'guest-test-infra/concourse/tasks/daisy-build-package-image.yaml',
                vars: {
                  'source-image': 'projects/debian-cloud/global/images/family/debian-10',
                  'dest-image': 'debian-10-((.:build-id))',
                  'gcs-package-path': 'gs://gcp-guest-package-uploads/guest-agent/google-guest-agent_((.:package-version))-g1_amd64.deb',
                  'machine-type': 'e2-medium',
                  'worker-image': 'projects/compute-image-tools/global/images/family/debian-10-worker',
                },
              },
              {
                task: 'build-package-image-debian-11',
                file: 'guest-test-infra/concourse/tasks/daisy-build-package-image.yaml',
                vars: {
                  'source-image': 'projects/debian-cloud/global/images/family/debian-11',
                  'dest-image': 'debian-11-((.:build-id))',
                  'gcs-package-path': 'gs://gcp-guest-package-uploads/guest-agent/google-guest-agent_((.:package-version))-g1_amd64.deb',
                  'machine-type': 'e2-medium',
                  'worker-image': 'projects/compute-image-tools/global/images/family/debian-10-worker',
                },
              },
              {
                task: 'build-package-image-debian-11-arm64',
                file: 'guest-test-infra/concourse/tasks/daisy-build-package-image.yaml',
                vars: {
                  'source-image': 'projects/debian-cloud-testing/global/images/family/debian-11-arm64',
                  'dest-image': 'debian-11-arm64-((.:build-id))',
                  'gcs-package-path': 'gs://gcp-guest-package-uploads/guest-agent/google-guest-agent_((.:package-version))-g1_arm64.deb',
                  'machine-type': 't2a-standard-2',
                  'worker-image': 'projects/compute-image-tools/global/images/family/debian-11-worker-arm64',
                },
              },
              {
                task: 'build-package-image-centos-7',
                file: 'guest-test-infra/concourse/tasks/daisy-build-package-image.yaml',
                vars: {
                  'source-image': 'projects/centos-cloud/global/images/family/centos-7',
                  'dest-image': 'centos-7-((.:build-id))',
                  'gcs-package-path': 'gs://gcp-guest-package-uploads/guest-agent/google-guest-agent-((.:package-version))-g1.el7.x86_64.rpm',
                  'machine-type': 'e2-medium',
                  'worker-image': 'projects/compute-image-tools/global/images/family/debian-10-worker',
                },
              },
              {
                task: 'build-package-image-rhel-7',
                file: 'guest-test-infra/concourse/tasks/daisy-build-package-image.yaml',
                vars: {
                  'source-image': 'projects/rhel-cloud/global/images/family/rhel-7',
                  'dest-image': 'rhel-7-((.:build-id))',
                  'gcs-package-path': 'gs://gcp-guest-package-uploads/guest-agent/google-guest-agent-((.:package-version))-g1.el7.x86_64.rpm',
                  'machine-type': 'e2-medium',
                  'worker-image': 'projects/compute-image-tools/global/images/family/debian-10-worker',
                },
              },
              {
                task: 'build-package-image-rhel-8',
                file: 'guest-test-infra/concourse/tasks/daisy-build-package-image.yaml',
                vars: {
                  'source-image': 'projects/rhel-cloud/global/images/family/rhel-8',
                  'dest-image': 'rhel-8-((.:build-id))',
                  'gcs-package-path': 'gs://gcp-guest-package-uploads/guest-agent/google-guest-agent-((.:package-version))-g1.el8.x86_64.rpm',
                  'machine-type': 'e2-medium',
                  'worker-image': 'projects/compute-image-tools/global/images/family/debian-10-worker',
                },
              },
            ],
          },
        },
        {
          in_parallel: {
            fail_fast: true,
            steps: [
              {
                task: 'guest-agent-image-tests-amd64',
                file: 'guest-test-infra/concourse/tasks/image-test-args.yaml',
                vars: {
                  images: 'projects/gcp-guest/global/images/debian-9-((.:build-id)),projects/gcp-guest/global/images/debian-10-((.:build-id)),projects/gcp-guest/global/images/debian-11-((.:build-id)),projects/gcp-guest/global/images/centos-7-((.:build-id)),projects/gcp-guest/global/images/rhel-7-((.:build-id)),projects/gcp-guest/global/images/rhel-8-((.:build-id))',
                  'machine-type': 'e2-medium',
                  'extra-args': '-exclude=(image)|(disk)|(security)|(oslogin)',
                },
              },
              {
                task: 'guest-agent-image-tests-arm64',
                file: 'guest-test-infra/concourse/tasks/image-test-args.yaml',
                vars: {
                  images: 'projects/gcp-guest/global/images/debian-11-arm64-((.:build-id))',
                  'machine-type': 't2a-standard-2',
                  'extra-args': '-exclude=(image)|(disk)|(security)|(oslogin)',
                },
              },
            ],
          },
        },
      ],
      uploads: [
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent_((.:package-version))-g1_amd64.deb"}',
          repo: 'google-guest-agent-stretch',
          type: 'uploadToStaging',
          universe: 'cloud-apt',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent_((.:package-version))-g1_amd64.deb"}',
          repo: 'google-guest-agent-buster',
          type: 'uploadToStaging',
          universe: 'cloud-apt',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent_((.:package-version))-g1_amd64.deb"},{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent_((.:package-version))-g1_arm64.deb"}',
          repo: 'google-guest-agent-bullseye',
          type: 'uploadToStaging',
          universe: 'cloud-apt',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version))-g1.el7.x86_64.rpm"}',
          repo: 'google-guest-agent-el7',
          type: 'uploadToStaging',
          universe: 'cloud-yum',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version))-g1.el8.x86_64.rpm"}',
          repo: 'google-guest-agent-el8',
          type: 'uploadToStaging',
          universe: 'cloud-yum',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version))-g1.el9.x86_64.rpm"}',
          repo: 'google-guest-agent-el9',
          type: 'uploadToStaging',
          universe: 'cloud-yum',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-compute-engine-windows.x86_64.((.:package-version)).0@1.goo"}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-windows',
          type: 'uploadToStaging',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-compute-engine-metadata-scripts.x86_64.((.:package-version)).0@1.goo"}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-metadata-scripts',
          type: 'uploadToStaging',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-compute-engine-metadata-scripts.x86_64.((.:package-version)).0@1.goo"}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-metadata-scripts',
          TYPE: 'uploadToStaging',
        },
      ],
    },
    promotepackagejob {
      package: 'guest-agent',
      dest: 'stable',
      promotions: [
        promotepackagestabletask { repo: 'google-guest-agent-stretch', universe: 'cloud-apt' },
        promotepackagestabletask { repo: 'google-guest-agent-buster', universe: 'cloud-apt' },
        promotepackagestabletask { repo: 'google-guest-agent-bullseye', universe: 'cloud-apt' },
        promotepackagestabletask { repo: 'google-guest-agent-el7', universe: 'cloud-yum' },
        promotepackagestabletask { repo: 'google-guest-agent-el8', universe: 'cloud-yum' },
        promotepackagestabletask { repo: 'google-guest-agent-el9', universe: 'cloud-yum' },
      ],
    },
    promotepackagejob {
      package: 'guest-agent',
      dest: 'stable',
      name: 'promote-guest-agent-windows-stable',
      tag: false,
      promotions: [
        promotepackagestabletask { repo: 'google-compute-engine-windows', universe: 'cloud-yuck' },
      ],
    },
    promotepackagejob {
      package: 'guest-agent',
      dest: 'stable',
      name: 'promote-metadata-scripts-windows-stable',
      tag: false,
      promotions: [
        promotepackagestabletask { repo: 'google-compute-engine-metadata-scripts', universe: 'cloud-yuck' },
      ],
    },
    buildpackagejob {
      package: 'guest-oslogin',
      repos: ['deb9', 'deb10', 'deb11', 'deb11-arm64', 'el7', 'el8', 'el9'],
      gcs_dir: 'oslogin',
      uploads: [
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb9_amd64.deb"}',
          repo: 'gce-google-compute-engine-oslogin-stretch',
          universe: 'cloud-apt',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb10_amd64.deb"}',
          repo: 'gce-google-compute-engine-oslogin-buster',
          universe: 'cloud-apt',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb11_amd64.deb"}, {"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb11_arm64.deb"}',
          repo: 'gce-google-compute-engine-oslogin-bullseye',
          universe: 'cloud-apt',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el7.x86_64.rpm"}',
          repo: 'gce-google-compute-engine-oslogin-el7',
          universe: 'cloud-yum',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el8.x86_64.rpm"}',
          repo: 'gce-google-compute-engine-oslogin-el8',
          universe: 'cloud-yum',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el9.x86_64.rpm"}',
          repo: 'gce-google-compute-engine-oslogin-el9',
          universe: 'cloud-yum',
        },
      ],
    },
    promotepackagejob {
      package: 'guest-oslogin',
      dest: 'stable',
      promotions: [
        promotepackagestabletask { universe: 'cloud-apt', repo: 'gce-google-compute-engine-oslogin-stretch' },
        promotepackagestabletask { universe: 'cloud-apt', repo: 'gce-google-compute-engine-oslogin-buster' },
        promotepackagestabletask { universe: 'cloud-apt', repo: 'gce-google-compute-engine-oslogin-bullseye' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'gce-google-compute-engine-oslogin-el7' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'gce-google-compute-engine-oslogin-el8' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'gce-google-compute-engine-oslogin-el9' },
      ],
    },
    buildpackagejob {
      package: 'osconfig',
      repos: ['deb10', 'deb11-arm64', 'el7', 'el8', 'el9', 'goo'],
      uploads: [
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent_((.:package-version))-g1+deb9_amd64.deb"}',
          repo: 'google-osconfig-agent-stretch',
          universe: 'cloud-apt',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent_((.:package-version))-g1+deb10_amd64.deb"}',
          repo: 'google-osconfig-agent-buster',
          universe: 'cloud-apt',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent_((.:package-version))-g1+deb11_amd64.deb"}, {"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent_((.:package-version))-g1+deb11_arm64.deb"}',
          repo: 'google-osconfig-agent-bullseye',
          universe: 'cloud-apt',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version))-g1.el7.x86_64.rpm"}',
          repo: 'google-osconfig-agent-el7',
          universe: 'cloud-yum',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version))-g1.el8.x86_64.rpm"}',
          repo: 'google-osconfig-agent-el8',
          universe: 'cloud-yum',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version))-g1.el9.x86_64.rpm"}',
          repo: 'google-osconfig-agent-el9',
          universe: 'cloud-yum',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent.x86_64.((.:package-version)).0+win@1.goo"}',
          repo: 'google-osconfig-agent',
          universe: 'cloud-yuck',
        },
      ],
    },
    promotepackagejob {
      package: 'osconfig',
      dest: 'staging',
      promotions: [
        promotepackagestagingtask { universe: 'cloud-apt', repo: 'google-osconfig-agent-stretch' },
        promotepackagestagingtask { universe: 'cloud-apt', repo: 'google-osconfig-agent-buster' },
        promotepackagestagingtask { universe: 'cloud-apt', repo: 'google-osconfig-agent-bullseye' },
        promotepackagestagingtask { universe: 'cloud-yum', repo: 'google-osconfig-agent-el7' },
        promotepackagestagingtask { universe: 'cloud-yum', repo: 'google-osconfig-agent-el8' },
        promotepackagestagingtask { universe: 'cloud-yum', repo: 'google-osconfig-agent-el9' },
        promotepackagestagingtask { universe: 'cloud-yuck', repo: 'google-osconfig-agent' },
      ],
    },
    promotepackagejob {
      package: 'osconfig',
      dest: 'stable',
      promotions: [
        promotepackagestabletask { universe: 'cloud-apt', repo: 'google-osconfig-agent-stretch' },
        promotepackagestabletask { universe: 'cloud-apt', repo: 'google-osconfig-agent-buster' },
        promotepackagestabletask { universe: 'cloud-apt', repo: 'google-osconfig-agent-bullseye' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'google-osconfig-agent-el7' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'google-osconfig-agent-el8' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'google-osconfig-agent-el9' },
        promotepackagestabletask { universe: 'cloud-yuck', repo: 'google-osconfig-agent' },
      ],
    },
    buildpackagejob {
      package: 'guest-diskexpand',
      repos: ['deb9', 'el7', 'el8', 'el9'],
      gcs_dir: 'gce-disk-expand',
      uploads: [
        packageuploadtask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"gce-disk-expand/gce-disk-expand_((.:package-version))-g1_all.deb"}',
          repo: 'gce-disk-expand',
          universe: 'cloud-apt',
        },
        packageuploadtask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"gce-disk-expand/gce-disk-expand-((.:package-version))-g1.el7.x86_64.rpm"}',
          universe: 'cloud-yum',
          repo: 'gce-disk-expand-el7',
        },
        packageuploadtask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"gce-disk-expand/gce-disk-expand-((.:package-version))-g1.el8.x86_64.rpm"}',
          universe: 'cloud-yum',
          repo: 'gce-disk-expand-el8',
        },
        packageuploadtask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"gce-disk-expand/gce-disk-expand-((.:package-version))-g1.el9.x86_64.rpm"}',
          universe: 'cloud-yum',
          repo: 'gce-disk-expand-el9',
        },

      ],
    },
    promotepackagejob {
      package: 'guest-diskexpand',
      dest: 'stable',
      promotions: [
        promotepackagestabletask { universe: 'cloud-apt', repo: 'gce-disk-expand' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'gce-disk-expand-el7' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'gce-disk-expand-el8' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'gce-disk-expand-el9' },
      ],
    },
    buildpackagejob {
      package: 'guest-configs',
      repos: ['deb9', 'el7', 'el8', 'el9'],
      gcs_dir: 'google-compute-engine',
      uploads: [
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb"}',
          universe: 'cloud-apt',
          repo: 'gce-google-compute-engine-stretch',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb"}',
          universe: 'cloud-apt',
          repo: 'gce-google-compute-engine-buster',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb"}',
          universe: 'cloud-apt',
          repo: 'gce-google-compute-engine-bullseye',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine-((.:package-version))-g1.el7.noarch.rpm"}',
          universe: 'cloud-yum',
          repo: 'gce-google-compute-engine-el7',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine-((.:package-version))-g1.el8.noarch.rpm"}',
          universe: 'cloud-yum',
          repo: 'gce-google-compute-engine-el8',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine-((.:package-version))-g1.el9.noarch.rpm"}',
          universe: 'cloud-yum',
          repo: 'gce-google-compute-engine-el9',
        },
      ],
    },
    promotepackagejob {
      package: 'guest-configs',
      dest: 'stable',
      promotions: [
        promotepackagestabletask { universe: 'cloud-apt', repo: 'gce-google-compute-engine-stretch' },
        promotepackagestabletask { universe: 'cloud-apt', repo: 'gce-google-compute-engine-buster' },
        promotepackagestabletask { universe: 'cloud-apt', repo: 'gce-google-compute-engine-bullseye' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'gce-google-compute-engine-el7' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'gce-google-compute-engine-el8' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'gce-google-compute-engine-el9' },
      ],
    },
    buildpackagejob {
      package: 'artifact-registry-yum-plugin',
      repos: ['el7', 'el8', 'el9'],
      uploads: [
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"yum-plugin-artifact-registry/yum-plugin-artifact-registry-((.:package-version))-g1.el7.noarch.rpm"}',
          universe: 'cloud-yum',
          repo: 'yum-plugin-artifact-registry',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version))-g1.el8.noarch.rpm"}',
          universe: 'cloud-yum',
          repo: 'dnf-plugin-artifact-registry',
        },
        packageuploadtask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version))-g1.el9.noarch.rpm"}',
          universe: 'cloud-yum',
          repo: 'dnf-plugin-artifact-registry',
        },
      ],
    },
    promotepackagejob {
      package: 'artifact-registry-yum-plugin',
      dest: 'stable',
      promotions: [
        promotepackagestabletask { universe: 'cloud-yum', repo: 'yum-plugin-artifact-registry' },
        promotepackagestabletask { universe: 'cloud-yum', repo: 'dnf-plugin-artifact-registry' },
      ],
    },
    buildpackagejob {
      package: 'artifact-registry-apt-transport',
      repos: ['deb9', 'deb11-arm64'],
      uploads: [
        packageuploadtask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"apt-transport-artifact-registry/apt-transport-artifact-registry_((.:package-version))-g1_amd64.deb"},{"bucket":"gcp-guest-package-uploads","object":"apt-transport-artifact-registry/apt-transport-artifact-registry_((.:package-version))-g1_arm64.deb"}',
          universe: 'cloud-apt',
          repo: 'apt-transport-artifact-registry',
        },
      ],
    },
    promotepackagejob {
      package: 'artifact-registry-apt-transport',
      dest: 'stable',
      promotions: [
        promotepackagestabletask { universe: 'cloud-apt', repo: 'apt-transport-artifact-registry' },
      ],
    },
    buildpackagejob {
      package: 'compute-image-windows',
      repos: ['goo'],
      uploads: [
        packageuploadtask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"compute-image-windows/certgen.x86_64.((.:package-version)).0@1.goo"}',
          universe: 'cloud-yuck',
          repo: 'certgen',
        },
        packageuploadtask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"compute-image-windows/google-compute-engine-auto-updater.((.:package-version)).0@1.goo"}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-auto-updater',
        },
        packageuploadtask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"compute-image-windows/google-compute-engine-powershell.((.:package-version)).0@1.goo"}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-powershell',
        },
        packageuploadtask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"compute-image-windows/google-compute-engine-sysprep.((.:package-version)).0@1.goo"}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-sysprep',
        },
      ],
    },
    promotepackagejob {
      package: 'compute-image-windows',
      name: 'promote-certgen-stable',
      dest: 'stable',
      tag: false,
      promotions: [
        promotepackagestabletask { universe: 'cloud-yuck', repo: 'certgen' },
      ],
    },
    promotepackagejob {
      package: 'compute-image-windows',
      name: 'promote-auto-updater-stable',
      dest: 'stable',
      promotions: [
        promotepackagestabletask { universe: 'cloud-yuck', repo: 'google-compute-engine-auto-updater' },
      ],
    },
    promotepackagejob {
      package: 'compute-image-windows',
      name: 'promote-powershell-stable',
      dest: 'stable',
      promotions: [
        promotepackagestabletask { universe: 'cloud-yuck', repo: 'google-compute-engine-powershell' },
      ],
    },
    promotepackagejob {
      package: 'compute-image-windows',
      name: 'promote-sysprep-stable',
      dest: 'stable',
      promotions: [
        promotepackagestabletask { universe: 'cloud-yuck', repo: 'google-compute-engine-sysprep' },
      ],
    },
  ],
  resources: [
    {
      name: 'guest-agent',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/guest-agent.git',
        branch: 'main',
        fetch_tags: true,
      },
    },
    {
      name: 'guest-agent-tag',
      type: 'github-release',
      source: {
        owner: 'GoogleCloudPlatform',
        repository: 'guest-agent',
        access_token: '((github-token.token))',
      },
    },
    {
      name: 'guest-oslogin',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/guest-oslogin.git',
        branch: 'master',
        fetch_tags: true,
      },
    },
    {
      name: 'guest-oslogin-tag',
      type: 'github-release',
      source: {
        owner: 'GoogleCloudPlatform',
        repository: 'guest-oslogin',
        access_token: '((github-token.token))',
      },
    },
    {
      name: 'osconfig',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/osconfig.git',
        branch: 'master',
        fetch_tags: true,
      },
    },
    {
      name: 'osconfig-tag',
      type: 'github-release',
      source: {
        owner: 'GoogleCloudPlatform',
        repository: 'osconfig',
        access_token: '((github-token.token))',
      },
    },
    {
      name: 'guest-diskexpand',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/guest-diskexpand.git',
        branch: 'master',
        fetch_tags: true,
      },
    },
    {
      name: 'guest-diskexpand-tag',
      type: 'github-release',
      source: {
        owner: 'GoogleCloudPlatform',
        repository: 'guest-diskexpand',
        access_token: '((github-token.token))',
      },
    },
    {
      name: 'guest-configs',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/guest-configs.git',
        branch: 'master',
        fetch_tags: true,
      },
    },
    {
      name: 'guest-configs-tag',
      type: 'github-release',
      source: {
        owner: 'GoogleCloudPlatform',
        repository: 'guest-configs',
        access_token: '((github-token.token))',
      },
    },
    {
      name: 'artifact-registry-yum-plugin',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/artifact-registry-yum-plugin.git',
        branch: 'main',
        fetch_tags: true,
      },
    },
    {
      name: 'artifact-registry-yum-plugin-tag',
      type: 'github-release',
      source: {
        owner: 'GoogleCloudPlatform',
        repository: 'artifact-registry-yum-plugin',
        access_token: '((github-token.token))',
      },
    },
    {
      name: 'artifact-registry-apt-transport',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/artifact-registry-apt-transport.git',
        branch: 'main',
        fetch_tags: true,
      },
    },
    {
      name: 'artifact-registry-apt-transport-tag',
      type: 'github-release',
      source: {
        owner: 'GoogleCloudPlatform',
        repository: 'artifact-registry-apt-transport',
        access_token: '((github-token.token))',
      },
    },
    {
      name: 'guest-test-infra',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/guest-test-infra.git',
        branch: 'master',
        fetch_tags: true,
      },
    },
    {
      name: 'compute-image-windows',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/compute-image-windows.git',
        branch: 'master',
      },
    },
    {
      name: 'compute-image-windows-tag',
      type: 'github-release',
      source: {
        owner: 'GoogleCloudPlatform',
        repository: 'compute-image-windows',
        access_token: '((github-token.token))',
      },
    },
  ],
  groups: [
    {
      name: 'guest-agent',
      jobs: [
        'build-guest-agent',
        'inject-guest-agent-linux-staging',
        'promote-guest-agent-linux-stable',
        'inject-guest-agent-windows-staging',
        'promote-guest-agent-windows-stable',
        'inject-metadata-scripts-windows-staging',
        'promote-metadata-scripts-windows-stable',
      ],
    },
    {
      name: 'guest-oslogin',
      jobs: [
        'build-guest-oslogin',
        'promote-guest-oslogin-staging',
        'promote-guest-oslogin-stable',
      ],
    },
    {
      name: 'osconfig',
      jobs: [
        'build-osconfig',
      ],
    },
    {
      name: 'disk-expand',
      jobs: [
        'build-guest-diskexpand',
      ],
    },
    {
      name: 'google-compute-engine',
      jobs: [
        'build-guest-configs',
        'promote-guest-configs-staging',
        'promote-guest-configs-stable',
      ],
    },
    {
      name: 'artifact-registry-plugins',
      jobs: [
        'build-artifact-registry-el-plugins',
        'build-artifact-registry-apt-transport',
        'promote-artifact-registry-el-plugins-staging',
        'promote-artifact-registry-apt-transport-staging',
        'promote-artifact-registry-el-plugins-stable',
        'promote-artifact-registry-apt-transport-stable',
      ],
    },
    {
      name: 'compute-image-windows',
      jobs: [
        'build-compute-image-windows',
        'inject-windows-certgen-staging',
        'inject-windows-auto-updater-staging',
        'inject-windows-powershell-staging',
        'inject-windows-sysprep-staging',
        'promote-windows-certgen-stable',
        'promote-windows-auto-updater-stable',
        'promote-windows-powershell-stable',
        'promote-windows-sysprep-stable',
      ],
    },
  ],
}
