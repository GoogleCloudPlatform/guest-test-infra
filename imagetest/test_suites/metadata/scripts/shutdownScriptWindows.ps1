$data = "shutdown_success"
Invoke-RestMethod -Method Put -Body $data -Headers @{'Metadata-Flavor' = 'Google'} -Uri 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/testing/result' -ContentType "application/json; charset=utf-8" -UseBasicParsing
