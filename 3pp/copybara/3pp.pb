# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

create {
  platform_re: "linux-amd64"
  source {
    git {
      repo: "https://github.com/google/copybara.git"
      fixed_commit: "1f70a5bce39ffc842b7633b5683d9891339f0a86"
    }
    patch_version: "cr0"
  }
  build {
    install: "install.sh"
    tool: "tools/bazel_bootstrap"
    external_dep: "chromium/third_party/jdk@2@jdk-11.0.17+8.fb2337e598"
  }
}

upload {
  universal: true
  pkg_prefix: "tools"
}
