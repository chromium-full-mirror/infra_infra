create {
  platform_re: "linux-*"
  source {
    url {}
  }
  build {
    dep: "tests/unavailable_arm64"
  }
}

upload { pkg_prefix: "tests" }
