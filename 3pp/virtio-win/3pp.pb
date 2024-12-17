# Copyright 2024 The Chromium Authors
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
    tool: "tools/7z"
  }
}

upload { pkg_prefix: "tools" }
