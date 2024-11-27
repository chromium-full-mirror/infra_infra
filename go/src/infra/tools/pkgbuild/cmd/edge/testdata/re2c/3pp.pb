create {
  platform_re: "linux-.*|mac-.*"
  source {
    url {
      download_url: "https://github.com/skvadrik/re2c/releases/download/9999.9.9/re2c-9999.9.9.tar.gz"
      version: "9999.9.9"
    }
    unpack_archive: true
    cpe_base_address: "cpe:/a:re2c:re2c"
  }
  build {}
}

upload { pkg_prefix: "tools" }
