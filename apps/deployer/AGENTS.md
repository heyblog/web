# Deployer

- `server.go` owns the signed webhook, deployment status, and recovery journal.
- `compose.go` owns production container changes and the public Web smoke check.
- `infra/docker/docker-compose.production.yaml` and
  `infra/docker/heyblog-deploy-docker.sh` define the host command contract.
- `infra/DEPLOYMENT.md` owns the host and GitHub setup instructions.
- Validate changes with `mise run deployer:test` and `mise run compose:check`.
