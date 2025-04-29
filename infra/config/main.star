#!/usr/bin/env lucicfg
# Copyright 2019 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""LUCI project configuration for the production instance of LUCI.

After modifying this file execute it ('./main.star') to regenerate the configs.
This is also enforced by PRESUBMIT.py script.

Includes CI configs for the following subprojects:
  * Codesearch.
  * Gsubtreed crons.
  * WPT autoroller crons.
  * Chromium Gerrit plugins.
  * Chromium tarball publisher.
  * Chromium LKGR finder cron.
  * https://chromium.googlesource.com/chromium/tools/build
  * https://chromium.googlesource.com/chromium/tools/depot_tools
  * https://chromium.googlesource.com/infra/infra
  * https://chromium.googlesource.com/infra/luci/luci-go
  * https://chromium.googlesource.com/infra/luci/luci-py
  * https://chromium.googlesource.com/infra/luci/recipes-py
  * https://chromium.googlesource.com/infra/testing/expect_tests
"""

lucicfg.check_version("1.45.0", "Please update depot_tools")

# Tell lucicfg what files it is allowed to touch.
lucicfg.config(
    config_dir = "generated",
    tracked_files = [
        "commit-queue.cfg",
        "cr-buildbucket.cfg",
        "luci-logdog.cfg",
        "luci-milo.cfg",
        "luci-notify.cfg",
        "luci-notify/email-templates/*.template",
        "luci-scheduler.cfg",
        "project.cfg",
        "realms.cfg",
    ],
    fail_on_warnings = True,
    lint_checks = ["default"],
)

luci.project(
    name = "infra",
    buildbucket = "cr-buildbucket.appspot.com",
    logdog = "luci-logdog.appspot.com",
    milo = "luci-milo.appspot.com",
    notify = "luci-notify.appspot.com",
    scheduler = "luci-scheduler.appspot.com",
    swarming = "chromium-swarm.appspot.com",
    acls = [
        # Publicly readable.
        acl.entry(
            roles = [
                acl.BUILDBUCKET_READER,
                acl.LOGDOG_READER,
                acl.PROJECT_CONFIGS_READER,
                acl.SCHEDULER_READER,
            ],
            groups = "all",
        ),
        # Allow committers to use CQ and to force-trigger and stop CI builds.
        acl.entry(
            roles = [
                acl.SCHEDULER_OWNER,
                acl.CQ_COMMITTER,
            ],
            groups = "project-infra-committers",
        ),
        # Ability to launch CQ dry runs.
        acl.entry(
            roles = acl.CQ_DRY_RUNNER,
            groups = "project-infra-tryjob-access",
        ),
        # Ability to trigger new patchset runs on CV.
        acl.entry(
            roles = acl.CQ_NEW_PATCHSET_RUN_TRIGGERER,
            groups = "project-infra-new-patchset-run-access",
        ),
        # Group with bots that have write access to the Logdog prefix.
        acl.entry(
            roles = acl.LOGDOG_WRITER,
            groups = "luci-logdog-chromium-writers",
        ),
    ],
    bindings = [
        luci.binding(
            roles = "role/configs.validator",
            users = "infra-try-builder@chops-service-accounts.iam.gserviceaccount.com",
        ),
        luci.binding(
            roles = "role/analysis.reader",
            groups = "all",
        ),
        luci.binding(
            roles = "role/analysis.queryUser",
            groups = "project-infra-committers",
        ),
        luci.binding(
            roles = "role/analysis.editor",
            groups = "project-infra-committers",
        ),
        luci.binding(
            roles = "role/swarming.poolOwner",
            groups = "mdb/chrome-troopers",
        ),
        luci.binding(
            roles = "role/swarming.taskTriggerer",
            groups = "project-infra-internal-led-breakglass",
        ),
        luci.binding(
            roles = "role/buildbucket.triggerer",
            groups = "project-infra-internal-led-breakglass",
        ),
    ],
)

# Per-service tweaks.
luci.logdog(
    gs_bucket = "chromium-luci-logdog",
    cloud_logging_project = "chrome-infra-logs",
)
luci.milo(
    logo = "https://storage.googleapis.com/chrome-infra-public/logo/chrome-infra-logo-200x200.png",
    favicon = "https://storage.googleapis.com/chrome-infra-public/logo/favicon.ico",
)
luci.cq(
    status_host = "chromium-cq-status.appspot.com",
    honor_gerrit_linked_accounts = True,
)
luci.notify(tree_closing_enabled = True)

# Global builder defaults.
luci.builder.defaults.execution_timeout.set(45 * time.minute)
luci.builder.defaults.experiments.set({
    "luci.buildbucket.backend_alt": 100,
})

# Global recipe defaults
luci.recipe.defaults.cipd_version.set("refs/heads/main")
luci.recipe.defaults.use_bbagent.set(True)

# Resources shared by all subprojects.

luci.bucket(name = "ci")

# Shadow bucket of `ci`, for led builds.
# TODO(crbug.com/1420100): Review and fix the permissions.
luci.bucket(
    name = "ci.shadow",
    shadows = "ci",
    constraints = luci.bucket_constraints(
        pools = ["luci.infra.ci"],
        service_accounts = ["infra-ci-builder@chops-service-accounts.iam.gserviceaccount.com"],
    ),
    bindings = [
        luci.binding(
            roles = "role/buildbucket.creator",
            groups = "flex-ci-led-users",
        ),
    ],
    dynamic = True,
)

luci.bucket(
    name = "try",
    bindings = [
        luci.binding(
            roles = "role/buildbucket.triggerer",
            users = [
                # For b/211053378 allow direct buildbucket triggers for github
                # integration experimentation.
                #
                # Remove after January 30, 2022 (cobalt should have its own LUCI
                # project & builders by then).
                "github-integration@cobalt-tools.iam.gserviceaccount.com",
            ],
            projects = "infra-internal",
            groups = [
                "project-infra-tryjob-access",
                "service-account-cq",
            ],
        ),
    ],
)

# Shadow bucket of `try`, for led builds.
# TODO(crbug.com/1420100): Review and fix the permissions.
luci.bucket(
    name = "try.shadow",
    shadows = "try",
    constraints = luci.bucket_constraints(
        pools = ["luci.infra.try"],
        service_accounts = [
            "infra-try-builder@chops-service-accounts.iam.gserviceaccount.com",
            "infra-try-recipes-tester@chops-service-accounts.iam.gserviceaccount.com",
        ],
    ),
    bindings = [
        luci.binding(
            roles = "role/buildbucket.creator",
            groups = "flex-try-led-users",
        ),
    ],
    dynamic = True,
)

luci.bucket(
    name = "cron",
)

# Shadow bucket of `cron`, for led builds.
# TODO(crbug.com/1420100): Review and fix the permissions.
luci.bucket(
    name = "cron.shadow",
    shadows = "cron",
    constraints = luci.bucket_constraints(
        pools = ["luci.infra.cron"],
        service_accounts = [
            "chromium-lkgr-finder-builder@chops-service-accounts.iam.gserviceaccount.com",
            "chromium-tarball-builder@chops-service-accounts.iam.gserviceaccount.com",
        ],
    ),
    bindings = [
        luci.binding(
            roles = "role/buildbucket.creator",
            groups = "mdb/chrome-troopers",
        ),
    ],
    dynamic = True,
)

# Config realms for infra pools.

luci.realm(name = "pools/cron")

luci.realm(
    name = "pools/ci",
)

luci.realm(
    name = "pools/try",
)

luci.notifier_template(
    name = "status",
    body = "{{ stepNames .MatchingFailedSteps }} on {{ buildUrl . }} {{ .Build.Builder.Builder }}{{ if .Build.Output.GitilesCommit }} from {{ .Build.Output.GitilesCommit.Id }}{{end}}",
)

luci.list_view(name = "cron")

# Setup Swarming permissions (in particular for LED).
# TODO(crbug.com/1420100): Review these permissions.

load("//lib/led.star", "led")

led.users(
    groups = "flex-ci-led-users",
    task_realm = "ci",
    pool_realm = "pools/ci",
)

led.users(
    groups = "flex-try-led-users",
    task_realm = "try",
    pool_realm = "pools/try",
)

# Per-subproject resources. They may refer to the shared resources defined
# above by name.

exec("//subprojects/build.star")
exec("//subprojects/codesearch.star")
exec("//subprojects/depot_tools.star")
exec("//subprojects/expect_tests.star")
exec("//subprojects/gerrit-plugins.star")
exec("//subprojects/infra.star")
exec("//subprojects/lkgr.star")
exec("//subprojects/luci-go.star")
exec("//subprojects/luci-py.star")
exec("//subprojects/python-adb.star")
exec("//subprojects/recipe_engine.star")
exec("//subprojects/tarballs.star")
exec("//subprojects/wpt.star")
