{
  local tl = self,

  getsecrettask:: {
    local task = self,

    secret_name:: error 'must set secret_name in gcp-secret-manager template',
    output_path:: self.secret_name,
    project:: 'gcp-guest',
    version:: 'latest',

    platform: 'linux',
    image_resource: {
      type: 'registry-image',
      source: {
        repository: 'google/cloud-sdk',
        tag: 'alpine',
      },
    },
    outputs: [{ name: 'gcp-secret-manager' }],
    run: {
      path: 'sh',
      args: [
        '-exc',
        'dir=$(dirname ./gcp-secret-manager/%s);' % task.output_path +
        'mkdir -p "$dir";' +
        'gcloud secrets versions access %s --secret=%s' % [task.version, task.secret_name] +
        ' --project=%s > gcp-secret-manager/%s' % [task.project, task.output_path],
      ],
    },
  },
}
