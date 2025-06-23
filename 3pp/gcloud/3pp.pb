create {
  platform_re: ".*-(amd64|arm64)"
  source {
    script { name: "fetch.py" }
    unpack_archive: true
    no_archive_prune: true
    patch_version: "chromium.4"
  }
  build {
    no_toolchain: true
    tool: "tools/cpython38"
    tool: "tools/sed"
  }
}

create {
  platform_re: "windows-.*"
  build {
    no_toolchain: true
    tool: "tools/cpython38"
  }
}

upload { pkg_prefix: "tools" }
