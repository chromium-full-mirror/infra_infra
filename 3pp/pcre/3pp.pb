create {
  platform_re: "linux-.*|mac-.*"
  source {
    url {
      download_url: "https://sourceforge.net/projects/pcre/files/pcre/8.45/pcre-8.45.tar.gz/download"
      version: "8.45"
    }
    unpack_archive: true
    patch_dir: "patches"
    patch_version: "chromium.2"
    cpe_base_address: "cpe:/a:pcre:pcre"
  }
  build {
    tool: "tools/sed"
  }
}

upload { pkg_prefix: "static_libs" }
