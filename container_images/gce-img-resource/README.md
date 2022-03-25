# GCE Image Resource

Versions [Google Compute Engine][gce] (GCE) images, either by image creation date or versioned image name.

This resource is based on [frodenas/gcs-resource][frodenas], which itself is based on the official
[S3 resource][s3-resource].

## Source Configuration

* `project`: *Required.* The name of the GCP project.

* `family`: *Optional.* GCE Image family name. If set, versions are only produced for images with this family.

* `regexp`: *Optional.* **Not yet implemented** Regular expression to apply to image names. Must contain a
  capture group which matches a version in the image name. If provided, images are ordered by this version rather than creation date, and images which don't match or provide a valid version in the capture group will be skipped. Example:

    ```yaml
    regexp: my-image-v([0-9]+)
    ```

* `json_key`: *Optional.* **Not yet implemented** Raw JSON key to use for service account credentials. If not
  set, uses Google Default Credentials (recommended on GCP platforms).

### Example

Resource configuration that produces a version for *every* GCE image published to your project (the default):

```yaml
---
resources:
- name: latest-image
  type: gce-image
  source:
    project: my-gcp-project

jobs:
- name: my-job
  plan:
  - get: latest-image
    trigger: true
  - task: some-task
    config: {}
```

## Behavior

Principally this resource is used for representing GCE Image objects in Concourse for tracking and
job-triggering. GCE Images don't have any downloadable content, and image creation is done by reference.

### `check`: Get image versions from the project.

Discover image versions. Images will be versioned either by image creation date or by `regexp`, if provided.
If `family` is provided, only images in the specified family will be returned.

### `in`: Get data about an image.

Places the following files in the destination:

* `creation_timestamp`: the image creation time in Unix time.
* `name`: the image name.
* `url`: the image self link, a canonical reference.
* `version`: the image version, which is either the creation timestamp or if regexp is provided the matching
  version string from the image name.

### `out`: Create an image. **Not implemented**.
