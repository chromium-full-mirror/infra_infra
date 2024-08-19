create {
  platform_re: "linux-amd64"
  source {
    url {
      download_url: "https://github.com/github/codeql-action/releases/download/codeql-bundle-v2.18.2/codeql-bundle-linux64.tar.gz",
      version: "2.18.2"
    }
    unpack_archive: true
  }
}

upload { pkg_prefix: "tools" }
