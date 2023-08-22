cd C:\
gsutil cp gs://gce-image-build-resources/windows/fio-3.35-x64.msi C:\
msiexec.exe /A "C:\fio-3.35-x64.msi" /quiet /norestart
