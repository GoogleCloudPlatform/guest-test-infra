try {
	gsutil cp gs://gce-image-build-resources/windows/fio.exe 'C:\\fio.exe'
}
catch {
	Write-Output "failed to copy fio.exe"
	Write-Output $_
}
