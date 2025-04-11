create {
  platform_re: "linux-.*|mac-.*"
  source {
    url {
      download_url: "https://github.com/skvadrik/re2c/releases/download/4.1/re2c-4.1.tar.xz"
      version: "4.1.0"
    }
    unpack_archive: true
    cpe_base_address: "cpe:/a:re2c:re2c"
  }
  build {
    tool: "tools/sed"
  }
}

upload { pkg_prefix: "tools" }
