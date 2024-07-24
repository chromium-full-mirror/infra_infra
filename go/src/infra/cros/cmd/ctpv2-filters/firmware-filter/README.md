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
container_uprev cli -label $USER-test
```

Be sure to note the digest printed by this command.

# Testing

## Locally

1) Get a request.json from a luci job such as https://logs.chromium.org/logs/chromeos/buildbucket/cr-buildbucket/8745841814552538721/+/u/ctpv2_sub-build__async_/u/step/39/log/1 and save it to ~/request.json
2) [Build and install](#firmware-filter-binary)
3) Run

```shell
firmware-filter server -port 0 -serviceAccountCred ~/.config/gcloud/legacy_credentials/jbettis@google.com/adc.json -ro firmwareBoardBranch
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
