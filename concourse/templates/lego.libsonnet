{
  local tl = self,

  legotask:: {
    local task = self,
    local expanded_domains = [
      '--domains=' + domain
      for domain in self.domains
    ],

    email:: error 'must set email in legotask template',
    acme_server:: error 'must set acme_server in legotask template',
    domains:: error 'must set domains in legotask template',
    project:: '',
    input:: 'tls',

    platform: 'linux',
    image_resource: {
      type: 'docker-image',
      source: { repository: 'goacme/lego' },
    },
    params: {
      // Set optional environment variable.
      [if task.project != '' then 'GCE_PROJECT']: task.project,
    },
    inputs: [
      // The input must already have the correct file layout to reuse the input key.
      { name: task.input },
    ],
    outputs: self.inputs,
    run: {
      path: 'lego',
      args: expanded_domains + [
        '--email=' + task.email,
        '--server=https://' + task.acme_server + '/directory',
        '--accept-tos',
        '--eab',
        '--dns.resolvers=ns-cloud-b2.googledomains.com:53',
        '--dns=gcloud',
        '--path=./' + task.input + '/',
        'run',
      ],
    },
  },
}
