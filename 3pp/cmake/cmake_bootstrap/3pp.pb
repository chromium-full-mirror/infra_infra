create {
  platform_re: "linux-amd64|mac-.*"
  source {
    git {
      repo: "https://chromium.googlesource.com/external/github.com/Kitware/CMake"
      tag_pattern: "v%s"
      # CMake includes tags for release candidates like v3.22.0-rc1. Filter
      # the tag list to released versions, so the builder does not get stuck on
      # a prereleased one.
      tag_filter_re: "v[0-9.]*$"
      version_restriction {
        op: GE
        val: "4.0"
      }
    }
    patch_version: "chromium.2"
  }

  build {
    tool: "tools/sed"
  }
}

create {
  platform_re: "windows-amd64"
  source {
    script { name: "fetch_win.py" }
    unpack_archive: true
  }
}

upload { pkg_prefix: "build_support" }
