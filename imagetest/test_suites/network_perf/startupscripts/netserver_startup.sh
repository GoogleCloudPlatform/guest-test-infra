#!/bin/bash

# This script installs iperf on a VM and starts an iperf server for the client
# to test the network bandwidth between the two VMs.

if [[ -f /usr/bin/apt ]]; then
  echo "apt found Installing iperf."
  sudo apt update && sudo apt install iperf
elif [[ -f /bin/dnf ]]; then
  echo "dnf found Installing iperf."
  os=$(cat /etc/redhat-release)
  arch=$(uname -p)
  if [[ "$os" == *"release 9"* ]]; then
    if [[ "$os" == *"Red Hat"* ]]; then
      sudo subscription-manager repos --enable codeready-builder-for-rhel-9-$arch-rpms
      sudo dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm
    else
      sudo dnf config-manager --set-enabled crb
      sudo dnf -y install epel-release
    fi
  fi
  if [[ "$os" == *"release 8"* ]]; then
    if [[ "$os" == *"Red Hat"* ]]; then
      sudo subscription-manager repos --enable codeready-builder-for-rhel-8-$arch-rpms
      sudo dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
    else
      sudo dnf config-manager --set-enabled powertools
      sudo dnf -y install epel-release
    fi
  fi
  sudo sudo dnf makecache && sudo dnf -y install iperf
elif [[ -f /bin/yum ]]; then
  os=$(cat /etc/redhat-release)
  echo "yum found Installing iperf."
  if [[ "$os" == *"Red Hat"* ]]; then
    subscription-manager repos --enable rhel-*-optional-arams --enable  rhel-*-extras-rpms --enable rhel-ha-for-rhel-*-server-rpms
    sudo yum install https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm
  fi
  sudo sudo yum makecache && sudo yum -y install iperf
elif [[ -f /usr/bin/zypper ]]; then
  echo "zypper found Installing iperf."
  arch=$(uname -p)
  version=$(cat /etc/os-release | grep VERSION_ID | cut -d '=' -f 2 | xargs)
  sudo SUSEConnect --product PackageHub/$version/$arch
  sudo zypper refresh

  # Installs iperf3 by default.
  sudo zypper --non-interactive install iperf
fi

echo "Starting iperf server"
if [[ -f /bin/iperf ]]; then
  iperf -s -P 16
else
  iperf3 -s -1
fi
