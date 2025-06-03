# Setting up a CTP plugin filter

## 1. Create a new filter using this template

Create a new folder: `infra/go/src/infra/cros/cmd/ctpv2-filters/<your-new-filter>`

Copy the main.go of the example-filter into your new folder. Update the structures to reflect your filters name.

## 2. Create a CIPD yaml definition.

Create a CIPD yaml definition for your new filter. This file will be used to package and deploy your filter to the CIPD package storage.

Go to `infra/build/packages` and create a new file for your filter.
```yaml
package: chromiumos/infra/ctpv2-filters/example-filter/${platform}
description: CTPv2 example filter to use as boilerplate.
platforms:
  - linux-amd64
go_packages:
  - go.chromium.org/infra/cros/cmd/ctpv2-filters/example-filter
root: ../../go/bin
data:
  - file: example-filter${exe_suffix}
  - version_file: .versions/example-filter${exe_suffix}.cipd_version
```

## 2.a Request a TSE member to add staging and prod tags

Once your initial CL has landed to create your filter's CIPD package, request a TSE member to run these lines for your filter to set the initial staging and prod tags. This is necessary for the uprev system to move those tags properly as it is currently not automated to initialize these tags. See https://g3doc.corp.google.com/company/teams/chrome/ops/chromeos/chromeos-infra/continuous_integration/guides/golang.md?cl=head
```bash
cipd set-ref chromiumos/infra/ctpv2-filters/example-filter/linux-amd64 -version latest -ref staging &&
cipd set-ref chromiumos/infra/ctpv2-filters/example-filter/linux-amd64 -version latest -ref prod
```

# 2.b Add your CIPD package to the CIPD uprever

Replicate this CL: https://crrev.com/i/8310158

## 2.c Skipping the CIPD package creation

For testing purposes only, you can skip the CIPD package creation and instead directly insert the binary of the filter into the container when building. After you setup the Dockerfile in step 3, set as:
```Dockerfile
FROM us-docker.pkg.dev/cros-registry/base-images/ubuntu:ubuntuProd

COPY <your-new-filter-binary> /usr/bin/
RUN chmod 755 /usr/bin/<your-new-filter-binary>
```
And add the binary to the resources folder in container_uprev and add to the resources of the config.
```go
configs := []*UprevConfig{
  ...
  {
    Name: "example-filter",
    Resources: []string{"<your-new-filter-binary>"},
    CloudRunConfig: &cloudrun.Config{},
  },
  ...
}
```

## 3. Create an uprev config

Filters run as containers within a Cloud Run instance. To automatically push out new versions of your filter to its own Cloud Run instance, you will need to create a config within container_uprev. This includes creating a Dockerfile for the filter.

See: `infra/go/src/infra/cros/cmd/container_uprev/`

```Dockerfile
FROM us-docker.pkg.dev/cros-registry/base-images/ubuntu:ubuntuProd

COPY example-filter /usr/bin/
```

```go
configs := []*UprevConfig{
  ...
  {
    Name: "example-filter",
    CIPDPackages: []*CIPDPackage{
      NewCIPDPackage("chromiumos/infra/ctpv2-filters/example-filter/${platform}"),
    },
    CloudRunConfig: &cloudrun.Config{},
  },
  ...
}
```

## 4. Add the filter to a Kron suite configuration

Filters do not run unless they are added as part of a CTP request, unless they are added to the default filters that run every time (rare).

To add your filter to a Kron suite configuration, set the field `karbon_filters` to include your new filter.

```python
def example_filter_config():
    """Defines an example filter suite

    Returns:
      ScheduleConfig object.
    """
    return config_gen.create_config(
        name = "ExampleFilterConfig",
        ...
        karbon_filters = [
            config_gen.create_known_udf(
                "example-filter",
                # Optional.
                # Set if your filter requires some arguments passed in.
                args = [
                    "-arg1",
                    "value1",
                ],
            ),
        ],
        ...
    )
```

# Testing your filter

## Local

TODO(cdelagarza): Add local filter testing client.

## LED

`led get-build <bbid> > build.json`

Edit build.json to add your filter to the request properties.
```json
...
"softwareDependencies": [...],
"userDefinedFilters": [
  {
    "containerInfo": {
      "container": {
        "name": "example-filter",
        // Optional.
        // Set if you published your filter to a custom tag.
        // This would be for testing non-staging and non-prod.
        "tags": ["custom-tag"]
      },
      // Optional.
      // Set if your filter test requires some arguments passed in.
      "binaryArgs": ["-arg1", "value1"]
    }
  }
],
...
```
`cat build.json | led launch`

# Important Notes

* Filters run as a Cloud Run instance and handle requests in parallel. Ensure that your filter is thread safe.
