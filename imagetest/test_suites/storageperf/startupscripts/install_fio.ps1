# If testing windows performance locally, you may need to put a copy of fio.exe in a local bucket. Ensure the service account has permissions to access the bucket, and change the gcs path here.
gsutil cp gs://gce-image-build-resources/windows/fio.exe 'C:\\fio.exe'
