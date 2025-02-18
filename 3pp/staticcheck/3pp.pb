# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

create {
  source {
     git {
      repo: "https://github.com/dominikh/go-tools.git"
      tag_pattern: "v%s"
    }
  }
  build {
    tool: "tools/go"
  }
}

upload { pkg_prefix: "tools" }
