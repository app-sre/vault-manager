// Package vault implements a wrapper around a Vault API client that retrieves
// credentials from the operating system environment.
package vault

import (
	"os"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
)

/*

func (c *EnvClient) ListSecrets(path string) (map[string]interface{}, error) {
	secret, err := clientFromEnv().Logical().List(path)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		return nil, ErrNilSecret
	}

	return secret.Data, nil
}

func (c *EnvClient) ReadSecret(path string) (map[string]interface{}, error) {
	secret, err := clientFromEnv().Logical().Read(path)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		return nil, ErrNilSecret
	}

	return secret.Data, nil
}

func (c *EnvClient) WriteSecret(path string, options map[string]interface{}) (err error) {
	_, err = clientFromEnv().Logical().Write(path, options)
	return err
}

func (c *EnvClient) DeleteSecret(path string) (err error) {
	_, err = clientFromEnv().Logical().Delete(path)
	return err
}

func (c *EnvClient) ListAuditDevices() (map[string]*api.Audit, error) {
	return clientFromEnv().Sys().ListAudit()
}

func (c *EnvClient) EnableAuditDevice(path string, opts *api.EnableAuditOptions) error {
	return clientFromEnv().Sys().EnableAuditWithOptions(path, opts)
}

func (c *EnvClient) DisableAuditDevice(path string) error {
	return clientFromEnv().Sys().DisableAudit(path)
}

func (c *EnvClient) ListAuthBackends() (map[string]*api.AuthMount, error) {
	return clientFromEnv().Sys().ListAuth()
}

func (c *EnvClient) EnableAuthBackend(path string, opts *api.EnableAuthOptions) error {
	return clientFromEnv().Sys().EnableAuthWithOptions(path, opts)
}

func (c *EnvClient) DisableAuthBackend(path string) error {
	return clientFromEnv().Sys().DisableAuth(path)
}

func (c *EnvClient) ListPolicies() ([]string, error) {
	return clientFromEnv().Sys().ListPolicies()
}

func (c *EnvClient) GetPolicy(name string) (string, error) {
	return clientFromEnv().Sys().GetPolicy(name)
}

func (c *EnvClient) PutPolicy(name, rules string) error {
	return clientFromEnv().Sys().PutPolicy(name, rules)
}

func (c *EnvClient) DeletePolicy(name string) error {
	return clientFromEnv().Sys().DeletePolicy(name)
}

func (c *EnvClient) ListSecretsEngines() (map[string]*api.MountOutput, error) {
	return clientFromEnv().Sys().ListMounts()
}

func (c *EnvClient) EnableSecretsEngine(path string, opts *api.MountInput) error {
	return clientFromEnv().Sys().Mount(path, opts)
}

func (c *EnvClient) DisableSecretsEngine(path string) error {
	return clientFromEnv().Sys().Unmount(path)
}

*/

// ClientFromEnv initializes a Vault client using the environment variables:
// VAULT_ADDR, VAULT_ROLE_ID, VAULT_SECRET_ID, VAULT_TOKEN.
//
// Because individual tokens have usage limits, we re-authenticate for each new
// client.
func ClientFromEnv() *api.Client {
	vaultCFG := api.DefaultConfig()
	vaultCFG.Address = mustGetenv("VAULT_ADDR")

	client, err := api.NewClient(vaultCFG)
	if err != nil {
		logrus.WithError(err).Fatal("failed to initialize Vault client")
	}

	switch authType := defaultGetenv("VAULT_AUTHTYPE", "approle"); strings.ToLower(authType) {
	case "approle":
		roleID := mustGetenv("VAULT_ROLE_ID")
		secretID := mustGetenv("VAULT_SECRET_ID")

		secret, err := client.Logical().Write("auth/approle/login", map[string]interface{}{
			"role_id":   roleID,
			"secret_id": secretID,
		})
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"role":   roleID,
				"secret": secretID,
			}).Fatal("failed to login to Vault with AppRole")
		}
		client.SetToken(secret.Auth.ClientToken)
	case "token":
		client.SetToken(mustGetenv("VAULT_TOKEN"))
	default:
		logrus.WithField("authType", authType).Fatal("unsuported auth type")
	}

	return client
}

func mustGetenv(name string) string {
	env := os.Getenv(name)
	if env == "" {
		logrus.WithField("env", name).Fatal("required environment variable is unset")
	}
	return env
}

func defaultGetenv(name, defaultName string) string {
	env := os.Getenv(name)
	if env == "" {
		env = defaultName
	}
	return env
}

// ListSecretData returns the data stored inside a secret list.
func ListSecretData(client *api.Client, path string) (map[string]interface{}, error) {
	secret, err := client.Logical().List(path)
	if err != nil {
		return nil, err
	}

	return secret.Data, nil
}
