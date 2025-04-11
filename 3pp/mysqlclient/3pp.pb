# Package definition for just the mysql client library.
create {
  platform_re: "linux-amd64|mac-.*"
  source {
    url {
      download_url: "https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-boost-8.0.26.tar.gz"
      version: "8.0.26"
    }
    unpack_archive: true
  }

  build {
    dep: "static_libs/openssl"
    tool: "tools/cmake3"
  }
}

upload { pkg_prefix: "static_libs" }
