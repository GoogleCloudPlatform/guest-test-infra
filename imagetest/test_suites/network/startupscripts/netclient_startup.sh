#!/bin/bash

# This simple script installs iperf on a VM and attempts to connect to an iperf
# server to test the network bandwidth between the two VMs.

vmname=$(curl http://metadata.google.internal/computeMetadata/v1/instance/hostname -H "Metadata-Flavor: Google" | cut -d"." -f1)
outfile=$(curl http://metadata.google.internal/computeMetadata/v1/instance/hostname -H "Metadata-Flavor: Google" | cut -d"." -f1).txt
iperftarget=$(curl http://metadata.google.internal/computeMetadata/v1/instance/attributes/iperftarget -H "Metadata-Flavor: Google")

echo "MTU: "
/sbin/ifconfig | grep mtu | tee -a "$outfile"

if [[ -f /usr/bin/apt ]]; then
  echo "$(date +"%Y-%m-%d %T"): apt found Installing iperf." | tee -a "$outfile"
  sudo apt update && sudo apt install iperf | tee -a "$outfile"
fi 

if [[ -f /bin/apt-get ]]; then
  echo "$(date +"%Y-%m-%d %T"): apt found Installing iperf." | tee -a "$outfile"
  sudo apt-get update && sudo apt-get install iperf | tee -a "$outfile"
fi 

if [[ -f /bin/dnf ]]; then
  echo "$(date +"%Y-%m-%d %T"): dnf found Installing iperf." | tee -a "$outfile"
  os=$(cat /etc/redhat-release)
  if [[ "$os" == *"release 9."* ]]; then
    sudo dnf -y install https://dl.fedoraproject.org/pub/epel/9/Everything/x86_64/Packages/i/iperf-2.1.6-2.el9.x86_64.rpm | tee -a "$outfile"
  fi
  if [[ "$os" == *"release 8."* ]]; then
    sudo dnf -y install https://dl.fedoraproject.org/pub/epel/8/Everything/x86_64/Packages/i/iperf-2.1.6-2.el8.x86_64.rpm | tee -a "$outfile"
  fi
  sudo dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm | tee -a "$outfile"
  sudo dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm | tee -a "$outfile"
  sudo sudo dnf makecache && sudo dnf -y install iperf | tee -a "$outfile"
fi

if [[ ! -f /bin/dnf ]] && [[ -f /bin/yum ]]; then
  echo "$(date +"%Y-%m-%d %T"): yum found Installing iperf." | tee -a "$outfile"
  yum install https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm | tee -a "$outfile"
  sudo sudo yum makecache && sudo yum -y install iperf | tee -a "$outfile"
fi 

if [[ -f /usr/bin/zypper ]]; then
  echo "$(date +"%Y-%m-%d %T"): zypper found Installing iperf." | tee -a "$outfile"
  sudo zypper --no-gpg-checks refresh | tee -a "$outfile"
  sudo zypper --no-gpg-checks --non-interactive install https://iperf.fr/download/opensuse/iperf-2.0.5-14.1.2.x86_64.rpm | tee -a "$outfile"
fi 

echo "$(date +"%Y-%m-%d %T"): Running iperf client with target $iperftarget. iperf version: $(iperf -v)" | tee -a "$outfile"
iperf -t 30 -c "$iperftarget" -P 16 | tee -a "$outfile"

echo "$(date +"%Y-%m-%d %T"): Test Results $results" | tee -a "$outfile"
echo "$(date +"%Y-%m-%d %T"): Sending results to metadata." | tee -a "$outfile"
results=$(cat "./$outfile")
curl -X PUT --data "$results" http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/testing/results -H "Metadata-Flavor: Google"
