{
  local tl = self,

  daisytask:: {
    local task = self,
    local expanded_vars = [
      '-var:' + var
      for var in self.vars
    ],

    project:: 'gce-image-builder',
    zone:: 'us-central1-c',
    vars:: [],
    workflow:: error 'must set workflow in daisy template',
    workflow_prefix:: 'compute-image-tools/daisy_workflows/',

    // Start of output.
    platform: 'linux',
    image_resource: {
      type: 'docker-image',
      source: {
        repository: 'gcr.io/compute-image-tools/daisy',
        tag: 'latest',
      },
    },
    inputs: [
      // Currently all our daisy workflows are in this repo. No need to make this overrideable because
      // Concourse has 'input_mapping' to do that if needed.
      { name: 'compute-image-tools' },
      { name: 'credentials' },
    ],
    params: {
      GOOGLE_APPLICATION_CREDENTIALS: 'credentials/credentials.json',
    },
    run: {
      path: '/daisy',
      args:
        [
          '-compute_endpoint_override=https://www.googleapis.com/compute/beta/projects/',
          '-project=' + task.project,
          '-zone=' + task.zone,
        ] +
        expanded_vars +
        [task.workflow_prefix + task.workflow],
    },
  },

  daisyimagetask:: tl.daisytask {
    local task = self,

    // Add additional overrideable attrs.
    build_date:: '',
    gcs_url:: error 'must set gcs_url in daisy image task',

    workflow_prefix+: 'build-publish/',
    vars+: [
      // Always reference workflow assets from Concourse input rather than container copy.
      // This is interpreted by Daisy relative to the workflow location, so two directories up is e.g. out of
      // enterprise_linux and then out of build-publish, ending in daisy_workflows
      'workflow_root=../../',
      'gcs_url=' + task.gcs_url,
    ] + if self.build_date == '' then
      []
    else
      [
        'build_date=' + task.build_date,
      ],
  },
}
