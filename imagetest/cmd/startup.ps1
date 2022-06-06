# This is temporary until the next windows build happens which will contain the new metadata-scripts package

function Get-MetadataValue {
  <#
    .SYNOPSIS
      Attempt to retrieve the value for a given metadata key.
      Returns null if not found.
    .PARAMETER $key
      The metadata key to retrieve.
    .PARAMETER $default
      The value to return if the key is not found.
    .RETURNS
      The value for the key or null.
  #>
  param (
    [parameter(Mandatory=$true)]
      [string]$key,
    [parameter(Mandatory=$false)]
      [string]$default
  )

  # Returns the provided metadata value for a given key.
  $url = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/$key"
  try {
    $client = New-Object Net.WebClient
    $client.Headers.Add('Metadata-Flavor', 'Google')
    $value = ($client.DownloadString($url)).Trim()
    Write-Host "Retrieved metadata for key $key with value $value."
    return $value
  }
  catch [System.Net.WebException] {
    if ($default) {
      Write-Host "Failed to retrieve value for $key, returning default of $default."
      return $default
    }
    else {
      Write-Host "Failed to retrieve value for $key."
      return $null
    }
  }
}

googet -noconfirm install -sources https://packages.cloud.google.com/yuck/repos/google-compute-engine-stable google-compute-engine-metadata-scripts
$wrapper_path = Get-MetadataValue -key 'daisy-sources-path'
$wrapper_path = "{0}/{1}" -f $wrapper_path, "wrapper.exe"
if ($wrapper_path) {
    gsutil cp $wrapper_path ./wrapper.exe
    wrapper.exe
}
else {
    throw "Could not find wrapper.exe."
}