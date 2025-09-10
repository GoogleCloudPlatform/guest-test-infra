{
  local tl = self,

  daisytask:: {
    local task = self,
    local expanded_vars = [
      '-var:' + var
      for var in self.vars
    ],

    project:: 'gce-image-builder',
    zone:: null,
    zones:: null,
    vars:: [],
    workflow:: error 'must set workflow in daisy template',
    workflow_prefix:: 'compute-image-tools/daisy_workflows/',

    // Start of output.
    platform: 'linux',
    image_resource: {
      type: 'registry-image',
      source: {
        repository: 'gcr.io/compute-image-tools/daisy',
        tag: 'latest',
      },
    },
    inputs: [{ name: 'compute-image-tools' }],
    run: if task.zone != null then {
      path: '/daisy',
      args:
        [
          '-compute_endpoint_override=https://www.googleapis.com/compute/beta/projects/',
          '-project=' + task.project,
          '-zone=' + task.zone,
        ] +
        expanded_vars +
        [task.workflow_prefix + task.workflow],
    } else if task.zones != null then {
      path: 'sh',
      args: [
        '-ec',
        local zones_list = std.join(' ', task.zones);
        'zones=(%s)\n' % zones_list +
        'zone=${zones[ $RANDOM % ${#zones[@]} ]}\n' +
        'echo "Executing daisy in zone: $zone"\n' +
        '/daisy -compute_endpoint_override=https://www.googleapis.com/compute/beta/projects/ ' +
        '-project=%s ' % task.project +
        '-zone=$zone ' +
        std.join(' ', expanded_vars) +
        ' ' +
        task.workflow_prefix + task.workflow,
      ],
    } else {
      path: '/daisy',
      args:
        [
          '-compute_endpoint_override=https://www.googleapis.com/compute/beta/projects/',
          '-project=' + task.project,
          '-zone=us-central1-a',
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
    sbom_destination:: '.',
    shasum_destination:: '.',

    workflow_prefix+: 'build-publish/',
    vars+: [
      // Always reference workflow assets from Concourse input rather than container copy.
      // This is interpreted by Daisy relative to the workflow location, so two directories up is e.g. out of
      // enterprise_linux and then out of build-publish, ending in daisy_workflows
      'workflow_root=../../',
      'gcs_url=' + task.gcs_url,
      'sbom_destination=' + task.sbom_destination,
      'sha256_txt=' + task.shasum_destination
    ] + if self.build_date == '' then
      []
    else
      ['build_date=' + task.build_date],
  },

  daisywindowsinstallmediatask:: tl.daisytask {
    local task = self,

    // Add additional overrideable attrs.
    gcs_url:: error 'must set gcs_url in daisy image task',
    iso_path_2025:: error 'must set iso_path_2025 in daisy image task',
    iso_path_2022:: error 'must set iso_path_2022 in daisy image task',
    iso_path_2019:: error 'must set iso_path_2019 in daisy image task',
    iso_path_2016:: error 'must set iso_path_2016 in daisy image task',
    iso_path_2012r2:: error 'must set iso_path_2012r2 in daisy image task',
    updates_path_2025:: error 'must set updates_path_2025 in daisy image task',
    updates_path_2022:: error 'must set updates_path_2022 in daisy image task',
    updates_path_2019:: error 'must set updates_path_2019 in daisy image task',
    updates_path_2016:: error 'must set updates_path_2016 in daisy image task',
    updates_path_2012r2:: error 'must set updates_path_2012r2 in daisy image task',

    workflow_prefix+: 'build-publish/',
    vars+: [
      // Always reference workflow assets from Concourse input rather than container copy.
      // This is interpreted by Daisy relative to the workflow location, so two directories up is e.g. out of
      // enterprise_linux and then out of build-publish, ending in daisy_workflows
      'workflow_root=../../',
      'gcs_url=' + task.gcs_url,
      'iso_path_2025=' + task.iso_path_2025,
      'iso_path_2022=' + task.iso_path_2022,
      'iso_path_2019=' + task.iso_path_2019,
      'iso_path_2016=' + task.iso_path_2016,
      'iso_path_2012r2=' + task.iso_path_2012r2,
      'updates_path_2025=' + task.updates_path_2025,
      'updates_path_2022=' + task.updates_path_2022,
      'updates_path_2019=' + task.updates_path_2019,
      'updates_path_2016=' + task.updates_path_2016,
      'updates_path_2012r2=' + task.updates_path_2012r2,
    ],
  },
}
