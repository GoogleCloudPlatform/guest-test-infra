{
  local tl = self,

  prod_topic:: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
  prod_bucket:: 'artifact-releaser-prod-rtp',
  prod_package_bucket:: 'gcp-guest-package-uploads',
  autopush_package_bucket:: 'artifact-releaser-autopush-rtp',
  sbom_bucket:: 'gce-image-sboms',
  debian_image_prefixes:: {
    'debian-9': 'debian-9-stretch',
    'debian-10': 'debian-10-buster',
    'debian-11': 'debian-11-bullseye',
    'debian-11-arm64': 'debian-11-bullseye-arm64',
    'debian-12': 'debian-12-bookworm',
    'debian-12-arm64': 'debian-12-bookworm-arm64',
    'debian13': 'debian-13-trixie'
  },

  gitresource:: {
    local resource = self,

    org:: 'GoogleCloudPlatform',
    branch:: 'master',

    name: error 'must set name in gitresource template',
    type: 'git',
    source: {
      uri: 'https://github.com/%s/%s.git' % [resource.org, resource.name],
      branch: resource.branch,
    },
  },

  GitResource(name):: self.gitresource {
    name: name,
  },

  gcsimgresource:: {
    local resource = self,

    regexp:: if self.gcs_dir != '' then
      '%s/%s-v([0-9]+).tar.gz' % [self.gcs_dir, self.image]
    else
      error 'must set regexp or gcs_dir in gcsimgresource',

    gcs_dir:: '',
    image:: error 'must set image in gcsimgresource template',
    bucket:: tl.prod_bucket,

    name: self.image + '-gcs',
    type: 'gcs',
    source: {
      bucket: resource.bucket,
      regexp: resource.regexp,
    },
  },

  GcsImgResource(image, gcs_dir):: self.gcsimgresource {
    image: image,
    gcs_dir: gcs_dir,
  },

  gcssbomresource:: {
    local resource = self,

    regexp:: if self.sbom_destination != '' then
      'sboms/%s/%s/%s-v([0-9]+)-([0-9]+).sbom.json' % [self.sbom_destination, self.image_prefix, self.image_prefix]
    else
      error 'must set regexp or sbom_destination in gcssbomresource',

    sbom_destination:: '',
    image_prefix:: self.image,
    image:: error 'must set image in gcssbomresource template',
    bucket:: tl.sbom_bucket,

    name: self.image + '-sbom',
    type: 'gcs',
    source: {
      bucket: resource.bucket,
      regexp: resource.regexp,
    },
  },

  GcsSbomResource(image, sbom_destination):: self.gcssbomresource {
    image: image,
    sbom_destination: sbom_destination,
  },


  gcsshasumresource:: {
    local resource = self,

    regexp:: if self.shasum_destination != '' then
      'sboms/%s/%s/%s-v([0-9]+)-([0-9]+)-shasum.txt' % [self.shasum_destination, self.image_prefix, self.image_prefix]
    else
      error 'must set regexp or shasum_destination in gcsshasumresource',

    shasum_destination:: '',
    image_prefix:: self.image,
    image:: error 'must set image in gcsshasumresource template',
    bucket:: tl.sbom_bucket,

    name: self.image + '-shasum',
    type: 'gcs',
    source: {
      bucket: resource.bucket,
      regexp: resource.regexp,
    },
  },

  GcsShasumResource(image, shasum_destination):: self.gcsshasumresource {
    image: image,
    shasum_destination: shasum_destination,
  },

  default_linux_image_build_cit_filter:: '^(cvm|livemigrate|suspendresume|loadbalancer|guestagent|hostnamevalidation|imageboot|licensevalidation|network|security|hotattach|lssd|disk|packagevalidation|ssh|metadata|vmspec)$',
  default_cit_project:: 'gcp-guest',
  default_cit_test_projects: 'compute-image-test-pool-002,compute-image-test-pool-003,compute-image-test-pool-004,compute-image-test-pool-005',
  default_cit_zone:: 'us-central1-b',

  imagetesttask:: {
    local task = self,
    images:: error 'must set images in imagetesttask',
    exclude:: if task.filter == '' then error 'must set one of filter or exclude in imagetesttask' else '',
    filter:: if task.exclude == '' then error 'must set one of filter or exclude in imagetesttask' else '',
    project:: tl.default_cit_project,
    zone:: tl.default_cit_zone,
    test_projects:: tl.default_cit_test_projects,
    extra_args:: [],
    platform: 'linux',
    image_resource: {
      type: 'registry-image',
      source: { repository: 'gcr.io/compute-image-tools/cloud-image-tests' },
    },
    run: {
      path: '/manager',
      args: [
        '-project=%s' % task.project,
        '-zone=%s' % task.zone,
        '-test_projects=%s' % task.test_projects,
        '-exclude=%s' % task.exclude,
        '-filter=%s' % task.filter,
        '-images=' + task.images,
      ] + task.extra_args,
    },
  },

  publishresulttask:: {
    local task = self,

    project:: 'gcp-guest',
    zone:: 'us-central1-a',
    pipeline:: error 'must set pipeline in publishresulttask',
    job:: error 'must set job in publishresulttask',
    result_state:: error 'must set result_state in publishresulttask',
    start_timestamp:: error 'must set start_timestamp in publishresulttask',

    // Start of output.
    platform: 'linux',
    image_resource: {
      type: 'registry-image-forked',
      source: {
        repository: 'gcr.io/gcp-guest/concourse-metrics',
        tag: 'latest',
        // Use workload id to pull image
        google_auth: true,
        debug: true,
      },
    },
    run: {
      path: '/publish-job-result',
      args:
        [
          '--project-id=' + task.project,
          '--zone=' + task.zone,
          '--pipeline=' + task.pipeline,
          '--job=' + task.job,
          '--task=publish-job-result',
          '--result-state=' + task.result_state,
          '--start-timestamp=' + task.start_timestamp,
          '--metric-path=concourse/job/duration',
        ],
    },
  },
  RegistryImagePrivate:: {
    name: 'registry-image-private',
    type: 'registry-image',
    source: { repository: 'gcr.io/compute-image-tools/registry-image-forked' },
  },
}
