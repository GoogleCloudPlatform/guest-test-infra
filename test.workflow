{
  "Name": "network",
  "Project": "hanga-testing",
  "Zone": "us-west1-c",
  "GCSPath": "gs:///2021-07-14T18:27:06Z/network/debian-10",
  "Sources": {
    "testpackage": "/network.test",
    "wrapper": "/wrapper"
  },
  "Steps": {
    "create-disks": {
      "CreateDisks": [
        {
          "name": "vm",
          "sourceImage": "projects/debian-cloud/global/images/family/debian-10"
        },
        {
          "name": "vm2",
          "sourceImage": "projects/debian-cloud/global/images/family/debian-10"
        }
      ]
    },
    "create-networks": {
      "CreateNetworks": [
        {
          "name": "test-net",
          "autoCreateSubnetworks": false
        }
      ]
    },
    "create-vms": {
      "CreateInstances": {
        "Instances": [
          {
            "Scopes": [
              "https://www.googleapis.com/auth/devstorage.read_write"
            ],
            "StartupScript": "wrapper",
            "disks": [
              {
                "source": "vm"
              }
            ],
            "name": "vm",
            "metadata": {
              "_test_package_url": "${SOURCESPATH}/testpackage",
              "_test_results_url": "${OUTSPATH}/vm.txt",
              "_test_run": "TestDefaultMTU",
              "_test_vmname": "vm"
            }
          },
          {
            "Scopes": [
              "https://www.googleapis.com/auth/devstorage.read_write"
            ],
            "StartupScript": "wrapper",
            "disks": [
              {
                "source": "vm2"
              }
            ],
            "name": "vm2",
            "networkInterfaces": [
              {
                "accessConfigs": [
                  {
                    "type": "ONE_TO_ONE_NAT"
                  }
                ],
                "aliasIpRanges": [
                  {
                    "ipCidrRange": "10.14.1.0/24",
                    "subnetworkRangeName": "secondary-range"
                  }
                ],
                "network": "test-net"
              }
            ],
            "metadata": {
              "_test_package_url": "${SOURCESPATH}/testpackage",
              "_test_results_url": "${OUTSPATH}/vm2.txt",
              "_test_run": "TestAliasAfterOnBoot|TestAliasAfterReboot|TestAliasAgentRestart",
              "_test_vmname": "vm2"
            }
          }
        ]
      }
    }
  },
  "Dependencies": {
    "create-vms": [
      "create-disks",
      "create-networks"
    ]
  },
  "DefaultTimeout": "10m",
  "ForceCleanupOnError": false
}

