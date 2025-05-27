# SatLab

### Requirement
- .service_account.json: The credential is used to validate the GCS Bucket connection, and we can get this credential by logging in Google's partner account.
- SSH rsa_key: The credential is used to validate the SSH connection, and we need to set the `keys` path before building the `SatLab Server`.

### How to config `SSH rsa key` path
- `SSHKeyPath` needs to be on the path set in `utils/constants/constants.go`

### Build
Before running `go build`, we need to confirm the environment.

- .service_account.json
- SSH RSA key

### Format, lint, test and build

To run all the format, lint checks, tests, and build the binary, run:

```
eval `~/infra/infra/go/env.py` && \
( cd ~/infra/infra/go/src/infra && go fmt go.chromium.org/infra/cros/satlab/satlabrpcserver/... go.chromium.org/infra/cros/satlab/common/... ) && \
( cd ~/infra/infra/go/src/infra && golangci-lint run --fix cros/satlab/satlabrpcserver/... cros/satlab/common/... ) && \
( cd ~/infra/infra/go/src/infra && go test go.chromium.org/infra/cros/satlab/satlabrpcserver/... go.chromium.org/infra/cros/satlab/common/... ) && \
( cd ~/infra/infra/go/src/infra && go install go.chromium.org/infra/cros/satlab/satlabrpcserver)
```

### Call the satlabrpcserver locally

To make a manual grpc call to a satlab rpcserver running on a satlab,
forward a port then use the grpc_cli tool.

```
ssh -L 3333:satlab_rpcserver:6003 moblab@your-satlab-ip
grpc_cli call --channel_creds_type=insecure localhost:3333 \
  satlabrpcserver.SatlabRpcService/StartServod \
  'servod_docker_container_name:"some-host-docker_servod"'
```
