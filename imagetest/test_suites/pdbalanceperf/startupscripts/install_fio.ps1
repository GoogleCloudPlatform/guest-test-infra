try {
	gsutil cp 'gs://gce-image-build-bucket/windows/fio.exe' 'C:\\fio.exe'
}
catch {
	Write-Output "failed to copy fio"
	Write-Output $_
}
