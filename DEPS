use_relative_paths = True
git_dependencies = 'SYNC'
vars = {
  "chromium_git": "https://chromium.googlesource.com",
  "external_github": "https://chromium.googlesource.com/external/github.com",

  # This can be used to override the python used for generating the ENV python
  # environment. This is only needed when developing or testing python 2.7
  # appengine apps.
  "infra_env_python": "disabled",
  #
  # This is used during the transition phase of moving infra repos to git
  # submodules. To add new deps here check with the Chrome Source Team.
  "infra_superproject_checkout": False,

  # 'magic' text to tell depot_tools that git submodules should be accepted but
  # but parity with DEPS file is expected.
  'SUBMODULE_MIGRATION': 'True'
}

deps = {
  "luci":
     "{chromium_git}/infra/luci/luci-py@" +
     "bb79f92490843b892b1efda51cb83be868da65c5",

  "go/src/go.chromium.org/luci":
     "{chromium_git}/infra/luci/luci-go@" +
     "b889fb8080f065a9a8941fe3d9f1189f0a7ea43c",

  "go/src/go.chromium.org/chromiumos/config":
     "{chromium_git}/chromiumos/config@" +
     "7a09a163a7133a811efab2e442b13a00815d59d8",

  "go/src/go.chromium.org/chromiumos/infra/proto":
     "{chromium_git}/chromiumos/infra/proto@" +
     "45489ee50743d798a84f8130f27b9b09f583d170",

  # Appengine third_party DEPS
  "appengine/third_party/bootstrap":
     "{external_github}/twbs/bootstrap.git@" +
     "b4895a0d6dc493f17fe9092db4debe44182d42ac",

  "appengine/third_party/cloudstorage":
     "{external_github}/GoogleCloudPlatform/appengine-gcs-client.git@" +
     "76162a98044f2a481e2ef34d32b7e8196e534b78",

  "appengine/third_party/six":
     "{external_github}/benjaminp/six.git@" +
     "65486e4383f9f411da95937451205d3c7b61b9e1",

  "appengine/third_party/oauth2client":
     "{external_github}/google/oauth2client.git@" +
     "e8b1e794d28f2117dd3e2b8feeb506b4c199c533",

  "appengine/third_party/uritemplate":
     "{external_github}/uri-templates/uritemplate-py.git@" +
     "1e780a49412cdbb273e9421974cb91845c124f3f",

  "appengine/third_party/httplib2":
     "{external_github}/jcgregorio/httplib2.git@" +
     "058a1f9448d5c27c23772796f83a596caf9188e6",

  "appengine/third_party/endpoints-proto-datastore":
     "{external_github}/GoogleCloudPlatform/endpoints-proto-datastore.git@" +
     "971bca8e31a4ab0ec78b823add5a47394d78965a",

  "appengine/third_party/difflibjs":
     "{external_github}/qiao/difflib.js.git@"
     "e11553ba3e303e2db206d04c95f8e51c5692ca28",

  "appengine/third_party/pipeline":
     "{external_github}/GoogleCloudPlatform/appengine-pipelines.git@" +
     "58cf59907f67db359fe626ee06b6d3ac448c9e15",

  "appengine/third_party/google-api-python-client":
     "{external_github}/google/google-api-python-client.git@" +
     "49d45a6c3318b75e551c3022020f46c78655f365",

  "appengine/third_party/gae-pytz":
     "{chromium_git}/external/code.google.com/p/gae-pytz/@" +
     "4d72fd095c91f874aaafb892859acbe3f927b3cd",

  "appengine/third_party/npm_modules": {
     "url":
        "{chromium_git}/infra/third_party/npm_modules.git@" +
        "81e9eccce6089f4155fdc1935b1d430bd6411b95",
     "condition": "checkout_linux or checkout_mac",
  },

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
        'version': 'version:3@29.3',
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
        'version': 'version:3@1.62.0',
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

hooks = [
]

recursedeps = ['luci']
