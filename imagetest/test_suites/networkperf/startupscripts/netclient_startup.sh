#!/bin/bash

# This simple script installs iperf on a VM and attempts to connect to an iperf
# server to test the network bandwidth between the two VMs.

outfile=$(curl http://metadata.google.internal/computeMetadata/v1/instance/hostname -H "Metadata-Flavor: Google" | cut -d"." -f1).txt
iperftarget=$(curl http://metadata.google.internal/computeMetadata/v1/instance/attributes/iperftarget -H "Metadata-Flavor: Google")
sleepduration=5
maxtimeout=60
timeout=0

echo "MTU: "
/sbin/ifconfig | grep mtu

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
  sudo zypper --no-gpg-checks refresh
  sudo zypper --no-gpg-checks --non-interactive install https://iperf.fr/download/opensuse/iperf-2.0.5-14.1.2.x86_64.rpm netcat
fi

# Ensure the server is up and running.
timeout 2 nc -v -w 1 "$iperftarget" 5001 &> /tmp/nc_iperf
until [[ $(< /tmp/nc_iperf) == *"succeeded"* || $(< /tmp/nc_iperf) == *"Connected"* || "$timeout" -ge "$maxtimeout" ]]; do
  cat /tmp/nc_iperf
  echo Failed to connect to server. Trying again in 5s
  sleep "$sleepduration"

  # timeout ensures the command stops. On some versions of netcat,
  # the -w flag seems nonfunctional. This is the workaround.
  timeout 2 nc -v -w 1 "$iperftarget" 5001 &> /tmp/nc_iperf
done
sleep "$sleepduration"

# Run iperf
echo "$(date +"%Y-%m-%d %T"): Running iperf client with target $iperftarget"
iperf -t 30 -c "$iperftarget" -P 12 | grep SUM | tr -s ' ' | tee -a "$outfile"

echo "$(date +"%Y-%m-%d %T"): Test Results $results"
echo "$(date +"%Y-%m-%d %T"): Sending results to metadata."
results=$(cat "./$outfile")
for i in $(seq 0 2); do
	sleep $i
	curl -X PUT --data "$results" http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/testing/results -H "Metadata-Flavor: Google" && break
done
