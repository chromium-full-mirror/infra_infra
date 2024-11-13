create {
  source {
    url {
      download_url: "https://curl.se/ca/cacert-2024-09-24.pem"
      version: "20240924"
      extension: ".pem"
    }
  }
}

upload {
  universal: true
  pkg_prefix: "build_support"
}
