Overview
--------

Scripts and files in this directory describe how to build CIPD packages from
the source code in infra.git repo. These packages are typically executables
compiled from Go code, but may also bundle Python scripts or arbitrary files.


Package definition
------------------

A package is defined in a YAML file that is parsed by `build.py` script before
being passed to the CIPD client.

The package file has the following structure:

```yaml
# Name of the package in CIPD repository.
package: infra/example/package/${platform}

# Human readable description of the package.
description: Example package

# Set to true to exclude the package from the build.
disabled: false

# Set to false to not update the 'latest' CIPD ref.
update_latest_ref: false

# Filter with a list of CIPD platform suffixes for which to build this package:
# whenever build.py script is invoked with some `--cipd-platform` flag, it looks
# at this list to decide whether to build the package or not. The list of
# platforms to attempt to build for is thus outside of build.py control: it is
# specified by whoever calls build.py (see infra_continuous.py recipe).
#
# See https://pkg.go.dev/go.chromium.org/luci/cipd/client/cipd/ensure for the
# list of supported values here and how they map to Go build environment flags.
#
# If your package is platform agnostic (and doesn't really use ".../${platform}"
# package name suffix), just specify some single platform (usually
# "linux-amd64") to make sure there's a builder that builds this package.
platforms:
  - linux-amd64
  ...

# Optional list of go packages to 'go install' before zipping this package.
#
# Go binaries will be built under the build environment (GOOS, GOARCH, GOARM)
# set based on the `--cipd-platform` flag.
go_packages:
  - go.chromium.org/luci/cipd/client/cmd/cipd
  - ...

# Additional Go build environment to use when building go code.
go_build_environ:
  # If given, sets CGO_ENABLED env var to this value. Defaults to 0 if missing
  # (i.e. cgo is disabled by default).
  #
  # Note that it is also possible to specify this on per CIPD platform basis,
  # by using a dictionary as a value. Keys are either CIPD OSs or CIPD
  # platforms.
  cgo:
    mac: 1  # enable on all Mac archs
    windows-amd64: 1  # enable only on amd64 Windows

  # If given, attach `-race` flag to the go build command.
  #
  # Note that it is also possible to specify this on per CIPD platform basis,
  # by using a dictionary as a value, similar to the `cgo` above.
  #
  # The flag will only be attached to the go build command if:
  #   * the value is set to 1
  #   * `-race` is supported on the target platform:
  #     * As of Aug 2022, it is only supported on
  #       * linux-amd64
  #       * freebsd-amd64
  #       * mac-amd64
  #       * windows-amd64
  #       * linux-ppc64le
  #       * linux-arm64 (only for 48-bit VMA)
  #   * CGO is enabled.
  race: 0

# Path to the root of the package source files on the system we're building
# the package from. Can be absolute or relative to the path of the *.yaml
# file itself.
root: ../..

# Copies apply after the build, but before creating the package. `dst` files
# will automatically be added as data 'file' entries.
#
# generate_bat_shim works as it does for data 'file' entries below, and is
# optional. platforms optionally specifies which platforms to target. If
# unspecified, targets all platforms.
copies:
  - src: some_built_file
    dst: some_other_file
    generate_bat_shim: true
    platforms:
    - windows-386
    - windows-amd64
    - windows-arm64

# Symlinks apply after the build, but before creating the package. `dst` links
# are automatically be added as data 'file' entries. These will only run when
# targetting posix platforms (mac, linux), and are otherwise ignored.
posix_symlinks:
  - src: some_file
    dst: some_link_name

# CIPD packages may be exported into the build directory by embedding an
# ensure file (https://pkg.go.dev/go.chromium.org/luci/cipd/client/cipd/ensure).
# In this specification, ${target_platform} may be used for the cross-compile
# target, where as ${platform} refers to the host platform.
# Make sure to also include this path in `data` below.
cipd_export: |
  @Subdir pkg/path
  cipd/pkg/path/${target_platform} version:2.0

data:
  # 'dir' section adds a subdirectory of 'root' to the package. In this case
  # it will scan directory <yaml_path>/../../a/b/c and put files into a/b/c
  # directory of the package.
  - dir: a/b/c
    # A list of regular expressions for files to exclude from the package.
    # Syntax is defined at http://golang.org/pkg/regexp/syntax/. Each expression
    # is implicitly wrapped into ^...$. The tests are applied to paths relative
    # to 'dir', e.g. 'bin/active' regexp matches only single file
    # <yaml_path>/../../a/b/c/bin/active.
    exclude:
      - bin/activate
      - .*\.pyc

  # 'file' section adds a single file to the package.
  - file: run.py

  # Exe files can also be augmented with *.bat shims, residing in same dir.
  # Kicks in only when the package is targeting Windows.
  - file: cipd.exe
    generate_bat_shim: true

  # Only uploading the package when specified files in the cipd package changed.
  # It will generate on-change tags and search if there is any package with
  # same set of tags. If no such package available, package will be uploaded. In
  # other case only tag will be updated on the package with same set of tags.
  - file: vpython
    upload_on_change: true

  # Include the CIPD package installed above.
  - dir: pkg/path
```

Following features of the package definition are implemented by `build.py`
(basically anything related to the process of building the code and preparing
all necessary files for packaging):

* `platforms`
* `go_build_environ`
* `go_packages`
* `copies`
* `posix_symlinks`
* `generate_bat_shim`
* `upload_on_change`


Strings interpolation
---------------------

Any string in package definition can reference a variable via ${var_name}, for
example:

```yaml
package: infra/tools/cipd/${platform}
```

Available variables are defined in [build.py](build.py) in `get_package_vars`:

* `${exe_suffix}` is `.exe` on Windows and empty string on other platforms. If
  cross-compiling to Windows, it is also set to `.exe` regardless of the host
  platform.
* `${platform}` defines where build.py is running (if not cross-compiling) or
  what the target platform is (when cross-compiling), as `(flavor)-(bitness)`
  string. It is suitable for packages that do not depend much on the exact
  version of the OS, for example packages with statically linked binaries.
  Examples of possible combinations:
    * linux-amd64
    * linux-386
    * linux-armv6l
    * mac-amd64
    * mac-386
    * windows-amd64
    * windows-arm64
    * windows-386
    * ...

See [packages](packages/) for examples of package definitions.


Build script
------------

[build.py](build.py) script does the following:

* Rebuilds all necessary Go code from scratch and installs binaries into
  `GOBIN`.
* Enumerates `packages/` directory for package definition files, builds and
  (if `--upload` option is passed) uploads CIPD packages to
  [the repository](https://chrome-infra-packages.appspot.com).
* Stores built packages into `out/` (as `*.cipd` files).

Package definition files can assume that Go infra code is built and all
artifacts are installed in `GOBIN` (which is go/bin).

You can also pass one or more *.yaml file names to build only specific packages:

    build.py bbagent cipd_client


Verifying a package
-------------------

To install a built package locally use cipd client binary (it is built by
build.py as well). For example, to rebuild and install bbagent.cipd into
./install_dir, run:

    cd infra.git/
    rm -rf install_dir
    ./build/build.py bbagent
    ./go/bin/cipd pkg-deploy -root=install_dir build/out/bbagent.cipd
    cd install_dir


Package tests
-------------

test_package.py script can be used to run simple package integrity tests to
verify a built package looks good after deploy.

For each *.yaml in packages/* there can be corresponding *.py file in tests/*
that is invoked by test_package.py to check that deployed package looks good.

Basically test_package.py does the following:

* Installs a CIPD file to a local directory or update currently installed
  version there (if `--work-dir` is used).
* Runs `python test/<name>.py` with cwd == installation directory.
* If test returns 0, considers it success, otherwise - failure.

Thus to test that bbagent.cipd package works, one can do the following:

    ./build/build.py bbagent
    ./build/test_packages.py bbagent

test_packages.py is used on CI builders to verify packages look good before
uploading them.


Cross compilation of Go code
----------------------------

If `--cipd-platform` is given, `build.py` sets up matching `GOOS`, `GOARCH`,
`GOARM` etc. environment variables, overriding any existing values. This enables
Go code cross-compilation for the target platform. This is what usually happens
on CI builders.

If `--cipd-platform` is not set, its value will be derived based on existing
`GOOS`, `GOARCH`, `GOARM`, etc. env vars in the environment (if any). If they
aren't set, `--cipd-platform` will be derived based on the host environment.
This is what usually happens when running `build.py` locally to test stuff.

Built packages have `+${platform}` suffix in file names and coexist with
host-native package in build output directory. When uploading packages (via
`build.py --no-rebuild --upload`), the same `--cipd-platform` logic is used to
figure out what flavor of built packages to pick (i.e. what `+${platform}`
suffix to filter built packages on).

Cross compiling toolset doesn't include C compiler, so the binaries are built in
`CGO_ENABLED=0` mode by default, meaning some stdlib functions that depend on
libc are not working or working differently compared to natively built
executables.
