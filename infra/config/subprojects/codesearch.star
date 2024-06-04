# Copyright 2019 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Definitions of resources for Code Search system."""

load("//lib/build.star", "build")
load("//lib/infra.star", "infra")
load("//lib/led.star", "led")

luci.bucket(
    name = "codesearch",
    acls = [
        acl.entry(
            roles = acl.BUILDBUCKET_TRIGGERER,
            users = "luci-scheduler@appspot.gserviceaccount.com",
        ),
        acl.entry(
            roles = acl.BUILDBUCKET_TRIGGERER,
            groups = "mdb/chrome-ops-source",
        ),
    ],
)

# Shadow bucket of `codesearch`, for led builds.
luci.bucket(
    name = "codesearch.shadow",
    shadows = "codesearch",
    constraints = luci.bucket_constraints(
        pools = ["luci.infra.codesearch"],
    ),
    bindings = [
        luci.binding(
            roles = "role/buildbucket.creator",
            groups = ["flex-try-led-users", "mdb/chrome-troopers", "google/luci-task-force@google.com"],
        ),
    ],
    dynamic = True,
)

luci.realm(name = "pools/codesearch")

led.users(
    groups = [
        "mdb/chrome-troopers",
        "google/luci-task-force@google.com",
        "flex-try-led-users",
    ],
    task_realm = "codesearch",
    pool_realm = "pools/codesearch",
)

luci.console_view(
    name = "codesearch",
    repo = "https://chromium.googlesource.com/chromium/src",
    include_experimental_builds = True,
    refs = ["refs/heads/main"],
)

def builder(
        name,
        executable,

        # Builder props.
        os = None,
        cpu_cores = None,
        cpu = None,
        machine_type = None,
        properties = None,
        builder_group_property_name = "mastername",
        caches = None,
        execution_timeout = None,

        # Console presentation.
        category = None,
        short_name = None,

        # Scheduler parameters.
        triggered_by = None,
        schedule = None):
    """A generic code search builder.

    Args:
      name: name of the builder.
      executable: a recipe to run.
      os: the target OS dimension.
      cpu_cores: the CPU cores count dimension (as string).
      cpu: the CPU architecture (as string)
      machine_type: the machine type.
      properties: a dict with properties to pass to the recipe.
      builder_group_property_name: the name of the property to set with the
        builder group.
      caches: a list of swarming.cache(...).
      execution_timeout: how long it is allowed to run.
      category: the console category to put the builder under.
      short_name: a short name for the console.
      triggered_by: a list of builders that trigger this one.
      schedule: if given, run the builder periodically under this schedule.
    """

    # Add mastername property so that the gen recipes can find the right
    # config in mb_config.pyl.
    properties = properties or {}
    properties[builder_group_property_name] = "chromium.infra.codesearch"
    scandeps_server = False
    if os and os.lower().startswith("mac"):
        scandeps_server = True

    properties["$build/reclient"] = {
        "instance": "rbe-chromium-trusted",
        "metrics_project": "chromium-reclient-metrics",
        "scandeps_server": scandeps_server,
    }

    luci.builder(
        name = name,
        bucket = "codesearch",
        executable = executable,
        properties = properties,
        dimensions = {
            "os": os or "Ubuntu-22",
            "cpu": cpu or "x86-64",
            "cores": cpu_cores or "8",
            "machine_type": machine_type or "n1-standard-8",
            "pool": "luci.infra.codesearch",
        },
        caches = caches,
        service_account = "infra-codesearch@chops-service-accounts.iam.gserviceaccount.com",
        execution_timeout = execution_timeout,
        build_numbers = True,
        triggered_by = [triggered_by] if triggered_by else None,
        schedule = schedule,
        experiments = {"luci.recipes.use_python3": 100},
        shadow_service_account = "chromium-try-builder@chops-service-accounts.iam.gserviceaccount.com",
    )

    luci.console_view_entry(
        builder = name,
        console_view = "codesearch",
        category = category,
        short_name = short_name,
    )

def chromium_genfiles(
        short_name,
        name,
        recipe_properties,
        os = None,
        cpu_cores = None,
        cpu = None,
        machine_type = None,
        xcode_build_version = None):
    """A builder for generating kzips for chromium/src.

      In recipe_properties, you can specify the following parameters:
      - compile_targets: the compile targets.
      - platform: The platform for which the code is compiled.
      - experimental: Whether to mark Kythe uploads as experimental.
      - sync_generated_files: Whether to sync generated files into a git repo.
      - corpus: Kythe corpus to specify in the kzip.
      - build_config: Kythe build config to specify in the kzip.
      - gen_repo_branch: Which branch in the generated files repo to sync to.
      - gen_repo_out_dir: Which directory under src/out to write gen files to.
    """
    builder(
        name = name,
        executable = build.recipe("chromium_codesearch"),
        properties = {
            "recipe_properties": recipe_properties,
            "xcode_build_version": xcode_build_version,
        },
        builder_group_property_name = "builder_group",
        os = os,
        cpu_cores = cpu_cores,
        cpu = cpu,
        machine_type = machine_type,
        caches = [swarming.cache(
            path = "generated",
            name = "codesearch_git_genfiles_repo",
        )],
        execution_timeout = 9 * time.hour,
        category = "gen",
        short_name = short_name,
        # Gen builders are triggered by the initiator's recipe.
        triggered_by = "codesearch-gen-chromium-initiator",
    )

# buildifier: disable=function-docstring
def update_submodules_mirror(
        name,
        short_name,
        source_repo,
        target_repo,
        extra_submodules = None,
        triggered_by = None,
        ref_patterns = None,
        push_to_refs_cs = False,
        execution_timeout = time.hour):
    properties = {
        "source_repo": source_repo,
        "target_repo": target_repo,
    }
    if extra_submodules:
        properties["extra_submodules"] = extra_submodules
    if ref_patterns:
        properties["ref_patterns"] = ref_patterns
    properties["push_to_refs_cs"] = push_to_refs_cs
    builder(
        name = name,
        execution_timeout = execution_timeout,
        executable = infra.recipe("update_submodules_mirror"),
        properties = properties,
        caches = [swarming.cache("codesearch_update_submodules_mirror")],
        category = "update-submodules-mirror",
        short_name = short_name,
        triggered_by = triggered_by,
    )

# Runs every four hours (at predictable times).
builder(
    name = "codesearch-gen-chromium-initiator",
    executable = build.recipe("chromium_codesearch_initiator"),
    properties = {
        "builders": [
            "codesearch-gen-chromium-android",
            "codesearch-gen-chromium-chromiumos",
            "codesearch-gen-chromium-cronet",
            "codesearch-gen-chromium-fuchsia",
            "codesearch-gen-chromium-ios",
            "codesearch-gen-chromium-lacros",
            "codesearch-gen-chromium-linux",
            "codesearch-gen-chromium-mac",
            "codesearch-gen-chromium-webview",
            "codesearch-gen-chromium-win",
        ],
        "source_repo": "https://chromium.googlesource.com/codesearch/chromium/src",
    },
    builder_group_property_name = "builder_group",
    execution_timeout = 5 * time.hour,
    category = "gen|init",
    schedule = "0 */4 * * *",
)

chromium_genfiles(
    short_name = "and",
    name = "codesearch-gen-chromium-android",
    recipe_properties = {
        "compile_targets": ["all"],
        "platform": "android",
        "sync_generated_files": True,
        "gen_repo_branch": "main",
        # Generated files will end up in out/android-Debug/gen.
        "gen_repo_out_dir": "android-Debug",
        "corpus": "chromium.googlesource.com/codesearch/chromium/src//main",
        "build_config": "android",
    },
    machine_type = "n1-highmem-8",
)

chromium_genfiles(
    short_name = "cronet",
    name = "codesearch-gen-chromium-cronet",
    recipe_properties = {
        "compile_targets": [
            "cronet_package",
            "cronet_perf_test_apk",
            "cronet_sample_test_apk",
            "cronet_smoketests_missing_native_library_instrumentation_apk",
            "cronet_smoketests_platform_only_instrumentation_apk",
            "cronet_test_instrumentation_apk",
            "cronet_unittests_android",
            "net_unittests",
        ],
        "platform": "cronet",
        "sync_generated_files": True,
        "gen_repo_branch": "main",
        # Generated files will end up in out/android-Debug/gen.
        "gen_repo_out_dir": "cronet-Debug",
        "corpus": "chromium.googlesource.com/codesearch/chromium/src//main",
        "build_config": "cronet",
    },
)

chromium_genfiles(
    short_name = "wbv",
    name = "codesearch-gen-chromium-webview",
    recipe_properties = {
        "compile_targets": ["system_webview_apk"],
        "platform": "webview",
        "sync_generated_files": True,
        "gen_repo_branch": "main",
        # Generated files will end up in out/webview-Debug/gen.
        "gen_repo_out_dir": "webview-Debug",
        "corpus": "chromium.googlesource.com/codesearch/chromium/src//main",
        "build_config": "webview",
    },
)

chromium_genfiles(
    short_name = "cro",
    name = "codesearch-gen-chromium-chromiumos",
    recipe_properties = {
        # TODO(crbug.com/1323934): Get the below compile targets from the
        # chromium_tests recipe module.
        # The below compile targets were used by the "Linux ChromiumOS Full"
        # builder on (2016-12-16).
        "compile_targets": [
            "base_unittests",
            "browser_tests",
            "chromeos_unittests",
            "components_unittests",
            "compositor_unittests",
            "content_browsertests",
            "content_unittests",
            "crypto_unittests",
            "dbus_unittests",
            "device_unittests",
            "gcm_unit_tests",
            "google_apis_unittests",
            "gpu_unittests",
            "interactive_ui_tests",
            "ipc_tests",
            "media_unittests",
            "message_center_unittests",
            "nacl_loader_unittests",
            "net_unittests",
            "ppapi_unittests",
            "printing_unittests",
            "remoting_unittests",
            "sandbox_linux_unittests",
            "sql_unittests",
            "ui_base_unittests",
            "unit_tests",
            "url_unittests",
            "views_unittests",
        ],
        "platform": "chromeos",
        "sync_generated_files": True,
        "gen_repo_branch": "main",
        # Generated files will end up in out/chromeos-Debug/gen.
        "gen_repo_out_dir": "chromeos-Debug",
        "corpus": "chromium.googlesource.com/codesearch/chromium/src//main",
        "build_config": "chromeos",
    },
)

chromium_genfiles(
    short_name = "fch",
    name = "codesearch-gen-chromium-fuchsia",
    recipe_properties = {
        "compile_targets": ["all"],
        "platform": "fuchsia",
        "sync_generated_files": True,
        "gen_repo_branch": "main",
        # Generated files will end up in out/fuchsia-Debug/gen.
        "gen_repo_out_dir": "fuchsia-Debug",
        "corpus": "chromium.googlesource.com/codesearch/chromium/src//main",
        "build_config": "fuchsia",
    },
)

chromium_genfiles(
    short_name = "ios",
    name = "codesearch-gen-chromium-ios",
    recipe_properties = {
        "compile_targets": ["all"],
        "platform": "ios",
        "sync_generated_files": True,
        # Generated files will end up in out/ios-Debug/gen.
        "gen_repo_out_dir": "ios-Debug",
        "gen_repo_branch": "main",
        "corpus": "chromium.googlesource.com/codesearch/chromium/src//main",
        "build_config": "ios",
    },
    os = "Mac-13|Mac-14",
    cpu_cores = "8",
    cpu = "arm64",
    machine_type = "n1-highcpu-8",
    xcode_build_version = "15a240d",
)

chromium_genfiles(
    short_name = "lcr",
    name = "codesearch-gen-chromium-lacros",
    recipe_properties = {
        "compile_targets": ["all"],
        "platform": "lacros",
        "sync_generated_files": True,
        "gen_repo_branch": "main",
        # Generated files will end up in out/lacros-Debug/gen.
        "gen_repo_out_dir": "lacros-Debug",
        "corpus": "chromium.googlesource.com/codesearch/chromium/src//main",
        "build_config": "lacros",
    },
)

chromium_genfiles(
    short_name = "lnx",
    name = "codesearch-gen-chromium-linux",
    recipe_properties = {
        "compile_targets": ["all"],
        "platform": "linux",
        "sync_generated_files": True,
        "gen_repo_branch": "main",
        # Generated files will end up in out/linux-Debug/gen.
        "gen_repo_out_dir": "linux-Debug",
        "corpus": "chromium.googlesource.com/codesearch/chromium/src//main",
        "build_config": "linux",
    },
)

chromium_genfiles(
    short_name = "mac",
    name = "codesearch-gen-chromium-mac",
    recipe_properties = {
        "compile_targets": ["all"],
        "platform": "mac",
        "sync_generated_files": True,
        # Generated files will end up in out/mac-Debug/gen.
        "gen_repo_out_dir": "mac-Debug",
        "gen_repo_branch": "main",
        "corpus": "chromium.googlesource.com/codesearch/chromium/src//main",
        "build_config": "mac",
    },
    os = "Mac-13|Mac-14",
    cpu_cores = "8",
    cpu = "arm64",
    machine_type = "n1-highcpu-8",
)

chromium_genfiles(
    short_name = "win",
    name = "codesearch-gen-chromium-win",
    recipe_properties = {
        "compile_targets": ["all"],
        "platform": "win",
        "sync_generated_files": True,
        "gen_repo_branch": "main",
        # Generated files will end up in out/win-Debug/gen.
        "gen_repo_out_dir": "win-Debug",
        "corpus": "chromium.googlesource.com/codesearch/chromium/src//main",
        "build_config": "win",
    },
    os = "Windows-10",
    cpu_cores = "32",
    machine_type = "n1-standard-32",
)

update_submodules_mirror(
    name = "codesearch-update-submodules-mirror-src",
    short_name = "src",
    source_repo = "https://chromium.googlesource.com/chromium/src",
    target_repo = "https://chromium.googlesource.com/codesearch/chromium/src",
    extra_submodules = ["src/out=https://chromium.googlesource.com/chromium/src/out"],
    ref_patterns = [
        "refs/heads/main",
        "refs/branch-heads/4044",  # M81
        "refs/branch-heads/4103",  # M83
    ],
    triggered_by = luci.gitiles_poller(
        name = "codesearch-src-trigger",
        bucket = "codesearch",
        repo = "https://chromium.googlesource.com/chromium/src",
    ),
    execution_timeout = 2 * time.hour,
)
