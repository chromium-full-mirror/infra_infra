create {
  platform_re: "linux-.*|mac-.*"
  source {
    git {
      repo: "https://github.com/PCRE2Project/pcre2.git"
      tag_pattern: "pcre2-%s"
    }
    cpe_base_address: "cpe:/a:pcre:pcre2"
    patch_version: "chromium.1"
  }
  build {
    tool: "tools/sed"
  }
}

upload { pkg_prefix: "static_libs" }
