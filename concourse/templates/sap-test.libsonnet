{
  saptestjob:: {
    local tl = self,

    image:: error 'must be set',

    name: 'sap-workload-test-' + self.image,

    plan: [
    {
      get: tl.image + '-gcs',
      passed: [
        'publish-to-testing-' + tl.image,
      ],
      params: {
        skip_download: 'true',
      }
    },
    { get: 'guest-test-infra' },
    {
      task: 'generate-timestamp',
      file: 'guest-test-infra/concourse/tasks/generate-timestamp.yaml',
    },
    {
      load_var: 'id',
      file: 'timestamp/timestamp-ms',
    },
    {
      task: 'generate-post-script',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: {
            repository: 'google/cloud-sdk',
            tag: 'latest',
          },
        },
        inputs: [
          { name: "guest-test-infra" },
        ],
        run: {
          path: 'sh',
          args: [
            '-exc',
            |||
            cd guest-test-infra/concourse/workload-tests/SAP/
            # We want to upload this actual script with the unique id
            sed -i 's/$1/gcp-guest-test-outputs/g' sap_post_script.sh
            sed -i 's/$2/((.:id))/g' sap_post_script.sh
            gsutil cp sap_post_script.sh gs://gcp-guest-test-outputs/workload-tests/sap/((.:id))/sap_post_script.sh
|||,
          ],
        },
      },
    },
    {
      task: 'create-sap-tf-environment',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: {
            repository: 'hashicorp/terraform',
            tag: 'latest',
          },
        },
        inputs: [
          { name: "guest-test-infra" },
        ],
        outputs: [
          { name: "tf-state" },
        ],
        run: {
          path: 'sh',
          args: [
            '-exc',
            |||
            cp guest-test-infra/concourse/workload-tests/SAP/sap_hana.tf tf-state/
            cd tf-state

            terraform init
            terraform init -upgrade
            terraform apply -auto-approve \
              -var="instance_name=hana-instance-((.:id))" \
              -var="post_deployment_script=gs://gcp-guest-test-outputs/workload-tests/sap/((.:id))/sap_post_script.sh" \
|||
            + "-var='linux_image=%(image)s-ha'" % {image: tl.image},
          ]
        },
      },
    },
    {
      task: 'wait-for-and-check-post-script-results',
      timeout: '30m',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: {
            repository: 'google/cloud-sdk',
            tag: 'latest',
          },
        },
        run: {
          path: 'sh',
          args: [
            '-exc',
            |||
            until gsutil -q stat gs://gcp-guest-test-outputs/workload-tests/sap/((.:id))/run_result
            do
              echo "Waiting for results..."
              sleep 60
            done

            gsutil cat gs://gcp-guest-test-outputs/workload-tests/sap/((.:id))/run_result | grep -q "SUCCESS"
|||,
          ]
        },
      },
    },
    {
      task: 'destroy-sap-tf-environment',
      config: {
        platform: 'linux',
        image_resource: {
          type: 'registry-image',
          source: {
            repository: 'hashicorp/terraform',
            tag: 'latest',
          },
        },
        inputs: [
          { name: "tf-state" },
        ],
        run: {
          path: 'sh',
          args: [
            '-exc',
            |||
            cd tf-state
            terraform destroy -auto-approve \
              -var="instance_name=hana-instance-((.:id))"
|||,
          ]
        },
      },
    }],
  },
}
