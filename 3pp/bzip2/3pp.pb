create {
  source {
    url {
      download_url: "https://gitlab.com/bzip2/bzip2/-/archive/bzip2-1.0.8/bzip2-bzip2-1.0.8.tar.gz"
      version: "1.0.8"
    }
    unpack_archive: true
    cpe_base_address: "cpe:/a:bzip:bzip2"
    patch_version: "chromium.2"
  }
}

create {
  platform_re: "linux-.*|mac-.*"
  build {
    tool: "tools/sed"
  }
}

create {
  platform_re: "windows-.*"
  build { install: "install_win.sh" }
}

upload { pkg_prefix: "static_libs" }
