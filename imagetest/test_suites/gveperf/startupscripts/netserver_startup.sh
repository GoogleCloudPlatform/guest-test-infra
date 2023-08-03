#!/bin/bash

# This script installs iperf on a VM and starts an iperf server for the a client
# to test the network bandwidth between the two VMs.

vmname=$(curl http://metadata.google.internal/computeMetadata/v1/instance/hostname -H "Metadata-Flavor: Google" | cut -d"." -f1)
outfile=$(curl http://metadata.google.internal/computeMetadata/v1/instance/hostname -H "Metadata-Flavor: Google" | cut -d"." -f1).txt
sleepduration=600

if [[ -f /usr/bin/apt ]]; then
  echo "$(date +"%Y-%m-%d %T"): apt found Installing iperf." | tee -a "$outfile"
  sudo apt update && sudo apt install iperf | tee -a "$outfile"
elif [[ -f /bin/apt-get ]]; then
  echo "$(date +"%Y-%m-%d %T"): apt found Installing iperf." | tee -a "$outfile"
  sudo apt-get update && sudo apt-get install iperf | tee -a "$outfile"
elif [[ -f /bin/dnf ]]; then
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
elif [[ ! -f /bin/dnf ]] && [[ -f /bin/yum ]]; then
  echo "$(date +"%Y-%m-%d %T"): yum found Installing iperf." | tee -a "$outfile"
  yum install https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm | tee -a "$outfile"
  sudo sudo yum makecache && sudo yum -y install iperf | tee -a "$outfile"
elif [[ -f /usr/bin/zypper ]]; then
  echo "$(date +"%Y-%m-%d %T"): zypper found Installing iperf." | tee -a "$outfile"
  sudo zypper --no-gpg-checks refresh | tee -a "$outfile"
  sudo zypper --no-gpg-checks --non-interactive install https://iperf.fr/download/opensuse/iperf-2.0.5-14.1.2.x86_64.rpm | tee -a "$outfile"
fi

echo "$(date +"%Y-%m-%d %T"): Starting iperf server. iperf version: $(iperf -v)" | tee -a "$outfile"
iperf -s &>> "$outfile"

echo "$(date +"%Y-%m-%d %T"): Waiting $sleepduration seconds for iperf test to run." | tee -a "$outfile"
sleep "$sleepduration"
shutdown -h now
