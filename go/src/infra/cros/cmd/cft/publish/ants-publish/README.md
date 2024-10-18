# ants-publish


## Design doc
go/ants-publish

## Testing services

### ants-publish (CLI)
All provided paths should absolute paths.
```shell
 ants-publish$ go build .
 ants-publish$ ./ants-publish cli -input ./testdata/publishrequest.json -output <desired_output_file_path> -log-path <desired_log_path>
```
By default, it uses `/tmp/ants-publish/` to log all messages. -log-path var can be used to point it to another location.

### ants-publish (Server)

Start ants-publish server
```shell
 ants-publish$ go build .
 ants-publish$ ./ants-publish server --port <port_number>
server_command.go: running server mode:
ants_publish_server.go: ants-publish-service listen to request at  [::]:44349
```
Examples:
port_number = 44349 (Provide 0 if we want to start the server on a random available port)

Once the server has started, you may use [`grpc_cli`](http://go/grpc_cli) to
interact with the services. Example:
```shell
$ grpc_cli ls localhost:44349 chromiumos.test.api.GenericPublishService --channel_creds_type insecure
Publish
Publish
```
Create `key.json` file in the same folder(ants-publish) where the binary lives. This file should have the GCS credentials that will be used for GCS upload. This info will be made part of the input soon and will be expected to provide this as part of input.
```shell
$ grpc_cli call localhost:38869 chromiumos.test.api.GenericPublishService.Publish --infile=input.textproto --channel_creds_type insecure
connecting to localhost:44349
name: "operations/655953d1-968f-4e61-96c8-8929c53c002a"
done: true
response {
  type_url: "type.googleapis.com/chromiumos.test.api.PublishResponse"
}
Rpc succeeded with OK status
```
Examples:
1. input.textproto = http://go/paste/6585018561331200 (Please modify the aritifact_dir_path.path and gcs_path.path correctly)
2. More examples are included in the [examples](http://cs/chromeos_public/src/platform/dev/src/chromiumos/test/publish/examples/) folder.

### ants-publish (testing)

This only allows us to read from Ants. Writing to Ants is restricted to service accounts and bots only and cannot be replicated locally. There is a way to impersonate a service account locally, but is currently not supported.

```shell
 ants-publish$ go build .
 ants-publish$ ./ants-publish test -invocation-id "I123456" -workunit-id "WU123456" -testresult-id 1234
```

This will print out the details for invocation, workunit or test result accordingly. You can choose to get details for one or all three.
If invocation id is not empty, along with invocation details, it will also print out all the work units and test results associated with that invocation.
