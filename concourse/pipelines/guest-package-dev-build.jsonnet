local underscore(input) = std.strReplace(input, '-', '_');
local commaSeparatedString(inputArray) = std.join(',', inputArray);
local gcp_secret_manager = import '../templates/gcp-secret-manager.libsonnet';

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
        '--pipeline=guest-package-build-dev',
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
  extra_repo_owner:: '',
  secret_name:: '',
  spec_name:: '',
  test_suite:: '',
  abbr_name:: '',

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
  '-var:extra_repo_owner=' + tl.extra_repo_owner,
  // TODO: Remove the pinned commit when phase 3 is rolled out.
  // Temporarily build on phase 2 guest agent until phase 3 is complete.
  if tl.extra_repo == 'google-guest-agent' then
    '-var:extra_git_ref=b87e965fb35a54892442ff26456d77e7705c2f88'
  else
    '-var:extra_git_ref=((.:extra-commit-sha))',
] else [],

  // Fetch LKG secrets if secret_name is defined
  fetch_lkg_steps:: if tl.secret_name != '' then [
    {
      task: 'get-secret-' + tl.secret_name,
      config: gcp_secret_manager.getsecrettask { secret_name: tl.secret_name },
    },
    {
      load_var: tl.secret_name + '-secret',
      file: 'gcp-secret-manager/' + tl.secret_name,
    },
  ] else [], 

  local lkg_daisy_vars =  if tl.secret_name != '' then [
    '-var:lkg_gcs_path=((.:' + tl.secret_name + '-secret))'
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

  plan: tl.parallel_triggers + tl.load_sha_steps + tl.fetch_lkg_steps + [
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
            task: 'guest-package-dev-build-%s-%s' % [tl.package, build],
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
                  '-project=guest-package-builder',
                  '-zone=us-west1-a',
                  '-var:repo_owner=GoogleCloudPlatform',
                  '-var:repo_name=' + tl.repo_name,
                  // TODO: Remove the pinned commit when phase 3 is rolled out.
                  // Temporarily build on phase 2 guest agent until phase 3 is complete.
                  if tl.repo_name == 'guest-agent' then
                    '-var:git_ref=1a3694aec8b63212634afdcd98e7aa4016858421'
                  else
                    '-var:git_ref=((.:commit-sha))',
                  '-var:version=((.:package-version))',
                  '-var:gcs_path=gs://gcp-guest-package-uploads/' + tl.gcs_dir,
                  '-var:build_dir=' + tl.build_dir,
                  '-var:spec_name=' + tl.spec_name,
                ] + tl.extra_daisy_args + lkg_daisy_vars + [
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
        '-project=guest-package-builder',
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
        '-project=guest-package-builder',
        '-zone=us-central1-a',
        '-var:source_image=' + tl.source_image,
        '-var:gcs_package_path=' + tl.gcs_package_path,
        '-var:dest_image=' + tl.dest_image,
        './compute-image-tools/daisy_workflows/image_build/install_package/windows/install_package.wf.json',
      ],
    },
  },
};

// Build derivative windows images with googet and certgen to run CIT validation against them.
local build_goo = buildpackagejob {
  local tl = self,

  package:: error 'must set package in build_goo',
  uploads: [],
  builds: ['goo'],

  local allCITSuites = 'packagevalidation|ssh|winrm',
  test_suite_to_run::
    if tl.test_suite == '' then
      allCITSuites
    else
      tl.test_suite,

  local x86WindowsImagesToTest = [
//    'projects/guest-package-builder/global/images/windows-server-2008-r2-dc-((.:build-id))',
//    'projects/guest-package-builder/global/images/windows-server-2012-dc-((.:build-id))',
    'projects/guest-package-builder/global/images/windows-server-2012-r2-dc-((.:build-id))',
    'projects/guest-package-builder/global/images/windows-server-2016-dc-((.:build-id))',
    'projects/guest-package-builder/global/images/windows-server-2019-dc-((.:build-id))',
    'projects/guest-package-builder/global/images/windows-server-2022-dc-((.:build-id))',
    'projects/guest-package-builder/global/images/windows-server-2025-dc-((.:build-id))',
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
            'buildid=$(date "+%s"); echo ' + tl.abbr_name + '-$buildid | tee build-id-dir/build-id',
          ],
        },
      },
    },
    { load_var: 'build-id', file: 'build-id-dir/build-id' },
    { get: 'compute-image-tools' },
    {
      in_parallel: {
        steps: [
//          buildpackageimagetaskwindows {
//            image_name: 'windows-2008-r2',
//            source_image: 'projects/bct-prod-images/global/images/family/windows-2008-r2',
//            dest_image: 'windows-server-2008-r2-dc-((.:build-id))',
//            gcs_package_path: '"gs://gcp-guest-package-uploads/%s/%s.x86_64.((.:package-version)).0@1.goo"' % [tl.package, tl.spec_name],
//          },
//          buildpackageimagetaskwindows {
//            image_name: 'windows-2012',
//            source_image: 'projects/bct-prod-images/global/images/family/windows-2012',
//            dest_image: 'windows-server-2012-dc-((.:build-id))',
//            gcs_package_path: 'gs://gcp-guest-package-uploads/guest-agent/google-compute-engine-windows.x86_64.20251009.01.0@1.goo,gs://gcp-guest-package-uploads/guest-agent/google-compute-engine-metadata-scripts.x86_64.20251009.01.0@1.goo,"gs://gcp-guest-package-uploads/%s/%s.x86_64.((.:package-version)).0@1.goo"' % [tl.package, tl.spec_name],
//          },
          buildpackageimagetaskwindows {
            image_name: 'windows-2012-r2',
            source_image: 'projects/bct-prod-images/global/images/family/windows-2012-r2',
            dest_image: 'windows-server-2012-r2-dc-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/guest-agent/google-compute-engine-windows.x86_64.20251009.01.0@1.goo,gs://gcp-guest-package-uploads/guest-agent/google-compute-engine-metadata-scripts.x86_64.20251009.01.0@1.goo,"gs://gcp-guest-package-uploads/%s/%s.x86_64.((.:package-version)).0@1.goo"' % [tl.package, tl.spec_name],
          },
          buildpackageimagetaskwindows {
            image_name: 'windows-2016',
            source_image: 'projects/windows-cloud/global/images/family/windows-2016',
            dest_image: 'windows-server-2016-dc-((.:build-id))',
            gcs_package_path: '"gs://gcp-guest-package-uploads/%s/%s.x86_64.((.:package-version)).0@1.goo"' % [tl.package, tl.spec_name],
          },
          buildpackageimagetaskwindows {
            image_name: 'windows-2019',
            source_image: 'projects/windows-cloud/global/images/family/windows-2019',
            dest_image: 'windows-server-2019-dc-((.:build-id))',
            gcs_package_path: '"gs://gcp-guest-package-uploads/%s/%s.x86_64.((.:package-version)).0@1.goo"' % [tl.package, tl.spec_name],
          },
          buildpackageimagetaskwindows {
            image_name: 'windows-2022',
            source_image: 'projects/windows-cloud/global/images/family/windows-2022',
            dest_image: 'windows-server-2022-dc-((.:build-id))', 
            gcs_package_path: '"gs://gcp-guest-package-uploads/%s/%s.x86_64.((.:package-version)).0@1.goo"' % [tl.package, tl.spec_name],
          },
          buildpackageimagetaskwindows {
            image_name: 'windows-2025',
            source_image: 'projects/windows-cloud/global/images/family/windows-2025',
            dest_image: 'windows-server-2025-dc-((.:build-id))',
            gcs_package_path: '"gs://gcp-guest-package-uploads/%s/%s.x86_64.((.:package-version)).0@1.goo"' % [tl.package, tl.spec_name],
          },
        ],
      },
    },
    {
      in_parallel: {
        fail_fast: true,
        steps: [
          {
            task: '%s-windows-image-tests-amd64' % [tl.package],
            config: {
              platform: 'linux',
              image_resource: {
                type: 'registry-image',
                source: { repository: 'gcr.io/compute-image-tools/cloud-image-tests' },
              },
              run: {
                path: '/manager',
                args: [
                  '-project=guest-package-builder',
                  '-zones=us-west1-a,us-east1-b,us-west1-b,us-west1-c,us-east1-c,us-east1-d',
                  '-x86_shape=n1-standard-4',
                  '-timeout=45m',
                  '-images=%s' % commaSeparatedString(x86WindowsImagesToTest),
                  '-filter=^(%s)$' % tl.test_suite_to_run,
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
        '-project=guest-package-builder',
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
  builds: ['deb13', 'el10'],
  gcs_dir: 'google-compute-engine',
  uploads: [
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb"',
      os_type: 'TRIXIE_APT',
      pkg_inside_name: 'google-compute-engine',
      pkg_name: 'guest-configs',
      pkg_version: '((.:package-version))',
      reponame: 'gce-google-compute-engine-trixie',
      sbom_file: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version)).sbom.json',
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/google-compute-engine/gce-configs-trixie_((.:package-version))-g1_all.deb"',
      os_type: 'TRIXIE_APT',
      pkg_inside_name: 'gce-configs-trixie',
      pkg_name: 'gce-configs-trixie',
      pkg_version: '((.:package-version))',
      reponame: 'gce-configs-trixie',
      sbom_file: 'gs://gcp-guest-package-uploads/google-compute-engine/gce-configs-trixie-((.:package-version)).sbom.json',
    },
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el10.noarch.rpm"',
      os_type: 'EL10_YUM',
      pkg_inside_name: 'google-compute-engine',
      pkg_name: 'guest-configs',
      pkg_version: '((.:package-version))',
      reponame: 'gce-google-compute-engine-el10',
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
            image_name: 'debian-13',
            source_image: 'projects/bct-prod-images/global/images/family/debian-13',
            dest_image: 'debian-13-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-13-worker',
          },
          buildpackageimagetask {
            image_name: 'debian-13-arm64',
            source_image: 'projects/bct-prod-images/global/images/family/debian-13-arm64',
            dest_image: 'debian-13-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine_((.:package-version))-g1_all.deb',
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-13-worker-arm64',
          },
          // buildpackageimagetask {
          //   image_name: 'centos-stream-10',
          //   dest_image: 'centos-stream-10-((.:build-id))',
          //   source_image: 'projects/bct-prod-images/global/images/family/centos-stream-10',
          //   gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el10.noarch.rpm',
          //   worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker',
          // },
          // buildpackageimagetask {
          //   image_name: 'centos-stream-10-arm64',
          //   source_image: 'projects/bct-prod-images/global/images/family/centos-stream-10-arm64',
          //   dest_image: 'centos-stream-10-arm64-((.:build-id))',
          //   gcs_package_path: 'gs://gcp-guest-package-uploads/google-compute-engine/google-compute-engine-((.:package-version))-g1.el10.noarch.rpm',
          //   machine_type: 'c4a-standard-2',
          //   disk_type: 'hyperdisk-balanced',
          //   worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          // },
        ],
      },
    },
  ],
};

local build_guest_agent = buildpackagejob {
  local tl = self,

  package:: error 'must set package in build_guest_agent',
  uploads: [],
  builds: ['deb13', 'deb13-arm64', 'el10', 'el10-arm64'],
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
          buildpackageimagetask {
            image_name: 'debian-13',
            source_image: 'projects/bct-prod-images/global/images/family/debian-13',
            dest_image: 'debian-13-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_amd64.deb' % [tl.package],
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker',
          },
          buildpackageimagetask {
            image_name: 'debian-13-arm64',
            source_image: 'projects/bct-prod-images/global/images/family/debian-13-arm64',
            dest_image: 'debian-13-arm64-((.:build-id))',
            gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent_((.:package-version))-g1_arm64.deb' % [tl.package],
            machine_type: 'c4a-standard-2',
            disk_type: 'hyperdisk-balanced',
            worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          },
          // buildpackageimagetask {
          //   image_name: 'centos-stream-10',
          //   source_image: 'projects/bct-prod-images/global/images/family/centos-stream-10',
          //   dest_image: 'centos-stream-10-((.:build-id))',
          //   gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el10.x86_64.rpm' % [tl.package],
          //   worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker',
          //
          // },
          // buildpackageimagetask {
          //   image_name: 'centos-stream-10-arm64',
          //   source_image: 'projects/bct-prod-images/global/images/family/centos-stream-10-arm64',
          //   dest_image: 'centos-stream-10-arm64-((.:build-id))',
          //   gcs_package_path: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el10.aarch64.rpm' % [tl.package],
          //   machine_type: 'c4a-standard-2',
          //   disk_type: 'hyperdisk-balanced',
          //   worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          // },
        ],
      },
    },
    // {
      // in_parallel: {
      //   fail_fast: true,
      //   steps: [
      //     {
      //       task: '%s-image-tests-amd64' % [tl.package],
      //       config: {
      //         platform: 'linux',
      //         image_resource: {
      //           type: 'registry-image',
      //           source: { repository: 'gcr.io/compute-image-tools/cloud-image-tests' },
      //         },
      //         run: {
      //           path: '/manager',
      //           args: [
      //             '-project=guest-package-builder',
      //             '-zone=us-central1-a',
      //             '-test_projects=compute-image-test-pool-002,compute-image-test-pool-003,compute-image-test-pool-004,compute-image-test-pool-005',
      //             '-images=projects/guest-package-builder/global/images/debian-13-((.:build-id))',
      //             '-filter=^(cvm|loadbalancer|guestagent|hostnamevalidation|network|packagevalidation|ssh|metadata|vmspec|compatmanager|pluginmanager)$',
      //             '-parallel_count=15',
      //           ],
      //         },
      //       },
      //     },
      //     {
      //       task: '%s-image-tests-arm64' % [tl.package],
      //       config: {
      //         platform: 'linux',
      //         image_resource: {
      //           type: 'registry-image',
      //           source: { repository: 'gcr.io/compute-image-tools/cloud-image-tests' },
      //         },
      //         inputs: [{ name: 'guest-test-infra' }],
      //         run: {
      //           path: '/manager',
      //           args: [
      //             '-project=guest-package-builder',
      //             '-zone=us-central1-a',
      //             '-test_projects=compute-image-test-pool-002,compute-image-test-pool-003,compute-image-test-pool-004,compute-image-test-pool-005',
      //             '-images=projects/guest-package-builder/global/images/debian-13-arm64-((.:build-id))',
      //             '-filter=^(cvm|loadbalancer|guestagent|hostnamevalidation|network|packagevalidation|ssh|metadata|vmspec|compatmanager|pluginmanager)$',
      //             '-parallel_count=15',
      //           ],
      //         },
      //       },
      //     },
      //   ],
      // },
    // },
  ],
};

local build_and_upload_guest_agent = build_guest_agent {
  local tl = self,

  package:: error 'must set package in build_and_upload_guest_agent',

  uploads: [
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
      gcs_files: '"gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el10.x86_64.rpm","gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version))-g1.el10.aarch64.rpm"' % [tl.package, tl.package],
      os_type: 'EL10_YUM',
      pkg_inside_name: 'google-guest-agent',
      pkg_name: 'guest-agent',
      pkg_version: '((.:package-version))',
      reponame: 'google-guest-agent-el10',
      sbom_file: 'gs://gcp-guest-package-uploads/%s/google-guest-agent-((.:package-version)).sbom.json' % [tl.package],
    },
  ],
};

local build_and_upload_oslogin = buildpackagejob {
  local tl = self,
  package:: error 'must set package in build_and_upload_oslogin',
  gcs_dir:: error 'must set gcs_dir in build_and_upload_oslogin',
  builds: ['deb13', 'deb13-arm64', 'el10', 'el10-arm64'],
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
    // {
      // in_parallel: {
      //   fail_fast: true,
      //   steps: [
          // buildpackageimagetask {
          //   image_name: 'debian-13',
          //   source_image: 'projects/bct-prod-images/global/images/family/debian-13',
          //   dest_image: 'debian-13-((.:build-id))',
          //   gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb13_amd64.deb',
          //   worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker',
          // },
          // buildpackageimagetask {
          //   image_name: 'debian-13-arm64',
          //   source_image: 'projects/bct-prod-images/global/images/family/debian-13-arm64',
          //   dest_image: 'debian-13-arm64-((.:build-id))',
          //   gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb13_arm64.deb',
          //   machine_type: 'c4a-standard-2',
          //   disk_type: 'hyperdisk-balanced',
          //   worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          // }
          // buildpackageimagetask {
          //   image_name: 'centos-stream-10',
          //   source_image: 'projects/bct-prod-images/global/images/family/centos-stream-10',
          //   dest_image: 'centos-stream-10-((.:build-id))',
          //   gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el10.x86_64.rpm',
          //   worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker',
          // },
          // buildpackageimagetask {
          //   image_name: 'centos-stream-10-arm64',
          //   source_image: 'projects/bct-prod-images/global/images/family/centos-stream-10-arm64',
          //   dest_image: 'centos-stream-10-arm64-((.:build-id))',
          //   gcs_package_path: 'gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin-((.:package-version))-g1.el10.aarch64.rpm',
          //   machine_type: 'c4a-standard-2',
          //   disk_type: 'hyperdisk-balanced',
          //   worker_image: 'projects/compute-image-tools/global/images/family/debian-12-worker-arm64',
          // },
      //   ],
      // },
    // },
    // {
    //   in_parallel: {
    //     fail_fast: true,
    //     steps: [
    //       {
    //         task: 'oslogin-image-tests-amd64',
    //         config: {
    //           platform: 'linux',
    //           image_resource: {
    //             type: 'registry-image',
    //             source: { repository: 'gcr.io/compute-image-tools/cloud-image-tests' },
    //           },
    //           run: {
    //             path: '/manager',
    //             args: [
    //               '-project=guest-package-builder',
    //               '-zone=us-central1-a',
    //               '-test_projects=oslogin-cit',
    //               '-parallel_count=2',
    //               '-images=projects/guest-package-builder/global/images/debian-13-((.:build-id))',
    //               '-filter=oslogin',
    //             ],
    //           },
    //         },
    //       },
    //       {
    //         task: 'oslogin-image-tests-arm64',
    //         config: {
    //           platform: 'linux',
    //           image_resource: {
    //             type: 'registry-image',
    //             source: { repository: 'gcr.io/compute-image-tools/cloud-image-tests' },
    //           },
    //           inputs: [{ name: 'guest-test-infra' }],
    //           run: {
    //             path: '/manager',
    //             args: [
    //               '-project=guest-package-builder',
    //               '-zone=us-central1-a',
    //               '-test_projects=oslogin-cit',
    //               '-images=projects/guest-package-builder/global/images/debian-13-arm64-((.:build-id))',
    //               '-parallel_count=2',
    //               '-filter=oslogin',
    //             ],
    //           },
    //         },
    //       },
    //     ],
    //   },
    // },
  ],
  uploads: [
    uploadpackageversiontask {
      gcs_files: '"gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb13_amd64.deb","gs://gcp-guest-package-uploads/oslogin/google-compute-engine-oslogin_((.:package-version))-g1+deb13_arm64.deb"',
      os_type: 'TRIXIE_APT',
      pkg_inside_name: 'google-compute-engine-oslogin',
      pkg_name: 'guest-oslogin',
      pkg_version: '((.:package-version))',
      reponame: 'gce-google-compute-engine-oslogin-trixie',
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
      package: 'artifact-registry-yum-plugin',
      builds: ['el10', 'el10-arm64'],
      gcs_dir: 'yum-plugin-artifact-registry',
      uploads: [
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
      builds: ['deb13', 'deb13-arm64'],
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
    build_goo {
      name: 'build-googet',
      spec_name: 'googet',
      package: 'compute-image-windows',
      builds: ['goo'],
      extra_repo: 'googet',
      extra_repo_owner: 'google',
      secret_name: 'googet',
      test_suite: 'packagevalidation',
      abbr_name: 'ciw-googet',
      uploads: [
      ],
    },
    build_goo {
      name: 'build-certgen',
      spec_name: 'certgen',
      package: 'compute-image-windows',
      builds: ['goo'],
      secret_name: 'certgen',
      test_suite: 'ssh|winrm',
      abbr_name: 'ciw-certgen',
      uploads: [
      ],
    },
    build_goo {
      name: 'build-google-compute-engine-auto-updater',
      spec_name: 'google-compute-engine-auto-updater',
      package: 'compute-image-windows',
      builds: ['goo'],
      abbr_name: 'ciw-gce-auto-updater',
      uploads: [
      ],
    },
    build_goo {
      name: 'build-google-compute-engine-powershell',
      spec_name: 'google-compute-engine-powershell',
      package: 'compute-image-windows',
      builds: ['goo'],
      abbr_name: 'ciw-gce-powershell',
      uploads: [
      ],
    },
    build_goo {
      name: 'build-google-compute-engine-ssh',
      spec_name: 'google-compute-engine-ssh',
      package: 'compute-image-windows',
      builds: ['goo'],
      abbr_name: 'ciw-gce-ssh',
      uploads: [
      ],
    },
    build_goo {
      name: 'build-google-compute-engine-sysprep',
      spec_name: 'google-compute-engine-sysprep',
      package: 'compute-image-windows',
      builds: ['goo'],
      abbr_name: 'ciw-gce-sysprep',
      uploads: [
      ],
    },
    buildpackagejob {
      package: 'compute-image-tools',
      builds: ['goo'],
      name: 'build-diagnostics',
      uploads: [
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
      name: 'googet',
      type: 'git',
      source: {
        uri: 'https://github.com/google/googet.git',
        branch: 'master',
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
       // 'build-compute-image-windows',
        'build-googet',
        'build-certgen',
        'build-google-compute-engine-auto-updater',
        'build-google-compute-engine-powershell',
        'build-google-compute-engine-ssh',
        'build-google-compute-engine-sysprep',
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
