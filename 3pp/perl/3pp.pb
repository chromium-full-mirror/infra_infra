create {
  platform_re: "windows-amd64"
  source {
    url {
      download_url: "https://github.com/StrawberryPerl/Perl-Dist-Strawberry/releases/download/SP_53822_64bit/strawberry-perl-5.38.2.2-64bit-portable.zip"
      version: "5.38.2.2"
      extension: ".zip"
    }
    unpack_archive: true
    cpe_base_address: "cpe:/a:perl:perl"
  }

  build {}
}

upload { pkg_prefix: "tools" }
