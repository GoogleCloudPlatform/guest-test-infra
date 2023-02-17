local project = 'google.com:rhel-infra';

local gatejob = {
  local job = self,

  name: error 'must set name on gatejob',
  trigger:: true,
  passed:: [],
  plan: [
    {
      get: imageResource,
      trigger: job.trigger,
      passed: job.passed,
    }
    for imageResource in ['cds-image', 'rhua-image']
  ],
};

local gcloudmigtask = {
  local task = self,

  node:: error 'must set node on gcloudmigtask',
  stage:: 'prod',
  region:: error 'must set region on gcloudmigtask',
  action:: error 'must set action on gcloudmigtask',

  task: error 'must set task on gcloudmigtask',
  config: {
    image_resource: {
      source: {
        repository: 'google/cloud-sdk',
        tag: 'alpine',
      },
      type: 'registry-image',
    },
    platform: 'linux',
    run: {
      args: [
        'compute',
        'instance-groups',
        'managed',
      ] + task.action + [
        '--quiet',
        '%s-mig-%s-%s' % [task.node, task.stage, task.region],
        '--region=' + task.region,
        '--project=' + project,
      ],
      path: 'gcloud',
    },
  },
};

local deployjob = {
  local job = self,

  stage:: 'prod',
  passed:: [],
  region:: error 'must set region on deployjob',
  plan: [
    {
      get: 'cds-image',
      passed: job.passed,
      trigger: if job.stage == 'prod' then false else true,
    },
    {
      get: 'rhua-image',
      passed: job.passed,
      trigger: if job.stage == 'prod' then false else true,
    },
    gcloudmigtask {
      task: 'rhua-start-rolling-update',
      node: 'rhua',
      stage: job.stage,
      region: job.region,
      action: ['rolling-action', 'replace'],
    },
    gcloudmigtask {
      task: 'rhua-wait-for-version-target',
      node: 'rhua',
      stage: job.stage,
      region: job.region,
      action: ['wait-until', '--version-target-reached'],
    },
    gcloudmigtask {
      task: 'rhua-wait-for-stable',
      node: 'rhua',
      stage: job.stage,
      region: job.region,
      action: ['wait-until', '--stable'],
    },

    gcloudmigtask {
      task: 'cds-start-rolling-update',
      node: 'cds',
      stage: job.stage,
      region: job.region,
      action: ['rolling-action', 'replace'],
    },
    gcloudmigtask {
      task: 'cds-wait-for-version-target',
      node: 'cds',
      stage: job.stage,
      region: job.region,
      action: ['wait-until', '--version-target-reached'],
    },
    gcloudmigtask {
      task: 'cds-wait-for-stable',
      node: 'cds',
      stage: job.stage,
      region: job.region,
      action: ['wait-until', '--stable'],
    },
  ],
};

{
  resource_types: [
    {
      name: 'registry-image-private',
      type: 'registry-image',
      source: { repository: 'gcr.io/compute-image-tools/registry-image-forked' },
    },
    {
      name: 'gce-img',
      type: 'registry-image-private',
      source: {
        google_auth: true,
        repository: 'gcr.io/gcp-guest/gce-img-resource',
      },
    },
  ],
  resources: [
    {
      name: nodeType + '-image',
      type: 'gce-img',
      source: {
        project: project,
        family: nodeType,
      },
    }
    for nodeType in ['cds', 'rhua']
  ],
  jobs: [
    gatejob {
      name: 'manual-trigger',
      trigger: false,
    },
    deployjob {
      name: 'deploy-staging-us-west1',
      stage: 'staging',
      region: 'us-west1',
      passed: ['manual-trigger'],
    },
    deployjob {
      name: 'deploy-staging-europe-north1',
      stage: 'staging',
      region: 'europe-north1',
      passed: ['manual-trigger'],
    },
    deployjob {
      name: 'deploy-prod-us-central1',
      region: 'us-central1',
      passed: ['deploy-staging-us-west1'],
    },
  ],
}
