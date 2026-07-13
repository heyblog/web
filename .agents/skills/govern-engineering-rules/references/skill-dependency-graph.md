# Skill Dependency Graph

## API 纵向链路

- `implement-api-endpoint` -> `implement-api-auth`
- `implement-api-endpoint` -> `implement-api-data-access`
- `implement-api-data-access` -> `define-db-schema`

## 实时链路

- `orchestrate-realtime-backend` -> `implement-api-endpoint`
- `orchestrate-realtime-backend` -> `implement-api-data-access`
- `implement-realtime-frontend` -> `manage-web-runtime`
- `run-worker-jobs` -> `orchestrate-realtime-backend`

## Web 与 UI 链路

- `manage-web-runtime` -> `build-ui-system`
- `operate-status-service` -> `build-ui-system`（仅改 UI 时）
