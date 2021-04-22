# Cloud Image Tests #

The **Cloud Image Tests** are a testing framework and a set of test suites used
for testing GCE Images.

## Invocation ##

Testing components are built into a container image. The entrypoint is
`/manager` and the image can be used as:

    $ images="projects/debian-cloud/global/images/family/debian-10,"
    $ images+="projects/debian-cloud/global/images/family/debian-9"
    $ docker run gcr.io/gcp-guest/cloud-image-tests --project $PROJECT \
      --zone $ZONE --images $images

The manager will exit with 0 if all tests completed successfully, 1 otherwise.
JUnit format XML will also be output.

### Running test locally ###

Go to imagetest subfolders to build manager, wrapper, and each test suites. Move 
all build binaries to `/out`

    $ /out/manager -project $PROJECT -zone $ZONE -image $IMAGE

For example:

    $ /out/manager -project gcp-guest -zone us-west1-c -image projects/debian \
    -cloud/global/images/family/debian-10
## Writing tests ##

Tests are organized into go packages in the test\_suites directory and are
written in go. Each package must at a minimum contain a setup file (by
conventioned named setup.go) and at least one test file (by convention named
$packagename\_test.go).

The setup.go file describes the workflow to run including the VMs and other GCE
resources to create, any necessary configuration for those resources, which
specific tests to run, etc.. It is here where you can also skip an entire test
package based on inputs e.g. image, zone or compute endpoint or other
conditions.

Tests themselves are written in the test file(s) as go unit tests. Tests may use
any of the test fixtures provided by the standard `testing` package.  These will
be packaged into a binary and run on the test VMs created during setup using the
Google Compute Engine startup script runner. The test files must specify a
TestMain function adding a command line flag, in order to prevent the presubmits
on this repository from invoking the image tests.

It is suggested to start by copying an existing test package.

## Building the container image ##

From the root of this repository:

    $ docker build -t cloud-image-tests -f imagetest/Dockerfile .

 
