# cros-fw-provision

This directory contains cros-fw-provision: a service for preparing
devices-under-test with specific application processor ("AP") and embedded
controller ("EC") firmware ("fw") builds. Preparing devices with specific builds
is called "provisioning".

See go/cft-fw-provision-v2 for background.

## Input proto

The request to provision comes in as a chromiumos.test.api.InstallRequest with
the metadata field populated with
chromiumos.test.api.FirmwareProvisionInstallMetadata. The fields in
firmware_config can be populated in these configurations:

*   main_ro_payload - The AP RO and AP RW A/B will be updated with this image.
    As a side effect, the EC RW will also be updated to the version embedded in
    the AP RW image. The EC RO will remain untouched.
*   main_rw_payload - The AP RW A/B will be updated with this image. As a side
    effect, the EC RW will also be updated to the version embedded in the AP RW
    image. The AP & EC RO will remain untouched.
*   ec_ro_payload - The EC RO will be updated with this image. The AP RO/RW & EC
    RW will be left untouched.
*   ec_rw_payload - The EC RW from this image will be embedded into the AP RW
    for A and B, and flashed to AP RW A/B.

Here are some common combinations, and where the image comes from:

Payloads                          | AP RO   | EC RO | AP RW   | EC RW   | Note
--------------------------------- | ------- | ----- | ------- | ------- | ----
main_ro + ec_ro                   | main_ro | ec_ro | main_ro | main_ro | Typical for faft tests w/o EC branch
main_ro + ec_ro + main_rw         | main_ro | ec_ro | main_rw | main_rw | Typical for FW Qual w/o EC branch
main_ro + ec_ro + main_rw + ec_rw | main_ro | ec_ro | main_rw | ec_rw   | Typical for FW Qual w/ EC branch
main_ro + ec_ro + ec_rw           | main_ro | ec_ro | main_ro | ec_rw   | Typical for faft tests w/ EC branch
main_rw                           | -       | -     | main_rw | main_rw |
main_rw + ec_rw                   | -       | -     | main_rw | ec_rw   |

After the firmware is updated, the version numbers will be checked to ensure the
update was successful. The AP firmware version for the inactive bank (A or B)
will not be checked, only the active firmware.

### Paths

The paths in the payload fields above should be a gs path to the archive
containing the image for the correct model. However, if the image is a
firmware_from_source archive, the code will look first for a model specific file
in the `gs://firmware-image-archive` bucket. I.e. If you are flashing a
brya/omnigul DUT, then a url like
`gs://chromeos-image-archive/firmware-brya-14505.B-branch/R100-14505.832.0-1-8730368903603296945/brya/firmware_from_source.tar.bz2`
will search for
`gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/omnigul.14505.832.0.tar.bz2`
first, and if it is not found, extract `image-omnigul.bin` from the giant
`firmware_from_source.tar.bz2` archive.

## Launching

The cros-fw-provision binary is normally run from docker.

### Building locally

To build and run locally:

These example assume some vars are set in your shell:

```
BOARD=myBoard
MODEL=myModel
DUT_HOSTNAME=192.168.0.0
D=localhost:2222 # Forwarded ssh port
```

Run the cros-dut server in the chroot

```
CACHE_SERVER=192.168.100.1
~/chromiumos/src/platform/dev/fast_build.sh -b go.chromium.org/chromiumos/test/dut/cmd/cros-dut -o ~/go/bin/cros-dut && \
~/go/bin/cros-dut -cache_address ${CACHE_SERVER?}:8082 -dut_address ${D?} -port 8123
```

Run the servo-nexus server from infra repo (outside chroot)

```
eval `~/infra/infra/go/env.py` && \
export CGO_ENABLED=0 && \
(cd ~/infra/infra/go/src/infra && go install go.chromium.org/infra/cros/cmd/cft/dut/cros-servod) && \
~/infra/infra/go/bin/cros-servod server -server_port 8124
```

If you are running with a local dut, you will need a cache server. Then set the
CACHE_SERVER variable below with your IP as seen from the DUT, otherwise set the
CACHE_SERVER variable to any cache server that is visible from the DUT.

```
docker run -d --rm -P --cap-add=NET_RAW --name visible-cache-server --expose 8082 -p 8082:8082 --volume $HOME/.config/:/root/.config --volume /tmp/local-cft/cacheserver:/tmp/cacheserver us-docker.pkg.dev/cros-registry/test-services/cacheserver:prod cacheserver -location /tmp/cacheserver -port 8082
```

Alternatively, you can build the cache server from source and run it.

```
sudo ufw allow 8082 # Open the port in the linux firewall.
~/chromiumos/src/platform/dev/fast_build.sh -b go.chromium.org/chromiumos/prototytpe/cache/cmd/cacheserver -o ~/go/bin/cacheserver && ~/go/bin/cacheserver -port 8082
```

To find the cache server on a lab machine:

```
ssh $DUT_HOSTNAME 'for devserver in 192.168.100.1 100.115.168.190 10.128.176.210 100.115.21.212 100.115.245.199 100.115.245.200 100.115.219.131 100.115.219.132 100.115.219.133 100.115.219.134 100.115.219.137; do if curl -f --connect-timeout 3 "http://${devserver?}:8082/check_health" >/dev/null ; then echo CACHE_SERVER=${devserver?} ; fi; done'
```

Run cros-fw-provision outside chroot

```
CACHE_SERVER=192.168.100.1
FW_IMAGE=$(gsutil ls -l gs://chromeos-image-archive/firmware-{brya,glados,oak,reef,${BOARD?}}*-branch/*/${BOARD?}/firmware_from_source.tar.bz2 | grep -v TOTAL | sort -k2 | tail -1 | awk '{print $3}')
EC_BRANCH=$(gsutil ls -d gs://firmware-image-archive/firmware-ec-R* | tail -1)
EC_IMAGE=$(gsutil ls -l ${EC_BRANCH}*/${BOARD}/firmware_from_source.tar.bz2 | grep -v TOTAL | sort -k2 | tail -1 | awk '{print $3}')
cat >startup.json <<STARTUP
{
        "dut": {
                "chromeos": {
                        "dut_model": {
                                "build_target": "${BOARD?}",
                                "model_name": "${MODEL?}"
                        },
                        "servo": {
                                "present": true,
                                "servodAddress": {
                                        "address": "localhost:${LOCAL_SERVO_SSH_PORT?}",
                                        "port": ${SERVO_PORT?}
                                },
                                "serial": "${SERVO_SERIAL?}"
                        }
                },
                "cache_server": {
                        "address": {
                                "address": "${CACHE_SERVER?}",
                                "port": 8082
                        }
                }
        },
        "dut_server": {
                "address": "localhost",
                "port": 8123
        },
        "servoNexusAddr": {
                "address": "localhost",
                "port": 8124
        }
}
STARTUP
# Typical case
cat >install.json <<INSTALL
{
        "metadata": {
                "@type": "chromiumos.test.api.FirmwareProvisionInstallMetadata",
                "firmware_config": {
                        "mainRoPayload": {
                                "firmwareImagePath": {
                                        "hostType": "GS",
                                        "path": "${FW_IMAGE?}"
                                }
                        },
                        "ecRoPayload": {
                                "firmwareImagePath": {
                                        "hostType": "GS",
                                        "path": "${EC_IMAGE:-${FW_IMAGE?}}"
                                }
                        },
                        "ecRwPayload": {
                                "firmwareImagePath": {
                                        "hostType": "GS",
                                        "path": "${EC_IMAGE:-${FW_IMAGE?}}"
                                }
                        }
                }
        }
}
INSTALL
eval `~/infra/infra/go/env.py` && \
export CGO_ENABLED=0 && \
(cd ~/infra/infra/go/src/infra && go install go.chromium.org/infra/cros/cmd/provision/v2/cros-fw-provision) && \
~/infra/infra/go/bin/cros-fw-provision cli -startup startup.json -install install.json
```

If you want to test specific versions like a firmware qual would, this command
should help you find the path:

```
BOARD=rex
VERSION=16151.2.19
gsutil ls -d gs://{firmware,chromeos}-image-archive/firmware-{ec,${BOARD}}*/*${VERSION}*
```

## Dependencies

cros-fw-provision expects cros-cache, and either cros-dut or cros-servod, to be
running.

In the lab, the cache server should already be up. Local runs of
cros-fw-provision will need the cache server started.

`cros-dut` is needed if provisioning directly.

`cros-servod` is needed if provisioning over
[servo]("https://chromium.googlesource.com/chromiumos/third_party/hdctools/+/HEAD/README.md").

## Testing

Run the test with go test:

```
eval `~/infra/infra/go/env.py` && \
export CGO_ENABLED=0 && \
(cd ~/infra/infra/go/src/infra && go test go.chromium.org/chromiumos/test/provision/v2/cros-fw-provision/...)
```

## Production deployment

1.  The code is built automatically by
    [infra-packager-linux-64](https://ci.chromium.org/ui/p/infra-internal/builders/prod/infra-packager-linux-64)
    and pushed to
    [CIPD](https://chrome-infra-packages.appspot.com/p/chromiumos/infra/cft/provision/cros-fw-provision/linux-amd64).
    This builder is triggered after every commit.
2.  The CIPD package is tagged `staging` by
    [ctp-uprev-staging](https://ci.chromium.org/ui/p/chromeos/builders/infra/ctp-uprev-staging)
    multiple times per day. This same builder also builds the docker container
    at
    [us-docker.pkg.dev/cros-registry/test-services/cros-fw-provision](http://us-docker.pkg.dev/cros-registry/test-services/cros-fw-provision)
    and labels it `staging_cros-fw-provision`.
3.  The CIPD package is tagged `prod` & docker container is tagged
    `prod_cros-fw-provision` by
    [ctp-uprev-prod](https://ci.chromium.org/ui/p/chromeos/builders/infra/ctp-uprev-prod).
    This is started manually by the
    [CTP oncall once a week](go/ctp-oncall#releasing-new-versions-to-production-services)
    following the instructions at go/ctp-release-doc.
4.  When a Ctpv2 test runs it picks the cros-fw-provision sha256 sum to use by
    the `prod_cros-fw-provision` label. If a test is run with Ctpv2 non-prod, it
    uses the `staging_cros-fw-provision` label instead.

### Finding the current prod version

If you want to know, is my change in production yet?

Look at the latest
[ctp-uprev-prod](https://ci.chromium.org/ui/p/chromeos/builders/infra/ctp-uprev-prod)
build, and find the `package
chromiumos/infra/cft/provision/cros-fw-provision/linux-amd64` step.

Open the `stdout` of the `apply the "prod" ref of the
"chromiumos/infra/cft/provision/cros-fw-provision/linux-amd64" package to
"staging"` step.

Copy the instance id, and then visit
[CIPD](https://chrome-infra-packages.appspot.com/p/chromiumos/infra/cft/provision/cros-fw-provision/linux-amd64),
and click on the instance with the id from the `stdout` file. It will be tagged
with all the commit ids that build this instance. If one of those commit ids is
equal to or later than your change, then your change is included in that
instance.
