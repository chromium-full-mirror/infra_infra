# cros-servod

This service manages servod during test provisioning.

## Format, lint, test and build

To run all the format, lint checks, tests, and build the binary, run:

```
eval `~/infra/infra/go/env.py` && \
( cd ~/infra/infra/go/src/infra && go fmt go.chromium.org/infra/cros/cmd/cft/dut/cros-servod/... ) && \
( cd ~/infra/infra/go/src/infra && golangci-lint run --fix cros/cmd/cft/dut/cros-servod/... ) && \
( cd ~/infra/infra/go/src/infra && go test go.chromium.org/infra/cros/cmd/cft/dut/cros-servod/... ) && \
( cd ~/infra/infra/go/src/infra && go install go.chromium.org/infra/cros/cmd/cft/dut/cros-servod)
```

## Running locally

When running locally, you need to redirect some ports and pass extra command
line flags.

### Satlab

TODO: Port 2375 forwarding doesn't work.

Example for satlab-0wgtfqin18508111-steelix-c0-0

Start port redirects for the satlab rpcserver and docker api.

```
ssh -L 3333:satlab_rpcserver:6003 -L 7575:192.168.231.1:2375 moblab@satlab-0wgtfqin18508111
```

Then start cros-servod

```
~/infra/infra/go/bin/cros-servod server -server_port 8124 -satlab_rpc_server localhost:3333 -docker_host tcp://localhost:7575
```

#### Start servod

```
grpc_cli call --channel_creds_type=insecure localhost:8124 chromiumos.test.api.ServodService/StartServod 'servod_docker_container_name:"satlab-0wgtfqin18508111-steelix-c0-0-docker_servod"'
```

### Mainlab

TODO: Add documentation here.
