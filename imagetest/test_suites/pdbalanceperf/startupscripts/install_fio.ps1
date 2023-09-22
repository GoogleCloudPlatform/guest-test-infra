try {
	gsutil cp 'gs://gce-image-build-bucket/windows/fio.exe' 'C:\\fio.exe'
	$windowsDriveLetter = (Invoke-RestMethod -Headers @{'Metadata-Flavor' = 'Google'} -Uri "http://metadata.google.internal/computeMetadata/v1/instance/attributes/windowsDriveLetter")
	Initialize-Disk -PartitionStyle GPT -Number 1 -PassThru | New-Partition -DriveLetter $windowsDriveLetter -UseMaximumSize | Format-Volume -FileSystem NTFS -NewFileSystemLabel 'Perf-Test' -Confirm:$false
}
catch {
	Write-Output "failed to initialize disk or copy fio"
	Write-Output $_
}
