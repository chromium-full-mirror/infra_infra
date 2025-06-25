# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

create {
  platform_re: "linux-amd64"
  source {
    git {
      repo: "https://github.com/google/copybara.git"
      fixed_commit: "2450737ba36adafb13b5794da8948edfcb78afe3"
    }
    patch_version: "cr0"
  }
  build {
    install: "install.sh"
    tool: "tools/bazel_bootstrap@3@8.2.1.chromium.1" # 8.3.0 or later require glibc 2.25
    external_dep: "chromium/third_party/jdk@2@jdk-11.0.17+8.fb2337e598"
  }
}

upload {
  universal: true
  pkg_prefix: "tools"
}
