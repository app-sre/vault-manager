package vault

import (
	"fmt"
	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

// write secret to vault
func WriteSecret(secretPath string, secretData map[string]interface{}) {
	if !DataInSecret(secretData, secretPath) {
		_, err := ClientFromEnv().Logical().Write(secretPath, secretData)
		if err != nil {
			log.WithField("package", "vault").WithError(err).WithField("path", secretPath).Fatalf("failed to write Vault secret ")
		}
	}
}

// read secret from vault
func ReadSecret(secretPath string) *api.Secret {
	secret, err := ClientFromEnv().Logical().Read(secretPath)
	if err != nil {
		log.WithField("package", "vault").WithError(err).WithField("path", secretPath).Fatal("failed to read Vault secret")
	}
	return secret
}

// list secrets
func ListSecrets(path string) *api.Secret {
	secretsList, err := ClientFromEnv().Logical().List(path)
	if err != nil {
		log.WithField("package", "vault").WithError(err).WithField("path", path).Fatal("failed to list Vault secrets")
	}
	return secretsList
}

// delete secret from vault
func DeleteSecret(secretPath string) {
	_, err := ClientFromEnv().Logical().Delete(secretPath)
	if err != nil {
		log.WithField("package", "vault").WithError(err).WithField("path", secretPath).Fatal("failed to delete Vault secret")
	}
}

// DataInSecret compare given data with data stored in the vault secret
func DataInSecret(data map[string]interface{}, path string) bool {
	// read desired secret
	secret := ReadSecret(path)
	if secret == nil {
		return false
	}
	for k, v := range data {
		if strings.HasSuffix(k, "ttl") || strings.HasSuffix(k, "period") {
			dur, err := time.ParseDuration(v.(string))
			if err != nil {
				log.WithError(err).WithField("option", k).Fatal("failed to parse duration from data")
			}
			v = int64(dur.Seconds())
		}
		if fmt.Sprintf("%v", secret.Data[k]) == fmt.Sprintf("%v", v) {
			continue
		}
		return false
	}
	return true
}
