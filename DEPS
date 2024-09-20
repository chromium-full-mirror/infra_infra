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
  # Used to initiate bootstrapping.
  #
  # This commit resolves to tag "16.7.12".
  "bootstrap/virtualenv-ext":
     "{external_github}/pypa/virtualenv@" +
     "fdfec65ff031997503fb409f365ee3aeb4c2c89f",

  "luci":
     "{chromium_git}/infra/luci/luci-py@" +
     "d6e7503c2fbe25419c69f5731745fe1bdd3c59f1",

  "go/src/go.chromium.org/luci":
     "{chromium_git}/infra/luci/luci-go@" +
     "dde1036169e325f6a7602e1cbd96b6b000980c55",

  "go/src/go.chromium.org/chromiumos/config":
     "{chromium_git}/chromiumos/config@" +
     "ef28f15ea6d7c691a4a3d4516adf90b204f0fc52",

  "go/src/go.chromium.org/chromiumos/infra/proto":
     "{chromium_git}/chromiumos/infra/proto@" +
     "becc5d0c2a77d27e16efffd365bc6ddacdb116ea",

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
        'package': 'infra/3pp/tools/gcloud/${{os=mac,linux}}-${{arch=amd64}}',
        'version': 'version:2@463.0.0.chromium.4',
      }
    ],
    'dep_type': 'cipd',
  },

  "cipd": {
    'packages': [
      {
        'package': 'infra/3pp/tools/protoc/${{os}}-${{arch=amd64}}',
        'version': 'version:2@26.1',
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
        'version': 'git_revision:b91a98c4be4d7a48beccb785f88c76656925d187',
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
        'version': 'version:2@1.54.2',
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
  {
    "pattern": ".",
    "condition": "'{infra_env_python}' != 'disabled'",
    "action": [
      Var("infra_env_python"), "-u", "./bootstrap/bootstrap.py",
      "--deps_file", "bootstrap/deps.pyl", "ENV"
    ],
  },
]

recursedeps = ['luci']
