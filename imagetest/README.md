# Cloud Image Tests #

The **Cloud Image Tests** are a testing framework and a set of test suites used
for testing GCE Images.

## Invocation ##

Testing components are built into a container image. The entrypoint is
`/manager`, which supports the following options:

    Usage:
      -images string
            comma separated list of images to test
      -out_path string
            junit xml path (default "junit.xml")
      -parallel_count int
            TestParallelCount (default 5)
      -print
            print out the parsed test workflows and exit
      -project string
            project to be used for tests
      -validate
            validate all the test workflows and exit
      -zone string
            zone to be used for tests


It can be invoked via docker as:

    $ images="projects/debian-cloud/global/images/family/debian-10,"
    $ images+="projects/debian-cloud/global/images/family/debian-9"
    $ docker run gcr.io/gcp-guest/cloud-image-tests --project $PROJECT \
      --zone $ZONE --images $images

### Credentials ###

The test manager is designed to be run in a Google Cloud environment, and will
use application default credentials. If you are not in a Google Cloud
environment and need to specify the credentials to use, you can provide them as
a docker volume and specify the path with the GOOGLE\_APPLICATION\_CREDENTIALS
environment variable.

Assuming your application default or service account credentials are in a file
named credentials.json:

    $ docker run -v /path/to/local/creds:/creds \
      -e GOOGLE_APPLICATION_CREDENTIALS=/creds/credentials.json \
      gcr.io/gcp-guest/cloud-image-tests -project $PROJECT \
      -zone $ZONE -images $images

The manager will exit with 0 if all tests completed successfully, 1 otherwise.
JUnit format XML will also be output.

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
