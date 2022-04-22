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

  task: error 'must set task',
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

local deployjob = gatejob {
  local job = self,

  stage:: 'prod',
  region:: error 'must set region on deployjob',
  plan: super.plan + std.flattenArrays([
    [
      gcloudmigtask {
        task: nodeType + '-start-rolling-update',
        node: nodeType,
        stage: job.stage,
        region: job.region,
        action: ['rolling-action', 'replace'],
      },
      gcloudmigtask {
        task: nodeType + '-wait-for-version-target',
        node: nodeType,
        stage: job.stage,
        region: job.region,
        action: ['wait-until', '--version-target-reached'],
      },
      gcloudmigtask {
        task: nodeType + '-wait-for-stable',
        node: nodeType,
        stage: job.stage,
        region: job.region,
        action: ['wait-until', '--stable'],
      },
    ]
    for nodeType in ['rhua', 'cds']
  ]),
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
  ] + [
    deployjob {
      name: 'deploy-prod-' + region,
      region: region,
      passed: ['deploy-staging-us-west1'],
    }
    for region in ['europe-west1', 'us-central1']
  ] + [
    gatejob {
      name: 'gate-1',
      passed: ['deploy-prod-' + region for region in ['europe-west1', 'us-central1']],
    },
    deployjob {
      name: 'deploy-prod-asia-southeast1',
      region: 'asia-southeast1',
      passed: ['gate-1'],
    },
  ],
}
