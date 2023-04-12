{
  local tl = self,

  prod_topic:: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
  prod_bucket:: 'artifact-releaser-prod-rtp',
  sbom_bucket:: 'gce-image-sboms/sboms',
  debian_image_prefixes:: {
    'debian-9': 'debian-9-stretch',
    'debian-10': 'debian-10-buster',
    'debian-11': 'debian-11-bullseye',
    'debian-11-arm64': 'debian-11-bullseye-arm64',
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
      '%s/%s-v([0-9]+)-([0-9]+).sbom.json' % [self.sbom_destination, self.image]
    else
      error 'must set regexp or sbom_destination in gcssbomresource',

    sbom_destination:: '',
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
}
