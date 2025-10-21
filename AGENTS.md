# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

vault-manager is a Go-based automation tool for managing HashiCorp Vault configurations. It runs as a reconciliation loop that ensures Vault instances match desired state defined in GraphQL-sourced configuration data.

The application queries a GraphQL server for configuration data and applies changes to one or more Vault instances to bring them into compliance with the desired state.

## Development Commands

### Building and Testing
- `make gotest` - Run Go unit tests with CGO disabled
- `make gobuild` - Build the binary (runs tests first)
- `make build` - Build container image using podman/docker
- `make generate` - Generate OpenShift templates from Helm charts

### Local Development
- `./local-dev.sh` - Set up local development environment with containers
- `source dev-env` - Source environment variables for local development
- `make build-test-container` - Build test container for integration testing
- `make test-with-compose` - Run full integration test suite using compose

### Container Operations
- `make push` - Push container images to registry
- `make down` - Stop and clean up compose environment

## Architecture

### Core Components

**Main Application** (`cmd/vault-manager/main.go`):
- Entry point with CLI flag parsing
- Orchestrates reconciliation loop with configurable priority ordering
- Handles GraphQL configuration fetching and Vault client initialization
- Supports both one-time execution and continuous loop modes

**Vault Client Package** (`pkg/vault/`):
- Wrapper around HashiCorp Vault API client
- Provides methods for all Vault operations (secrets, policies, auth, etc.)
- Manages multiple Vault instance connections
- Handles different KV engine versions (v1 and v2)

**Top-level Configuration System** (`toplevel/`):
- Plugin-style architecture for different Vault configuration types
- Each subdirectory handles a specific Vault component:
  - `audit/` - Audit device management
  - `auth/` - Authentication backend configuration
  - `entity/` - Identity entity management
  - `group/` - Identity group management
  - `policy/` - Policy management
  - `role/` - Role configuration (AppRole, K8s, OIDC)
  - `secretsengine/` - Secret engine management

### Configuration Priority Order
The application applies configurations in priority order:
1. vault_instances (1)
2. vault_policies (2)  
3. vault_audit_backends (3)
4. vault_secret_engines (4)
5. vault_auth_backends (5)
6. vault_roles (6)
7. vault_entities (7)
8. vault_groups (8)

### Authentication Methods
- Token-based authentication (`VAULT_AUTHTYPE=token`)
- AppRole authentication (`VAULT_AUTHTYPE=approle`)
- Kubernetes authentication support (via `--kube-auth` flag)

## Key Environment Variables

**Vault Connection**:
- `VAULT_ADDR` - Vault instance URL
- `VAULT_TOKEN` - Direct token authentication
- `VAULT_AUTHTYPE` - Authentication method (token/approle)
- `VAULT_ROLE_ID`, `VAULT_SECRET_ID` - AppRole credentials

**GraphQL Configuration**:
- `GRAPHQL_SERVER` - GraphQL server URL (default: http://localhost:4000/graphql)
- `GRAPHQL_QUERY_FILE` - Path to GraphQL query file (default: /query.graphql)
- `GRAPHQL_USERNAME`, `GRAPHQL_PASSWORD` - Basic auth credentials

**Runtime Configuration**:
- `RECONCILE_SLEEP_TIME` - Sleep duration between reconciliation loops
- `METRICS_SERVER_PORT` - Prometheus metrics port (default: 9090)
- `LOG_FILE_LOCATION` - Optional log file path

## Testing Infrastructure

The project uses BATS (Bash Automated Testing System) for integration testing with a comprehensive test setup:

- **Compose Environment** (`tests/compose.yml`): Multi-container setup with Keycloak, qontract-server, and dual Vault instances
- **Test Data** (`tests/app-interface/`): Complete app-interface data bundle for testing all Vault configurations
- **Fixtures** (`tests/fixtures/`): GraphQL queries for specific test scenarios
- **Test Scripts**: `tests/run-tests.sh` and `tests/run-tests-compose.sh`

### Local Testing Setup
1. Run `./local-dev.sh` to start containers (Keycloak on 8080, Vault on 8200/8202, qontract-server on 4000)
2. Source `dev-env` for environment variables
3. Run vault-manager with `--dry-run` flag for safe testing

## Common Development Tasks

### Running Locally Against Test Environment
```bash
./local-dev.sh
source dev-env
go run cmd/vault-manager/main.go --dry-run
```

### Updating Test Data
When modifying schemas or queries, regenerate test data:
1. Update `SCHEMAS_IMAGE_TAG` in `tests/app-interface/.env`
2. Run `make data` in `tests/app-interface/`
3. Commit the updated `data.json`

### Adding New Vault Configuration Types
1. Create new package under `toplevel/`
2. Implement the `Configuration` interface
3. Register via `RegisterConfiguration()` in init function
4. Add import to `cmd/vault-manager/main.go`
5. Update priority ordering if needed

## Important Notes

- Never commit secrets or credentials to the repository
- The application is designed to be idempotent - safe to run multiple times
- Use `--dry-run` flag extensively during development
- Vault audit device configuration may require specific container permissions
- AppRole output_path validation requires pre-existing secret engines