create {
  platform_re: ".*-386"
  unsupported: true
}

create {
  source {
    git {
      repo: "https://chromium.googlesource.com/external/github.com/ninja-build/ninja"
      tag_pattern: "v%s"
    }
    patch_version: "chromium.4"
  }
  build {
    tool: "tools/cpython38"
  }
}

create {
  platform_re: "mac-.*|linux-amd64"
  build {
    tool: "tools/cpython38"
    tool: "tools/re2c"  # Required if we ever patch ninja source
    tool: "tools/sed"
    tool: "tools/cmake" # Required for running tests
  }
}

create {
  platform_re: "linux-arm.*|linux-mips.*|linux-riscv.*"
  build {
    tool: "tools/cpython38"
    tool: "tools/ninja"  # Depend on the bootstrapped version when cross-compiling
    tool: "tools/re2c"  # Required if we ever patch ninja source
    tool: "tools/sed"
  }
}

create {
  platform_re: "windows-amd64"
  build {
    tool: "tools/cmake" # Required for running tests
    tool: "build_support/ca-bundle" # CMake can't find default CA on windows
  }
}

create {
  platform_re: "windows-arm64"
  build {
    tool: "tools/cpython38"
    tool: "tools/ninja"  # Depend on the bootstrapped version when cross-compiling
  }
}

upload { pkg_prefix: "tools" }
