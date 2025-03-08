use_relative_paths = True
git_dependencies = 'SUBMODULES'

deps = {
  "cipd/gcloud": {
    'packages': [
      {
        'package': 'infra/3pp/tools/gcloud/${{os=mac,linux}}-${{arch=amd64,arm64}}',
        'version': 'version:2@463.0.0.chromium.4',
      }
    ],
    'dep_type': 'cipd',
  },

  "cipd": {
    'packages': [
      {
        'package': 'infra/3pp/tools/protoc/${{os}}-${{arch=amd64,arm64}}',
        'version': 'version:3@30.0',
      },

      {
        'package': 'infra/3pp/tools/nodejs/${{os=linux,mac}}-${{arch}}',
        'version': 'version:2@20.10.0',
      },

      {
        'package': 'infra/3pp/tools/cloud-tasks-emulator/${{os=linux,mac}}-${{arch}}',
        'version': 'version:2@1.1.1',
      },

      # TODO: Remove the arch pinning for the following two packages below when
      # they're rolled forward to a version that includes linux-arm64.
      {
        'package': 'infra/tools/luci/logdog/logdog/${{os}}-${{arch=amd64}}',
        'version': 'git_revision:fe9985447e6b95f4907774f05e9774f031700775',
      },

      {
        'package': 'infra/tools/cloudbuildhelper/${{os=mac,linux}}-${{arch=amd64}}',
        'version': 'git_revision:a3e0efd8e13d9f77fffc7f536da450d9fdd7fd56',
      },

      # TODO: These should be built via 3pp instead.
      # See: docs/legacy/build_adb.md
      {
        'package': 'infra/adb/${{platform=linux-amd64}}',
        'version': 'adb_version:1.0.36',
      },
      # See: docs/legacy/build_fastboot.md
      {
        'package': 'infra/fastboot/${{platform=linux-amd64}}',
        'version': 'fastboot_version:5943271ace17',
      },

      {
        'package': 'infra/3pp/tools/golangci-lint/${{platform}}',
        'version': 'version:3@1.64.5',
      },
    ],
    'dep_type': 'cipd',
  },

  # Hosts legacy packages needed by the infra environment
  "cipd/legacy": {
    'packages': [
      {
        'package': 'infra/3pp/tools/protoc/${{os}}-${{arch=amd64}}',
        'version': 'version:2@3.17.3',
      },
      # Needed for python2 test.py tests
      {
        'package': 'infra/tools/luci/vpython/${{platform}}',
        'version': 'git_revision:9c01a50642c8e86b36b3d1fe9b829bca9ffb8e47',
      },
    ],
    'dep_type': 'cipd',
  },

  "cipd/result_adapter": {
    'packages': [
      {
        'package': 'infra/tools/result_adapter/${{platform}}',
        'version': 'git_revision:85566a4c3a814796e0f98fd0a0682fa178ddb268',
      },
    ],
    'dep_type': 'cipd',
  },
}

recursedeps = ['luci']
