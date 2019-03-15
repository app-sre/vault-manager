package vault

import (
	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
)

// write secret to vault
func WriteSecret(secretPath string, secretData map[string]interface{}) {
	if !DataInSecret(secretData, secretPath) {
		_, err := Client().Logical().Write(secretPath, secretData)
		if err != nil {
			log.WithField("package", "vault").WithError(err).WithField("path", secretPath).Fatalf("failed to write Vault secret ")
		}
	}
}

// read secret from vault
func ReadSecret(secretPath string) *api.Secret {
	secret, err := Client().Logical().Read(secretPath)
	if err != nil {
		log.WithField("package", "vault").WithError(err).WithField("path", secretPath).Fatal("failed to read Vault secret")
	}
	return secret
}

// list secrets
func ListSecrets(path string) *api.Secret {
	secretsList, err := Client().Logical().List(path)
	if err != nil {
		log.WithField("package", "vault").WithError(err).WithField("path", path).Fatal("failed to list Vault secrets")
	}
	return secretsList
}

// delete secret from vault
func DeleteSecret(secretPath string) {
	_, err := Client().Logical().Delete(secretPath)
	if err != nil {
		log.WithField("package", "vault").WithError(err).WithField("path", secretPath).Fatal("failed to delete Vault secret")
	}
}
