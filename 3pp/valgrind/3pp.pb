create {
  platform_re: "linux-(amd64|arm64)"

  source {
    git {
      repo: "https://sourceware.org/git/valgrind.git"
      tag_pattern: "VALGRIND_%s"
      version_join: "_"
    }
    cpe_base_address: "cpe:/a:valgrind:valgrind"
    patch_version: "chromium.1"
  }
  build {
    tool: "tools/sed"
  }
}

upload { pkg_prefix: "tools" }
