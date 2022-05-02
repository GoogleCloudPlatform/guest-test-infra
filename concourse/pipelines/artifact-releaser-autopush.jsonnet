local imgpublishjob = {
  local job = self,

  image:: error 'must set image in imgpublishjob',
  env:: error 'must set publish env in imgpublishjob',
  workflow:: error 'must set workflow in imgpublishjob',
  gcs:: error 'must set gcs in imgpublishjob',
  source_version:: error 'must set source_version in imgpublishjob',

  // Start of job.
  name: 'publish-to-%s-%s' % [job.env, job.image],
  plan: [
    { get: 'guest-test-infra' },
    { get: 'compute-image-tools' },
    {
      task: 'generate-version',
      file: 'guest-test-infra/concourse/tasks/generate-version.yaml',
    },
    {
      load_var: 'publish-version',
      file: 'publish-version/version',
    },
    {
      task: 'arle-publish-' + job.image,
      config: arle.arlepublishtask {
        gcs_image_path: job.gcs,
        source_version: job.source_version,
        publish_version: '((.:publish-version))',
        wf: job.workflow,
        image_name: job.image,
        topic: 'projects/artifact-releaser-autopush/topics/gcp-guest-image-release-autopush',
        type: 'insertImage',
      },
    },
  ],
};

local pkgpublishjob = {
  local job = self,

  package:: error 'must set package in pkgpublishjob',
  gcs_path:: error 'must set gcs_path in pkgpublishjob',
  universe:: error 'must set universe in pkgpublishjob',
  repo:: error 'must set repo in pkgpublishjob',
  type:: common.package_task,

  // Start of job.
  name: 'publish-' + job.package,
  plan: [
    { get: 'guest-test-infra' },
    {
      task: 'arle-publish-' + job.package,
      config: arle.arlepackageoperation {
        topic: 'projects/artifact-releaser-autopush/topics/gcp-guest-package-upload-autopush',
        type: job.type,
        object: job.gcs_path,
        universe: job.universe,
        repo: job.repo,
      },
    },
  ],
}

{
  resources: [
    common.gitresource { name: 'compute-image-tools' },
    common.gitresource { name: 'guest-test-infra' },
  ],
  jobs: [
    imgpublishjob {
      image: 'debian_10',
      workflow: 'debian/debian_10.publish.json',
      gcs: 'gs://artifact-releaser-autopush-rtp/debian',
      source_version: 'v20211027',
      env: 'autopush',
    },
    imgpublishjob {
      image: 'almalinux_8',
      workflow: 'enterprise_linux/almalinux_8.publish.json',
      gcs: 'gs://artifact-releaser-autopush-rtp/almalinux',
      source_version: 'v20211027',
      env: 'autopush',
    },
    pkgpublishjob {
      package: 'guestagent-deb11',
      type: 'uploadToStaging',
      repo: 'guest-arle-autopush-trusty',
      universe: 'cloud-apt',
      gcs_path: 'artifact-releaser/google-guest-agent_20211019.00-g1_amd64.deb',
    },
    pkgpublishjob {
      package: 'guestagent-el7',
      type: 'uploadToStaging',
      repo: 'guest-arle-autopush-el7-x86_64',
      universe: 'cloud-yum',
      gcs_path: 'artifact-releaser/google-guest-agent-20211019.00-g1.el8.x86_64.rpm',
    },
    pkgpublishjob {
      package: 'guestagent-win',
      type: 'uploadToStaging',
      repo: 'guest-arle-autopush',
      universe: 'cloud-yuck',
      gcs_path: 'artifact-releaser/google-compute-engine-windows.x86_64.20211019.00.0@1.goo',
    },
    pkgpublishjob {
      package: 'guestagent-el7-prod',
      type: 'insertPackage',
      repo: 'guest-arle-autopush-el7-x86_64',
      universe: 'cloud-yum',
      gcs_path: 'artifact-releaser/google-guest-agent-20211019.00-g1.el8.x86_64.rpm',
    },
    {
      name: 'test-workload-identity',
      plan: [
        task: 'show-workload-identity',
        config: {
          platform: 'linux',
          image_resource: {
            type: 'registry-image',
            source: { repository: 'google/cloud-sdk', tag: 'alpine' },
          },
          run: {
            path: 'sh',
            args: [
              '-exc',
              'curl -H Metadata-Flavor:Google http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/email',
            ],
          },
        },
      ],
    },
  ],
}