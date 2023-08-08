#!/bin/bash

# This script installs iperf on a VM and starts an iperf server for the client
# to test the network bandwidth between the two VMs.

sleepduration=120

if [[ -f /usr/bin/apt ]]; then
  echo "apt found Installing iperf."
  sudo apt update && sudo apt install iperf
elif [[ -f /bin/dnf ]]; then
  echo "dnf found Installing iperf."
  os=$(cat /etc/redhat-release)
  if [[ "$os" == *"release 9."* ]]; then
    sudo dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm
  fi
  if [[ "$os" == *"release 8."* ]]; then
    sudo dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
  fi
  sudo sudo dnf makecache && sudo dnf -y install iperf
elif [[ -f /bin/yum ]]; then
  echo "yum found Installing iperf."
  yum install https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm
  sudo sudo yum makecache && sudo yum -y install iperf
elif [[ -f /usr/bin/zypper ]]; then
  echo "zypper found Installing iperf."
  sudo zypper --no-gpg-checks refresh
  sudo zypper --no-gpg-checks --non-interactive install iperf
fi

echo "Starting iperf server. iperf version: $(iperf -v)"
iperf -s -t "$sleepduration"
