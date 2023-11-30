# Cloud Image Tests #

The **Cloud Image Tests** are a testing framework and a set of test suites used
for testing GCE Images.

## Invocation ##

Testing components are built into a container image. The entrypoint is
`/manager`, which supports the following options:

    Usage:
      -filter string
            only run tests matching filter
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
$packagename\_test.go). Due to golang style conventions, the package name cannot contain an underscore. Thus, for the test suite name to match the packagename, the name of the test suite should not contain an
underscore. For example, if a new test suite was created to test image licenses,
it should be called imagelicensing, not image_licensing.

The setup.go file describes the workflow to run including the VMs and other GCE
resources to create, any necessary configuration for those resources, which
specific tests to run, etc.. It is here where you can also skip an entire test
package based on inputs e.g. image, zone or compute endpoint or other
conditions.

Tests themselves are written in the test file(s) as go unit tests. Tests may use
any of the test fixtures provided by the standard `testing` package.  These will
be packaged into a binary and run on the test VMs created during setup using the
Google Compute Engine startup script runner. The test files must specify a
'cit' build constraint in order to prevent the presubmits on this repository
from invoking the image tests.

When writing tests to run against both Linux and Windows, it is preferred to
use separate functions within the same test where appropriate based on
differences between OSes (ex. powershell vs bash commands). This makes the
test definitions easier to read and maintain.

For example, if the test TestSomeCondition() needs to run different commands to
achieve similar results (and the test is located in the directory "mydefaulttest"):

```go

package mydefaulttest

import (
    "runtime"
    "testing"
)

func RunTestConditionWindows() {
    //Test something in Windows
}
func RunTestCondition() {
    //Test something in Linux
}

func TestSomeCondition(t *testing.T) {
    if runtime.GOOS == "windows" {
    RunTestConditionWindows()
    } else {
        RunTestCondition()
    }
}
```

Note that there are also functions available in the [test_utils.go](utils/test_utils.go)
for some OS level abstractions such as running a Windows powershell command or
checking if a Linux binary exists.

It is suggested to start by copying an existing test package. Do not forget to add
your test to the relevant `setup.go` file in order to add the test to the test suite.

### Modifying test behavior based on image properties ###

For tests that need to behave different based on whether an image is arm or x86, or linux or windows, it is preferred to use compute API properties rather than relying on image naming conventions. These properties can be found on the testworkflow Image value. The list of values can be found in the compute API documentation [here](https://pkg.go.dev/google.golang.org/api/compute/v1#Image). Some examples are in the following code snippet.

```go
func Setup(t *imagetest.Testworkflow) {
	if t.Image.Architecture == "ARM64" {
	//...
	} else if utils.HasFeature(t.Image, "GVNIC") {
	//...
	}
}
```

### Testing features in compute beta API ###

Tests that need to run against features in the beta API can do so by creating TestVMs using `CreateTestVMBeta` or `CreateTestVMFromInstanceBeta` to use the beta instance API. However, due to limitation with daisy's create instances step, if one instance in a TestWorkflow uses the beta API all instances in that workflow must use the beta API.


## Building the container image ##

From the root directory of this repository:

    $ docker build -t cloud-image-tests -f imagetest/Dockerfile .

## Testing on a local machine ##

From the `imagetest` directory of this repository, where outspath is
the folder where test outputs are stored:

    $ local_build.sh -o $outspath

By default, all test suites are built. To build only one test suite:

    $ local_build.sh -o $outspath -s $test_suite_name

To build from a directory other than `imagetest`

    $ local_build.sh -o $outspath -i $path_to_imagetest

To run the tests, cd into $outspath, set the shell variables and run

    $ manager -zone $ZONE -project $PROJECT -images $images -filter $test_suite_name -local_path .


## What is being tested ##

The tests are a combination of various types - end to end tests on certain
software components, image validations and feature validations, etc. The
collective whole represents the quality assurance bar for releasing a [supported
GCE Image][gce-images], and the test suites here must all pass before Google
engineers will release a new GCE image.

The tests are documented in [the test\_suites directory](test_suites/README.md).
