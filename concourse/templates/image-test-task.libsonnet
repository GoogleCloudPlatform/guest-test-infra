{
  local tl = self,

  imagetesttask:: {
    local task = self,

    images:: error 'must set images in imagetesttask',
    extra_args:: [],

    // Start of task
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
        '-exclude=oslogin',
        '-images=' + task.images,
      ] + task.extra_args,
    },
  }
}