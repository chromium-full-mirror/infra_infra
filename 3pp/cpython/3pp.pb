create {
  platform_re: "linux-amd64"
  verify { test: "python_test.py" }
  source {
    url {
      download_url: "https://www.python.org/ftp/python/2.7.18/Python-2.7.18.tgz"
      version: "2.7.18"
      extension: ".tgz"
    }
    unpack_archive: true
    patch_dir: "patches"

    patch_version: "chromium.48"
    cpe_base_address: "cpe:/a:python:python"
  }
  build {
    tool: "build_support/pip_bootstrap@2@pip20.3.4.setuptools44.1.1.wheel0.37.1.chromium4"
    tool: "tools/autoconf"
    tool: "tools/sed"  # Used by python's makefiles

    dep: "static_libs/bzip2"
    dep: "static_libs/ncurses"
    dep: "static_libs/openssl"
    dep: "static_libs/readline"
    dep: "static_libs/sqlite"
    dep: "static_libs/zlib"
    dep: "static_libs/nsl"

  }
  package { version_file: ".versions/cpython.cipd_version" }
}

upload { pkg_prefix: "tools" }
