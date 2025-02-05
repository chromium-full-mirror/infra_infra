# Using firmware-filter

There are several flags that can be passed to the firmware-filter.

Arg | Description
--|--
`-ro SPEC` | Flash the RO and RW AP & EC firmware from specified location.
`-rw SPEC` | Flash the RW AP & EC firmware from specified location. This takes place after the RO flashing.
`-fallbackToCros` | If `-ro` or -`rw` is set to `firmwareBoardBranch`, and no branch build can be found, fallback to `cros` instead. This is handy for new boards that don't have a branch yet.

The SPEC arg can be one of the following:
SPEC | Description
--|--
`gs://tar.bz2` url | Specify a specific url to a firmware_from_source.tar.bz2 archive.
`firmwareBoardBranch` | Find the latest branch build for the DUT's board.
`cros` | Use the firmware_from_source.tar.bz2 from the OS build. NOTE: Many launched devices do not build firmware in the OS build.

## In suite schedule

Add this to your `config_gen.create_config()` call in your star file.

```starlark
config_gen.create_config(
    ... # existing args
    karbon_filters = [
        config_gen.create_known_udf(
            "firmware-filter",
            args = ["-ro", "firmwareBoardBranch", "-fallbackToCros"],
        ),
    ],
    run_via_trv2 = True,
    dynamic_trv2 = True,
)
```

## From crosfleet

Here is an example of running the tast test `firmware.FwSplashScreen` on
`nissa/joxer` in pool `faft-test` with the latest firmware branch build flashed
to RO and RW.

```shell
crosfleet run test -trv2 -dynamic-trv2 -harness=tast \
  -board nissa -model joxer -pool faft-test \
  -user-defined-filter='{"containerInfo":{"binaryArgs":["-ro","firmwareBoardBranch","-fallbackToCros"],"container": {"name":"firmware-filter"}}}' \
  -exit-early -dim label-servo_state:WORKING firmware.FwSplashScreen
```

# Production deployment

1) The code is built automatically by
[infra-packager-linux-64](https://ci.chromium.org/ui/p/infra-internal/builders/prod/infra-packager-linux-64)
and pushed to [CIPD](https://chrome-infra-packages.appspot.com/p/chromiumos/infra/ctpv2-filters/firmware-filter/linux-amd64).
This builder is triggered after every commit.
1) The CIPD package is tagged `staging` by [ctp-uprev-staging](https://ci.chromium.org/ui/p/chromeos/builders/infra/ctp-uprev-staging)
multiple times per day. This same builder also builds the docker container at
[us-docker.pkg.dev/cros-registry/test-services/firmware-filter](http://us-docker.pkg.dev/cros-registry/test-services/firmware-filter)
and labels it `staging`.
1) The CIPD package & docker container is tagged `prod` by [ctp-uprev-prod](https://ci.chromium.org/ui/p/chromeos/builders/infra/ctp-uprev-prod).
This is started manually by the [CTP oncall once a week](go/ctp-oncall#releasing-new-versions-to-production-services) following the instructions at go/ctp-release-doc.
1) When a Ctpv2 test runs it picks the firmware-filter sha256 sum to use by the `prod` label. If a test is run with Ctpv2 non-prod, it uses the `staging` label instead.

## Finding the current prod version

If you want to know, is my change in production yet? Look at [CIPD](https://chrome-infra-packages.appspot.com/p/chromiumos/infra/ctpv2-filters/firmware-filter/linux-amd64),
and click on a instance. It will be tagged with all the commit ids that build this instance. If one of those commit ids is equal to or later than your
change, then your change is included in that instance.

# Debugging

If you are looking at a [failed test run](http://go/bbid/8741669481538299105/infra), and you want to see the logs, find the section `ctpv2 sub-build (async)` -> `Suite Executions (async)`, then open the logs under `Read Container Logs`, `Container Start: firmware-filter`, and `Filter execution: firmware-filter`.

# Checkout the code

See https://chromium.googlesource.com/infra/infra/+/main/doc/source.md#checkout-code

# Building

## firmware-filter binary

```shell
# If you fetched to somewhere other than ~/infra, change these paths.
eval `~/infra/infra/go/env.py` && \
cd ~/infra/infra/go/src/infra && \
export CGO_ENABLED=0 && \
go install infra/cros/cmd/ctpv2-filters/firmware-filter
```

The binary will be installed into ~/infra/infra/go/bin

## Upload to CIPD
Build the binary (above), then run these commands to upload to CIPD:

```shell
cipd create -pkg-def ~/infra/infra/build/packages/firmware-filter.yaml -ref $USER-test -pkg-var exe_suffix: -verbose
```

## Uprev docker container
After you have uploaded to CIPD

```shell
cd ~/infra/infra/go/src/infra
go install infra/cros/cmd/container_uprev
container_uprev cli -label $USER-test -target firmware-filter
```

Be sure to note the digest printed by this command.

# Testing

## Lint and formatting

```
eval `~/infra/infra/go/env.py` && \
cd ~/infra/infra/go/src/infra && \
export CGO_ENABLED=0
go fmt infra/cros/cmd/ctpv2-filters/firmware-filter && \
golangci-lint run --fix cros/cmd/ctpv2-filters/firmware-filter
```

## Unit tests

```shell
# If you fetched to somewhere other than ~/infra, change these paths.
eval `~/infra/infra/go/env.py` && \
cd ~/infra/infra/go/src/infra && \
export CGO_ENABLED=0
go test infra/cros/cmd/ctpv2-filters/firmware-filter
```

## Locally

1) Get a request.json from a luci job such as https://logs.chromium.org/logs/chromeos/buildbucket/cr-buildbucket/8745841814552538721/+/u/ctpv2_sub-build__async_/u/step/39/log/1 and save it to ~/request.json
2) [Build and install](#firmware-filter-binary)
3) Run

```shell
firmware-filter server -port 0 -serviceAccountCred ~/.config/gcloud/legacy_credentials/*/adc.json -ro firmwareBoardBranch
```

4) In another window make an RPC call

```shell
source ~/.cftmeta && \
/google/bin/releases/cloud-commerce-producer/tools/textproto2json/json2textproto.par \
--type_url=type.googleapis.com/chromiumos.test.api.InternalTestplan <~/request.json | \
grpc_cli call localhost:$SERVICE_PORT chromiumos.test.api.GenericFilterService.Execute \
--channel_creds_type=insecure --call_creds=none
```

## Using LED

This will launch a test job on buildbucket, but with a modified config of your choosing.

1) [Rebuild and push docker image](#uprev-docker-container)
2) Pick an existing test run that is similar to what you want to run. Go to the ancestor build. Or you can start at go/kron- to find a test run. Example: http://go/bbid/8741669481538299105/infra
3) `led get-build 8741669481538299105 >~/job.json`
4) Edit job.json, and find all of the `firmware-filter` and add the digest printed by `docker push` above, and add the repository information.
   1) If needed, also edit the `firmware-filter`'s `binaryArgs`.
   2) Example

```json
"container": {
        "name": "firmware-filter",
        "digest": "sha256:e525e3466a11325c265bad9f49cf607aa24147567d95803c4aa97337a0633d58",
        "repository": {
          "hostname": "us-docker.pkg.dev",
          "project": "cros-registry/test-services"
        }
}
```
5) Run job: `cd ~/chromiumos/infra/recipes ; cat ~/job.json | led edit-recipe-bundle | led launch`
