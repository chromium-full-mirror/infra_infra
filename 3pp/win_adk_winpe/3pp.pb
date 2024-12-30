create {
  platform_re: "windows-amd64"
  source {
    url {
      download_url: "https://go.microsoft.com/fwlink/?linkid=2289981"
      version: "10.1.26100.2454"
      extension: ".exe"
    }
  }

  build {
    install: "install_win.sh"
  }
}

upload { pkg_prefix: "tools" }
