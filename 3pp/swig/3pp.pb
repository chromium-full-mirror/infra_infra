create {
  platform_re: "windows-386"
  unsupported: true
}

create {
  source {
    patch_version: "chromium.3"
  }
}

create {
  platform_re: "linux-.*|mac-.*"
  source {
    git {
      repo: "https://chromium.googlesource.com/external/github.com/swig/swig"
      tag_pattern: "rel-%s"
    }
    patch_dir: "patches"
  }
  build {
    tool: "tools/autoconf"
    tool: "tools/automake"
    tool: "tools/sed"

    dep: "static_libs/pcre"
  }
}

create {
  platform_re: "windows-amd64"
  source {
    script { name: "fetch_win.py" }
    unpack_archive: true
  }
  build {
    install: "install_win.sh"
    no_toolchain: true
  }
}

upload { pkg_prefix: "tools" }
