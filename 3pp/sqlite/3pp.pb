create {
  platform_re: "linux-.*|mac-.*"
  source {
    url {
      download_url: "https://www.sqlite.org/2025/sqlite-autoconf-3490100.tar.gz"
      version: "3.49.1"
    }
    unpack_archive: true
    cpe_base_address: "cpe:/a:sqlite:sqlite"
    patch_version: "chromium.2"
  }
  build {
    tool: "tools/sed"
    tool: "tools/jimtcl"
  }
}

upload { pkg_prefix: "static_libs" }
