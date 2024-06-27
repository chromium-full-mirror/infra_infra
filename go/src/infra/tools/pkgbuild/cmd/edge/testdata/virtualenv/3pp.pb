create {
  source {
    git {
      repo: "https://github.com/pypa/get-virtualenv.git"
    }
    patch_version: "chromium.8"
    cpe_base_address: "cpe:/a:virtualenv:virtualenv"
  }
  build {}
}

upload {
  pkg_prefix: "tools"
  universal: true
}