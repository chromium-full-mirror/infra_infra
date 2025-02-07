# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

create {
  platform_re: "windows-amd64"
  source {
    script { name: "fetch.py" }
    unpack_archive: false
    patch_version: "chromium.1"
  }
  build {
    install: "install_win.sh"
  }
}

upload { pkg_prefix: "tools" }
