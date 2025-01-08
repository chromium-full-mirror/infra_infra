create {
  platform_re: "linux-amd64|mac-.*|windows-.*"
  source {
    url {
      download_url: "https://www.7-zip.org/a/7z2409-src.tar.xz"
      version: "24.09"
      extension: "tar.xz"
    }
    patch_dir: "patches"
    unpack_archive: true
  }
  build {}
}

upload { pkg_prefix: "tools" }
