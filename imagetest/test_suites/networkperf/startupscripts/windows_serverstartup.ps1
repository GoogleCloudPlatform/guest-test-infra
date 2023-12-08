$iperfurl="https://iperf.fr/download/windows/iperf-2.0.9-win64.zip"
$iperfzippath="iperf.zip"
$zipdir="C:\iperf"
$exepath="C:\iperf\iperf-2.0.9-win64"
$timeout=60

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest -Uri $iperfurl -OutFile $iperfzippath
Expand-Archive -Path $iperfzippath -DestinationPath $zipdir
New-NetFirewallRule -DisplayName "allow-iperf" -Direction Inbound -LocalPort 5001 -Protocol TCP -Action Allow

cd $exepath
./iperf -s -P 16 -t $timeout
# print iperf output to logs
Write-Host $_
