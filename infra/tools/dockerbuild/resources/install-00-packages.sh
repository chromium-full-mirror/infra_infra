#!/bin/bash
# Copyright 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Load our installation utility functions.
. /install-util.sh

# Install missing packages for system Python modules.
if [ -x /usr/bin/apt-get ]; then
  apt-get install -y zlib1g-dev libbz2-dev libltdl-dev texi2html texinfo
  apt-get clean --yes
elif [ -x /usr/bin/yum ]; then
  # Switch to vault repo if it's CentOS since manylinux-2014 passed eol.
  sed -i '/mirrorlist/d' /etc/yum.repos.d/{CentOS-Base,CentOS-SCLo-scl-rh}.repo
  sed -i 's/^#baseurl/baseurl/' /etc/yum.repos.d/{CentOS-Base,CentOS-SCLo-scl-rh}.repo
  sed -i 's/mirror\.centos\.org/vault\.centos\.org/' /etc/yum.repos.d/{CentOS-Base,CentOS-SCLo-scl-rh}.repo

  yum install -y zlib-devel bzip2-devel ncurses-devel sqlite-devel texi2html texinfo
  yum clean all
else
  echo "UKNOWN package platform."
  exit 1
fi

# The CentOS images also don't have `nproc`, so add it.
if ! which nproc; then
  echo '#!/bin/bash' > /usr/bin/nproc
  echo 'grep processor < /proc/cpuinfo | wc -l' >> /usr/bin/nproc
  chmod +x /usr/bin/nproc
fi
