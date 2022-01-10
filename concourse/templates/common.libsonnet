{
  local tl = self,

  prod_topic:: 'projects/artifact-releaser-prod/topics/gcp-guest-image-release-prod',
  prod_bucket:: 'artifact-releaser-prod-rtp',
  debian_image_prefixes:: {
    'debian-9': 'debian-9-stretch',
    'debian-10': 'debian-10-buster',
    'debian-11': 'debian-11-bullseye',
  },

  gitresource:: {
    local resource = self,

    org:: 'GoogleCloudPlatform',
    branch:: 'master',

    name: error 'must set name in gitresource template',
    type: 'git',
    source: {
      uri: 'https://github.com/' + resource.org + '/' + resource.name + '.git',
      branch: resource.branch,
    },
  },

  GitResource(name):: self.gitresource {
    name: name,
  },

  gcsimgresource:: {
    local resource = self,

    regexp:: self.gcs_dir + self.image + '-v([0-9]+).tar.gz',
    gcs_dir:: error 'must set gcs_dir in gcsimgresource template',
    image:: error 'must set image in gcsimgresource template',
    bucket:: tl.prod_bucket,

    name: self.image + '-gcs',
    type: 'gcs',
    source: {
      bucket: resource.bucket,
      json_key: '((gcs-key.credential))\n',
      regexp: resource.regexp,
    },
  },

  GcsImgResource(image, gcs_dir):: self.gcsimgresource {
    image: image,
    gcs_dir: gcs_dir,
  },
}
