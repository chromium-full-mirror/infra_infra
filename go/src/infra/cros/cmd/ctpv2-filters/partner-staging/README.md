# CTPv2 Staging filter service
## Overview
Staging in this application is implemented in the same way as in [CTPV1](https://source.corp.google.com/h/chrome-internal/chromeos/superproject/+/main:infra/recipes/recipes/test_platform/cros_test_platform.py;l=340;drc=d83cac3c70be448d7a4781658464363c9829c7b6) (Chrome Test Platform Version 1), but with some improvements. Specifically, instead of waiting a fixed 120 seconds to check if the staging process has completed (as was done in CTPV1), this application now actively checks the staging process status via requests

## Local Run
### Prerequisites
- **Moblab API Credentials**: [Service account](https://pantheon.corp.google.com/storage/browser/_details/chromeos-distributed-fleet-s4p/pubsub-key-do-not-delete.json;tab=live_object?e=-13802955&invt=AbgvvQ&mods=component_inspector&project=chromeos-partner-moblab) with the necessary permissions to use the Moblab API
### Run
- Build:
```bash
CGO_ENABLED=0 go build -o partner-staging
```
- Run
```bash
SERVICE_ACCOUNT_KEY_PATH=$HOME/service_account/chromeos-distributed-fleet-s4p.json ./partner-staging server
```
- Get request json:
    - [Download](https://logs.chromium.org/logs/chromeos/buildbucket/cr-buildbucket/8732218067024237217/+/u/ctpv2_sub-build__async_/u/step/112/log/1) request example from LUCI.
    - Manually create an `InternalTestplan` request by specifying the desired images in the `SchedulingUnits`->`swReq` field. Here's an example:
```json
{
  "testCases": [],
  "suiteInfo": {
    "suiteMetadata": {
      "pool": "DUT_POOL_QUOTA",
      "executionMetadata": {
        "args": [
          {}
        ]
      },
      "schedulerInfo": {
        "scheduler": "QSCHEDULER",
        "qsAccount": "legacypool-suites"
      },
      "dynamicUpdates": [],
      "schedulingUnits": [
        {
          "primaryTarget": {
            "swarmingDef": {
              "dutInfo": {
                "chromeos": {
                  "dutModel": {
                    "buildTarget": "brask"
                  }
                }
              },
              "provisionInfo": []
            },
            "swReq": {
              "build": "release",
              "gcsPath": "gs://chromeos-distributed-fleet-s4p/brask-release/16089.0.0",
              "keyValues": [
                {
                  "key": "chromeos_build",
                  "value": "brask-release/16089.0.0"
                },
                {
                  "key": "chromeos_build_gcs_bucket",
                  "value": "chromeos-distributed-fleet-s4p"
                }
              ]
            }
          },
          "dynamicUpdateLookupTable": {
            "board": "brask",
            "installPath": "gs://chromeos-distributed-fleet-s4p/brask-release/16089.0.0"
          }
        }
      ]
    },
    "suiteRequest": {
      "testSuite": {
        "name": "bvt-tast-informational",
        "testCaseTagCriteria": {
          "tags": [
            "group:mainline",
            "informational"
          ]
        }
      },
      "maximumDuration": "108000s",
      "analyticsName": "ChromeosTastInformational"
    }
  }
}
```
- Make a request
```bash
source ~/.cftmeta && \
/google/bin/releases/cloud-commerce-producer/tools/textproto2json/json2textproto.par \
--type_url=type.googleapis.com/chromiumos.test.api.InternalTestplan <~/example_json.json | \
grpc_cli call localhost:$SERVICE_PORT chromiumos.test.api.GenericFilterService.Execute \
--channel_creds_type=insecure --call_creds=none
```
### Debug
#### Debug with VS Code
Create `.vscode/launch.json`
```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch Package",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/ctpv2/partner-staging/main.go",
            "args": [
                "server"
            ],
            "env": {
                "SERVICE_ACCOUNT_KEY_PATH": "${env:HOME}/service_account/pubsub-key-do-not-delete.json"
            }
        }
    ]
}
```

## Running Tests
```bash
go test -v go.chromium.org/chromiumos/test/ctpv2/partner-staging
```

## Run with LUCI
- Build binary
- Build docker image:
```bash
docker build --file ../../container_uprev/internal/dockerfiles/Dockerfile_partner-staging -t us-docker.pkg.dev/cros-registry/partner-test-services/partner-staging .
```
- Push docker image:
```bash
docker push us-docker.pkg.dev/cros-registry/partner-test-services/partner-staging
```
- Get request json from one of the CTPv2 jobs. E.G.:
```bash
led get-build b8731269999534234785 > request.json
```
- Add user defined filter with your sha256 digest in request.json::`buildbucket > bbagent_args > build > input > properties > requests > dedede.TSEStagingSchedukeHour10.calibration > params > userDefinedFilters` :
```json
"userDefinedFilters": [
  {
    "containerInfo": {
      "binaryArgs": [],
      "binaryName": "partner-staging",
      "container": {
        "digest": "sha256:XXX",
        "name": "partner-staging",
        "repository": {
        "hostname": "us-docker.pkg.dev",
        "project": "cros-registry/partner-test-services"
        },
        "tags": [
          "latest"
        ]
      }
    }
  }
]
```
- Run LUCI job:
```bash
cat request.json | led launch
```
- Check the [logs](https://screenshot.googleplex.com/97rd7jercDAxuHX) of the job.

## Run with docker
- [Build docker image](#Run-with-LUCI)
- Run container:
```bash
docker run --rm -it -v ~/service_account/chromeos-distributed-fleet-s4p.json:/creds/service_accounts/service-account-chromeos.json -v ~/.cftmeta:/root/.cftmeta -p $SERVICE_PORT:$SERVICE_PORT -e SERVICE_ACCOUNT_KEY_PATH=/creds/service_accounts/service-account-chromeos.json --port $SERVICE_PORT us-docker.pkg.dev/cros-registry/partner-test-services/partner-staging /usr/bin/partner-staging server
```