create {
  platform_re: "linux-amd64"
  source {
    url {}
  }
  build {}
}

upload { pkg_prefix: "tests" }
