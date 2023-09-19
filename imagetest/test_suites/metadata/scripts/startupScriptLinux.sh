#!/bin/bash

data="startup_success"
curl -X PUT http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/testing/result -H "Metadata-Flavor: Google" -d $data
