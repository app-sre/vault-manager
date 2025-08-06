# vault-manager integration testing

This project uses Go-based integration tests with testcontainers for reliable, containerized testing.

Upon commit, a build is triggered in the CI pipeline which runs the tests.
The tests automatically manage their own container lifecycle using testcontainers-go.

## Testcontainers

[Testcontainers](https://testcontainers.com/) is a library that provides easy and clean APIs to start throwaway containers for testing. The vault-manager project uses testcontainers-go to spin up:

- Primary and Secondary Vault instances
- Keycloak for OIDC authentication
- qontract-server for GraphQL data
- All dependencies in isolated container environments

## Running tests locally

From the top level directory of the project, you have several options:

### Run all testcontainers tests
```bash
make test-testcontainers
```

### Run only pod-based tests (individual containers per test)
```bash
make test-testcontainers-pod
```

### Run only shared container tests (optimized performance)
```bash
make test-testcontainers-shared
```

### Run specific tests
```bash
cd tests/testcontainers
go test -v -run TestVaultManagerSecretEnginesShared
```

## Test Architecture

The tests use two approaches:

### Pod-based Tests (Individual Containers)
- Each test creates its own isolated pod with all required services
- Tests ending with `Pod` (e.g., `TestVaultManagerSecretEnginesPod`)
- Complete isolation but slower execution (~95 seconds per test)

### Shared Container Tests (Optimized Performance)
- One-time container setup shared across all tests in the suite
- Tests ending with `Shared` (e.g., `TestVaultManagerSecretEnginesShared`)
- Vault state reset between tests for isolation
- Faster execution (~15 seconds per test after ~90 second setup)

## Test Cases

The following test cases are implemented:

* **Secret Engines**: Tests KV secret engines configuration (app-interface/, app-sre/)
* **Entities**: Tests Vault identity entities and OIDC aliases creation
* **Groups**: Tests identity groups with policy mappings
* **Roles**: Tests AppRole creation and configuration
* **Auth Backends**: Tests authentication backend configuration with policy mappings
* **Audit Devices**: Tests audit device enablement and configuration
* **Policies**: Tests Vault policy creation and management
* **Flags**: Tests dry-run flag functionality and error handling
* **Errors**: Tests error handling and instance isolation

## Performance

The shared container approach provides significant performance improvements:
- Individual tests: ~95 seconds each
- Shared container tests: ~15 seconds each (after one-time ~90 second setup)
- Overall test suite improvement: ~50% faster execution

## Container Requirements

Tests require either Docker or Podman to be available on the system. The testcontainers library will automatically detect and use the available container runtime.