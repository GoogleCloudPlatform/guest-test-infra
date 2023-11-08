local underscore(input) = std.strReplace(input, '-', '_');

// task which publishes a 'result' metric per job, with either success or failure value.
local publishresulttask = {
  local tl = self,

  result:: error 'must set result in publishresulttask',
  package:: error 'must set package in publishresulttask',

  // start of output.
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
        '--pipeline=guest-package-build',
        '--job=build-' + tl.package,
        '--task=publish-job-result',
        '--result-state=' + tl.result,
        '--start-timestamp=((.:start-timestamp-ms))',
        '--metric-path=concourse/job/duration',
      ],
    },
  },
};

// task which generates timestamps used in metric publishing steps. common between build and promote jobs.
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

// job which builds a package - environments to build and individual upload tasks are passed in
local base_buildpackagejob = {
  local tl = self,

  package:: error 'must set package in buildpackagejob',
  repo_name:: tl.package,
  gcs_dir:: tl.package,
  builds:: error 'must set builds in buildpackagejob',
  uploads:: error 'must set uploads in buildpackagejob',
  extra_tasks:: [],
  build_dir:: '',

  // Start of output.
  name: 'build-' + tl.package,
  plan: [
    // Prep build variables and content.
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
    generatetimestamptask,
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    // Prep package version by reading tags in the git repo. New versions are YYYYMMDD.NN, where .NN
    // increments within a given day.
    {
      task: 'generate-package-version',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: { repository: 'alpine/git' },
        },

        inputs: [{ name: tl.package, path: 'repo' }],
        outputs: [{ name: 'package-version' }],

        run: {
          path: 'ash',
          args: [
            '-exc',
            // Ugly way to produce multi-line script. TODO: maybe move to scripts?
            std.lines([
              'latest=$(cd repo;git tag -l "20*"|tail -1)',
              'latest_date=${latest/.*}',
              'todays_date=$(date "+%Y%m%d")',
              'latest_build=0',
              'if [[ $latest_date == $todays_date ]]; then',
              '  latest_build=${latest/*.}',
              '  latest_build=$((latest_build+1))',
              'fi',
              'printf "%s.%02d\n" "${todays_date}" "${latest_build}" | tee package-version/version',
            ]),
          ],
        },
      },
    },
    { load_var: 'package-version', file: 'package-version/version' },
    // Invoke daisy build workflows for all specified builds.
    {
      in_parallel: {
        steps: [
          {
            task: 'guest-package-build-%s-%s' % [tl.package, build],
            config: {
              platform: 'linux',
              image_resource: {
                type: 'registry-image',
                source: { repository: 'gcr.io/compute-image-tools/daisy' },
              },
              inputs: [{ name: 'guest-test-infra' }],
              run: {
                path: '/daisy',
                args: [
                  '-project=gcp-guest',
                  '-zone=us-west1-a',
                  '-var:repo_owner=GoogleCloudPlatform',
                  '-var:repo_name=' + tl.repo_name,
                  '-var:git_ref=((.:commit-sha))',
                  '-var:version=((.:package-version))',
                  '-var:gcs_path=gs://gcp-guest-package-uploads/' + tl.gcs_dir,
                  '-var:sbom_util_gcs_root=gs://gce-image-sbom-util',
                  '-var:build_dir=' + tl.build_dir,
                  'guest-test-infra/packagebuild/workflows/build_%s.wf.json' % underscore(build),
                ],
              },
            },
          }
          for build in tl.builds
        ],
      },
    },
    // Layer in any provided additional tasks after build but before upload.
  ] + tl.extra_tasks + tl.extended_tasks
};

local buildpackagejob = base_buildpackagejob {
  local tl = self,

  extended_tasks: [
    // Run provided upload tasks.
    {
      in_parallel: {
        fail_fast: true,
        steps: tl.uploads,
      },
    },
    // Put the version tag onto the repo after uploads are complete.
    {
      put: '%s-tag' % tl.repo_name,
      params: {
        name: 'package-version/version',
        tag: 'package-version/version',
        commitish: '%s/.git/ref' % tl.repo_name,
      },
    },
  ],

  // Publish success/failure metrics.
  on_success: publishresulttask {
    result: 'success',
    package: tl.package,
  },
  on_failure: publishresulttask {
    result: 'failure',
    package: tl.package,
  },
};

// job which promotes a package - individual promotion tasks are passed in.
local promotepackagejob = {
  local tl = self,

  package:: error 'must set package in promotepackagejob',
  promotions:: error 'must set promotions in promotepackagejob',
  dest:: error 'must set dest in promotepackagejob',
  passed:: 'build-' + tl.package,

  // Start of output.
  name: 'promote-%s-%s' % [tl.package, tl.dest],
  plan: [
    // Prep variables and content.
    {
      get: '%s-tag' % tl.package,
      passed: [tl.passed],
    },
    {
      get: tl.package,
      params: { fetch_tags: true },
    },
    { get: 'guest-test-infra' },
    generatetimestamptask,
    { load_var: 'start-timestamp-ms', file: 'timestamp/timestamp-ms' },
    // Run provided promotion tasks.
    { in_parallel: tl.promotions },
  ],
  // Publish success/failure metrics.
  on_success: publishresulttask {
    result: 'success',
    package: tl.package,
  },
  on_failure: publishresulttask {
    result: 'failure',
    package: tl.package,
  },
};

// task which uploads a package using the 'uploadToStaging' or 'uploadToUnstable' ARLE RPCs
local uploadpackagetask = {
  local tl = self,

  package_paths:: error 'must set package_paths in uploadpackagetask',
  sbom_file:: error 'must set sbom_file in uploadpackagetask',
  repo:: error 'must set rapture_repo in uploadpackagetask',
  topic:: 'projects/artifact-releaser-prod/topics/gcp-guest-package-upload-prod',
  type:: 'uploadToStaging',
  universe:: error 'must set universe in uploadpackagetask',

  task: 'upload-' + tl.repo,
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
        '{"type": "%s", "request": {"gcsfiles": [%s], "sbomfile": %s, "universe": "%s", "repo": "%s"}}' %
        [tl.type, tl.package_paths, tl.sbom_file, tl.universe, tl.repo],
      ],
    },
  },
};

// task which promotes a package using the 'promoteToStaging' ARLE RPC
local promotepackagestagingtask = {
  local tl = self,

  repo:: error 'must set repo in promotepackagestagingtask',
  universe:: error 'must set universe in promotepackagestagingtask',
  topic:: 'projects/artifact-releaser-prod/topics/gcp-guest-package-upload-prod',

  // Start of output.
  task: 'promote-staging-' + tl.repo,
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
        '{"type": "promoteToStaging", "request": {"gcsfiles": [], "universe": "%s", "repo": "%s"}}' %
        [tl.universe, tl.repo],
      ],
    },
  },
};

// task which builds a derivative OS image with a specific package added, for use in tests
local buildpackageimagetask = {
  local tl = self,

  image_name:: error 'must set image_name in buildpackageimagetask',
  source_image:: error 'must set source_image in buildpackageimagetask',
  dest_image:: error 'must set dest_image in buildpackageimagetask',
  gcs_package_path:: error 'must set gcs_package_path in buildpackageimagetask',
  machine_type:: 'e2-medium',
  worker_image:: 'projects/compute-image-tools/global/images/family/debian-11-worker',

  // Start of output.
  task: 'build-derivative-%s-image' % tl.image_name,
  config: {
    platform: 'linux',
    image_resource: {
      type: 'registry-image',
      source: { repository: 'gcr.io/compute-image-tools/daisy' },
    },
    inputs: [{ name: 'compute-image-tools' }],
    run: {
      path: '/daisy',
      args: [
        '-project=gcp-guest',
        '-zone=us-central1-a',
        '-var:source_image=' + tl.source_image,
        '-var:gcs_package_path=' + tl.gcs_package_path,
        '-var:dest_image=' + tl.dest_image,
        '-var:machine_type=' + tl.machine_type,
        '-var:worker_image=' + tl.worker_image,
        './compute-image-tools/daisy_workflows/image_build/install_package/install_package.wf.json',
      ],
    },
  },
};

local build_guest_agent = buildpackagejob {
      local tl = self,

      uploads: [],
      builds: ['deb10', 'deb11-arm64', 'el7', 'el8', 'el8-arm64', 'el9', 'el9-arm64', 'goo'],
      // The guest agent has additional testing steps to build derivative images then run CIT against them.
      extra_tasks: [
        {
          task: 'generate-build-id',
          config: {
            platform: 'linux',
            image_resource: {
              type: 'registry-image',
              source: { repository: 'busybox' },
            },
            outputs: [{ name: 'build-id-dir' }],
            run: {
              path: 'sh',
              args: [
                '-exc',
                'buildid=$(date "+%s"); echo $buildid | tee build-id-dir/build-id',
              ],
            },
          },
        },
        { load_var: 'build-id', file: 'build-id-dir/build-id' },
        { get: 'compute-image-tools' },
        {
          in_parallel: {
            steps: [
              buildpackageimagetask {
                image_name: 'debian-10',
                source_image: 'projects/debian-cloud/global/images/family/debian-10',
                dest_image: 'debian-10-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_amd64.deb' % [tl.package],
              },
              buildpackageimagetask {
                image_name: 'debian-11',
                source_image: 'projects/debian-cloud/global/images/family/debian-11',
                dest_image: 'debian-11-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_amd64.deb' % [tl.package],
              },
              buildpackageimagetask {
                image_name: 'debian-11-arm64',
                source_image: 'projects/debian-cloud/global/images/family/debian-11-arm64',
                dest_image: 'debian-11-arm64-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_arm64.deb' % [tl.package],
                machine_type: 't2a-standard-2',
                worker_image: 'projects/compute-image-tools/global/images/family/debian-11-worker-arm64',
              },
              buildpackageimagetask {
                image_name: 'debian-12',
                source_image: 'projects/bct-prod-images/global/images/family/debian-12',
                dest_image: 'debian-12-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_amd64.deb' % [tl.package],
              },
              buildpackageimagetask {
                image_name: 'debian-12-arm64',
                source_image: 'projects/bct-prod-images/global/images/family/debian-12-arm64',
                dest_image: 'debian-12-arm64-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_arm64.deb' % [tl.package],
                machine_type: 't2a-standard-2',
                worker_image: 'projects/compute-image-tools/global/images/family/debian-11-worker-arm64',
              },
              buildpackageimagetask {
                image_name: 'centos-7',
                source_image: 'projects/centos-cloud/global/images/family/centos-7',
                dest_image: 'centos-7-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el7.x86_64.rpm' % [tl.package],
              },
              buildpackageimagetask {
                image_name: 'rhel-7',
                source_image: 'projects/rhel-cloud/global/images/family/rhel-7',
                dest_image: 'rhel-7-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el7.x86_64.rpm' % [tl.package],
              },
              buildpackageimagetask {
                image_name: 'rhel-8',
                source_image: 'projects/rhel-cloud/global/images/family/rhel-8',
                dest_image: 'rhel-8-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el8.x86_64.rpm' % [tl.package],
              },
              buildpackageimagetask {
                image_name: 'rocky-linux-8-optimized-gcp-arm64',
                source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-8-optimized-gcp-arm64',
                dest_image: 'rocky-linux-8-optimized-gcp-arm64-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el8.aarch64.rpm' % [tl.package],
                machine_type: 't2a-standard-2',
                worker_image: 'projects/compute-image-tools/global/images/family/debian-11-worker-arm64',
              },
              buildpackageimagetask {
                image_name: 'rhel-9',
                source_image: 'projects/rhel-cloud/global/images/family/rhel-9',
                dest_image: 'rhel-9-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.x86_64.rpm' % [tl.package],
              },
              buildpackageimagetask {
                image_name: 'rhel-9-arm64',
                source_image: 'projects/rhel-cloud/global/images/family/rhel-9-arm64',
                dest_image: 'rhel-9-arm64-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.aarch64.rpm' % [tl.package],
                machine_type: 't2a-standard-2',
                worker_image: 'projects/compute-image-tools/global/images/family/debian-11-worker-arm64',
              },
            ],
          },
        },
        {
          in_parallel: {
            fail_fast: true,
            steps: [
              {
                task: '%s-image-tests-amd64' % [tl.package],
                config: {
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
                      '-images=projects/gcp-guest/global/images/debian-10-((.:build-id)),projects/gcp-guest/global/images/debian-11-((.:build-id)),projects/gcp-guest/global/images/debian-12-((.:build-id)),projects/gcp-guest/global/images/centos-7-((.:build-id)),projects/gcp-guest/global/images/rhel-7-((.:build-id)),projects/gcp-guest/global/images/rhel-8-((.:build-id)),projects/gcp-guest/global/images/rhel-9-((.:build-id))',
                      '-exclude=(image)|(disk)|(security)|(oslogin)|(storageperf)|(networkperf)|(shapevalidation)|(hotattach)',
                    ],
                  },
                },
              },
              {
                task: '%s-image-tests-arm64' % [tl.package],
                config: {
                  platform: 'linux',
                  image_resource: {
                    type: 'registry-image',
                    source: { repository: 'gcr.io/compute-image-tools/cloud-image-tests' },
                  },
                  inputs: [{ name: 'guest-test-infra' }],
                  run: {
                    path: '/manager',
                    args: [
                      '-project=gcp-guest',
                      '-zone=us-central1-a',
                      '-test_projects=compute-image-test-pool-002,compute-image-test-pool-003,compute-image-test-pool-004,compute-image-test-pool-005',
                      '-images=projects/gcp-guest/global/images/debian-11-arm64-((.:build-id)),projects/gcp-guest/global/images/debian-12-arm64-((.:build-id)),projects/gcp-guest/global/images/rocky-linux-8-optimized-gcp-arm64-((.:build-id)),projects/gcp-guest/global/images/rhel-9-arm64-((.:build-id))',
                      '-machine_type=t2a-standard-2',
                      '-exclude=(image)|(disk)|(security)|(oslogin)|(storageperf)|(networkperf)|(shapevalidation)|(hotattach)',
                    ],
                  },
                },
              },
            ],
          },
        },
      ],
};

local build_and_upload_guest_agent = build_guest_agent {
      uploads: [
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent_((.:package-version))-g1_amd64.deb"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version)).sbom.json"}',
          repo: 'google-guest-agent-buster',
          universe: 'cloud-apt',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent_((.:package-version))-g1_amd64.deb"},{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent_((.:package-version))-g1_arm64.deb"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version)).sbom.json"}',
          repo: 'google-guest-agent-bullseye',
          universe: 'cloud-apt',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent_((.:package-version))-g1_amd64.deb"},{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent_((.:package-version))-g1_arm64.deb"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version)).sbom.json"}',
          repo: 'google-guest-agent-bookworm',
          universe: 'cloud-apt',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version))-g1.el7.x86_64.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version)).sbom.json"}',
          repo: 'google-guest-agent-el7',
          universe: 'cloud-yum',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version))-g1.el8.x86_64.rpm"},{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version))-g1.el8.aarch64.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version)).sbom.json"}',
          repo: 'google-guest-agent-el8',
          universe: 'cloud-yum',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version))-g1.el9.x86_64.rpm"},{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version))-g1.el9.aarch64.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-guest-agent-((.:package-version)).sbom.json"}',
          repo: 'google-guest-agent-el9',
          universe: 'cloud-yum',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-compute-engine-windows.x86_64.((.:package-version)).0@1.goo"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":""}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-windows',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-compute-engine-metadata-scripts.x86_64.((.:package-version)).0@1.goo"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":""}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-metadata-scripts',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"guest-agent/google-compute-engine-metadata-scripts.x86_64.((.:package-version)).0@1.goo"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":""}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-metadata-scripts',
        },
      ],
};

// Start of output
{
  jobs: [
    build_and_upload_guest_agent{
	package: 'guest-agent',
    },
    build_guest_agent{
	package: 'guest-agent-dev',
	repo_name: 'guest-agent',
    },
    buildpackagejob {
      package: 'guest-oslogin',
      builds: ['deb10', 'deb11', 'deb11-arm64', 'deb12', 'deb12-arm64', 'el7', 'el8', 'el8-arm64', 'el9', 'el9-arm64'],
      gcs_dir: 'oslogin',
      extra_tasks: [
        {
          task: 'generate-build-id',
          config: {
            platform: 'linux',
            image_resource: {
              type: 'registry-image',
              source: { repository: 'busybox' },
            },
            outputs: [{ name: 'build-id-dir' }],
            run: {
              path: 'sh',
              args: [
                '-exc',
                'buildid=$(date "+%s"); echo $buildid | tee build-id-dir/build-id',
              ],
            },
          },
        },
        { load_var: 'build-id', file: 'build-id-dir/build-id' },
        { get: 'compute-image-tools' },
        {
          in_parallel: {
            fail_fast: true,
            steps: [
              buildpackageimagetask {
                image_name: 'debian-10',
                source_image: 'projects/debian-cloud/global/images/family/debian-10',
                dest_image: 'debian-10-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb10_amd64.deb',
              },
              buildpackageimagetask {
                image_name: 'debian-11',
                source_image: 'projects/debian-cloud/global/images/family/debian-11',
                dest_image: 'debian-11-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb11_amd64.deb',
              },
              buildpackageimagetask {
                image_name: 'debian-11-arm64',
                source_image: 'projects/debian-cloud/global/images/family/debian-11-arm64',
                dest_image: 'debian-11-arm64-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb11_arm64.deb',
                machine_type: 't2a-standard-2',
                worker_image: 'projects/compute-image-tools/global/images/family/debian-11-worker-arm64',
              },
              buildpackageimagetask {
                image_name: 'debian-12',
                source_image: 'projects/bct-prod-images/global/images/family/debian-12',
                dest_image: 'debian-12-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb12_amd64.deb',
              },
              buildpackageimagetask {
                image_name: 'debian-12-arm64',
                source_image: 'projects/bct-prod-images/global/images/family/debian-12-arm64',
                dest_image: 'debian-12-arm64-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb12_arm64.deb',
                machine_type: 't2a-standard-2',
                worker_image: 'projects/compute-image-tools/global/images/family/debian-11-worker-arm64',
              },
              buildpackageimagetask {
                image_name: 'centos-7',
                source_image: 'projects/centos-cloud/global/images/family/centos-7',
                dest_image: 'centos-7-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el7.x86_64.rpm',
              },
              buildpackageimagetask {
                image_name: 'rhel-7',
                source_image: 'projects/rhel-cloud/global/images/family/rhel-7',
                dest_image: 'rhel-7-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el7.x86_64.rpm',
              },
              buildpackageimagetask {
                image_name: 'rhel-8',
                source_image: 'projects/rhel-cloud/global/images/family/rhel-8',
                dest_image: 'rhel-8-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el8.x86_64.rpm',
              },
              buildpackageimagetask {
                image_name: 'rocky-linux-8-optimized-gcp-arm64',
                source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-8-optimized-gcp-arm64',
                dest_image: 'rocky-linux-8-optimized-gcp-arm64-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el8.aarch64.rpm',
                machine_type: 't2a-standard-2',
                worker_image: 'projects/compute-image-tools/global/images/family/debian-11-worker-arm64',
              },
              buildpackageimagetask {
                image_name: 'rhel-9',
                source_image: 'projects/rhel-cloud/global/images/family/rhel-9',
                dest_image: 'rhel-9-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el9.x86_64.rpm',
              },
              buildpackageimagetask {
                image_name: 'rhel-9-arm64',
                source_image: 'projects/rhel-cloud/global/images/family/rhel-9-arm64',
                dest_image: 'rhel-9-arm64-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el9.aarch64.rpm',
                machine_type: 't2a-standard-2',
                worker_image: 'projects/compute-image-tools/global/images/family/debian-11-worker-arm64',
              },
            ],
          },
        },
        {
          in_parallel: {
            fail_fast: true,
            steps: [
              {
                task: 'oslogin-image-tests-amd64',
                config: {
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
                      '-test_projects=oslogin-testing-project',
                      '-images=projects/gcp-guest/global/images/debian-10-((.:build-id)),projects/gcp-guest/global/images/debian-11-((.:build-id)),projects/gcp-guest/global/images/debian-12-((.:build-id)),projects/gcp-guest/global/images/centos-7-((.:build-id)),projects/gcp-guest/global/images/rhel-7-((.:build-id)),projects/gcp-guest/global/images/rhel-8-((.:build-id)),projects/gcp-guest/global/images/rhel-9-((.:build-id))',
                      '-filter=oslogin',
                    ],
                  },
                },
              },
              {
                task: 'oslogin-image-tests-arm64',
                config: {
                  platform: 'linux',
                  image_resource: {
                    type: 'registry-image',
                    source: { repository: 'gcr.io/compute-image-tools/cloud-image-tests' },
                  },
                  inputs: [{ name: 'guest-test-infra' }],
                  run: {
                    path: '/manager',
                    args: [
                      '-project=gcp-guest',
                      '-zone=us-central1-a',
                      '-test_projects=oslogin-testing-project',
                      '-images=projects/gcp-guest/global/images/debian-11-arm64-((.:build-id)),projects/gcp-guest/global/images/debian-12-arm64-((.:build-id)),projects/gcp-guest/global/images/rocky-linux-8-optimized-gcp-arm64-((.:build-id)),projects/gcp-guest/global/images/rhel-9-arm64-((.:build-id))',
                      '-machine_type=t2a-standard-2',
                      '-filter=oslogin',
                    ],
                  },
                },
              },
            ],
          },
        },
      ],
      uploads: [
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb10_amd64.deb"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version)).sbom.json"}',
          repo: 'gce-google-compute-engine-oslogin-buster',
          universe: 'cloud-apt',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb11_amd64.deb"},{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb11_arm64.deb"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version)).sbom.json"}',
          repo: 'gce-google-compute-engine-oslogin-bullseye',
          universe: 'cloud-apt',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb12_amd64.deb"},{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb12_arm64.deb"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version)).sbom.json"}',
          repo: 'gce-google-compute-engine-oslogin-bookworm',
          universe: 'cloud-apt',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el7.x86_64.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version)).sbom.json"}',
          repo: 'gce-google-compute-engine-oslogin-el7',
          universe: 'cloud-yum',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el8.x86_64.rpm"},{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el8.aarch64.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version)).sbom.json"}',
          repo: 'gce-google-compute-engine-oslogin-el8',
          universe: 'cloud-yum',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el9.x86_64.rpm"},{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el9.aarch64.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"oslogin/google-compute-engine-oslogin-((.:package-version)).sbom.json"}',
          repo: 'gce-google-compute-engine-oslogin-el9',
          universe: 'cloud-yum',
        },
      ],
    },
    buildpackagejob {
      package: 'osconfig',
      builds: ['deb10', 'deb11-arm64', 'el7', 'el8', 'el8-arm64', 'el9', 'el9-arm64', 'goo'],
      uploads: [
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent_((.:package-version))-g1_amd64.deb"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version)).sbom.json"}',
          repo: 'google-osconfig-agent-buster',
          universe: 'cloud-apt',
          type: 'uploadToUnstable',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent_((.:package-version))-g1_amd64.deb"},{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent_((.:package-version))-g1_arm64.deb"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version)).sbom.json"}',
          repo: 'google-osconfig-agent-bullseye',
          universe: 'cloud-apt',
          type: 'uploadToUnstable',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent_((.:package-version))-g1_amd64.deb"},{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent_((.:package-version))-g1_arm64.deb"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version)).sbom.json"}',
          repo: 'google-osconfig-agent-bookworm',
          universe: 'cloud-apt',
          type: 'uploadToUnstable',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version))-g1.el7.x86_64.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version)).sbom.json"}',
          repo: 'google-osconfig-agent-el7',
          universe: 'cloud-yum',
          type: 'uploadToUnstable',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version))-g1.el8.x86_64.rpm"},{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version))-g1.el8.aarch64.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version)).sbom.json"}',
          repo: 'google-osconfig-agent-el8',
          universe: 'cloud-yum',
          type: 'uploadToUnstable',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version))-g1.el9.x86_64.rpm"},{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version))-g1.el9.aarch64.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent-((.:package-version)).sbom.json"}',
          repo: 'google-osconfig-agent-el9',
          universe: 'cloud-yum',
          type: 'uploadToUnstable',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"osconfig/google-osconfig-agent.x86_64.((.:package-version)).0+win@1.goo"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":""}',
          repo: 'google-osconfig-agent',
          universe: 'cloud-yuck',
          type: 'uploadToUnstable',
        },
      ],
    },
    promotepackagejob {
      package: 'osconfig',
      dest: 'staging',
      promotions: [
        promotepackagestagingtask { universe: 'cloud-apt', repo: 'google-osconfig-agent-buster' },
        promotepackagestagingtask { universe: 'cloud-apt', repo: 'google-osconfig-agent-bullseye' },
        promotepackagestagingtask { universe: 'cloud-apt', repo: 'google-osconfig-agent-bookworm' },
        promotepackagestagingtask { universe: 'cloud-yum', repo: 'google-osconfig-agent-el7' },
        promotepackagestagingtask { universe: 'cloud-yum', repo: 'google-osconfig-agent-el8' },
        promotepackagestagingtask { universe: 'cloud-yum', repo: 'google-osconfig-agent-el9' },
        promotepackagestagingtask { universe: 'cloud-yuck', repo: 'google-osconfig-agent' },
      ],
    },
    buildpackagejob {
      package: 'guest-diskexpand',
      builds: ['deb10', 'el7', 'el8', 'el9'],
      gcs_dir: 'gce-disk-expand',
      uploads: [
        uploadpackagetask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"gce-disk-expand/gce-disk-expand_((.:package-version))-g1_all.deb"}',
          sbom_file:
            '{"bucket":"gcp-guest-package-uploads","object":"gce-disk-expand/gce-disk-expand-((.:package-version)).sbom.json"}',
          repo: 'gce-disk-expand',
          universe: 'cloud-apt',
        },
        uploadpackagetask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"gce-disk-expand/gce-disk-expand-((.:package-version))-g1.el7.noarch.rpm"}',
          sbom_file:
            '{"bucket":"gcp-guest-package-uploads","object":"gce-disk-expand/gce-disk-expand-((.:package-version)).sbom.json"}',
          universe: 'cloud-yum',
          repo: 'gce-disk-expand-el7',
        },
        uploadpackagetask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"gce-disk-expand/gce-disk-expand-((.:package-version))-g1.el8.noarch.rpm"}',
          sbom_file:
            '{"bucket":"gcp-guest-package-uploads","object":"gce-disk-expand/gce-disk-expand-((.:package-version)).sbom.json"}',
          universe: 'cloud-yum',
          repo: 'gce-disk-expand-el8',
        },
        uploadpackagetask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"gce-disk-expand/gce-disk-expand-((.:package-version))-g1.el9.noarch.rpm"}',
          sbom_file:
            '{"bucket":"gcp-guest-package-uploads","object":"gce-disk-expand/gce-disk-expand-((.:package-version)).sbom.json"}',
          universe: 'cloud-yum',
          repo: 'gce-disk-expand-el9',
        },

      ],
    },
    buildpackagejob {
      package: 'guest-configs',
      builds: ['deb10', 'el7', 'el8', 'el9'],
      gcs_dir: 'google-compute-engine',
      uploads: [
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine-((.:package-version)).sbom.json"}',
          universe: 'cloud-apt',
          repo: 'gce-google-compute-engine-buster',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine-((.:package-version)).sbom.json"}',
          universe: 'cloud-apt',
          repo: 'gce-google-compute-engine-bullseye',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine-((.:package-version)).sbom.json"}',
          universe: 'cloud-apt',
          repo: 'gce-google-compute-engine-bookworm',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine-((.:package-version))-g1.el7.noarch.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine-((.:package-version)).sbom.json"}',
          universe: 'cloud-yum',
          repo: 'gce-google-compute-engine-el7',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine-((.:package-version))-g1.el8.noarch.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine-((.:package-version)).sbom.json"}',
          universe: 'cloud-yum',
          repo: 'gce-google-compute-engine-el8',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine-((.:package-version))-g1.el9.noarch.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"google-compute-engine/google-compute-engine-((.:package-version)).sbom.json"}',
          universe: 'cloud-yum',
          repo: 'gce-google-compute-engine-el9',
        },
      ],
    },
    buildpackagejob {
      package: 'artifact-registry-yum-plugin',
      builds: ['el7', 'el8', 'el8-arm64', 'el9', 'el9-arm64'],
      gcs_dir: 'yum-plugin-artifact-registry',
      uploads: [
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"yum-plugin-artifact-registry/yum-plugin-artifact-registry-((.:package-version))-g1.el7.x86_64.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"yum-plugin-artifact-registry/yum-plugin-artifact-registry-((.:package-version)).sbom.json"}',
          universe: 'cloud-yum',
          repo: 'yum-plugin-artifact-registry-el7',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version))-g1.el8.x86_64.rpm"},{"bucket":"gcp-guest-package-uploads","object":"yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version))-g1.el8.aarch64.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version)).sbom.json"}',
          universe: 'cloud-yum',
          repo: 'dnf-plugin-artifact-registry-el8',
        },
        uploadpackagetask {
          package_paths: '{"bucket":"gcp-guest-package-uploads","object":"yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version))-g1.el9.x86_64.rpm"},{"bucket":"gcp-guest-package-uploads","object":"yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version))-g1.el9.aarch64.rpm"}',
          sbom_file: '{"bucket":"gcp-guest-package-uploads","object":"yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version)).sbom.json"}',
          universe: 'cloud-yum',
          repo: 'dnf-plugin-artifact-registry-el9',
        },
      ],
    },
    buildpackagejob {
      package: 'artifact-registry-apt-transport',
      builds: ['deb10', 'deb11-arm64'],
      uploads: [
        uploadpackagetask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"apt-transport-artifact-registry/apt-transport-artifact-registry_((.:package-version))-g1_amd64.deb"},{"bucket":"gcp-guest-package-uploads","object":"apt-transport-artifact-registry/apt-transport-artifact-registry_((.:package-version))-g1_arm64.deb"}',
          sbom_file:
            '{"bucket":"gcp-guest-package-uploads","object":"apt-transport-artifact-registry/apt-transport-artifact-registry-((.:package-version)).sbom.json"}',
          universe: 'cloud-apt',
          repo: 'apt-transport-artifact-registry',
        },
      ],
    },
    buildpackagejob {
      package: 'compute-image-windows',
      builds: ['goo'],
      uploads: [
        uploadpackagetask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"compute-image-windows/certgen.x86_64.((.:package-version)).0@1.goo"}',
          sbom_file:
            '{"bucket":"gcp-guest-package-uploads","object":""}',
          universe: 'cloud-yuck',
          repo: 'certgen',
        },
        uploadpackagetask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"compute-image-windows/google-compute-engine-auto-updater.noarch.((.:package-version))@1.goo"}',
          sbom_file:
            '{"bucket":"gcp-guest-package-uploads","object":""}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-auto-updater',
        },
        uploadpackagetask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"compute-image-windows/google-compute-engine-powershell.noarch.((.:package-version))@1.goo"}',
          sbom_file:
            '{"bucket":"gcp-guest-package-uploads","object":""}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-powershell',
        },
        uploadpackagetask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"compute-image-windows/google-compute-engine-sysprep.noarch.((.:package-version))@1.goo"}',
          sbom_file:
            '{"bucket":"gcp-guest-package-uploads","object":""}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-sysprep',
        },
        uploadpackagetask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"compute-image-windows/google-compute-engine-ssh.x86_64.((.:package-version)).0@1.goo"}',
          sbom_file:
            '{"bucket":"gcp-guest-package-uploads","object":""}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-ssh',
        },
      ],
    },
    buildpackagejob {
      package: 'compute-image-tools',
      builds: ['goo'],
      name: 'build-diagnostics',
      uploads: [
        uploadpackagetask {
          package_paths:
            '{"bucket":"gcp-guest-package-uploads","object":"compute-image-tools/google-compute-engine-diagnostics.x86_64.((.:package-version)).0@0.goo"}',
          sbom_file:
            '{"bucket":"gcp-guest-package-uploads","object":""}',
          universe: 'cloud-yuck',
          repo: 'google-compute-engine-diagnostics',
        },
      ],
      build_dir: 'cli_tools/diagnostics',
    },
  ],
  resources: [
    {
      name: 'guest-agent-dev',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/guest-agent.git',
        branch: 'dev',
        fetch_tags: false,
      },
    },
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
      name: 'compute-image-tools',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/compute-image-tools.git',
        branch: 'master',
        fetch_tags: true,
      },
    },
    {
      name: 'compute-image-tools-tag',
      type: 'github-release',
      source: {
        owner: 'GoogleCloudPlatform',
        repository: 'compute-image-tools',
        access_token: '((github-token.token))',
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
      ],
    },
    {
      name: 'guest-agent-dev',
      jobs: [
        'build-guest-agent-dev',
      ],
    },
    {
      name: 'guest-oslogin',
      jobs: [
        'build-guest-oslogin',
      ],
    },
    {
      name: 'osconfig',
      jobs: [
        'build-osconfig',
        'promote-osconfig-staging',
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
      ],
    },
    {
      name: 'artifact-registry-plugins',
      jobs: [
        'build-artifact-registry-yum-plugin',
        'build-artifact-registry-apt-transport',
      ],
    },
    {
      name: 'compute-image-windows',
      jobs: [
        'build-compute-image-windows',
      ],
    },
    {
      name: 'gce-diagnostics',
      jobs: [
        'build-diagnostics',
      ],
    },
  ],
}
