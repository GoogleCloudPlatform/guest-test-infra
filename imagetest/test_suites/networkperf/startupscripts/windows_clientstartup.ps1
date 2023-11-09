$iperfurl="https://iperf.fr/download/windows/iperf-2.0.9-win64.zip"
$metadata="http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/testing/results"
$iperfzippath="iperf.zip"
$zipdir="C:\iperf"
$exepath="C:\iperf\iperf-2.0.9-win64/"
$outfile="iperfoutput.txt"

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$iperftarget=Invoke-RestMethod -Uri "http://metadata.google.internal/computeMetadata/v1/instance/attributes/iperftarget" -Header @{"Metadata-Flavor" = "Google"} -UseBasicParsing
Invoke-WebRequest -Uri $iperfurl -OutFile $iperfzippath
Expand-Archive -Path $iperfzippath -DestinationPath $zipdir
New-NetFirewallRule -DisplayName "allow-iperf" -Direction Inbound -LocalPort 5001 -Protocol TCP -Action Allow

cd $exepath
Start-Sleep -s 5 # Wait for the server to start up.

# Perform the test, and upload results.
./iperf -c $iperftarget -t 30 -P 16 2>&1 > $outfile
for (($i = 0); $i -lt 3; $i++)
{
  Start-Sleep -Seconds $i
  (Get-Content -Path $outfile | Select-String -Pattern 'SUM') -replace "\s+"," " | Invoke-RestMethod -Method "Put" -Uri $metadata -Header @{"Metadata-Flavor" = "Google"} -ContentType "application/json; charset=utf-8" -UseBasicParsing
  if ($?) {
    break
  }
}
