create {
  verify { test: "python_test.py" }
  source { patch_version: "chromium.35" }
  package { version_file: ".versions/cpython3.cipd_version" }
}

create {
  platform_re: "linux-.*|mac-.*"
  source {
    git {
      repo: "https://chromium.googlesource.com/external/github.com/python/cpython"
      tag_pattern: "v%s",

      # Pin to last bugfix release of the Python 3.11 with windows binaries
      # TODO(crbug.com/365481207): Update latest cpython3 to 3.12.
      version_restriction: { op: LT val: "3.11.10"}
    }
    patch_dir: "patches"
  }
  build {
    # no binutils on mac since it includes some tools like 'ar' that we don't
    # actually want
    tool: "build_support/pip_bootstrap"
    tool: "tools/autoconf"
    tool: "tools/sed"
  }
}

create {
  platform_re: "mac-.*"
  build {
    dep: "static_libs/bzip2"
    dep: "static_libs/libuuid"
    dep: "static_libs/ncursesw"
    dep: "static_libs/openssl"
    dep: "static_libs/readline"
    dep: "static_libs/sqlite"
    dep: "static_libs/xzutils"
    dep: "static_libs/zlib"
  }
}

create {
  platform_re: "linux-.*"
  build {
    dep: "static_libs/bzip2"
    dep: "static_libs/libffi"
    dep: "static_libs/libuuid"
    dep: "static_libs/ncursesw"
    dep: "static_libs/openssl"
    dep: "static_libs/readline"
    dep: "static_libs/sqlite"
    dep: "static_libs/xzutils"
    dep: "static_libs/zlib"

    tool: "build_support/pip_bootstrap"
    tool: "tools/autoconf"
    tool: "tools/binutils"
    tool: "tools/sed"
  }
}

create {
  platform_re: "linux-arm.*|linux-mips.*|linux-riscv.*"
  build {
    tool: "build_support/pip_bootstrap"
    tool: "tools/autoconf"
    tool: "tools/binutils"
    tool: "tools/sed"            # Used by python's makefiles

    tool: "tools/cpython3"
  }
}

create {
  platform_re: "windows-.*"
  source { script { name: "fetch.py" } }
  build {
    tool: "build_support/pip_bootstrap"
    tool: "tools/lessmsi"

    install: "install_win.sh"
    # Note that the patches in patches_win are applied by install.sh
    # after the .msi files are extracted, rather than by the 3pp recipe.
  }
  verify { test: "python_test.py" }
}

upload { pkg_prefix: "tools" }
