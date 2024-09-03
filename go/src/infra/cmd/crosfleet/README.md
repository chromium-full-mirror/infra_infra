# Prepare to build

```
cd infra/go
eval `./env.py`
cd src/infra/cmd/crosfleet
```

# Build

`go build .`

# Run

`go run . help`

# Run unit tests

`go test ./...`

# Lint check files (vet)

`go vet ./...`

# Format

To format all files:
`go fmt ./...`

Or specific files:
`go fmt internal/run/testcommon.go
