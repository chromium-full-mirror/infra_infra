# Build locally

```shell
eval `~/infra/infra/go/env.py` && \
export CGO_ENABLED=0 && \
(cd ~/infra/infra/go/src/infra && go install go.chromium.org/infra/cros/cmd/cft/execution/cros-test)
```

# Run tests

```shell
eval `~/infra/infra/go/env.py` && \
export CGO_ENABLED=0 && \
(cd ~/infra/infra/go/src/infra && go test go.chromium.org/infra/cros/cmd/cft/execution/cros-test/...)
```

