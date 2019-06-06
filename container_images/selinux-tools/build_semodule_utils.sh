#!/bin/bash

set -e

# Alpine package repo has an older version of policycoreutils which does not
# include semodule-utils as the RHEL/Debian packages do. Build logic based on
# the policycoreutils APKBUILD, and could probably be made into one.

package="semodule-utils-2.8"

echo "Installing dependencies.."
apk add -X http://dl-cdn.alpinelinux.org/alpine/edge/testing libsemanage-dev
apk add gettext-dev libsepol-dev libselinux-dev fts-dev linux-pam-dev libcap-ng-dev audit-dev gawk musl-dev python3 flex make gcc bison python python3

echo "Downloading $package"
wget "https://raw.githubusercontent.com/wiki/SELinuxProject/selinux/files/releases/20180524/${package}.tar.gz"
tar xf "${package}.tar.gz"

cd "$package"
make
make install
