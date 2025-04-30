# Chrome OS Lab Inventory AppEngine Service

Chrome OS Lab inventory is a service which manages lab configs of all devices
in ChromeOS.

Services maintained:
  - Inventory v2 (Go Service)

## Inventory v2 Go Service
### Development
Run this for a devserver at http://localhost:8082:

```bash
make dev
```

### Deployment
To deploy staging, run the following:

```bash
# Upload and promote the Go service to staging
make up-dev
```

To deploy production, run the following:

```bash
# Deploy the Go service to production without routing traffic to it
make up-prod

# Route all traffic to the uploaded Go service
make switch-prod
```

