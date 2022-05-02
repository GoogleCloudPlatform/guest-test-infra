local common = import '../templates/common.libsonnet';

{
  gcepublishtask:: {
    local task = self,

    environment:: error 'must set environment in gcepublishtask',
    publish_version:: error 'must set publish_version in gcepublishtask',
    source_gcs_path:: error 'must set source_gcs_path in gcepublishtask',
    source_version:: error 'must set source_version in gcepublishtask',
    wf:: error 'must set wf in gcepublishtask',

    platform: 'linux',
    image_resource: {
      type: 'registry-image',
      source: { repository: 'gcr.io/compute-image-tools/gce_image_publish' },
    },
    inputs: [
      { name: 'compute-image-tools' },
    ],
    run: {
      path: '/gce_image_publish',
      args: [
        '-rollout_rate=0',
        '-skip_confirmation',
        '-replace',
        '-no_root',
        '-source_gcs_path=' + task.source_gcs_path,
        '-source_version=' + task.source_version,
        '-publish_version=' + task.publish_version,
        '-var:environment=' + task.environment,
        './compute-image-tools/daisy_workflows/build-publish/' + task.wf,
      ],
    },
  },

  arlepublishtask::
    {
      local task = self,

      topic:: common.prod_topic,
      type:: common.image_task,
      image_name:: error 'must set image_name in arlepublishtask',
      gcs_image_path:: error 'must set gcs_image_path in arlepublishtask',
      wf:: error 'must set wf in arlepublishtask',
      source_version:: error 'must set source_version in arlepublishtask',
      publish_version:: error 'must set publish_version in arlepublishtask',

      platform: 'linux',
      image_resource: {
        type: 'registry-image',
        source: { repository: 'google/cloud-sdk', tag: 'alpine' },
      },
      inputs: [{ name: 'compute-image-tools' }],
      run: {
        path: 'sh',
        args: [
          '-exc',
          "wf=$(sed 's/\\\"/\\\\\"/g' ./compute-image-tools/daisy_workflows/build-publish/%s | tr -d '\\n')\n" % task.wf +
          'gcloud pubsub topics publish "%s" --message "{\\"type\\": \\"%s\\", \\"request\\":\n{\\"image_name\\": \\"%s\\", \\"gcs_image_path\\": \\"%s\\", \\"image_publish_template\\": \\"${wf}\\",\n      \\"source_version\\": \\"%s\\", \\"publish_version\\": \\"%s\\", \\"release_notes\\": \\"\\"}}"\n' %
          [task.topic, task.type, task.image_name, task.gcs_image_path, task.source_version, task.publish_version],
        ],
      },
    },

  arlepackageoperation::
    {
      local task = self,

      topic:: common.prod_package_topic,
      type:: common.package_task,
      object:: error 'must set object in arlepackageoperation',
      universe:: error 'must set universe in arlepackageoperation',
      repo:: error 'must set repo in arlepackageoperation',

      platform: 'linux',
      image_resource: {
        type: 'registry-image',
        source: { repository: 'google/cloud-sdk', tag: 'alpine' },
      },
      run: {
        path: 'sh',
        args: [
          '-exc',
          'gcloud pubsub topics publish "%s" --message "{\"type\": \"%s\", \"request\": {\"bucket\": \"gcp-guest-package-uploads\", \"object\": \"%s\", \"universe\": \"%s\", \"repo\": \"%s\"}}"' %
          [task.topic, task.type, task.object, task.universe, task.repo],
        ],
      },
    },
}
