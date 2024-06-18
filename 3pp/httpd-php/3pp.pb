create {
  platform_re: "linux-amd64|mac-.*"
  source {
    script { name: "fetch.py" }
    unpack_archive: false
    patch_version: "chromium.2"
  }
  build {
    tool: "tools/autoconf"
    tool: "tools/sed"
  }
}

create {
  platform_re: "windows-arm64"
  source {
    script { name: "fetch_win.py" }
    unpack_archive: false
    patch_version: "chromium.5"
  }
  build {
    tool: "tools/cmake"
    tool: "tools/perl"
    tool: "tools/nasm"
    tool: "build_support/mingw@2@11.2.0-6"
    install: "install_win.py"
  }
}


upload { pkg_prefix: "tools" }
