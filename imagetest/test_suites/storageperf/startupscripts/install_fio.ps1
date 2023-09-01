
	gsutil cp gs://koln-bucket/fio.exe 'C:\\fio.exe'
try {
	$windowsDriveLetter = (Invoke-RestMethod -Headers @{'Metadata-Flavor' = 'Google'} -Uri "http://metadata.google.internal/computeMetadata/v1/instance/attributes/windowsDriveLetter")
	Initialize-Disk -PartitionStyle GPT -Number 1 -PassThru | New-Partition -DriveLetter $windowsDriveLetter -UseMaximumSize | Format-Volume -FileSystem NTFS -NewFileSystemLabel 'Perf-Test' -Confirm:$false
}
catch {
	Write-Output "failed to initialize disk"
	Write-Output $_
}
