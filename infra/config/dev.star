#!/usr/bin/env lucicfg
# Copyright 2019 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""LUCI project configuration for the development instance of LUCI.

After modifying this file execute it ('./dev.star') to regenerate the configs.
This is also enforced by PRESUBMIT.py script.
"""

load("//lib/infra.star", "infra")

lucicfg.check_version("1.39.4", "Please update depot_tools")

lucicfg.enable_experiment("crbug.com/1338648")

# Global recipe defaults
luci.recipe.defaults.cipd_version.set("refs/heads/main")
luci.recipe.defaults.use_bbagent.set(True)

lucicfg.config(
    config_dir = "generated",
    tracked_files = [
        "cr-buildbucket-dev.cfg",
        "luci-logdog-dev.cfg",
        "luci-milo-dev.cfg",
        "luci-notify-dev.cfg",
        "luci-notify-dev/email-templates/*",
        "luci-scheduler-dev.cfg",
        "realms-dev.cfg",
        "tricium-dev.cfg",
    ],
    fail_on_warnings = True,
    lint_checks = ["default"],
)

# Just copy tricium-dev.cfg as is to the outputs.
lucicfg.emit(
    dest = "tricium-dev.cfg",
    data = io.read_file("tricium-dev.cfg"),
)

luci.project(
    name = "infra",
    dev = True,
    buildbucket = "cr-buildbucket-dev.appspot.com",
    logdog = "luci-logdog-dev.appspot.com",
    milo = "luci-milo-dev.appspot.com",
    notify = "luci-notify-dev.appspot.com",
    scheduler = "luci-scheduler-dev.appspot.com",
    swarming = "chromium-swarm-dev.appspot.com",
    acls = [
        acl.entry(
            roles = [
                acl.BUILDBUCKET_READER,
                acl.LOGDOG_READER,
                acl.PROJECT_CONFIGS_READER,
                acl.SCHEDULER_READER,
            ],
            groups = "all",
        ),
        acl.entry(
            roles = acl.SCHEDULER_OWNER,
            groups = "project-infra-troopers",
        ),
        acl.entry(
            roles = acl.LOGDOG_WRITER,
            groups = "luci-logdog-chromium-dev-writers",
        ),
        acl.entry(
            roles = acl.BUILDBUCKET_TRIGGERER,
            users = "adhoc-testing@luci-token-server-dev.iam.gserviceaccount.com",
        ),
    ],
    bindings = [
        # LED users.
        luci.binding(
            roles = "role/swarming.taskTriggerer",
            groups = ["mdb/chrome-troopers", "mdb/chrome-sre-ops-syd-interns"],
        ),
    ],
    enforce_realms_in = [
        "cr-buildbucket-dev",
        "luci-scheduler-dev",
    ],
)

luci.logdog(
    gs_bucket = "chromium-luci-logdog",
    cloud_logging_project = "luci-logdog-dev",
)

luci.bucket(name = "ci")

luci.bucket(
    name = "ci.shadow",
    shadows = "ci",
    constraints = luci.bucket_constraints(
        pools = ["luci.chromium.ci"],
        service_accounts = ["adhoc-testing@luci-token-server-dev.iam.gserviceaccount.com"],
    ),
    bindings = [
        luci.binding(
            roles = "role/buildbucket.creator",
            groups = "mdb/chrome-troopers",
        ),
    ],
    dynamic = True,
)

#TODO(b/258041976): Create a new bucket for experimentation
luci.bucket(
    name = "vm",
    bindings = [
        # LED users.
        luci.binding(
            roles = "role/swarming.taskTriggerer",
            groups = "chromium-swarming-dev-led-access",
        ),
    ],
)

luci.builder.defaults.experiments.set({
    "luci.buildbucket.bbagent_getbuild": 100,
    "luci.buildbucket.backend_alt": 100,
})
luci.builder.defaults.execution_timeout.set(30 * time.minute)

luci.task_backend(
    name = "swarming_task_backend_dev",
    target = "swarming://chromium-swarm-dev",
    config = {"bot_ping_tolerance": 120},
)

luci.builder.defaults.backend_alt.set("swarming_task_backend_dev")
luci.builder.defaults.swarming_host.set("chromium-swarm-dev.appspot.com")

def ci_builder(
        name,
        os,
        recipe = "infra_continuous",
        tree_closing = False):
    infra.builder(
        name = name,
        bucket = "ci",
        executable = infra.recipe(recipe, use_python3 = True),
        os = os,
        cpu = "x86-64",
        pool = "luci.chromium.ci",
        service_account = "adhoc-testing@luci-token-server-dev.iam.gserviceaccount.com",
        triggered_by = [infra.poller()],
        notifies = ["dev tree closer"] if tree_closing else None,
    )

luci.tree_closer(
    name = "dev tree closer",
    tree_status_host = "infra-status.appspot.com",
    template = "default",
)

luci.notifier_template(
    name = "default",
    body = "{{ stepNames .MatchingFailedSteps }} on {{ buildUrl . }} {{ .Build.Builder.Builder }} from {{ .Build.Output.GitilesCommit.Id }}",
)
luci.notify(tree_closing_enabled = True)

ci_builder(name = "infra-continuous-jammy-64", os = "Ubuntu-22.04", tree_closing = True)
ci_builder(name = "infra-continuous-win10-64", os = "Windows-10", tree_closing = True)
ci_builder(name = "infra-continuous-win11-64", os = "Windows-11", tree_closing = True)

#TODO(b/258041976): Created for experimenting with mac os VMs
luci.builder(
    name = "mac-arm-vm-launcher",
    bucket = "vm",
    executable = infra.recipe("vm_launcher", use_python3 = True),
    service_account = "adhoc-testing@luci-token-server-dev.iam.gserviceaccount.com",
    dimensions = {
        "os": "Mac-11|Mac-12",
        "pool": "chromium.tests",
        "cpu": "arm64",
    },
)

def adhoc_builder(
        name,
        os,
        executable,
        extra_dims = None,
        properties = None,
        experiments = None,
        schedule = None,
        triggered_by = None,
        description_html = None,
        custom_metrics = None):
    dims = {"os": os, "cpu": "x86-64", "pool": "luci.chromium.ci"}
    if extra_dims:
        dims.update(**extra_dims)
    luci.builder(
        name = name,
        bucket = "ci",
        executable = executable,
        description_html = description_html,
        dimensions = dims,
        properties = properties,
        experiments = experiments,
        service_account = "adhoc-testing@luci-token-server-dev.iam.gserviceaccount.com",
        build_numbers = True,
        schedule = schedule,
        triggered_by = triggered_by,
        custom_metrics = custom_metrics,
    )

adhoc_builder(
    name = "build-proto-linux",
    os = "Ubuntu",
    executable = luci.recipe(
        name = "futures:examples/background_helper",
        cipd_package = "infra/recipe_bundles/chromium.googlesource.com/infra/luci/recipes-py",
        use_python3 = True,
    ),
    schedule = "with 10m interval",
)
adhoc_builder(
    name = "build-proto-win",
    os = "Windows-10",
    executable = luci.recipe(
        name = "futures:examples/background_helper",
        cipd_package = "infra/recipe_bundles/chromium.googlesource.com/infra/luci/recipes-py",
        use_python3 = True,
    ),
    schedule = "with 10m interval",
)

adhoc_builder(
    name = "linux-rel-buildbucket",
    os = "Ubuntu-22.04",
    executable = luci.recipe(
        name = "placeholder",
        cipd_package = "infra/recipe_bundles/chromium.googlesource.com/infra/luci/recipes-py",
        use_python3 = True,
    ),
    properties = {
        "status": "SUCCESS",
        "steps": [
            {
                "name": "can_outlive_parent child",
                "child_build": {
                    "buildbucket": {
                        "builder": {
                            "project": "infra",
                            "bucket": "ci",
                            "builder": "linux-rel-buildbucket-child",
                        },
                    },
                    "life_time": "DETACHED",
                },
            },
            {
                "name": "cannot_outlive_parent child",
                "child_build": {
                    "id": "bounded_child",
                    "buildbucket": {
                        "builder": {
                            "project": "infra",
                            "bucket": "ci",
                            "builder": "linux-rel-buildbucket-child",
                        },
                    },
                    "life_time": "BUILD_BOUND",
                },
            },
            {
                "name": "collect children",
                "collect_children": {
                    "child_build_step_ids": ["bounded_child"],
                },
            },
        ],
    },
    schedule = "with 10m interval",
)

adhoc_builder(
    name = "linux-rel-buildbucket-child",
    os = "Ubuntu-22.04",
    executable = luci.recipe(
        name = "placeholder",
        cipd_package = "infra/recipe_bundles/chromium.googlesource.com/infra/luci/recipes-py",
        use_python3 = True,
    ),
    properties = {
        "status": "SUCCESS",
        "steps": [
            {
                "name": "hello",
                "fake_step": {
                    "duration_secs": 90,
                },
            },
        ],
    },
    custom_metrics = [
        buildbucket.custom_metric(
            name = "/chrome/infra/custom/dev/infra/builds/started",
            predicates = ['build.tags.get_value("bounded_child")=="True"'],
            extra_fields = {
                "bounded_child": 'build.tags.get_value("bounded_child")',
                "child_tracking_service": 'build.tags.get_value("child_tracking_service")',
            },
        ),
        buildbucket.custom_metric(
            name = "/chrome/infra/custom/dev/infra/builds/count",
            predicates = ['build.tags.get_value("bounded_child")=="True"'],
        ),
    ],
)

adhoc_builder(
    name = "linux-rel-buildbucket-noop",
    os = "Ubuntu-22.04",
    executable = luci.recipe(
        name = "placeholder",
        cipd_package = "infra/recipe_bundles/chromium.googlesource.com/infra/luci/recipes-py",
        use_python3 = True,
    ),
    description_html = "No-op builder to measure and monitor bbagent overhead",
    properties = {
        "status": "SUCCESS",
        "steps": [
            {
                "name": "hello",
                "fake_step": {
                    "duration_secs": 0,
                },
            },
        ],
    },
    schedule = "with 10m interval",
    experiments = {
        # TODO(b/325342884): Make it to 0 to test the old swarming pubsub
        # notification flow when Swarming migrates it to Go. Re-enable it after
        # the testing is done.
        "luci.buildbucket.backend_alt": 0,
    },
)

adhoc_builder(
    name = "linux-rel-buildbucket-swarming-task-backend",
    os = "Ubuntu-22.04",
    executable = luci.recipe(
        name = "placeholder",
        cipd_package = "infra/recipe_bundles/chromium.googlesource.com/infra/luci/recipes-py",
        use_python3 = True,
    ),
    properties = {
        "status": "SUCCESS",
        "steps": [
            {
                "name": "hello",
                "fake_step": {
                    "duration_secs": 90,
                },
            },
        ],
    },
    schedule = "triggered",
)

luci.notifier(
    name = "luci-notify-test-alerts",
    on_success = True,
    on_failure = True,
    notify_emails = ["luci-notify-test-alerts@chromium.org"],
    template = "test",
    notified_by = ["infra-continuous-jammy-64"],
)

luci.notifier_template(
    name = "test",
    body = """{{.Build.Builder | formatBuilderID}} notification

<a href="{{buildUrl .}}">Build {{.Build.Number}}</a>
has completed.

{{template "steps" .}}
""",
)

luci.notifier_template(
    name = "steps",
    body = """Renders steps.

<ol>
{{range $s := .Build.Steps}}
  <li>{{$s.Name}}</li>
{{end}}
</ol>
""",
)

################################################################################
## Realms used by skylab-staging-bot-fleet for its pools and admin tasks.
#
# The corresponding realms in the prod universe live in "chromeos" project.
# There's no "chromeos" project in the dev universe, so we define the realms
# here instead.

SKYLAB_ADMIN_SCHEDULERS = [
    "project-chromeos-skylab-schedulers",
    "mdb/chromeos-build-deputy",
]

luci.realm(
    name = "pools/skylab",
    bindings = [
        luci.binding(
            roles = "role/swarming.poolOwner",
            groups = "administrators",
        ),
        luci.binding(
            roles = "role/swarming.poolUser",
            groups = SKYLAB_ADMIN_SCHEDULERS,
        ),
        luci.binding(
            roles = "role/swarming.poolViewer",
            groups = "chromium-swarm-dev-view-all-bots",
        ),
    ],
)

luci.realm(
    name = "skylab-staging-bot-fleet/admin",
    bindings = [
        luci.binding(
            roles = "role/swarming.taskServiceAccount",
            users = "skylab-admin-task@chromeos-service-accounts-dev.iam.gserviceaccount.com",
        ),
        luci.binding(
            roles = "role/swarming.taskTriggerer",
            groups = SKYLAB_ADMIN_SCHEDULERS,
        ),
    ],
)

# TODO(crbug.com/1238772): remove after dev configs get prepared in "chromeos"
# project.
luci.realm(
    name = "pools/chromeos",
    bindings = [
        luci.binding(
            roles = "role/swarming.poolOwner",
            groups = "administrators",
        ),
        luci.binding(
            roles = "role/swarming.poolUser",
            groups = "chromium-swarm-dev-privileged-users",
        ),
        luci.binding(
            roles = "role/swarming.poolViewer",
            groups = "chromium-swarm-dev-view-all-bots",
        ),
    ],
)

################################################################################
## Realms used for Swarming client integration tests.

luci.realm(
    name = "pools/tests",
    bindings = [
        luci.binding(
            roles = "role/swarming.poolOwner",
            groups = ["mdb/chrome-troopers"],
        ),
        luci.binding(
            roles = "role/swarming.poolViewer",
            groups = "chromium-swarm-dev-view-all-bots",
        ),
        luci.binding(
            roles = "role/swarming.poolUser",
            groups = "project-infra-tests-submitters",
        ),
    ],
)

luci.realm(
    name = "tests",
    bindings = [
        luci.binding(
            roles = "role/swarming.taskTriggerer",
            groups = "project-infra-tests-submitters",
        ),
    ],
)

################################################################################
## Resources used for Buildbucket and Swarming load test.

luci.realm(
    name = "pools/loadtest",
    bindings = [
        # For led.
        luci.binding(
            roles = "role/swarming.poolUser",
            groups = ["mdb/chrome-troopers", "mdb/chrome-sre-ops-syd-interns"],
            users = "swarming-bot@luci-backend-dev.iam.gserviceaccount.com",
        ),
        luci.binding(
            roles = "role/swarming.taskTriggerer",
            users = "swarming-bot@luci-backend-dev.iam.gserviceaccount.com",
        ),
        luci.binding(
            roles = "role/swarming.poolViewer",
            projects = "infra",
        ),
    ],
)

luci.bucket(
    name = "loadtest",
    bindings = [
        luci.binding(
            roles = "role/buildbucket.triggerer",
            groups = "mdb/chrome-troopers",
            users = "adhoc-testing@luci-token-server-dev.iam.gserviceaccount.com",
        ),
        luci.binding(
            roles = "role/buildbucket.creator",
            groups = "mdb/chrome-troopers",
        ),
    ],
    shadows = "loadtest",
)

def fakebuild_builder(name, steps, sleep_min_sec, sleep_max_sec, build_numbers, wait_missing_cache, schedule = None):
    luci.builder(
        name = name,
        bucket = "loadtest",
        executable = luci.executable(
            name = "fakebuild",
            cipd_package = "infra/experimental/swarming/fakebuild/${platform}",
            cipd_version = "latest",
            cmd = ["fakebuild"],
        ),
        dimensions = {
            "os": "Linux",
            "cpu": "x86-64",
            "pool": "infra.loadtest.0",
        },
        properties = {
            "steps": steps,
            "sleep_min_sec": sleep_min_sec,
            "sleep_max_sec": sleep_max_sec,
        },
        service_account = "adhoc-testing@luci-token-server-dev.iam.gserviceaccount.com",
        execution_timeout = sleep_max_sec * steps * time.second + 10 * time.minute,
        build_numbers = build_numbers,
        experiments = {
            "luci.buildbucket.omit_default_packages": 100,
        },
        caches = [
            swarming.cache(
                path = "missing1",
                name = "missing1",
                wait_for_warm_cache = 3 * time.minute,
            ),
            swarming.cache(
                path = "missing2",
                name = "missing2",
                wait_for_warm_cache = 5 * time.minute,
            ),
        ] if wait_missing_cache else [],
        wait_for_capacity = True if wait_missing_cache else None,
        schedule = schedule,
    )

# Finishes in ~1min with 10 steps.
fakebuild_builder("fake-1m", 10, 2, 10, True, False, schedule = "triggered")
fakebuild_builder("fake-1m-no-bn", 10, 2, 10, False, False, schedule = "triggered")
fakebuild_builder("fake-1m-exp-slices", 10, 2, 10, False, True, schedule = "triggered")

# Finishes in ~10min with 100 steps.
fakebuild_builder("fake-10m", 100, 2, 10, True, False, schedule = "triggered")
fakebuild_builder("fake-10m-no-bn", 100, 2, 10, False, False, schedule = "triggered")

# Finishes in ~30min with 300 steps.
fakebuild_builder("fake-30m", 300, 2, 10, True, False, schedule = "triggered")
fakebuild_builder("fake-30m-no-bn", 300, 2, 10, False, False, schedule = "triggered")

# Finishes in ~1h with 600 steps.
fakebuild_builder("fake-1h", 600, 2, 10, True, False, schedule = "triggered")
fakebuild_builder("fake-1h-no-bn", 600, 2, 10, False, False, schedule = "triggered")

def fakebuild_tree_builder(name, children, batch_size, builder, sleep_min_sec, sleep_max_sec, build_numbers, schedule = None, wait_for_children = False):
    luci.builder(
        name = name,
        bucket = "loadtest",
        executable = luci.executable(
            name = "fakebuild",
            cipd_package = "infra/experimental/swarming/fakebuild/${platform}",
            cipd_version = "latest",
            cmd = ["fakebuild"],
        ),
        dimensions = {
            "os": "Linux",
            "cpu": "x86-64",
            "pool": "infra.loadtest.0",
        },
        properties = {
            "child_builds": {
                "builder": {
                    "project": "infra",
                    "bucket": "loadtest",
                    "builder": builder,
                },
                "children": children,
                "batch_size": batch_size,
                "sleep_min_sec": sleep_min_sec,
                "sleep_max_sec": sleep_max_sec,
                "wait_for_children": wait_for_children,
            },
        },
        service_account = "adhoc-testing@luci-token-server-dev.iam.gserviceaccount.com",
        build_numbers = build_numbers,
        experiments = {
            "luci.buildbucket.omit_default_packages": 100,
        },
        schedule = schedule,
    )

# Total build in one build tree:
# 1 + 10 + 10*20 + 10*20*20 = 4211
fakebuild_tree_builder("fake-tree-0", 10, 0, "fake-tree-1", 2, 10, True)
fakebuild_tree_builder("fake-tree-0-no-bn", 10, 0, "fake-tree-1-no-bn", 2, 10, False)

fakebuild_tree_builder("fake-tree-1", 20, 0, "fake-tree-2", 2, 10, True)
fakebuild_tree_builder("fake-tree-1-no-bn", 20, 0, "fake-tree-2-no-bn", 2, 10, False)

def fakebuild_search_builder(name, steps, search_steps, sleep_min_sec, sleep_max_sec, build_numbers):
    luci.builder(
        name = name,
        bucket = "loadtest",
        executable = luci.executable(
            name = "fakebuild",
            cipd_package = "infra/experimental/swarming/fakebuild/${platform}",
            cipd_version = "latest",
            cmd = ["fakebuild"],
        ),
        dimensions = {
            "os": "Linux",
            "cpu": "x86-64",
            "pool": "infra.loadtest.0",
        },
        properties = {
            "steps": steps,
            "sleep_min_sec": sleep_min_sec,
            "sleep_max_sec": sleep_max_sec,
            "search_builds": {
                "steps": search_steps,
                "sleep_min_sec": sleep_min_sec,
                "sleep_max_sec": sleep_max_sec,
            },
        },
        service_account = "adhoc-testing@luci-token-server-dev.iam.gserviceaccount.com",
        build_numbers = build_numbers,
        experiments = {
            "luci.buildbucket.omit_default_packages": 100,
        },
    )

# Builders run 100 sleep steps then do 10 search builds.
fakebuild_search_builder("fake-search", 100, 10, 2, 10, True)
fakebuild_search_builder("fake-search-no-bn", 100, 10, 2, 10, False)

fakebuild_tree_builder("fake-tree-2", 20, 2, "fake-search", 2, 10, True)
fakebuild_tree_builder("fake-tree-2-no-bn", 20, 2, "fake-search-no-bn", 2, 10, False, wait_for_children = True)

################################################################################
## Resources used for Cloudbuild backend test builder.
## TODO(crbug.com/1495858): Remove this section once CB as a backend is done
## being tested.

luci.bucket(name = "cloudbuild")

luci.task_backend(
    name = "cloudbuild_poc_backend",
    target = "cloudbuild://cloud-build-backend-poc",
)

luci.builder(
    name = "test-placeholder-recipe",
    bucket = "cloudbuild",
    backend_alt = "cloudbuild_poc_backend",
    executable = luci.recipe(
        name = "placeholder",
        cipd_package = "infra/recipe_bundles/chromium.googlesource.com/infra/luci/recipes-py",
        use_python3 = True,
    ),
    experiments = {
        "luci.buildbucket.backend_alt": 100,
    },
    properties = {
        "status": "SUCCESS",
        "steps": [
            {
                "name": "can_outlive_parent child",
                "child_build": {
                    "buildbucket": {
                        "builder": {
                            "project": "infra",
                            "bucket": "cloudbuild",
                            "builder": "test-placeholder-recipe-child",
                        },
                    },
                    "life_time": "DETACHED",
                },
            },
            {
                "name": "cannot_outlive_parent child",
                "child_build": {
                    "id": "bounded_child",
                    "buildbucket": {
                        "builder": {
                            "project": "infra",
                            "bucket": "cloudbuild",
                            "builder": "test-placeholder-recipe-child",
                        },
                    },
                    "life_time": "BUILD_BOUND",
                },
            },
            {
                "name": "collect children",
                "collect_children": {
                    "child_build_step_ids": ["bounded_child"],
                },
            },
        ],
    },
    service_account = "adhoc-testing@luci-token-server-dev.iam.gserviceaccount.com",
    schedule = "triggered",
)

luci.builder(
    name = "test-placeholder-recipe-child",
    bucket = "cloudbuild",
    backend_alt = "cloudbuild_poc_backend",
    executable = luci.recipe(
        name = "placeholder",
        cipd_package = "infra/recipe_bundles/chromium.googlesource.com/infra/luci/recipes-py",
        use_python3 = True,
    ),
    experiments = {
        "luci.buildbucket.backend_alt": 100,
    },
    properties = {
        "status": "SUCCESS",
        "steps": [
            {
                "name": "hello",
                "fake_step": {
                    "duration_secs": 90,
                },
            },
        ],
    },
    service_account = "adhoc-testing@luci-token-server-dev.iam.gserviceaccount.com",
)

################################################################################
## Resources used for configuring a dynamic bucket to test TaskBackendLite.
## TODO(crbug.com/1502020): Remove it once the testing is done.

luci.bucket(
    name = "dynamic-hello-world",
    bindings = [
        luci.binding(
            roles = "role/buildbucket.triggerer",
            groups = "mdb/chrome-troopers",
            users = "adhoc-testing@luci-token-server-dev.iam.gserviceaccount.com",
        ),
        luci.binding(
            roles = "role/buildbucket.creator",
            groups = "mdb/chrome-troopers",
        ),
    ],
    dynamic = True,
    # TODO(yuanjunh): Make it non-public to test /build redirection.
    # Delete it after the testing.
    acls = [
        acl.entry(acl.BUILDBUCKET_READER, groups = ["googlers"]),
    ],
)

luci.task_backend(
    name = "taskbackendlite",
    target = "lite://bb-taskbackendlite",
)

luci.dynamic_builder_template(
    bucket = "dynamic-hello-world",
    properties = {
        "fake_prop": "testing_taskbackendlite",
    },
    execution_timeout = 2 * time.minute,
    heartbeat_timeout = 30 * time.second,
    expiration_timeout = 3 * time.hour,
    experiments = {
        "luci.buildbucket.backend_alt": 100,
    },
    backend = "taskbackendlite",
)

load("@stdlib//internal/luci/proto.star", "scheduler_pb")

# Add a luci-scheduler-dev job which needs to manully triggered via UI.
# It mimics how WebRTC-internal configs a luci-scheduler job in
# https://chrome-internal.googlesource.com/chrome/webrtc-internal/+/refs/heads/main/infra/config/main.star#162
def schedule_in_dynamic_bucket(ctx):
    cfg = ctx.output.get("luci-scheduler-dev.cfg")
    cfg.job.append(scheduler_pb.Job(
        id = "taskbackendlite-tester",
        realm = "dynamic-hello-world",
        buildbucket = scheduler_pb.BuildbucketTask(
            server = "cr-buildbucket-dev.appspot.com",
            bucket = "dynamic-hello-world",
            builder = "taskbackendlite-tester",
        ),
        schedule = "triggered",
    ))

lucicfg.generator(impl = schedule_in_dynamic_bucket)

# Add a builder running on TaskBackendLite to test.
luci.builder(
    name = "fake-on-lite",
    bucket = "ci",
    # Technically this executable should have no effect
    executable = luci.recipe(
        name = "placeholder",
        cipd_package = "infra/recipe_bundles/chromium.googlesource.com/infra/luci/recipes-py",
        use_python3 = True,
    ),
    properties = {
        "status": "SUCCESS",
        "steps": [
            {
                "name": "hello",
                "fake_step": {
                    "duration_secs": 90,
                },
            },
        ],
    },
    schedule = "triggered",
    backend_alt = "taskbackendlite",
)
