create {
  platform_re: ".*-386"
  unsupported: true
}

create {
  source {
    git {
      repo: "https://chromium.googlesource.com/external/github.com/Kitware/CMake"
      tag_pattern: "v%s"
      # CMake includes tags for release candidates like v3.22.0-rc1. Filter
      # the tag list to released versions, so the builder does not get stuck on
      # a prereleased one.
      tag_filter_re: "v[0-9.]*$"
      # Temporary limiting cmake to 3.x until we properly split cmake3 and
      # cmake.
      version_restriction {
        op: LT
        val: "4.0"
      }
    }
    patch_dir: "patches"
    cpe_base_address: "cpe:/a:cmake_project:cmake"
    patch_version: "chromium.8"
  }

  build {
    tool: "build_support/cmake_bootstrap"
    tool: "build_support/ninja-1_11_1"
    dep: "static_libs/openssl"
  }

  package {
    version_file: ".versions/cmake.cipd_version"
  }
}

create {
  platform_re: "linux-.*|mac-.*"
  build {
    tool: "build_support/cmake_bootstrap"
    tool: "tools/sed"
    tool: "build_support/ninja-1_11_1"
    dep: "static_libs/openssl"
  }
}

upload { pkg_prefix: "tools" }
