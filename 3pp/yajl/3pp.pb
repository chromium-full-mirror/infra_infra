create {
  platform_re: "linux-.*|mac-.*"
  source {
    git {
      repo: "https://github.com/lloyd/yajl.git"
      tag_pattern: "%s"
    }
    cpe_base_address: "cpe:/a:yajl_project:yajl"
  }
  build {
    tool: "tools/cmake3"
  }
}

upload { pkg_prefix: "static_libs" }

