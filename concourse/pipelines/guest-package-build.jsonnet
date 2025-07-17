local underscore(input) = std.strReplace(input, '-', '_');
local commaSeparatedString(inputArray) =  std.join(",", inputArray);

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
      type: 'registry-image-private',
      source: {
        repository: 'gcr.io/gcp-guest/concourse-metrics',
        google_auth: true,
      },
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
  extended_tasks:: [],
  build_dir:: '',
  extra_repo:: '',

  default_trigger_steps:: [
    {
      get: tl.package,
      trigger: true,
      params: { skip_download: true },
    },
    { get: 'guest-test-infra' },
  ],

  trigger_steps:: if tl.extra_repo != '' then tl.default_trigger_steps + [
    {
      get: tl.extra_repo,
      trigger: true,
      params: { skip_download: true },
    },
  ] else tl.default_trigger_steps,

  default_load_sha:: [
    { load_var: 'commit-sha', file: '%s/.git/ref' % tl.package },
  ],

  load_sha_steps:: if tl.extra_repo != '' then tl.default_load_sha + [
    {
      load_var: 'extra-commit-sha',
      file: '%s/.git/ref' % tl.extra_repo,
    },
  ] else tl.default_load_sha,

  extra_daisy_args:: if tl.extra_repo != '' then [
    '-var:extra_repo=' + tl.extra_repo,
    '-var:extra_git_ref=((.:extra-commit-sha))',
  ] else [],

  // Start of output.
  name: 'build-' + tl.package,

  parallel_triggers:: [
    // Prep build variables and content.
    {
      in_parallel: {
        steps: tl.trigger_steps,
      },
    },
  ],

  plan: tl.parallel_triggers + tl.load_sha_steps + [
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
              'latest=$(cd repo;git fetch --tags origin;git tag -l "20*"|tail -1)',
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
                ] + tl.extra_daisy_args + [
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
  ] + tl.extra_tasks + tl.extended_tasks,
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
      put: '%s-tag' % tl.package,
      params: {
        name: 'package-version/version',
        tag: 'package-version/version',
        commitish: '%s/.git/ref' % tl.package,
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

// task which uploads a package version using the 'uploadToArtifactReleaser' pubsub request type
local uploadpackageversiontask = {
  local tl = self,

  // Unlike other parameters, gcs_files must be enclosed in double quotes when passed in for json parsing.
  // For example, gcs_files: '"path1","path2"', or gcs_files: '"path"' if there is only one file.
  gcs_files:: error 'must set gcs_files in uploadpackageversiontask',
  os_type:: error 'must set os_type in uploadpackageversiontask',
  pkg_inside_name:: error 'must set pkg_inside_name in uploadpackageversiontask',
  pkg_name:: error 'must set pkgname in uploadpackageversiontask',
  pkg_version:: error 'must set pkgversion in uploadpackageversiontask',
  request_type:: 'uploadToArtifactReleaser',
  reponame:: error 'must set reponame in uploadpackageversiontask',
  sbom_file:: error 'must set sbom_file in uploadpackageversiontask',
  topic:: 'projects/artifact-releaser-prod/topics/artifact-registry-package-upload-prod',


  task: 'upload-' + tl.pkg_name + '-' + tl.os_type,
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
        '{"type": "%s", "request": {"ostype": "%s", "pkginsidename": "%s", "pkgname": "%s", "pkgversion": "%s", "reponame": "%s", "sbomfile": "%s", "gcsfiles": [%s]}}' %
        [tl.request_type, tl.os_type, tl.pkg_inside_name, tl.pkg_name, tl.pkg_version, tl.reponame, tl.sbom_file, tl.gcs_files],
      ],
    },
  },
};

// task which builds a windows derivative OS image with a specific package added, for use in tests
local buildpackageimagetaskwindows = {
  local tl = self,

  image_name:: error 'must set image_name in buildpackageimagetaskwindows',
  source_image:: error 'must set source_image in buildpackageimagetaskwindows',
  dest_image:: error 'must set dest_image in buildpackageimagetaskwindows',
  gcs_package_path:: error 'must set gcs_package_path in buildpackageimagetaskwindows',

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
        './compute-image-tools/daisy_workflows/image_build/install_package/windows/install_package.wf.json',
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
  disk_type:: 'pd-ssd',

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
        '-var:disk_type=' + tl.disk_type,
        './compute-image-tools/daisy_workflows/image_build/install_package/install_package.wf.json',
      ],
    },
  },
};

local buildpackageimagetaskcos = {
  local tl = self,

  image_name:: error 'must set image_name in buildpackageimagetaskcos',
  source_image:: error 'must set source_image in buildpackageimagetaskcos',
  dest_image:: error 'must set dest_image in buildpackageimagetaskcos',
  commit_sha:: error 'must set dest_image in buildpackageimagetaskcos',
  machine_type:: error 'must set machine_type in buildpackageimagetaskcos',
  worker_image:: error 'must set worker_image in buildpackageimagetaskcos',

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
        '-var:dest_image=' + tl.dest_image,
        '-var:commit_sha=' + tl.commit_sha,
        '-var:machine_type=' + tl.machine_type,
        '-var:worker_image=' + tl.worker_image,
        './compute-image-tools/daisy_workflows/image_build/install_package/cos/install_package_cos.wf.json',
      ],
    },
  },
};

local build_guest_configs = buildpackagejob {
  local tl = self,
  package:: error 'must set package for build_guest_configs',
  builds: ['deb12', 'deb11', 'el8', 'el9'],
  gcs_dir: 'google-compute-engine',
  uploads: [
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb"',
      os_type: 'BUSTER_APT',
      pkg_inside_name: 'google-compute-engine',
      pkg_name: 'guest-configs',
      pkg_version: '((.:package-version))',
      reponame: 'gce-google-compute-engine-buster',
      sbom_file: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version)).sbom.json',
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb"',
      os_type: 'BULLSEYE_APT',
      pkg_inside_name: 'google-compute-engine',
      pkg_name: 'guest-configs',
      pkg_version: '((.:package-version))',
      reponame: 'gce-google-compute-engine-bullseye',
      sbom_file: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version)).sbom.json',
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb"',
      os_type: 'BOOKWORM_APT',
      pkg_inside_name: 'google-compute-engine',
      pkg_name: 'guest-configs',
      pkg_version: '((.:package-version))',
      reponame: 'gce-google-compute-engine-bookworm',
      sbom_file: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version)).sbom.json',
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el8.noarch.rpm"',
      os_type: 'EL8_YUM',
      pkg_inside_name: 'google-compute-engine',
      pkg_name: 'guest-configs',
      pkg_version: '((.:package-version))',
      reponame: 'gce-google-compute-engine-el8',
      sbom_file: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version)).sbom.json',
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el9.noarch.rpm"',
      os_type: 'EL9_YUM',
      pkg_inside_name: 'google-compute-engine',
      pkg_name: 'guest-configs',
      pkg_version: '((.:package-version))',
      reponame: 'gce-google-compute-engine-el9',
      sbom_file: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version)).sbom.json',
    },
  ],
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
            'buildid=$(date "+%s"); echo ' + tl.package + '-$buildid | tee build-id-dir/build-id',
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
            image_name: 'debian-11',
            source_image: 'projects/debian-cloud/global/images/family/debian-11',
            dest_image: 'debian-11-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb',
          },
          buildpackageimagetask {
            image_name: 'debian-12',
            source_image: 'projects/bct-prod-images/global/images/family/debian-12',
            dest_image: 'debian-12-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb',
          },
          buildpackageimagetask {
            image_name: 'debian-12-arm64',
            source_image: 'projects/bct-prod-images/global/images/family/debian-12-arm64',
            dest_image: 'debian-12-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb',
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          buildpackageimagetask {
            image_name: 'rhel-8',
            source_image: 'projects/rhel-cloud/global/images/family/rhel-8',
            dest_image: 'rhel-8-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el8.noarch.rpm',
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-8-optimized-gcp-arm64',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-8-optimized-gcp-arm64',
            dest_image: 'rocky-linux-8-optimized-gcp-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el8.noarch.rpm',
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-8',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-8',
            dest_image: 'rocky-linux-8-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el8.noarch.rpm',
          },
          buildpackageimagetask {
            image_name: 'rhel-9',
            source_image: 'projects/rhel-cloud/global/images/family/rhel-9',
            dest_image: 'rhel-9-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el9.noarch.rpm',
          },
          buildpackageimagetask {
            image_name: 'rhel-9-arm64',
            source_image: 'projects/rhel-cloud/global/images/family/rhel-9-arm64',
            dest_image: 'rhel-9-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el9.noarch.rpm',
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-9',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-9',
            dest_image: 'rocky-linux-9-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el9.noarch.rpm',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker',
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-9-optimized-gcp',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-9-optimized-gcp',
            dest_image: 'rocky-linux-9-optimized-gcp-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el9.noarch.rpm',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker',
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-9-optimized-gcp-arm64',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-9-optimized-gcp-arm64',
            dest_image: 'rocky-linux-9-optimized-gcp-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el9.noarch.rpm',
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-9-arm64',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-9-arm64',
            dest_image: 'rocky-linux-9-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el9.noarch.rpm',
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
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
                  '-images=projects/gcp-guest/global/images/debian-11-((.:build-id)),projects/gcp-guest/global/images/debian-12-((.:build-id)),projects/gcp-guest/global/images/rhel-8-((.:build-id)),projects/gcp-guest/global/images/rocky-linux-8-((.:build-id)),projects/gcp-guest/global/images/rhel-9-((.:build-id)),projects/gcp-guest/global/images/rocky-linux-9-((.:build-id)),projects/gcp-guest/global/images/rocky-linux-9-optimized-gcp-((.:build-id))',
                  '-filter=^(packagemanager|networkinterfacenaming|cvm|loadbalancer|guestagent|hostnamevalidation|network|packagevalidation|ssh|metadata|mdsroutes|vmspec)$',
                  '-parallel_count=15',
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
                  '-images=projects/gcp-guest/global/images/debian-12-arm64-((.:build-id)),projects/gcp-guest/global/images/rocky-linux-8-optimized-gcp-arm64-((.:build-id)),projects/gcp-guest/global/images/rhel-9-arm64-((.:build-id)),projects/gcp-guest/global/images/rocky-linux-8-optimized-gcp-arm64-((.:build-id)),projects/gcp-guest/global/images/rocky-linux-9-arm64-((.:build-id))',
                  '-filter=^(cvm|loadbalancer|guestagent|hostnamevalidation|network|packagevalidation|ssh|metadata|mdsroutes|vmspec)$',
                  '-test_projects=compute-image-test-pool-002,compute-image-test-pool-003,compute-image-test-pool-004,compute-image-test-pool-005',
                  '-parallel_count=15',
                ],
              },
            },
          },
        ],
      },
    },
  ],
};

local build_guest_agent = buildpackagejob {
  local tl = self,

  package:: error 'must set package in build_guest_agent',
  uploads: [],
  builds: ['deb12', 'deb12-arm64', 'el8', 'el8-arm64', 'el9', 'el9-arm64', 'goo'],

  local x86ImagesToTest = [
    'projects/gcp-guest/global/images/debian-11-((.:build-id))',
    'projects/gcp-guest/global/images/debian-12-((.:build-id))',
    'projects/gcp-guest/global/images/centos-stream-9-((.:build-id))',
    'projects/gcp-guest/global/images/oracle-linux-8-((.:build-id))',
    'projects/gcp-guest/global/images/oracle-linux-9-((.:build-id))',
    'projects/gcp-guest/global/images/rhel-8-((.:build-id))',
    'projects/gcp-guest/global/images/rhel-9-((.:build-id))',
    'projects/gcp-guest/global/images/rhel-9-4-sap-ha-((.:build-id))',
    'projects/gcp-guest/global/images/rocky-linux-8-((.:build-id))',
    'projects/gcp-guest/global/images/rocky-linux-9-((.:build-id))',
    'projects/gcp-guest/global/images/rocky-linux-8-optimized-gcp-((.:build-id))',
    'projects/gcp-guest/global/images/rocky-linux-9-optimized-gcp-((.:build-id))',
    'projects/gcp-guest/global/images/ubuntu-2504-amd64-((.:build-id))',
    'projects/gcp-guest/global/images/sles-15-((.:build-id))',
    'projects/gcp-guest/global/images/windows-server-2022-dc-((.:build-id))'
  ],

  local arm64ImagesToTest = [
    'projects/gcp-guest/global/images/centos-stream-9-arm64-((.:build-id))',
    'projects/gcp-guest/global/images/debian-12-arm64-((.:build-id))',
    'projects/gcp-guest/global/images/rhel-8-arm64-((.:build-id))',
    'projects/gcp-guest/global/images/rhel-9-arm64-((.:build-id))',
    'projects/gcp-guest/global/images/rocky-linux-8-optimized-gcp-arm64-((.:build-id))',
    'projects/gcp-guest/global/images/rocky-linux-9-optimized-gcp-arm64-((.:build-id))',
    'projects/gcp-guest/global/images/rocky-linux-9-arm64-((.:build-id))',
    'projects/gcp-guest/global/images/ubuntu-2504-arm64-((.:build-id))'
  ],

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
            'buildid=$(date "+%s"); echo ' + tl.package + '-$buildid | tee build-id-dir/build-id',
          ],
        },
      },
    },
    { load_var: 'build-id', file: 'build-id-dir/build-id' },
    { get: 'compute-image-tools' },
    {
      in_parallel: {
        steps: [
          buildpackageimagetaskwindows {
            image_name: 'windows-2022',
            source_image: 'projects/windows-cloud/global/images/family/windows-2022',
            dest_image: 'windows-server-2022-dc-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-compute-engine-windows.x86_64.((.:package-version)).0@1.goo' % [tl.package],
          },
          buildpackageimagetask {
            image_name: 'debian-11',
            source_image: 'projects/debian-cloud/global/images/family/debian-11',
            dest_image: 'debian-11-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_amd64.deb' % [tl.package],
          },
          buildpackageimagetask {
            image_name: 'debian-12',
            source_image: 'projects/bct-prod-images/global/images/family/debian-12',
            dest_image: 'debian-12-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_amd64.deb' % [tl.package],
          },
          // TODO(b/431239519): We're temporarily force installing debian packages for testing on ubuntu images.
          // Update this once we have right packages.
          buildpackageimagetask {
            image_name: 'ubuntu-2504-amd64',
            source_image: 'projects/ubuntu-os-cloud/global/images/family/ubuntu-2504-amd64',
            dest_image: 'ubuntu-2504-amd64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_amd64.deb' % [tl.package],
          },
          buildpackageimagetask {
            image_name: 'ubuntu-2504-arm64',
            source_image: 'projects/ubuntu-os-cloud/global/images/family/ubuntu-2504-arm64',
            dest_image: 'ubuntu-2504-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_arm64.deb' % [tl.package],
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          buildpackageimagetask {
            image_name: 'debian-12-arm64',
            source_image: 'projects/bct-prod-images/global/images/family/debian-12-arm64',
            dest_image: 'debian-12-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_arm64.deb' % [tl.package],
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          buildpackageimagetask {
            image_name: 'rhel-8',
            source_image: 'projects/rhel-cloud/global/images/family/rhel-8',
            dest_image: 'rhel-8-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el8.x86_64.rpm' % [tl.package],
          },
          buildpackageimagetask {
            image_name: 'oracle-linux-8',
            source_image: 'projects/oracle-linux-cloud/global/images/family/oracle-linux-8',
            dest_image: 'oracle-linux-8-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el8.x86_64.rpm' % [tl.package],
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-8',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-8',
            dest_image: 'rocky-linux-8-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el8.x86_64.rpm' % [tl.package],
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker',
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-8-optimized-gcp',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-8-optimized-gcp',
            dest_image: 'rocky-linux-8-optimized-gcp-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el8.x86_64.rpm' % [tl.package],
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker',
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-8-optimized-gcp-arm64',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-8-optimized-gcp-arm64',
            dest_image: 'rocky-linux-8-optimized-gcp-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el8.aarch64.rpm' % [tl.package],
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          buildpackageimagetask {
            image_name: 'rhel-8-arm64',
            source_image: 'projects/rhel-cloud/global/images/family/rhel-8-arm64',
            dest_image: 'rhel-8-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el8.aarch64.rpm' % [tl.package],
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          buildpackageimagetask {
            image_name: 'rhel-9',
            source_image: 'projects/rhel-cloud/global/images/family/rhel-9',
            dest_image: 'rhel-9-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.x86_64.rpm' % [tl.package],
          },
          buildpackageimagetask {
            image_name: 'rhel-9-4-sap-ha',
            source_image: 'projects/rhel-sap-cloud/global/images/family/rhel-9-4-sap-ha',
            dest_image: 'rhel-9-4-sap-ha-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.x86_64.rpm' % [tl.package],
          },
          buildpackageimagetask {
            image_name: 'oracle-linux-9',
            source_image: 'projects/oracle-linux-cloud/global/images/family/oracle-linux-9',
            dest_image: 'oracle-linux-9-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.x86_64.rpm' % [tl.package],
          },
          buildpackageimagetask {
            image_name: 'centos-stream-9',
            source_image: 'projects/centos-cloud/global/images/family/centos-stream-9',
            dest_image: 'centos-stream-9-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.x86_64.rpm' % [tl.package],
          },
          buildpackageimagetask {
            image_name: 'rhel-9-arm64',
            source_image: 'projects/rhel-cloud/global/images/family/rhel-9-arm64',
            dest_image: 'rhel-9-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.aarch64.rpm' % [tl.package],
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          buildpackageimagetask {
            image_name: 'centos-stream-9-arm64',
            source_image: 'projects/centos-cloud/global/images/family/centos-stream-9-arm64',
            dest_image: 'centos-stream-9-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.aarch64.rpm' % [tl.package],
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-9',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-9',
            dest_image: 'rocky-linux-9-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.x86_64.rpm' % [tl.package],
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker',
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-9-arm64',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-9-arm64',
            dest_image: 'rocky-linux-9-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.aarch64.rpm' % [tl.package],
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-9-optimized-gcp',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-9-optimized-gcp',
            dest_image: 'rocky-linux-9-optimized-gcp-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.x86_64.rpm' % [tl.package],
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker',
          },
          buildpackageimagetask {
            image_name: 'rocky-linux-9-optimized-gcp-arm64',
            source_image: 'projects/rocky-linux-cloud/global/images/family/rocky-linux-9-optimized-gcp-arm64',
            dest_image: 'rocky-linux-9-optimized-gcp-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.aarch64.rpm' % [tl.package],
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          buildpackageimagetask {
            image_name: 'sles-15',
            source_image: 'projects/suse-cloud/global/images/family/sles-15',
            dest_image: 'sles-15-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.x86_64.rpm' % [tl.package],
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
                  '-images=%s' % commaSeparatedString(x86ImagesToTest),
                  '-filter=^(cvm|loadbalancer|guestagent|hostnamevalidation|network|packagevalidation|ssh|metadata|mdsroutes|vmspec|compatmanager|pluginmanager|mdsmtls)$',
                  '-parallel_count=15',
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
                  '-images=%s' % commaSeparatedString(arm64ImagesToTest),
                  '-filter=^(cvm|loadbalancer|guestagent|hostnamevalidation|network|packagevalidation|ssh|metadata|mdsroutes|vmspec|compatmanager|pluginmanager|mdsmtls)$',
                  '-parallel_count=15',
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
  local tl = self,

  package:: error 'must set package in build_and_upload_guest_agent',

  uploads: [
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_amd64.deb"' % [tl.package],
      os_type: 'BUSTER_APT',
      pkg_inside_name: 'google-guest-agent',
      pkg_name: 'guest-agent',
      pkg_version: '((.:package-version))',
      reponame: 'google-guest-agent-buster',
      sbom_file: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version)).sbom.json' % [tl.package],
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_amd64.deb","gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_arm64.deb"' % [tl.package, tl.package],
      os_type: 'BULLSEYE_APT',
      pkg_inside_name: 'google-guest-agent',
      pkg_name: 'guest-agent',
      pkg_version: '((.:package-version))',
      reponame: 'google-guest-agent-bullseye',
      sbom_file: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version)).sbom.json' % [tl.package],
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_amd64.deb","gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_arm64.deb"' % [tl.package, tl.package],
      os_type: 'BOOKWORM_APT',
      pkg_inside_name: 'google-guest-agent',
      pkg_name: 'guest-agent',
      pkg_version: '((.:package-version))',
      reponame: 'google-guest-agent-bookworm',
      sbom_file: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version)).sbom.json' % [tl.package],
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_amd64.deb","gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_arm64.deb"' % [tl.package, tl.package],
      os_type: 'TRIXIE_APT',
      pkg_inside_name: 'google-guest-agent',
      pkg_name: 'guest-agent',
      pkg_version: '((.:package-version))',
      reponame: 'google-guest-agent-trixie',
      sbom_file: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version)).sbom.json' % [tl.package],
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el8.x86_64.rpm","gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el8.aarch64.rpm"' % [tl.package, tl.package],
      os_type: 'EL8_YUM',
      pkg_inside_name: 'google-guest-agent',
      pkg_name: 'guest-agent',
      pkg_version: '((.:package-version))',
      reponame: 'google-guest-agent-el8',
      sbom_file: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version)).sbom.json' % [tl.package],
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.x86_64.rpm","gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el9.aarch64.rpm"' % [tl.package, tl.package],
      os_type: 'EL9_YUM',
      pkg_inside_name: 'google-guest-agent',
      pkg_name: 'guest-agent',
      pkg_version: '((.:package-version))',
      reponame: 'google-guest-agent-el9',
      sbom_file: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version)).sbom.json' % [tl.package],
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el10.x86_64.rpm","gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el10.aarch64.rpm"' % [tl.package, tl.package],
      os_type: 'EL10_YUM',
      pkg_inside_name: 'google-guest-agent',
      pkg_name: 'guest-agent',
      pkg_version: '((.:package-version))',
      reponame: 'google-guest-agent-el10',
      sbom_file: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version)).sbom.json' % [tl.package],
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/%s/google-compute-engine-windows.x86_64.((.:package-version)).0@1.goo"' % [tl.package],
      os_type: 'WINDOWS_ALL_GOOGET',
      pkg_inside_name: 'google-compute-engine-windows',
      pkg_name: 'google-compute-engine-windows',
      pkg_version: '((.:package-version))',
      reponame: 'google-compute-engine-windows',
      sbom_file: 'gs://gcp-guest-package-uploads/%s/google-compute-engine-windows-((.:package-version)).sbom.json' % [tl.package],
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/%s/google-compute-engine-metadata-scripts.x86_64.((.:package-version)).0@1.goo"' % [tl.package],
      os_type: 'WINDOWS_ALL_GOOGET',
      pkg_inside_name: 'google-compute-engine-metadata-scripts',
      pkg_name: 'google-compute-engine-metadata-scripts',
      pkg_version: '((.:package-version))',
      reponame: 'google-compute-engine-metadata-scripts',
      sbom_file: 'gs://gcp-guest-package-uploads/%s/google-compute-engine-metadata-scripts-((.:package-version)).sbom.json' % [tl.package],
    },
  ],
};

local build_and_upload_oslogin = buildpackagejob {
      local tl = self,
      package:: error 'must set package in build_and_upload_oslogin',
      gcs_dir:: error 'must set gcs_dir in build_and_upload_oslogin',
      builds: ['deb11', 'deb12', 'deb12-arm64', 'deb13', 'el8', 'el8-arm64', 'el9', 'el9-arm64', 'el10', 'el10-arm64'],
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
                'buildid=$(date "+%s"); echo ' + tl.package + '-$buildid | tee build-id-dir/build-id',
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
                image_name: 'debian-11',
                source_image: 'projects/debian-cloud/global/images/family/debian-11',
                dest_image: 'debian-11-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb11_amd64.deb',
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
                machine_type: 'c4a-standard-2',
                disk_type: 'hyperdisk-balanced',
                worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
              },
              buildpackageimagetask {
                image_name: 'debian-13',
                source_image: 'projects/bct-prod-images/global/images/family/debian-12',
                dest_image: 'debian-13-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb12_amd64.deb',
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
                machine_type: 'c4a-standard-2',
                disk_type: 'hyperdisk-balanced',
                worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
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
                machine_type: 'c4a-standard-2',
                disk_type: 'hyperdisk-balanced',
                worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
              },
              buildpackageimagetask {
                image_name: 'rhel-10',
                source_image: 'projects/rhel-cloud/global/images/family/rhel-9',
                dest_image: 'rhel-10-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el10.x86_64.rpm',
              },
              buildpackageimagetask {
                image_name: 'rhel-10-arm64',
                source_image: 'projects/rhel-cloud/global/images/family/rhel-10-arm64',
                dest_image: 'rhel-10-arm64-((.:build-id))',
                gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el10.aarch64.rpm',
                machine_type: 'c4a-standard-2',
                disk_type: 'hyperdisk-balanced',
                worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
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
                      '-test_projects=oslogin-cit',
                      '-parallel_count=2',
                      '-images=projects/gcp-guest/global/images/debian-11-((.:build-id)),projects/gcp-guest/global/images/debian-12-((.:build-id)),projects/gcp-guest/global/images/debian-13-((.:build-id)),projects/gcp-guest/global/images/rhel-8-((.:build-id)),projects/gcp-guest/global/images/rhel-9-((.:build-id)),projects/gcp-guest/global/images/rhel-10-((.:build-id))',
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
                      '-test_projects=oslogin-cit',
                      '-images=projects/gcp-guest/global/images/debian-12-arm64-((.:build-id)),projects/gcp-guest/global/images/rocky-linux-8-optimized-gcp-arm64-((.:build-id)),projects/gcp-guest/global/images/rhel-9-arm64-((.:build-id)),projects/gcp-guest/global/images/rhel-10-arm64-((.:build-id))',
                      '-parallel_count=2',
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
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb11_amd64.deb","gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb11_arm64.deb"',
          os_type: 'BULLSEYE_APT',
          pkg_inside_name: 'google-compute-engine-oslogin',
          pkg_name: 'guest-oslogin',
          pkg_version: '((.:package-version))',
          reponame: 'gce-google-compute-engine-oslogin-bullseye',
          sbom_file: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version)).sbom.json',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb12_amd64.deb","gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb12_arm64.deb"',
          os_type: 'BOOKWORM_APT',
          pkg_inside_name: 'google-compute-engine-oslogin',
          pkg_name: 'guest-oslogin',
          pkg_version: '((.:package-version))',
          reponame: 'gce-google-compute-engine-oslogin-bookworm',
          sbom_file: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version)).sbom.json',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb12_amd64.deb"',
          os_type: 'TRIXIE_APT',
          pkg_inside_name: 'google-compute-engine-oslogin',
          pkg_name: 'guest-oslogin',
          pkg_version: '((.:package-version))',
          reponame: 'gce-google-compute-engine-oslogin-trixie',
          sbom_file: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version)).sbom.json',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el8.x86_64.rpm","gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el8.aarch64.rpm"',
          os_type: 'EL8_YUM',
          pkg_inside_name: 'google-compute-engine-oslogin',
          pkg_name: 'guest-oslogin',
          pkg_version: '((.:package-version))',
          reponame: 'gce-google-compute-engine-oslogin-el8',
          sbom_file: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version)).sbom.json',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el9.x86_64.rpm","gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el9.aarch64.rpm"',
          os_type: 'EL9_YUM',
          pkg_inside_name: 'google-compute-engine-oslogin',
          pkg_name: 'guest-oslogin',
          pkg_version: '((.:package-version))',
          reponame: 'gce-google-compute-engine-oslogin-el9',
          sbom_file: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version)).sbom.json',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el10.x86_64.rpm","gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el10.aarch64.rpm"',
          os_type: 'EL10_YUM',
          pkg_inside_name: 'google-compute-engine-oslogin',
          pkg_name: 'guest-oslogin',
          pkg_version: '((.:package-version))',
          reponame: 'gce-google-compute-engine-oslogin-el10',
          sbom_file: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version)).sbom.json',
        },
      ],
    };


// Start of output
{
  jobs: [
    build_guest_configs {
      package: 'guest-configs',
    },
    build_and_upload_guest_agent {
      package: 'guest-agent',
      extra_repo: 'google-guest-agent',
    },
    build_and_upload_guest_agent {
      package: 'guest-agent-stable',
      gcs_dir: 'guest-agent-stable',
      extra_repo: 'google-guest-agent',
      repo_name: 'guest-agent',
    },
    build_guest_agent {
      package: 'guest-agent-dev',
      repo_name: 'guest-agent',
      extra_repo: 'google-guest-agent',
      extended_tasks: [],
    },
    build_and_upload_oslogin {
      package: 'guest-oslogin',
      gcs_dir: 'oslogin',
      repo_name: 'guest-oslogin',
    },
    build_and_upload_oslogin {
      package: 'guest-oslogin-stbl',
      gcs_dir: 'oslogin-stbl',
      repo_name: 'guest-oslogin',
    },
    buildpackagejob {
      package: 'guest-diskexpand',
      builds: ['deb12', 'deb13', 'el8', 'el9', 'el10'],
      gcs_dir: 'gce-disk-expand',
      uploads: [
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/gce-disk-expand/gce-disk-expand_((.:package-version))-g1_all.deb"',
          os_type: 'DEBIAN_ALL_APT',
          pkg_inside_name: 'gce-disk-expand',
          pkg_name: 'guest-diskexpand',
          pkg_version: '((.:package-version))',
          reponame: 'gce-disk-expand',
          sbom_file: 'gs://gcp-guest-package-uploads/gce-disk-expand/gce-disk-expand-((.:package-version)).sbom.json',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/gce-disk-expand/gce-disk-expand-((.:package-version))-g1.el8.noarch.rpm"',
          os_type: 'EL8_YUM',
          pkg_inside_name: 'gce-disk-expand',
          pkg_name: 'guest-diskexpand',
          pkg_version: '((.:package-version))',
          reponame: 'gce-disk-expand-el8',
          sbom_file: 'gs://gcp-guest-package-uploads/gce-disk-expand/gce-disk-expand-((.:package-version)).sbom.json',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/gce-disk-expand/gce-disk-expand-((.:package-version))-g1.el9.noarch.rpm"',
          os_type: 'EL9_YUM',
          pkg_inside_name: 'gce-disk-expand',
          pkg_name: 'guest-diskexpand',
          pkg_version: '((.:package-version))',
          reponame: 'gce-disk-expand-el9',
          sbom_file: 'gs://gcp-guest-package-uploads/gce-disk-expand/gce-disk-expand-((.:package-version)).sbom.json',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/gce-disk-expand/gce-disk-expand-((.:package-version))-g1.el10.noarch.rpm"',
          os_type: 'EL10_YUM',
          pkg_inside_name: 'gce-disk-expand',
          pkg_name: 'guest-diskexpand',
          pkg_version: '((.:package-version))',
          reponame: 'gce-disk-expand-el10',
          sbom_file: 'gs://gcp-guest-package-uploads/gce-disk-expand/gce-disk-expand-((.:package-version)).sbom.json',
        },
      ],
    },
    buildpackagejob {
      package: 'artifact-registry-yum-plugin',
      builds: ['el8', 'el8-arm64', 'el9', 'el9-arm64'],
      gcs_dir: 'yum-plugin-artifact-registry',
      uploads: [
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version))-g1.el8.x86_64.rpm","gs://gcp-guest-package-uploads/yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version))-g1.el8.aarch64.rpm"',
          os_type: 'EL8_YUM',
          pkg_inside_name: 'dnf-plugin-artifact-registry',
          pkg_name: 'artifact-registry-dnf-plugin',
          pkg_version: '((.:package-version))',
          reponame: 'dnf-plugin-artifact-registry-el8',
          sbom_file: 'gs://gcp-guest-package-uploads/yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version)).sbom.json',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version))-g1.el9.x86_64.rpm","gs://gcp-guest-package-uploads/yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version))-g1.el9.aarch64.rpm"',
          os_type: 'EL9_YUM',
          pkg_inside_name: 'dnf-plugin-artifact-registry',
          pkg_name: 'artifact-registry-dnf-plugin',
          pkg_version: '((.:package-version))',
          reponame: 'dnf-plugin-artifact-registry-el9',
          sbom_file: 'gs://gcp-guest-package-uploads/yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version)).sbom.json',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version))-g1.el10.x86_64.rpm","gs://gcp-guest-package-uploads/yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version))-g1.el10.aarch64.rpm"',
          os_type: 'EL10_YUM',
          pkg_inside_name: 'dnf-plugin-artifact-registry',
          pkg_name: 'artifact-registry-dnf-plugin',
          pkg_version: '((.:package-version))',
          reponame: 'dnf-plugin-artifact-registry-el10',
          sbom_file: 'gs://gcp-guest-package-uploads/yum-plugin-artifact-registry/dnf-plugin-artifact-registry-((.:package-version)).sbom.json',
        },
      ],
    },
    buildpackagejob {
      package: 'artifact-registry-apt-transport',
      builds: ['deb13', 'deb12', 'deb12-arm64'],
      uploads: [
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/artifact-registry-apt-transport/apt-transport-artifact-registry_((.:package-version))-g1_amd64.deb","gs://gcp-guest-package-uploads/artifact-registry-apt-transport/apt-transport-artifact-registry_((.:package-version))-g1_arm64.deb"',
          os_type: 'DEBIAN_ALL_APT',
          pkg_inside_name: 'apt-transport-artifact-registry',
          pkg_name: 'artifact-registry-apt-transport',
          pkg_version: '((.:package-version))',
          reponame: 'apt-transport-artifact-registry',
          sbom_file: 'gs://gcp-guest-package-uploads/artifact-registry-apt-transport/apt-transport-artifact-registry-((.:package-version)).sbom.json',
        },
      ],
    },
    buildpackagejob {
      package: 'compute-image-windows',
      builds: ['goo'],
      uploads: [
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/compute-image-windows/certgen.x86_64.((.:package-version)).0@1.goo"',
          os_type: 'WINDOWS_ALL_GOOGET',
          pkg_inside_name: 'certgen',
          pkg_name: 'certgen',
          pkg_version: '((.:package-version))',
          reponame: 'certgen',
          sbom_file: '',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/compute-image-windows/google-compute-engine-auto-updater.noarch.((.:package-version))@1.goo"',
          os_type: 'WINDOWS_ALL_GOOGET',
          pkg_inside_name: 'google-compute-engine-auto-updater',
          pkg_name: 'google-compute-engine-auto-updater',
          pkg_version: '((.:package-version))',
          reponame: 'google-compute-engine-auto-updater',
          sbom_file: '',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/compute-image-windows/google-compute-engine-powershell.noarch.((.:package-version))@1.goo"',
          os_type: 'WINDOWS_ALL_GOOGET',
          pkg_inside_name: 'google-compute-engine-powershell',
          pkg_name: 'google-compute-engine-powershell',
          pkg_version: '((.:package-version))',
          reponame: 'google-compute-engine-powershell',
          sbom_file: '',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/compute-image-windows/google-compute-engine-sysprep.noarch.((.:package-version))@1.goo"',
          os_type: 'WINDOWS_ALL_GOOGET',
          pkg_inside_name: 'google-compute-engine-sysprep',
          pkg_name: 'google-compute-engine-sysprep',
          pkg_version: '((.:package-version))',
          reponame: 'google-compute-engine-sysprep',
          sbom_file: '',
        },
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/compute-image-windows/google-compute-engine-ssh.x86_64.((.:package-version)).0@1.goo"',
          os_type: 'WINDOWS_ALL_GOOGET',
          pkg_inside_name: 'google-compute-engine-ssh',
          pkg_name: 'google-compute-engine-ssh',
          pkg_version: '((.:package-version))',
          reponame: 'google-compute-engine-ssh',
          sbom_file: '',
        },
      ],
    },
    buildpackagejob {
      package: 'compute-image-tools',
      builds: ['goo'],
      name: 'build-diagnostics',
      uploads: [
        uploadpackageversiontask {
          gcs_files: '"gs://gcp-guest-package-uploads/compute-image-tools/google-compute-engine-diagnostics.x86_64.((.:package-version)).0@0.goo"',
          os_type: 'WINDOWS_ALL_GOOGET',
          pkg_inside_name: 'google-compute-engine-diagnostics',
          pkg_name: 'google-compute-engine-diagnostics',
          pkg_version: '((.:package-version))',
          reponame: 'google-compute-engine-diagnostics',
          sbom_file: '',
        },
      ],
      build_dir: 'cli_tools/diagnostics',
    },
  ],
  resource_types: [
    {
      name: 'registry-image-private',
      type: 'registry-image',
      source: { repository: 'gcr.io/compute-image-tools/registry-image-forked' },
    },
  ],
  resources: [
    {
      name: 'guest-agent-stable',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/guest-agent.git',
        branch: 'topic-stable',
        fetch_tags: true,
      },
    },
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
      name: 'google-guest-agent',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/google-guest-agent.git',
        branch: 'main',
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
      name: 'guest-agent-stable-tag',
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
      name: 'guest-oslogin-stbl',
      type: 'git',
      source: {
        uri: 'https://github.com/GoogleCloudPlatform/guest-oslogin.git',
        branch: 'stable',
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
      name: 'guest-oslogin-stbl-tag',
      type: 'github-release',
      source: {
        owner: 'GoogleCloudPlatform',
        repository: 'guest-oslogin',
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
      name: 'guest-agent-stable',
      jobs: [
        'build-guest-agent-stable',
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
      name: 'guest-oslogin-stbl',
      jobs: [
        'build-guest-oslogin-stbl',
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
