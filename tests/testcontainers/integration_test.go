package testcontainers

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	keycloak "github.com/stillya/testcontainers-keycloak"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestKeycloakQontractIntegration tests both services running together
func TestKeycloakQontractIntegration(t *testing.T) {
	ctx := context.Background()

	t.Logf("🚀 Starting integration test - Keycloak + qontract-server")

	// Start Keycloak container
	t.Logf("🔧 Starting Keycloak container...")
	keycloakContainer, err := keycloak.Run(ctx,
		"quay.io/keycloak/keycloak:22.0.4",
		keycloak.WithAdminUsername("admin"),
		keycloak.WithAdminPassword("admin"),
		testcontainers.WithCmd("start-dev", "--http-port", "8180"),
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/realms/master").
				WithPort("8180/tcp").
				WithStartupTimeout(60*time.Second),
		),
		testcontainers.WithExposedPorts("8180:8180"),
	)
	require.NoError(t, err, "Failed to start Keycloak container")
	defer keycloakContainer.Terminate(ctx)

	// Get Keycloak endpoint
	keycloakHost, err := keycloakContainer.Host(ctx)
	require.NoError(t, err)
	keycloakPort, err := keycloakContainer.MappedPort(ctx, "8180")
	require.NoError(t, err)
	keycloakEndpoint := fmt.Sprintf("http://%s:%s", keycloakHost, keycloakPort.Port())
	t.Logf("✅ Keycloak started at: %s", keycloakEndpoint)

	// Configure Keycloak with test realm and vault client
	t.Logf("🔧 Configuring Keycloak...")
	err = configureKeycloak(keycloakEndpoint)
	require.NoError(t, err, "Failed to configure Keycloak")
	t.Logf("✅ Keycloak configured with test realm and vault client")

	// Start qontract-server container
	t.Logf("🔧 Starting qontract-server container...")
	bundlePath := "/home/jmosco/dev/work/oss/vault-manager/tests/app-interface"
	
	qontractReq := testcontainers.ContainerRequest{
		Image: "quay.io/redhat-services-prod/app-sre-tenant/qontract-server-master/qontract-server-master:f3fb9719c39b0413edc9e2254f942e725bc57344e72d16b4b947ae215d65c59b",
		ExposedPorts: []string{"4000/tcp"},
		Env: map[string]string{
			"LOAD_METHOD":    "fs",
			"DATAFILES_FILE": "/bundle/data.json",
		},
		Mounts: []testcontainers.ContainerMount{
			testcontainers.BindMount(bundlePath, "/bundle"),
		},
		WaitingFor: wait.ForHTTP("/healthz").
			WithPort("4000/tcp").
			WithStartupTimeout(60*time.Second),
	}

	qontractContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: qontractReq,
		Started:          true,
	})
	require.NoError(t, err, "Failed to start qontract-server container")
	defer qontractContainer.Terminate(ctx)

	// Get qontract-server endpoint
	qontractHost, err := qontractContainer.Host(ctx)
	require.NoError(t, err)
	qontractPort, err := qontractContainer.MappedPort(ctx, "4000")
	require.NoError(t, err)
	qontractEndpoint := fmt.Sprintf("http://%s:%s", qontractHost, qontractPort.Port())
	t.Logf("✅ qontract-server started at: %s", qontractEndpoint)

	// Verify both services are accessible
	t.Logf("🔍 Verifying service accessibility...")
	
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Test Keycloak health
	resp, err := client.Get(keycloakEndpoint + "/realms/master")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "Keycloak should be accessible")
	
	// Test Keycloak test realm
	resp, err = client.Get(keycloakEndpoint + "/realms/test")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "Keycloak test realm should be accessible")

	// Test qontract-server health
	resp, err = client.Get(qontractEndpoint + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "qontract-server should be accessible")

	t.Logf("✅ Both services verified - integration test successful!")
	
	// Log endpoints for vault-manager integration
	t.Logf("🎯 Integration endpoints ready:")
	t.Logf("   - Keycloak: %s", keycloakEndpoint)
	t.Logf("   - qontract-server: %s", qontractEndpoint)
	t.Logf("   - Test realm: %s/realms/test", keycloakEndpoint)
	t.Logf("   - GraphQL: %s/graphql", qontractEndpoint)
}

// TestVaultManagerDependencies simulates what vault-manager needs from both services
func TestVaultManagerDependencies(t *testing.T) {
	ctx := context.Background()

	// Start both services in parallel for speed
	t.Logf("🚀 Starting Keycloak and qontract-server in parallel...")

	// Start Keycloak
	keycloakContainer, err := keycloak.Run(ctx,
		"quay.io/keycloak/keycloak:22.0.4",
		keycloak.WithAdminUsername("admin"),
		keycloak.WithAdminPassword("admin"),
		testcontainers.WithCmd("start-dev", "--http-port", "8180"),
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/realms/master").WithPort("8180/tcp").WithStartupTimeout(60*time.Second),
		),
		testcontainers.WithExposedPorts("8180:8180"),
	)
	require.NoError(t, err)
	defer keycloakContainer.Terminate(ctx)

	// Start qontract-server
	bundlePath := "/home/jmosco/dev/work/oss/vault-manager/tests/app-interface"
	qontractReq := testcontainers.ContainerRequest{
		Image: "quay.io/redhat-services-prod/app-sre-tenant/qontract-server-master/qontract-server-master:f3fb9719c39b0413edc9e2254f942e725bc57344e72d16b4b947ae215d65c59b",
		ExposedPorts: []string{"4000/tcp"},
		Env: map[string]string{
			"LOAD_METHOD":    "fs", 
			"DATAFILES_FILE": "/bundle/data.json",
		},
		Mounts: []testcontainers.ContainerMount{
			testcontainers.BindMount(bundlePath, "/bundle"),
		},
		WaitingFor: wait.ForHTTP("/healthz").WithPort("4000/tcp").WithStartupTimeout(60*time.Second),
	}

	qontractContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: qontractReq,
		Started:          true,
	})
	require.NoError(t, err)
	defer qontractContainer.Terminate(ctx)

	// Get endpoints
	keycloakHost, _ := keycloakContainer.Host(ctx)
	keycloakPort, _ := keycloakContainer.MappedPort(ctx, "8180")
	keycloakEndpoint := fmt.Sprintf("http://%s:%s", keycloakHost, keycloakPort.Port())

	qontractHost, _ := qontractContainer.Host(ctx)
	qontractPort, _ := qontractContainer.MappedPort(ctx, "4000")
	qontractEndpoint := fmt.Sprintf("http://%s:%s", qontractHost, qontractPort.Port())

	// Configure Keycloak
	err = configureKeycloak(keycloakEndpoint)
	require.NoError(t, err)

	// Test vault-manager requirements
	t.Logf("🧪 Testing vault-manager requirements...")

	client := &http.Client{Timeout: 10 * time.Second}

	// 1. GraphQL server should respond (vault-manager queries this)
	resp, err := client.Get(qontractEndpoint + "/graphql")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest,
		"GraphQL endpoint should be available for vault-manager")

	// 2. OIDC configuration should be available (vault needs this for auth)
	token, err := getAdminToken(keycloakEndpoint, "admin", "admin")
	require.NoError(t, err, "Should be able to get admin token for configuration")
	require.NotEmpty(t, token, "Admin token should not be empty")

	// 3. Test realm should exist (vault OIDC points to this)
	resp, err = client.Get(keycloakEndpoint + "/realms/test")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "Test realm must exist for vault OIDC")

	t.Logf("✅ All vault-manager dependencies satisfied!")
	t.Logf("🎯 Ready for vault-manager integration:")
	t.Logf("   - GRAPHQL_SERVER=%s/graphql", qontractEndpoint)
	t.Logf("   - OIDC realm: %s/realms/test", keycloakEndpoint)
	t.Logf("   - Vault client: vault (secret: my-special-client-secret)")
}