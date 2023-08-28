cd C:\
gsutil cp gs://gce-image-build-resources/windows/fio-3.35-x64.msi C:\
try {
	msiexec.exe "C:\fio-3.35-x64.msi" /quiet /norestart
}
catch {
	Write-Output "msi file failed to resolve"
	Write-Output $_
}
