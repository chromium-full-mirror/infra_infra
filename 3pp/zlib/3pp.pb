create {
  source {
    url {
      download_url: "https://zlib.net/fossils/zlib-1.3.1.tar.gz"
      version: "1.3.1"
    }
    unpack_archive: true
    cpe_base_address: "cpe:/a:zlib:zlib"
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
  build {
    install: "install_win.sh"
  }
}

upload { pkg_prefix: "static_libs" }
