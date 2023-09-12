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
      '-test_projects=compute-image-test-pool-005',
      '-images=' + task.images,
    ] + task.extra_args,
  },
};

local imagetestjob = {
  local tl = self,

  image:: error 'must provide image in imagetesttestjob',

  // Start of output.
  name: tl.image,
  // Only allow one build with the shapevalidation tag to run at once
  serial_groups: ['shapevalidation'],
  plan: [
    {
        task: 'image-test-' + tl.image,
        config: imagetesttask {
          images: 'projects/bct-prod-images/global/images/family/%s' % tl.image,
          // Special case ARM for now.
          extra_args: if
            std.endsWith(tl.image, '-arm64')
          then
            ['-machine_type=t2a-standard-2']
          else [],
        },
        attempts: 1,
    },
  ],
};

local imagetestgroup = {
  local tl = self,

  images:: error 'must set images in imagetestgroup',

  name: error 'must set name in imagetestgroup',
  jobs: [
    image
    for image in tl.images
  ],
};

{
  local almalinux_images = ['almalinux-8', 'almalinux-9'],
  local debian_images = ['debian-10', 'debian-11', 'debian-11-arm64', 'debian-12', 'debian-12-arm64'],
  local centos_images = ['centos-7', 'centos-stream-8', 'centos-stream-9'],
  local rhel_sap_images = [
    'rhel-7-9-sap',
    'rhel-8-1-sap',
    'rhel-8-2-sap',
    'rhel-8-4-sap',
    'rhel-8-6-sap',
    'rhel-8-8-sap',
    'rhel-9-0-sap',
    'rhel-9-2-sap',
  ],
  local rhel_images = rhel_sap_images + [
    'rhel-7',
    'rhel-7-byos',
    'rhel-8',
    'rhel-8-byos',
    'rhel-9',
    'rhel-9-arm64',
    'rhel-9-byos',
  ],
  local rocky_linux_images = [
    'rocky-linux-8',
    'rocky-linux-8-optimized-gcp',
    'rocky-linux-8-optimized-gcp-arm64',
    'rocky-linux-9',
    'rocky-linux-9-arm64',
    'rocky-linux-9-optimized-gcp',
    'rocky-linux-9-optimized-gcp-arm64',
  ],
  local windows_server_images = [
    'windows-10-1709-x64',
    'windows-10-1709-x86',
    'windows-10-1803-x64',
    'windows-10-1803-x86',
    'windows-10-1809-x64',
    'windows-10-1809-x86',
    'windows-10-1903-x64',
    'windows-10-1903-x86',
    'windows-10-1909-x64',
    'windows-10-1909-x86',
    'windows-10-2004-x64',
    'windows-10-2004-x86',
    'windows-10-20h2-x64',
    'windows-10-20h2-x86',
    'windows-10-21h2-x64',
    'windows-11-21h2-x64',
    'windows-11-22h2-x64',
    'windows-1803-core',
    'windows-1909-core',
    'windows-1909-core-for-containers',
    'windows-2003-64',
    'windows-2004-core',
    'windows-2004-core-for-containers',
    'windows-2008',
    'windows-2008-r2',
    'windows-2008-r2-stress',
    'windows-2012',
    'windows-2012-r2',
    'windows-2012-r2-core',
    'windows-2012-r2-core-internal',
    'windows-2012-r2-core-nonvme',
    'windows-2012-r2-core-standard',
    'windows-2012-r2-internal',
    'windows-2012-r2-nonvme',
    'windows-2012-r2-standard',
    'windows-2012-r2-stress',
    'windows-2016',
    'windows-2016-core',
    'windows-2016-core-internal',
    'windows-2016-core-standard',
    'windows-2016-drivers-dev',
    'windows-2016-nonvme',
    'windows-2016-standard',
    'windows-2016-stress',
    'windows-2019',
    'windows-2019-core',
    'windows-2019-core-for-containers',
    'windows-2019-core-standard',
    'windows-2019-for-containers',
    'windows-2019-for-containers-ce',
    'windows-2019-nonvme',
    'windows-2019-standard',
    'windows-2019-standard-core',
    'windows-2019-standard-core-for-containers',
    'windows-2019-standard-for-containers',
    'windows-2022',
    'windows-2022-core',
    'windows-2022-core-standard',
    'windows-2022-nonvme',
    'windows-2022-standard',
    'windows-20h2-core',
    'windows-7-x64',
    'windows-7-x86',
    'windows-8-1-x64',
    'windows-8-1-x86',
    'windows-insider',
    'windows-install-media',
  ],
  local sql_server_images = [
    'sql-ent-2012-win-2012-r2',
    'sql-ent-2014-win-2012-r2',
    'sql-ent-2014-win-2012-r2-standard',
    'sql-ent-2014-win-2016',
    'sql-ent-2014-win-2016-standard',
    'sql-ent-2016-win-2012-r2',
    'sql-ent-2016-win-2016',
    'sql-ent-2016-win-2016-standard',
    'sql-ent-2016-win-2019',
    'sql-ent-2016-win-2019-standard',
    'sql-ent-2017-win-2016',
    'sql-ent-2017-win-2016-standard',
    'sql-ent-2017-win-2019',
    'sql-ent-2017-win-2019-standard',
    'sql-ent-2017-win-2022',
    'sql-ent-2019-win-2019',
    'sql-ent-2019-win-2022',
    'sql-ent-2019-win-2022-standard',
    'sql-ent-2022-win-2019',
    'sql-ent-2022-win-2022',
    'sql-ent-2022-win-2022-standard',
    'sql-exp-2017-win-2012-r2',
    'sql-exp-2017-win-2012-r2-standard',
    'sql-exp-2017-win-2016',
    'sql-exp-2017-win-2016-standard',
    'sql-exp-2017-win-2019',
    'sql-std-2012-win-2012-r2',
    'sql-std-2014-win-2012-r2',
    'sql-std-2014-win-2012-r2-standard',
    'sql-std-2016-win-2012-r2',
    'sql-std-2016-win-2012-r2-standard',
    'sql-std-2016-win-2016',
    'sql-std-2016-win-2016-standard',
    'sql-std-2016-win-2019',
    'sql-std-2016-win-2019-standard',
    'sql-std-2017-win-2016',
    'sql-std-2017-win-2019',
    'sql-std-2017-win-2019-standard',
    'sql-std-2017-win-2022',
    'sql-std-2017-win-2022-standard',
    'sql-std-2019-win-2019',
    'sql-std-2019-win-2022',
    'sql-std-2022-win-2019',
    'sql-std-2022-win-2022',
    'sql-web-2012-win-2012-r2',
    'sql-web-2014-win-2012-r2',
    'sql-web-2014-win-2012-r2-standard',
    'sql-web-2016-win-2012-r2',
    'sql-web-2016-win-2012-r2-standard',
    'sql-web-2016-win-2016',
    'sql-web-2016-win-2016-standard',
    'sql-web-2016-win-2019',
    'sql-web-2016-win-2019-standard',
    'sql-web-2017-win-2016',
    'sql-web-2017-win-2016-standard',
    'sql-web-2017-win-2019',
    'sql-web-2017-win-2019-standard',
    'sql-web-2017-win-2022',
    'sql-web-2017-win-2022-standard',
    'sql-web-2019-win-2019',
    'sql-web-2019-win-2019-standard',
    'sql-web-2019-win-2022',
    'sql-web-2022-win-2019',
    'sql-web-2022-win-2022',
  ],
  jobs : [
    imagetestjob {
      image: family
    },
    for family in almalinux_images
  ] + [
    imagetestjob {
      image: family
    },
    for family in debian_images
  ] + [
    imagetestjob {
      image: family
    },
    for family in centos_images
  ] + [
    imagetestjob {
      image: family
    },
    for family in rhel_images
  ] + [
    imagetestjob {
      image: family
    },
    for family in rocky_linux_images
  ] + [
    imagetestjob {
      image: family
    },
    for family in windows_server_images
  ] + [
    imagetestjob {
      image: family
    },
    for family in sql_server_images
  ],
  groups: [
    {
      name: 'alma-linux',
      jobs :  [
        family
        for family in almalinux_images
      ],
    },
    {
      name: 'debian',
      jobs :  [
        family
        for family in debian_images
      ],
    },
    {
      name: 'centos',
      jobs :  [
        family
        for family in centos_images
      ],
    },
    {
      name: 'rhel',
      jobs :  [
        family
        for family in rhel_images
      ],
    },
    {
      name: 'rocky-linux',
      jobs :  [
        family
        for family in rocky_linux_images
      ],
    },
    {
      name: 'windows server',
      jobs :  [
        family
        for family in windows_server_images
      ],
    },
    {
      name: 'sql server',
      jobs :  [
        family
        for family in sql_server_images
      ],
    },
  ],
}
