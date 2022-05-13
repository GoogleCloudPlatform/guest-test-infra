local project = 'google.com:rhel-infra';

// RHUA is deployed where NFS is available.
local wave1 = [
  'asia-northeast1',
  'australia-southeast2',
];
local wave2 = [
  'asia-east1',
  'australia-southeast1',
  'europe-west6',
  'us-west2',
];
local wave3 = [
  'asia-east2',
  'asia-south2',
  'europe-west2',
  'europe-west4',
  'northamerica-northeast2',
  'us-east1',
  'us-west1',
  'us-west4',
];
local wave4 = [
  'asia-northeast2',
  'asia-northeast3',
  'asia-south1',
  'asia-southeast1',
  'asia-southeast2',
  'europe-central2',
  'europe-north1',
  'europe-west1',
  'europe-west3',
  'northamerica-northeast1',
  'southamerica-east1',
  'southamerica-west1',
  'us-central1',
  'us-east4',
  'us-west3',
];

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
      trigger: true,
    },
    {
      get: 'rhua-image',
      passed: job.passed,
      trigger: true,
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
    },
    deployjob {
      name: 'deploy-staging-us-west1',
      stage: 'staging',
      region: 'us-west1',
      passed: ['manual-trigger'],
    },
  ] + [  // Wave 1
    deployjob {
      name: 'deploy-prod-' + region,
      region: region,
      passed: ['deploy-staging-us-west1'],
    }
    for region in wave1
  ] + [
    gatejob {
      name: 'gate-1',
      passed: [
        'deploy-prod-' + region
        for region in wave1
      ],
    },
  ] + [  // Wave 2
    deployjob {
      name: 'deploy-prod-' + region,
      region: region,
      passed: ['gate-1'],
    }
    for region in wave2
  ] + [
    gatejob {
      name: 'gate-2',
      passed: [
        'deploy-prod-' + region
        for region in wave2
      ],
    },
  ] + [  // Wave 3
    deployjob {
      name: 'deploy-prod-' + region,
      region: region,
      passed: ['gate-2'],
    }
    for region in wave3
  ] + [
    gatejob {
      name: 'gate-3',
      passed: [
        'deploy-prod-' + region
        for region in wave3
      ],
    },
  ] + [  // Wave 4
    deployjob {
      name: 'deploy-prod-' + region,
      region: region,
      passed: ['gate-3'],
    }
    for region in wave4
  ],
}
