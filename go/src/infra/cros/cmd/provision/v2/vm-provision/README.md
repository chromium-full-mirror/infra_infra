# vm_provision

A CFT service which leases a VM for VM testing flow. It communicates with VM leaser service to get host address and port of the VM, lease id etc.

# How to run
- Requires luci-auth token for authorization for prod endpoint. The service read the token from the file. This token is fetched during docker build step and pasted in a file. For local run make sure the authToken.txt exists. Use `luci-auth token` to generate the token.

- go build to build the binary

- Start the server
```
./vm_provision -port=8010 -log=<log path where logs file should be created>
```

# Output
```
Response: name:"operations/d270475f-294f-4bdd-ac3f-6787208828aa"  done:true  response:{[type.googleapis.com/chromiumos.test.api.InstallResponse]:{metadata:{[type.googleapis.com/chromiumos.test.api.LeaseVMResponse]:{lease_id:"vm-dbd34ae8-dce9-41e1-86c7-9a7e045f7512"  vm:{id:"vm-dbd34ae8-dce9-41e1-86c7-9a7e045f7512"  address:{host:"34.29.136.224"  port:22}}}}}}
```
