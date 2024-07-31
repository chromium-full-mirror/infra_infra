create {
  platform_re: "linux-amd64"

  source {
    script { name: "fetch.py" }
    unpack_archive: false
  }

  build {}
}

upload { pkg_prefix: "tools" }
