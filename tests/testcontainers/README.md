# TestContainers for Vault Manager

## Keycloak

```go
go get github.com/stillya/testcontainers-keycloak

keycloakContainer, err := keycloak.RunContainer(context.Background(),
  testcontainers.WithImage("quay.io/keycloak/keycloak:21.1"),
  testcontainers.WithWaitStrategy(wait.ForListeningPort("8080/tcp")),
  keycloak.WithContextPath("/auth"),
  keycloak.WithRealmImportFile("../testdata/realm-export.json"),
  keycloak.WithAdminUsername("admin"),
  keycloak.WithAdminPassword("admin"),
)
```
## Qontract Server

```go
func setupQontractServer(ctx context.Context, t *testing.T) (testcontainers.Container, string) {
		// volumes: - ../tests/app-interface:/bundle:Z
		// We create a bind mount from the local filesystem to the container.
		// The 'Z' SELinux label is not needed here.
		BindMounts: []testcontainers.BindMount{
			{
				Source:   bundlePath,
				Target:   "/bundle",
				ReadOnly: true, // It's good practice to mount test data as read-only.
			},
		},
}
```

## Vault

```go
go get github.com/testcontainers/testcontainers-go/modules/vault

vaultContainer, err := vault.Run(context.Background(),
  "hashicorp/vault:1.13.0",
  vault.WithToken("root-token"),
  vault.WithInitCommand("secrets enable transit", "write -f transit/keys/my-key"),
  vault.WithInitCommand("kv put secret/test1 foo1=bar1"),
)
```
## Vault Manager

```go
func setupVaultManager(ctx context.Context, t *testing.T) (testcontainers.Container, string) {
}
```
