package instance

import (
	"errors"
	"fmt"
	"strings"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	"gopkg.in/yaml.v2"

	log "github.com/sirupsen/logrus"
)

type config struct{}

var _ toplevel.Configuration = config{}

var InstanceAddresses []string

type Instance struct {
	Address string `yaml:"address"`
	Auth    auth   `yaml:"auth"`
}

type auth struct {
	Provider     string `yaml:"provider"`
	SecretEngine string `yaml:"secretEngine"`
	RoleID       secret `yaml:"roleID"`
	SecretID     secret `yaml:"secretID"`
	Token        secret `yaml:"token"`
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
	var instances []Instance
	if err := yaml.Unmarshal(entriesBytes, &instances); err != nil {
		log.WithError(err).Fatal("[Vault Instance] failed to decode instance configuration")
	}
	instanceCreds, err := processInstances(instances)
	if err != nil {
		log.WithError(err).Fatal("[Vault Instance] failed to retrieve access credentials")
	}
	// set package global for reference by other toplevels
	for address := range instanceCreds {
		InstanceAddresses = append(InstanceAddresses, address)
	}
	vault.InitClients(instanceCreds, threadPoolSize)
}

// generates map of instance addresses to access credentials stored in master vault
func processInstances(instances []Instance) (map[string]vault.AuthBundle, error) {
	instanceCreds := make(map[string]vault.AuthBundle)

	for _, i := range instances {
		bundle := vault.AuthBundle{
			SecretEngine: i.Auth.SecretEngine,
		}
		switch strings.ToLower(i.Auth.Provider) {
		case vault.APPROLE_AUTH:
			// ensure required values exist
			if i.Auth.RoleID.Field == "" || i.Auth.RoleID.Path == "" ||
				i.Auth.SecretID.Field == "" || i.Auth.SecretID.Path == "" {
				return nil, errors.New("A required approle authentication attribute is missing")
			}
			bundle.VaultSecrets = []*vault.VaultSecret{
				{
					Name:    vault.ROLE_ID,
					Type:    vault.APPROLE_AUTH,
					Path:    vault.FormatSecretPath(i.Auth.RoleID.Path, i.Auth.SecretEngine),
					Field:   i.Auth.RoleID.Field,
					Version: i.Auth.RoleID.Version,
				},
				{
					Name:    vault.SECRET_ID,
					Type:    vault.APPROLE_AUTH,
					Path:    vault.FormatSecretPath(i.Auth.SecretID.Path, i.Auth.SecretEngine),
					Field:   i.Auth.SecretID.Field,
					Version: i.Auth.SecretID.Version,
				},
			}
		case vault.TOKEN_AUTH:
			if i.Auth.Token.Field == "" || i.Auth.Token.Path == "" {
				return nil, errors.New("A required token authentication attribute is missing")
			}
			bundle.VaultSecrets = []*vault.VaultSecret{
				{
					Name:    vault.TOKEN,
					Type:    vault.TOKEN_AUTH,
					Path:    vault.FormatSecretPath(i.Auth.Token.Path, i.Auth.SecretEngine),
					Field:   i.Auth.Token.Field,
					Version: i.Auth.Token.Version,
				},
			}
		default:
			return nil, errors.New(fmt.Sprintf(
				"Unable to process `auth` attribute of instance definition with address %s", i.Address))
		}
		instanceCreds[i.Address] = bundle
	}

	return instanceCreds, nil
}
