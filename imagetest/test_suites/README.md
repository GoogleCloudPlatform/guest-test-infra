# Image test suites

## What is being tested

The tests are a combination of various types - end to end tests on certain
software components, image validations and feature validations, etc. The
collective whole represents the quality assurance bar for releasing a
[supported GCE Image](https://cloud.google.com/compute/docs/images/os-details),
and the test suites here must all pass before Google engineers will release a
new GCE image.

Tests are broken down by suite below:

## Test Suites

### Test suite: shapevalidation

Test that a VM can boot and access the virtual hardware of the large machine shape in a VM family.

#### Test`$FAMILY`Mem

Test that the available system memory is at least the expected amount of memory for this VM shape.

#### Test`$FAMILY`Cpu

Test the the number of active processors is equal to the number of processors expected for this VM shape.

#### Test`$FAMILY`Numa

Test the the number of active numa nodes is equal to the number of processors expected for this VM shape.

### Test suite: cvm

#### TestSEVEnabled/TestSEVSNPEnabled/TestTDXEnabled
Validate that an instance can boot with the specified confidential instance type and load its guest kernel module.

### Test suite: disk

#### TestDiskResize
Validate the filesystem is resized on reboot after a disk resize.

- <b>Background</b>: A convenience feature offered on supported GCE Images, if you resize the
underlying disk to be larger, then a set of scripts invoked during boot will
automatically resize the root partition and filesystem to take advantage of the
new space.

- <b>Test logic</b>: Launch a VM with the default disk size. Wait for it to boot up, then resize the
disk and reboot the VM via the API. Wait for the VM to boot again, and validate
the new size as reported by the operating system matches the expected size.

### Test suite: hostnamevalidation ###

Tests which verify that the metadata hostname is created and works with the DNS record.

#### TestHostname
Test that the system hostname is correctly set.

- <b>Background</b>: The hostname is one of many pieces of 'dynamic' configuration that supported
GCE Images will set for you. This is compared to the
'static' configuration which is present on the image to be tested. Dynamic
configuration allows a single GCE Image to be used on many VMs without
pre-modification.

- <b>Test logic</b>: Retrieve the intended FQDN from metadata (which is authoritative) and
compare the hostname part of it (first label) to the currently set hostname as
returned by the kernel.

#### TestFQDN
Test that the fully-qualified domain name is correctly set.

- <b>Background</b>: The FQDN is a complicated concept in Linux operating systems, and setting it in
an incorrect way can lead to unexpected behavior in some software.

- <b>Test logic</b>: Retrieve the intended FQDN from metadata and compare the full value to the
output of `/bin/hostname -f`. See `man 1 hostname` for more details.

#### TestCustomHostname
Test that custom domain names are correctly set.

- <b>Background</b>: The domain name for a VM matches the configured internal GCE DNS setting (https://cloud.google.com/compute/docs/internal-dns). By default, this will be the zonal or global DNS name. However, if you
specify a custom domain name at instance creation time, this will be used instead.

- <b>Test logic</b>: Launch a VM with a custom domain name. Validate the domain name as with TestFQDN.

#### TestHostKeysGeneratedOnce
Validate that SSH host keys are only generated once per instance.

- <b>Background</b>: The Google guest agent will generate new SSH hostkeys on the first boot of an
instance. This is a dynamic configuration to enable GCE Images to be used on
many instances, as multiple instances sharing host keys or having predictable
host keys is a security risk. However, the host keys should remain constant for
the lifetime of an instance, as changing them after the first generation may
prevent new SSH connections.

- <b>Test logic</b>: Launch a VM and confirm the guest agent generates unique host keys on startup.
Restart the guest agent and confirm the host keys are not changed.

### Test suite: hotattach

#### TestFileHotAttach
Validate that hot attach disks work: a file can be written to the disk, the disk can be detached and
reattached, and the file can still be read.

### Test suite: imageboot

#### TestGuestBoot
Test that the VM can boot.

#### TestGuestReboot
Test that the VM can reboot.

- <b>Background</b>: Some categories of errors can produce an OS image that boots but cannot
successfully reboot. Documenting these errors is out of scope for this document,
but this test is a regression test against this category of error.

- <b>Test logic</b>: Launch a VM and create a 'marker file' on disk. Reboot the VM and validate the
marker file exists on the second boot.

#### TestGuestSecureBoot
Test that VM launched with
[secure boot](https://cloud.google.com/security/shielded-cloud/shielded-vm#secure-boot)
features works properly.

- <b>Background</b>: Secure Boot is a Linux system feature that is supported on certain GCE Images
and VM types. Documenting how Secure Boot works is out of scope for this
document.

- <b>Test logic</b>: Launch a VM with Secure Boot enabled via the shielded instance config. Validate
that Secure Boot is enabled by querying the appropriate EFI variable through the
sysfs/efivarfs interface.


#### TestGuestShutdownScript
Test that shutdown scripts can run for around two minutes (as a proxy for
'forever')

- <b>Background</b>: We guarantee shutdown scripts will block the system shutdown process until the
script completes. For scripts which never complete, this would cause the server
to remain in a 'shutting down' state forever. However, VMs that are stopped via
the API are first sent an ACPI soft-shutdown signal which triggers the OS
shutdown process, invoking this script. But after a set amount of time
(currently 90 seconds), if the VM is still running, the GCE API will hard-reset
the VM. It's not possible to validate that the shutdown script will run
'forever'. However, we validate that it will run at least until hard-reset
occurs.

- <b>Test logic</b>: Launch a VM with a shutdown script in metadata. The shutdown script writes an
increasing counter value every second to a file on disk, forever. Since this
causes the graceful shutdown process to never succeed, the API hard-resets the
VM after 2 minutes. After the VM finishes shutdown, start the VM and inspect the
last value written to the file. It should be >110 to represent approximately 2
minute shutdown time.

### Test suite: licensevalidation ###

A suite which tests that linux licensing and windows activation are working successfully.

#### TestLinuxLicense
Validate the image has the appropriate license attached

- <b>Background</b>: Several of the supported GCE Images are subject to licensing agreements with the
OS vendor. This is represented with the GCE License resource, which is attached
to the GCE Image resource. Official GCE Images should not be released without
the appropriate license.

- <b>Test logic</b>: Connect to the metadata server from the VM and confirm the license available in
metadata matches the expected value.

### Test suite: network

#### TestDefaultMTU
Validate the primary interface has correct MTU of 1460

- <b>Background:</b> The default MTU for a GCE VPC is 1460. Setting the correct MTU on the network
interface to match will prevent unnecessary packet fragmentation.

- <b>Test logic:</b> Identify the primary network interface using metadata, and confirm it has the
correct MTU using the golang 'net' package, which uses the netlink interface on
Linux (same as the `ip` command).

### Test suite: networkperf

#### TestNetworkPerformance
Validate the network performance of an image reaches at least 85% of advertised
speeds.

- <b>Background</b>: Reaching advertised speeds is important, as failing to reach them means that
there are problems with the image or its drivers. The 85% number is chosen as
that is the baseline that the performance tests generally can match or exceed.
Reaching 100% of the advertised speeds is unrealistic in real scenarios.

- <b>Test logic</b>: Launch a server VM and client VM, then run an iperf test between the two to test
network speeds. This test launches up to 3 sets of servers and clients: default
network, jumbo frames network, and tier1 networking tier.

### Test suite: oslogin
Validate that the user can SSH using OSLogin, and that the guest agent can correctly provision a
VM to utilize OSLogin.

- <b>Background</b>: OSLogin is a utility that helps manage users' keys and access for SSH. It also provides
features such as the ability to authenticate users using 2FA, security keys, or certificates.

- <b>Test logic</b>: Launch a client VM and two server VMs. Each of the server VMs will perform a check to
make sure the guest agent responds correctly to OSLogin metadata changes, and the client VM will use
test users to SSH to each of the server VMs. The methods covered by this test are normal SSH and 2FA SSH.

### Test suite: packagevalidation

#### TestNTPService
Test that a time synchronization package is installed and properly configured.

- <b>Background</b>: Linux operating systems require a time synchronization sofware to be running to
correct any drift in the system clock. Correct clock time is required for a wide
variety of applications, and virtual machines are particularly prone to clock
drift.

- <b>Test logic</b>: Validate that an appropriate time synchronization package is installed using the
system package manager, and read its configuration file to verify that it is
configured to check the Google-provided time server.

#### TestStandardPrograms
Validate that Google-provided programs are present.

- <b>Background</b>: Google-provided Linux OS images come with certain Google utilities such as
`gsutil` and `gcloud` preinstalled as a convenience.

- <b>Test logic</b>: Attempt to invoke the utilities, confirming they are present, found in the PATH,
and executable.

#### TestGuestPackages
Validate that the Google guest environment packages are installed

- <b>Background</b>: Google-provided Linux OS images come with the Google guest environment
preinstalled. The guest environment enables many GCE features to function.

- <b>Test logic</b>: Validate that the guest environment packages are installed using the system
package manager.

### Test suite: security

#### TestKernelSecuritySettings
Validate sysctl tuneables have correct values

- <b>Background</b>: Linux has a wide variety of kernel tuneables exposed via the sysctl interface.
Supported GCE Images are built with some of these setting predefined for best
behavior in the GCE environment, for example
"net.ipv4.icmp\_echo\_ignore\_broadcasts", which configures the kernel not
respond to broadcast pings.

- <b>Test logic</b>: Read each sysctl option from the /proc/sys filesystem interface and confirm it
has the correct value.

#### TestAutomaticUpdates
Validate automatic security updates are enabled on supported distributions

- <b>Background</b>: Some Linux distributions provide a mechanism for automatic package updates that
are marked as security updates. We enable these updates in supported GCE Images.

- <b>Test logic</b>: Confirm the relevant automatic updates package is installed, and that the
relevant configuration options are set in the configuration files.

#### TestPasswordSecurity
Validate security settings for SSHD and system accounts

- <b>Background</b>: As part of the default configuration provided in supported GCE Images, certain
security validations are performed. These include ensuring that password based
logins and root logins via SSH are disabled, and that system accounts have the
correct password and shell settings.

- <b>Test logic</b>: Read the SSHD configuration file and confirm it has the 'PasswordAuthentication
no' and 'PermitRootLogin no' directives set. Read the /etc/passwd file and
confirm all users have disabled passwords, and that 'system account' users
(those with UID < 1000) have the correct shell set (typically set to 'nologin'
or 'false')

### Test suite: storageperf

This test suite verifies PD performance on linux and windows. The following documentation is relevant for working with these tests, as of January 2024.

Performance limits: https://cloud.google.com/compute/docs/disks/performance. In addition to machine type and vCPU performance limits, most disks have a performance limit per VM, as well as a performance limit per GB. 

FIO command options: https://cloud.google.com/compute/docs/disks/benchmarking-pd-performance. To reach maximum IOPS and bandwidth MB per second, the disk needs to be warmed up with a "random write" fio task before running the benchmarking test.

Hyperdisk limits: https://cloud.google.com/compute/docs/disks/benchmark-hyperdisk-performance. Hyperdisk disk types have a much higher performance limit and limit per GB of disk size. To reach the highest performance values on linux, some additional fio options may be required.

#### TestRandomReadIOPS and TestSequentialReadIOPS
Checks random and sequential read performance on files and compares it to an expected IOPS value
(in a future change, this will be compared to the documented IOPS value).

- <b>Background</b>: The public documentation for machine shapes and types lists certain values for
read IOPS. This test was designed to verify that the read IOPS which are attainable
are within a certain range (such as 97%) of the documented value.

- <b>Test logic</b>: FIO is downloaded based on the machine type and distribution. Next, the fio program
is run and the json output is returned. Out of the json output, we can get the read iops
value which was achieved, and check that it is above a certain threshold.

#### TestRandomWriteIOPS and TestSequentialWriteIOPS
Checks random and sequential file write performance on a disk and compares it to an expected IOPS value
(in a future change, this will be compared to a documented IOPS value).

- <b>Background</b>: Similar to the read iops tests, we want to verify that write IOPS on disks work at
the rate we expect for both random writes and throughput.

