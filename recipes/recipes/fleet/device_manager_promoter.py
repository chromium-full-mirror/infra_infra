# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Promote the fleet Device Manager to canary/prod.

The recipe parse the channel.json to update the canary/prod docker image to the
same version w/ the staging."""

from recipe_engine import post_process

from PB.recipe_engine import result as result_pb2
from PB.go.chromium.org.luci.buildbucket.proto import common as common_pb

DEPS = [
    "depot_tools/bot_update",
    "depot_tools/gclient",
    "depot_tools/git",
    "depot_tools/git_cl",
    "recipe_engine/cipd",
    "recipe_engine/context",
    "recipe_engine/file",
    "recipe_engine/json",
    "recipe_engine/path",
    "recipe_engine/step",
    "recipe_engine/url",
]


def get_image_name(service: str) -> str:
  return "gcr.io/chops-public-images-prod/device-manager/{}".format(service)


def gen_channels_json(service: str,
                      staging="v1",
                      canary="v1",
                      stable="v1") -> dict:
  return {
      "images": {
          get_image_name(service): {
              "canary": canary,
              "stable": stable,
              "staging": staging,
          }
      }
  }


def try_update_service(api, service: str) -> dict:
  channels_json = "projects/device-manager/{}/channels.json".format(service)
  image_name = get_image_name(service)

  channels = api.file.read_json(
      "read channels json for service {}".format(service),
      api.context.cwd / channels_json,
      test_data=gen_channels_json(service, canary="v2", stable="v3"),
  )
  versions = channels.get("images", {}).get(image_name, {})
  staging_ver = versions.get("staging")
  canary_ver = versions.get("canary")
  stable_ver = versions.get("stable")
  if staging_ver is None or canary_ver is None or stable_ver is None:
    raise api.step.StepFailure(
        "wrong channels.json format of service {}: {}".format(
            service,
            channels,
        ))

  if stable_ver == canary_ver and canary_ver == staging_ver:
    api.step.empty("all versions are the same for service {}".format(service))
    return None

  # Currently push canary and stable to the same version.
  origin_versions = channels["images"][image_name].copy()
  channels["images"][image_name].update({
      "canary": staging_ver,
      "stable": staging_ver
  })
  content = api.json.dumps(channels, indent=2) + "\n"
  api.file.write_text(
      "update channles json for service {}".format(service),
      api.context.cwd / channels_json,
      content,
  )
  return origin_versions


def container_tag_to_git_version(tag: str) -> str:
  """Parse the container tag and extract the part of git version.

  The container tag is like ci-2024.12.05-70377-332cc31, where the last part is
  the git version."""
  return tag.split("-")[-1]


def gen_cl_description(api, service_name: str, original_version: dict) -> str:
  """Generate the CL description and the gitiles link for the changes."""
  tot = container_tag_to_git_version(original_version["staging"])
  stable_base = container_tag_to_git_version(original_version["stable"])

  link_template = "https://chromium.googlesource.com/infra/infra/+log/{}..{}/go/src/infra/device_manager"
  link = link_template.format(stable_base, tot)
  desc = [service_name, link]
  v = api.url.get_json(
      link + "?format=json",
      log=True,
      strip_prefix=api.url.GERRIT_JSON_PREFIX,
      default_test_data={
          "log": [{
              "commit": "aaabbbccc",
              "message": "title\n\ndetails",
              "author": {
                  "name": "AUTHOR"
              },
              "committer": {
                  "time": "2024-01-01"
              }
          }]
      })

  for cl in v.output["log"]:
    desc.append("{}: '{}' by {}@{}".format(cl["commit"][:7],
                                           cl["message"].split("\n")[0],
                                           cl["author"]["name"],
                                           cl["committer"]["time"]))
  return "\n".join(desc)


def RunSteps(api):
  api.gclient.set_config("infradata_cloud_run")
  update_result = api.bot_update.ensure_checkout()
  source_dir = update_result.source_root.path

  packages_dir = api.path.cleanup_dir / "packages"
  ensure_file = api.cipd.EnsureFile()
  ensure_file.add_package("infra/tools/luci/lucicfg/${platform}", "latest")
  api.cipd.ensure(packages_dir, ensure_file)

  with api.context(cwd=source_dir):
    with api.step.nest("create local branch"):
      api.git("branch", "-D", "fleet_device_manager_promoter", ok_ret=(0, 1))
      api.git("checkout", "-t", "origin/main", "-b",
              "fleet_device_manager_promoter")

    with api.step.nest("check/update channels json"):
      c1 = try_update_service(api, "device-lease-service")
      c2 = try_update_service(api, "notifier-service")
      if c1 is None and c2 is None:
        api.step("Skip the push due to no changes", cmd=None)
        return result_pb2.RawResult(
            status=common_pb.SUCCESS,
            summary_markdown="push skipped due to no changes",
        )

    api.step("generate files", cmd=["lucicfg", "generate", "main.star"])

    with api.step.nest("upload CL") as upload_step:
      # The git repo is not cloned directly. Instead, it's initialized as an
      # empty repo and then fetch from the origin, which results in different
      # remote fetch URL (a local directory) and push URL (the URL of the repo)
      # and cause uploading issue. To fix this, we need to reset the fetch URL
      # with the push URL.
      repo_url = api.git.config_get("remote.origin.pushurl")
      # The git command used by recipe doesn't support `git config set`
      api.git("remote", "set-url", "origin", repo_url.decode('utf-8'))

      api.git("add", ".")
      desc = ["device-manager: push to prod", ""]
      desc.append(gen_cl_description(api, "device-lease-service", c1))
      desc.append("")
      desc.append(gen_cl_description(api, "notifier-service", c2))
      api.git("commit", "-m", "\n".join(desc))
      api.git_cl.upload(
          "\n".join(desc),
          name="git cl upload",
          upload_args=[
              "--force",
              "--bypass-hooks",
              "--use-commit-queue",
              # TODO b/379124711 comment out below line to enable auto
              # commit.
              # "--set-bot-commit",
          ],
      )
      # Put a link to the uploaded CL.
      step = api.git_cl(
          "issue",
          ["--json", api.json.output()],
          name="git cl issue",
          step_test_data=lambda: api.json.test_api.output({
              "issue": 123456789,
              "issue_url": "https://chromium-review.googlesource.com/c/1234567",
          }),
      )
      out = step.json.output
      step.presentation.links["Issue %s" % out["issue"]] = out["issue_url"]
      upload_step.links["Issue {}".format(out["issue"])] = out["issue_url"]

    api.step.empty("device manager push to prod: done")
    return result_pb2.RawResult(
        status=common_pb.SUCCESS,
        summary_markdown="push to prod done",
    )


def GenTests(api):
  yield api.test(
      "basic",
      api.post_process(post_process.DropExpectation),
  )

  yield api.test(
      "wrong channels.json format",
      api.override_step_data(
          "check/update channels json.read channels json for service"
          " device-lease-service",
          api.file.read_json({}),
      ),
      api.expect_status("FAILURE"),
  )
  yield api.test(
      "no update",
      api.override_step_data(
          "check/update channels json.read channels json for service"
          " device-lease-service",
          api.file.read_json(gen_channels_json("device-lease-service")),
      ),
      api.override_step_data(
          "check/update channels json.read channels json for service"
          " notifier-service",
          api.file.read_json(gen_channels_json("notifier-service")),
      ),
      api.post_process(post_process.SummaryMarkdown,
                       "push skipped due to no changes"),
      api.expect_status("SUCCESS"),
      api.post_process(post_process.DropExpectation),
  )
