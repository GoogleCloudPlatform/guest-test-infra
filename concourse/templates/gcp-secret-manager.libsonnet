{
  local tl = self,

  get_secret:: {
    local task = self,

    secret_name:: error 'must set secret_name in gcp-secret-manager template',
    project:: 'gcp-guest',
    version:: 'latest',

    platform: 'linux',
    image_resource: {
      type: 'docker-image',
      source: {
        repository: 'google/cloud-sdk',
        tag: 'alpine',
      },
    },
    inputs: [
      { name: 'credentials' },
    ],
    outputs: [
      { name: 'gcp-secret-manager' },
    ],
    run: {
      path: 'sh',
      args: [
        '-exc',
        // Note: the following is a single string.
        'gcloud auth activate-service-account --key-file=$PWD/credentials/credentials.json;' +
        'gcloud secrets versions access ' + task.version + ' --secret=' + task.secret_name +
        ' --project=' + task.project + ' > gcp-secret-manager/' + task.secret_name,
      ],
    },
  },
}
