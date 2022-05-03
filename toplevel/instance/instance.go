package instance

import (
	"errors"
	"strings"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	"gopkg.in/yaml.v2"

	log "github.com/sirupsen/logrus"
)

type config struct{}

var _ toplevel.Configuration = config{}

type instance struct {
	Address  string `yaml:"address"`
	AuthType string `yaml:"authType"`
	RoleID   secret `yaml:"roleID"`
	SecretID secret `yaml:"secretID"`
	Token    secret `yaml:"token"`
}

type secret struct {
	Path    string `yaml:"path"`
	Field   string `yaml:"field"`
	Version string `yaml:"version"`
}

func init() {
	toplevel.RegisterConfiguration("vault_instances", config{})
}

// Does not perform any reconciliation operations
// Instead, instance.Apply is utilized to initialize vault instance clients for use by
// other toplevel integrations
func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {
	var instances []instance
	if err := yaml.Unmarshal(entriesBytes, &instances); err != nil {
		log.WithError(err).Fatal("[Vault Instance] failed to decode instance configuration")
	}
	instanceCreds, err := processInstances(instances)
	if err != nil {
		log.WithError(err).Fatal("[Vault Instance] failed to retrieve access credentials")
	}
	vault.InitClients(instanceCreds)
}

// generates map of instance addresses to access credentials stored in master vault
func processInstances(instances []instance) (map[string][]*vault.VaultSecret, error) {
	instanceCreds := make(map[string][]*vault.VaultSecret)

	for _, i := range instances {
		switch strings.ToLower(i.AuthType) {
		case "approle":
			// ensure required values exist
			if i.RoleID.Field == "" || i.RoleID.Path == "" ||
				i.SecretID.Field == "" || i.SecretID.Path == "" {
				return nil, errors.New("A required approle authentication attribute is missing")
			}
			instanceCreds[i.Address] = []*vault.VaultSecret{
				{
					Path:    i.RoleID.Path,
					Field:   i.RoleID.Field,
					Version: i.RoleID.Version,
				},
				{
					Path:    i.SecretID.Path,
					Field:   i.SecretID.Field,
					Version: i.SecretID.Version,
				},
			}
		case "token":
			if i.Token.Field == "" || i.Token.Path == "" {
				return nil, errors.New("A required token authentication attribute is missing")
			}
			instanceCreds[i.Address] = []*vault.VaultSecret{
				{
					Path:    i.Token.Path,
					Field:   i.Token.Field,
					Version: i.Token.Version,
				},
			}
		}
	}

	return instanceCreds, nil
}
