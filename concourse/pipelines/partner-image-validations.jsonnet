local imagetesttask = {
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
};

local imagevalidationjob = {
  local tl = self,

  image:: error 'must provide image in imagevalidationjob',
  bucket:: error 'must provide bucket in imagevalidationjob',

  // Start of output.
  name: tl.image,
  plan: [
    {
      get: tl.image,
      trigger: true,
    },
    {
      task: 'create-started-json',
      input_mapping: { image: tl.image },
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: { repository: 'alpine' },
        },
        inputs: [{ name: 'image' }],
        outputs: [{ name: 'started' }],
        run: {
          path: 'sh',
          args: [
            '-exc',
            "apk add jq; timestamp=$(date +%s); image=$(cat image/name); jq --null-input --arg timestamp $timestamp --arg image $image '{timestamp: $timestamp, image: $image}' | tee started/started.json; echo $timestamp | tee started/latest-build.txt",
          ],
        },
      },
    },
    {
      task: 'transform-image-url',
      input_mapping: { image: tl.image },
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: { repository: 'busybox' },
        },
        inputs: [{ name: 'image' }],
        outputs: [{ name: 'image' }],
        run: {
          path: 'sh',
          args: [
            '-exc',
            "sed 's#.*v1/##' image/url | tee image/partial",
          ],
        },
      },
    },
    { load_var: 'startedjson', file: 'started/started.json' },
    { load_var: 'partial', file: 'image/partial' },
    {
      task: 'upload-started-json',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: {
            repository: 'google/cloud-sdk',
            tag: 'alpine',
          },
        },
        inputs: [{ name: 'started' }],
        run: {
          path: 'gsutil',
          args: [
            'cp',
            'started/started.json',
            'gs://%s/((.:startedjson.timestamp))/started.json' % tl.bucket,
          ],
        },
      },
    },
    {
      task: 'upload-latest-build-txt',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: {
            repository: 'google/cloud-sdk',
            tag: 'alpine',
          },
        },
        inputs: [{ name: 'started' }],
        run: {
          path: 'gsutil',
          args: [
            'cp',
            'started/latest-build.txt',
            'gs://%s/latest-build.txt' % tl.bucket,
          ],
        },
      },
    },
    {
      task: 'run-tests',
      on_success: {
        do: [
          {
            task: 'create-finished-json-success',
            config: {
              platform: 'linux',
              image_resource: {
                type: 'registry-image',
                source: { repository: 'alpine' },
              },
              outputs: [{ name: 'finished' }],
              run: {
                path: 'sh',
                args: [
                  '-exc',
                  "apk add jq; timestamp=$(date +%s); jq --null-input --arg timestamp $timestamp --arg result SUCCESS --argjson passed true '{timestamp: $timestamp, result: $result, passed: $passed}' | tee finished/finished.json",
                ],
              },
            },
          },
          {
            task: 'upload-finished-json-success',
            config: {
              platform: 'linux',
              image_resource: {
                type: 'registry-image',
                source: {
                  repository: 'google/cloud-sdk',
                  tag: 'alpine',
                },
              },
              inputs: [{ name: 'finished' }],
              run: {
                path: 'gsutil',
                args: [
                  'cp',
                  'finished/finished.json',
                  'gs://%s/((.:startedjson.timestamp))/finished.json' % tl.bucket,
                ],
              },
            },
          },
          {
            task: 'upload-junit-artifact',
            config: {
              platform: 'linux',
              image_resource: {
                type: 'registry-image',
                source: {
                  repository: 'google/cloud-sdk',
                  tag: 'alpine',
                },
              },
              inputs: [{ name: 'junit' }],
              run: {
                path: 'gsutil',
                args: [
                  'cp',
                  'junit/junit.xml',
                  'gs://%s/((.:startedjson.timestamp))/artifacts/junit.xml' % tl.bucket,
                ],
              },
            },
          },
        ],
      },
      on_failure: {
        do: [
          {
            task: 'create-finished-json-failure',
            config: {
              platform: 'linux',
              image_resource: {
                type: 'registry-image',
                source: { repository: 'alpine' },
              },
              outputs: [{ name: 'finished' }],
              run: {
                path: 'sh',
                args: [
                  '-exc',
                  "apk add jq; timestamp=$(date +%s); jq --null-input --arg timestamp $timestamp --arg result FAILURE --argjson passed false '{timestamp: $timestamp, result: $result, passed: $passed}' | tee finished/finished.json;",
                ],
              },
            },
          },
          {
            task: 'upload-finished-json-failure',
            config: {
              platform: 'linux',
              image_resource: {
                type: 'registry-image',
                source: {
                  repository: 'google/cloud-sdk',
                  tag: 'alpine',
                },
              },
              inputs: [{ name: 'finished' }],
              run: {
                path: 'gsutil',
                args: [
                  'cp',
                  'finished/finished.json',
                  'gs://%s/((.:startedjson.timestamp))/finished.json' % tl.bucket,
                ],
              },
            },
          },
          {
            task: 'upload-junit-artifact',
            config: {
              platform: 'linux',
              image_resource: {
                type: 'registry-image',
                source: {
                  repository: 'google/cloud-sdk',
                  tag: 'alpine',
                },
              },
              inputs: [{ name: 'junit' }],
              run: {
                path: 'gsutil',
                args: [
                  'cp',
                  'junit/junit.xml',
                  'gs://%s/((.:startedjson.timestamp))/artifacts/junit.xml' % tl.bucket,
                ],
              },
            },
          },
        ],
      },
      config: imagetesttask {
        images: '((.:partial))',
        outputs: [
          { name: 'junit', path: '.' },
        ],
      },
    },
  ],
};

local ubuntudevelimages = [
  'ubuntu-minimal-2204-lts',
  'ubuntu-1604-lts',
  'ubuntu-1804-lts',
  'ubuntu-2004-lts',
  'ubuntu-2110',
  'ubuntu-2204-lts',
  'ubuntu-minimal-1604-lts',
  'ubuntu-minimal-1804-lts',
  'ubuntu-minimal-2004-lts',
  'ubuntu-minimal-2110',
];

local ubuntuproposedimages = [
  //'ubuntu-1804-arm64-lts',
  'ubuntu-1804-lts',
  //'ubuntu-2004-arm64-lts',
  'ubuntu-2004-lts',
  'ubuntu-2110',
  'ubuntu-guest-1804-lts',
  'ubuntu-guest-2004-lts',
  'ubuntu-guest-2204-lts',
  //'ubuntu-2110-arm64',
  //'ubuntu-2204-arm64-lts',
  //'ubuntu-2204-lts',
  //'ubuntu-2210-amd64',
  //'ubuntu-2210-arm64',
  //'ubuntu-minimal-1804-arm64-lts',
  //'ubuntu-minimal-1804-lts',
  //'ubuntu-minimal-2004-arm64-lts',
  //'ubuntu-minimal-2004-lts',
  //'ubuntu-minimal-2110',
  //'ubuntu-minimal-2110-arm64',
  //'ubuntu-minimal-2204-arm64-lts',
  //'ubuntu-minimal-2204-lts',
  //'ubuntu-minimal-2210-amd64',
  //'ubuntu-minimal-2210-arm64',
  'ubuntu-pro-1604-lts',
  'ubuntu-pro-1804-lts',
  'ubuntu-pro-2004-lts',
  'ubuntu-pro-2204-lts',
  'ubuntu-pro-fips-1804-lts',
  'ubuntu-pro-fips-2004-lts',
];


// Start of output.
{
  resource_types: [
    {
      name: 'registry-image-private',
      type: 'registry-image',
      source: { repository: 'gcr.io/compute-image-tools/registry-image-forked' },
    },
    {
      name: 'gce-image',
      type: 'registry-image-private',
      source: {
        repository: 'gcr.io/gcp-guest/gce-img-resource',
        google_auth: true,
      },
    },
  ],
  resources: [
    {
      name: family + '-devel',
      type: 'gce-image',
      source: {
        project: 'ubuntu-os-cloud-devel',
        family: family,
      },
    }
    for family in ubuntudevelimages
  ] + [
    {
      name: family + '-proposed',
      type: 'gce-image',
      source: {
        project: 'ubuntu-os-cloud-image-proposed',
        family: family,
      },
    }
    for family in ubuntuproposedimages
  ] + [
    {
      name: 'sles-15',
      type: 'gce-image',
      source: {
        project: 'suse-cloud-testing',
        family: 'sles-15',
      },
    },
  ],
  jobs: [
    imagevalidationjob {
      image: family + '-devel',
      bucket: 'ubuntu-gce-validation-results',
    }
    for family in ubuntudevelimages
  ] + [
    imagevalidationjob {
      image: family + '-proposed',
      bucket: 'ubuntu-gce-validation-results',
    }
    for family in ubuntuproposedimages
  ] + [
    imagevalidationjob {
      image: 'sles-15',
      bucket: 'sles-gce-validation-results',
    },
  ],
  groups: [
    {
      name: 'ubuntu-devel',
      jobs: [
        family + '-devel'
        for family in ubuntudevelimages
      ],
    },
    {
      name: 'ubuntu-proposed',
      jobs: [
        family + '-proposed'
        for family in ubuntuproposedimages
      ],
    },
    {
      name: 'suse-cloud-testing',
      jobs: [
        'sles-15',
      ],
    },
  ],
}
