package testcontainers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	keycloak "github.com/stillya/testcontainers-keycloak"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestKeycloakBasicStartup(t *testing.T) {
	ctx := context.Background()

	// Start Keycloak container with same configuration as compose
	keycloakContainer, err := keycloak.Run(ctx,
		"quay.io/keycloak/keycloak:22.0.4",
		keycloak.WithAdminUsername("admin"),
		keycloak.WithAdminPassword("admin"),
		// Use custom command to match compose: start-dev --http-port 8180
		testcontainers.WithCmd("start-dev", "--http-port", "8180"),
		// Wait for the master realm endpoint to be available
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/realms/master").
				WithPort("8180/tcp").
				WithStartupTimeout(60*time.Second).
				WithPollInterval(2*time.Second),
		),
		// Map container port 8180 to host port 8180 (matching compose)
		testcontainers.WithExposedPorts("8180:8180"),
	)
	require.NoError(t, err, "Failed to start Keycloak container")

	// Ensure cleanup
	defer func() {
		if err := keycloakContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	// Get the container host and port
	host, err := keycloakContainer.Host(ctx)
	require.NoError(t, err)

	port, err := keycloakContainer.MappedPort(ctx, "8180")
	require.NoError(t, err)

	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())
	t.Logf("Keycloak started at: %s", endpoint)

	// Test basic connectivity to master realm
	resp, err := http.Get(endpoint + "/realms/master")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "Master realm should be accessible")

	// Test admin console accessibility
	resp, err = http.Get(endpoint + "/admin/")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "Admin console should be accessible")
}

func TestKeycloakWithCustomConfiguration(t *testing.T) {
	ctx := context.Background()

	// Start Keycloak container
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
	require.NoError(t, err)
	defer keycloakContainer.Terminate(ctx)

	// Get the container host and port
	host, err := keycloakContainer.Host(ctx)
	require.NoError(t, err)

	port, err := keycloakContainer.MappedPort(ctx, "8180")
	require.NoError(t, err)

	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())
	t.Logf("Keycloak ready for configuration at: %s", endpoint)

	// Import configuration (realm, client, user) matching the compose setup
	err = configureKeycloak(endpoint)
	require.NoError(t, err, "Failed to configure Keycloak with test realm and data")

	// Verify master realm is still accessible
	t.Logf("🔍 Verifying master realm...")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(endpoint + "/realms/master")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	t.Logf("✅ Master realm accessible")

	// Verify test realm was created and is accessible
	t.Logf("🔍 Verifying test realm...")
	resp, err = client.Get(endpoint + "/realms/test")
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Logf("❌ Test realm response: %d %s - Body: %s", resp.StatusCode, resp.Status, string(body))
	}
	require.Equal(t, http.StatusOK, resp.StatusCode, "Test realm should be accessible")
	t.Logf("✅ Test realm accessible")

	// Verify realm configuration by checking realm info (more reliable than OIDC endpoint)
	t.Logf("🔍 Verifying realm configuration...")

	// Get admin token for verification
	token, err := getAdminToken(endpoint, "admin", "admin")
	require.NoError(t, err)

	// Check if we can get realm details via admin API
	req, err := http.NewRequest("GET", endpoint+"/admin/realms/test", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Logf("❌ Realm admin API response: %d %s - Body: %s", resp.StatusCode, resp.Status, string(body))
	}
	require.Equal(t, http.StatusOK, resp.StatusCode, "Test realm should be accessible via admin API")
	t.Logf("✅ Realm configuration verified via admin API")

	t.Logf("✅ Keycloak configuration complete - test realm, vault client, and test user created")
}

// Keycloak API data structures
type KeycloakRealm struct {
	Realm   string `json:"realm"`
	Enabled bool   `json:"enabled"`
}

type KeycloakClient struct {
	ClientID                string   `json:"clientId"`
	Name                    string   `json:"name,omitempty"`
	Description             string   `json:"description,omitempty"`
	Enabled                 bool     `json:"enabled"`
	ClientAuthenticatorType string   `json:"clientAuthenticatorType,omitempty"`
	Secret                  string   `json:"secret,omitempty"`
	RedirectUris            []string `json:"redirectUris,omitempty"`
	WebOrigins              []string `json:"webOrigins,omitempty"`
}

type KeycloakUser struct {
	Username   string              `json:"username"`
	Email      string              `json:"email,omitempty"`
	Enabled    bool                `json:"enabled"`
	FirstName  string              `json:"firstName,omitempty"`
	LastName   string              `json:"lastName,omitempty"`
	Attributes map[string][]string `json:"attributes,omitempty"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// Helper function to get Keycloak admin token
func getAdminToken(endpoint, username, password string) (string, error) {
	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)
	data.Set("grant_type", "password")
	data.Set("client_id", "admin-cli")

	resp, err := http.PostForm(endpoint+"/realms/master/protocol/openid-connect/token", data)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// Helper function to make authenticated requests to Keycloak Admin API
func makeAuthenticatedRequest(method, url, token string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	return client.Do(req)
}

// Create realm in Keycloak
func createRealm(endpoint, token, realmName string) error {
	realm := KeycloakRealm{
		Realm:   realmName,
		Enabled: true,
	}

	fmt.Printf("   📡 POST %s/admin/realms\n", endpoint)
	resp, err := makeAuthenticatedRequest("POST", endpoint+"/admin/realms", token, realm)
	if err != nil {
		return fmt.Errorf("failed to create realm: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("   📋 Response: %d %s\n", resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("   ❌ Error body: %s\n", string(body))
		return fmt.Errorf("create realm failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Create client in Keycloak realm
func createClient(endpoint, token, realmName string, client KeycloakClient) error {
	url := fmt.Sprintf("%s/admin/realms/%s/clients", endpoint, realmName)

	resp, err := makeAuthenticatedRequest("POST", url, token, client)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create client failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Create user in Keycloak realm
func createUser(endpoint, token, realmName string, user KeycloakUser) error {
	url := fmt.Sprintf("%s/admin/realms/%s/users", endpoint, realmName)

	resp, err := makeAuthenticatedRequest("POST", url, token, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create user failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Configure Keycloak with realm, client, and user from JSON configs
func configureKeycloak(endpoint string) error {
	fmt.Printf("🔧 Starting Keycloak configuration at %s\n", endpoint)

	// Get admin token
	fmt.Printf("🔑 Getting admin token...\n")
	token, err := getAdminToken(endpoint, "admin", "admin")
	if err != nil {
		return fmt.Errorf("failed to get admin token: %w", err)
	}
	fmt.Printf("✅ Admin token obtained\n")

	// Create test realm
	fmt.Printf("🏰 Creating test realm...\n")
	if err := createRealm(endpoint, token, "test"); err != nil {
		return fmt.Errorf("failed to create realm: %w", err)
	}
	fmt.Printf("✅ Test realm created\n")

	// Create vault client (from realm-client.json)
	fmt.Printf("🔐 Creating vault client...\n")
	vaultClient := KeycloakClient{
		ClientID:                "vault",
		Name:                    "vault",
		Description:             "vault-dev",
		Enabled:                 true,
		ClientAuthenticatorType: "client-secret",
		Secret:                  "my-special-client-secret",
		RedirectUris:            []string{"*"},
		WebOrigins:              []string{"*"},
	}

	if err := createClient(endpoint, token, "test", vaultClient); err != nil {
		return fmt.Errorf("failed to create vault client: %w", err)
	}
	fmt.Printf("✅ Vault client created\n")

	// Create test user (from user.json)
	fmt.Printf("👤 Creating test user...\n")
	testUser := KeycloakUser{
		Username:  "tester",
		Email:     "tester@mail.de",
		Enabled:   true,
		FirstName: "Test",
		LastName:  "McTester",
		Attributes: map[string][]string{
			"locale": {"de"},
		},
	}

	if err := createUser(endpoint, token, "test", testUser); err != nil {
		return fmt.Errorf("failed to create test user: %w", err)
	}
	fmt.Printf("✅ Test user created\n")

	fmt.Printf("🎉 Keycloak configuration completed successfully\n")
	return nil
}
