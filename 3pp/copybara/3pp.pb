# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

create {
  platform_re: "linux-amd64"
  source {
    git {
      repo: "https://github.com/google/copybara.git"
      fixed_commit: "97a24b919365c51e76d96979507003c6d3f1f59c"
    }
    patch_version: "cr0"
  }
  build {
    install: "install.sh"
    tool: "tools/bazel_bootstrap"
    external_dep: "chromium/third_party/jdk@2@jdk-21.0.4+7.8fb6cd615f.cr4"
  }
}

upload {
  universal: true
  pkg_prefix: "tools"
}
