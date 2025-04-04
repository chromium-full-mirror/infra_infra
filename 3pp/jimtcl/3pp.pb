create {
  platform_re: "linux-amd64|mac-.*"
  source {
    git {
      repo: "https://github.com/msteveb/jimtcl.git"
      tag_pattern: "%s"
    }
  }
  build {
    tool: "tools/sed"
  }
}

upload { pkg_prefix: "tools" }
