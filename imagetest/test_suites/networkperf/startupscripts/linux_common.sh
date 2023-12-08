#!/bin/bash
maxtimeout=300
parallelcount=12

if [[ -f /usr/bin/apt ]]; then
  echo "$(date +"%Y-%m-%d %T"): apt found Installing iperf."
  sudo apt update && sudo apt install -y iperf netcat-openbsd
elif [[ -f /bin/dnf ]]; then
  echo "$(date +"%Y-%m-%d %T"): dnf found Installing iperf."
  os=$(cat /etc/redhat-release)
  arch=$(uname -p)
  if [[ "$os" == *"release 9"* ]]; then
    if [[ "$os" == *"Red Hat"* ]]; then
      sudo subscription-manager repos --enable codeready-builder-for-rhel-9-"$arch"-rpms
      sudo dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm
    else
      sudo dnf -y config-manager --set-enabled crb
      sudo dnf -y install epel-release
    fi
  fi
  if [[ "$os" == *"release 8"* ]]; then
    if [[ "$os" == *"Red Hat"* ]]; then
      sudo subscription-manager repos --enable codeready-builder-for-rhel-8-"$arch"-rpms
      sudo dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
    else
      sudo dnf -y config-manager --set-enabled powertools
      sudo dnf -y install epel-release
    fi
  fi
  sudo sudo dnf makecache && sudo dnf -y install iperf netcat
elif [[ -f /bin/yum ]]; then
  echo "$(date +"%Y-%m-%d %T"): yum found Installing iperf."
  yum install https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm
  sudo sudo yum makecache && sudo yum -y install iperf netcat
elif [[ -f /usr/bin/zypper ]]; then
  echo "$(date +"%Y-%m-%d %T"): zypper found Installing iperf."
  arch=$(uname -p)

  if [[ "$arch" = "aarch64" ]]; then
    # For now, only SLES15 SP5 and OpenSUSE are ARM64.
    curl -L "https://sourceforge.net/projects/iperf2/files/iperf-2.1.9.tar.gz/download" > /tmp/iperf.tar.gz
    cd /tmp
    tar -xvf iperf.tar.gz
    cd iperf-2.1.9
    sudo zypper --gpg-auto-import-keys --non-interactive install gcc gcc-c++ automake make
    ./configure
    sudo make
    sudo make install
    cd ..
  else
    sudo zypper --no-gpg-checks refresh
    sudo zypper --no-gpg-checks --non-interactive install https://iperf.fr/download/opensuse/iperf-2.0.5-14.1.2.x86_64.rpm
  fi
  parallelcount=4
fi
