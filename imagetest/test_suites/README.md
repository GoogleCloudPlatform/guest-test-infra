# Image test suites #

## What is being tested ##

The tests are a combination of various types - end to end tests on certain
software components, image validations and feature validations, etc. The
collective whole represents the quality assurance bar for releasing a
[supported GCE Image](https://cloud.google.com/compute/docs/images/os-details),
and the test suites here must all pass before Google engineers will release a
new GCE image.

Tests are broken down by suite below:

## Test suites ##

### Test suite: cvm ###

#### TestCVMEnabled ####
Validate that CVM is enabled.

### Test suite: disk ###

#### TestDiskResize ####
Validate the filesystem is resized on reboot after a disk resize.

Background

A convenience feature offered on supported GCE Images, if you resize the
underlying disk to be larger, then a set of scripts invoked during boot will
automatically resize the root partition and filesystem to take advantage of the
new space.

Test logic

Launch a VM with the default disk size. Wait for it to boot up, then resize the
disk and reboot the VM via the API. Wait for the VM to boot again, and validate
the new size as reported by the operating system matches the expected size.

### Test suite: image\_boot ###

#### TestGuestBoot
Test that the VM can boot.

#### TestGuestReboot
Test that the VM can reboot.

Background

Some categories of errors can produce an OS image that boots but cannot
successfully reboot. Documenting these errors is out of scope for this document,
but this test is a regression test against this category of error.

Test logic

Launch a VM and create a 'marker file' on disk. Reboot the VM and validate the
marker file exists on the second boot.

#### TestGuestSecureBoot
Test that VM launched with
[secure boot](https://cloud.google.com/security/shielded-cloud/shielded-vm#secure-boot)
features works properly.

Background

Secure Boot is a Linux system feature that is supported on certain GCE Images
and VM types. Documenting how Secure Boot works is out of scope for this
document.

Test logic

Launch a VM with Secure Boot enabled via the shielded instance config. Validate
that Secure Boot is enabled by querying the appropriate EFI variable through the
sysfs/efivarfs interface.


#### TestGuestShutdownScript
Test that shutdown scripts can run for around two minutes (as a proxy for
'forever')

Background

We guarantee shutdown scripts will block the system shutdown process until the
script completes. For scripts which never complete, this would cause the server
to remain in a 'shutting down' state forever. However, VMs that are stopped via
the API are first sent an ACPI soft-shutdown signal which triggers the OS
shutdown process, invoking this script. But after a set amount of time
(currently 90 seconds), if the VM is still running, the GCE API will hard-reset
the VM. It's not possible to validate that the shutdown script will run
'forever'. However, we validate that it will run at least until hard-reset
occurs.

Test logic

Launch a VM with a shutdown script in metadata. The shutdown script writes an
increasing counter value every second to a file on disk, forever. Since this
causes the graceful shutdown process to never succeed, the API hard-resets the
VM after 2 minutes. After the VM finishes shutdown, start the VM and inspect the
last value written to the file. It should be >110 to represent approximately 2
minute shutdown time.

### Test suite: image\_validation ###

#### TestNTPService
Test that a time synchronization package is installed and properly configured.

Background

Linux operating systems require a time synchronization sofware to be running to
correct any drift in the system clock. Correct clock time is required for a wide
variety of applications, and virtual machines are particularly prone to clock
drift.

Test logic

Validate that an appropriate time synchronization package is installed using the
system package manager, and read its configuration file to verify that it is
configured to check the Google-provided time server.

#### TestArePackagesLegal
Test that all installed packages are licensed for 'open source' use.

Background

The contents of GCE Images representing Linux distributions consist of hundreds
or thousands of software components that have not been created by Google or by
the OS vendor. In order to provide software written by other people, all the
software must be appropriately licensed in order to grant Google the legal right
for distribution.

Test logic

Look at the 'LICENSE' or 'copyright' files provided by every installed system
package, checking for known license names or identifying strings. Fail if any
license is found which is not in the known-good list.

#### TestStandardPrograms
Validate that Google-provided programs are present.

Background

Google-provided Linux OS images come with certain Google utilities such as
`gsutil` and `gcloud` preinstalled as a convenience.

Test Logic

Attempt to invoke the utilities, confirming they are present, found in the PATH,
and executable.

#### TestGuestPackages
Validate that the Google guest environment packages are installed

Background

Google-provided Linux OS images come with the Google guest environment
preinstalled. The guest environment enables many GCE features to function.

Test logic

Validate that the guest environment packages are installed using the system
package manager.

#### TestLinuxLicense
Validate the image has the appropriate license attached

Background

Several of the supported GCE Images are subject to licensing agreements with the
OS vendor. This is represented with the GCE License resource, which is attached
to the GCE Image resource. Official GCE Images should not be released without
the appropriate license.

Test logic

Connect to the metadata server from the VM and confirm the license available in
metadata matches the expected value.

#### TestHostname
Test that the system hostname is correctly set.

Background

The hostname is one of many pieces of 'dynamic' configuration that supported
GCE Images will set for you. This is compared to the
'static' configuration which is present on the image to be tested. Dynamic
configuration allows a single GCE Image to be used on many VMs without
pre-modification.

Test logic

Retrieve the intended FQDN from metadata (which is authoritative) and
compare the hostname part of it (first label) to the currently set hostname as
returned by the kernel.

#### TestFQDN
Test that the fully-qualified domain name is correctly set.

Background

The FQDN is a complicated concept in Linux operating systems, and setting it in
an incorrect way can lead to unexpected behavior in some software.

Test logic

Retrieve the intended FQDN from metadata and compare the full value to the
output of `/bin/hostname -A`. See `man 1 hostname` for more details.

#### TestCustomHostname
Test that custom hostnames are correctly set.

Background

The hostname for a VM matches the name of the VM by default. However, if you
specify a custom hostname at instance creation time, this will be used instead.

Test logic

Launch a VM with a custom hostname. Validate the hostname as with TestFQDN.

#### TestHostKeysGeneratedOnce
Validate that SSH host keys are only generated once per instance.

Background

The Google guest agent will generate new SSH hostkeys on the first boot of an
instance. This is a dynamic configuration to enable GCE Images to be used on
many instances, as multiple instances sharing host keys or having predictable
host keys is a security risk. However, the host keys should remain constant for
the lifetime of an instance, as changing them after the first generation may
prevent new SSH connections.

Test logic

Launch a VM and confirm the guest agent generates unique host keys on startup.
Restart the guest agent and confirm the host keys are not changed.

### Test suite: network ###

#### TestDefaultMTU
Validate the primary interface has correct MTU of 1460

Background

The default MTU for a GCE VPC is 1460. Setting the correct MTU on the network
interface to match will prevent unnecessary packet fragmentation.

Test logic

Identify the primary network interface using metadata, and confirm it has the
correct MTU using the golang 'net' package, which uses the netlink interface on
Linux (same as the `ip` command).

### Test suite: networkperf ###

#### TestNetworkPerformance ####

Validate the network performance of an image reaches at least 85% of advertised
speeds.

Background

Reaching advertised speeds is important, as failing to reach them means that
there are problems with the image or its drivers. The 85% number is chosen as
that is the baseline that the performance tests generally can match or exceed.
Reaching 100% of the advertised speeds is unrealistic in real scenarios.

Test logic

Launch a server VM and client VM, then run an iperf test between the two to test
network speeds. This test launches up to 3 sets of servers and clients: default
network, jumbo frames network, and tier1 networking tier.

### Test suite: security ###

#### TestKernelSecuritySettings
Validate sysctl tuneables have correct values

Background

Linux has a wide variety of kernel tuneables exposed via the sysctl interface.
Supported GCE Images are built with some of these setting predefined for best
behavior in the GCE environment, for example
"net.ipv4.icmp\_echo\_ignore\_broadcasts", which configures the kernel not
respond to broadcast pings.


Test logic

Read each sysctl option from the /proc/sys filesystem interface and confirm it
has the correct value.

#### TestAutomaticUpdates
Validate automatic security updates are enabled on supported distributions

Background

Some Linux distributions provide a mechanism for automatic package updates that
are marked as security updates. We enable these updates in supported GCE Images.

Test logic

Confirm the relevant automatic updates package is installed, and that the
relevant configuration options are set in the configuration files.

#### TestPasswordSecurity
Validate security settings for SSHD and system accounts

Background

As part of the default configuration provided in supported GCE Images, certain
security validations are performed. These include ensuring that password based
logins and root logins via SSH are disabled, and that system accounts have the
correct password and shell settings.

Test logic

Read the SSHD configuration file and confirm it has the 'PasswordAuthentication
no' and 'PermitRootLogin no' directives set. Read the /etc/passwd file and
confirm all users have disabled passwords, and that 'system account' users
(those with UID < 1000) have the correct shell set (typically set to 'nologin'
or 'false')

### Test suite: storageperf ###

#### TestRandomReadIOPS and TestSequentialReadIOPS
Checks random and sequential read performance on files and compares it to an expected IOPS value (in a future change, this will be compared to the documented IOPS value).

Background

The public documentation for machine shapes and types lists certain values for
read IOPS. This test was designed to verify that the read IOPS which are attainable
are within a certain range (such as 97%) of the documented value.

Test logic

FIO is downloaded based on the machine type and distribution. Next, the fio program is
run and the json output is returned. Out of the json output, we can get the read iops
value which was achieved, and check that it is above a certain threshold.

### TestRandomWriteIOPS and TestSequentialWriteIOPS
Checks random and sequential file write performance on a disk and compares it to an expected IOPS value (in a future change, this will be compared to a documented IOPS value).

Background

Similar to the read iops tests, we want to verify that write IOPS on disks work at the rate we expect for both random writes and throughput.

### Test suite: hotattach ###

### TestFileHotAttach
Validate that hot attach disks work: a file can be written to the disk, the disk can be detached and reattached, and the file can still be read.

Background
On windows and linux instances, we want to verify that we can add and remove additional disks. The hot attach functionality can be used to verify that disk memory is not lost after attach and detach operations.

Test Logic
The test writes a file to the disk, and then detaches and reattaches the disk. The test then verifies that the file can be read from. 
