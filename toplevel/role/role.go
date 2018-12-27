// Package role implements the application of a declarative configuration
// for Vault App Roles.
package role

import (
	"path/filepath"

	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
)

type entry struct {
	Name    string                 `yaml:"name"`
	Type    string                 `yaml:"type"`
	Mount   string                 `yaml:"mount"`
	Options map[string]interface{} `yaml:"options"`
}

var _ vault.Item = entry{}

func (e entry) Key() string {
	return e.Name
}

func (e entry) Equals(i interface{}) bool {
	entry, ok := i.(entry)
	if !ok {
		return false
	}

	return e.Name == entry.Name &&
		e.Type == entry.Type &&
		e.Mount == entry.Mount &&
		vault.OptionsEqual(e.Options, entry.Options)
}

func (e entry) Save(client *api.Client) {
	path := filepath.Join("auth", e.Mount, "role", e.Name)
	options := make(map[string]interface{})
	for k, v := range e.Options {
		// local_secret_ids can not be changed after creation so we skip this option
		if k == "local_secret_ids" {
			continue
		}
		options[k] = v
	}
	_, err := client.Logical().Write(path, options)
	if err != nil {
		logrus.WithError(err).WithField("path", path).WithField("type", e.Type).Fatalf("failed to write role to Vault instance")
	}
	logrus.WithField("path", path).WithField("type", e.Type).Info("successfully wrote role")
}

func (e entry) Delete(client *api.Client) {
	path := filepath.Join("auth", e.Mount, "role", e.Name)
	_, err := client.Logical().Delete(path)
	if err != nil {
		logrus.WithError(err).WithField("path", path).WithField("type", e.Type).Fatal("failed to delete role from Vault instance")
	}
	logrus.WithField("path", path).WithField("type", e.Type).Info("successfully deleted role from Vault instance")
}

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("roles", config{})
}

// Apply ensures that an instance of Vault's roles are configured exactly
// as provided.
//
// This function exits the program if an error occurs.
func (c config) Apply(entriesBytes []byte, dryRun bool) {
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		logrus.WithError(err).Fatal("failed to decode role configuration")
	}

	existingAuthBackends, err := vault.ClientFromEnv().Sys().ListAuth()
	if err != nil {
		logrus.WithError(err).Fatal("failed to list authentication backends from Vault instance")
	}

	var existingRoles []entry

	if existingAuthBackends != nil {
		for authBackend := range existingAuthBackends {
			// Get the secret with the existing App Roles.
			path := filepath.Join("auth", authBackend, "role")
			secret, err := vault.ClientFromEnv().Logical().List(path)
			if err != nil {
				logrus.WithError(err).Fatal("failed to list roles from Vault instance")
			}

			if secret != nil {
				// Build a list of all the existing entries.
				for _, roleName := range secret.Data["keys"].([]interface{}) {
					path := filepath.Join("auth", authBackend, "role", roleName.(string))
					roleSecret, err := vault.ClientFromEnv().Logical().Read(path)
					if err != nil {
						logrus.WithError(err).WithField("path", path).WithField("type", existingAuthBackends[authBackend].Type).Fatal("failed to read role secret")
					}

					existingRoles = append(existingRoles, entry{
						Name:    roleName.(string),
						Type:    existingAuthBackends[authBackend].Type,
						Mount:   authBackend,
						Options: roleSecret.Data,
					})
				}
			}
		}
	}

	// Diff the local configuration with the Vault instance.
	entriesToBeWritten, entriesToBeDeleted := vault.DiffItems(asItems(entries), asItems(existingRoles))

	if dryRun == true {
		for _, w := range entriesToBeWritten {
			logrus.Infof("[Dry Run]\tpackage=role\tentry to be written='%v'", w)
		}
		for _, d := range entriesToBeDeleted {
			logrus.Infof("[Dry Run]\tpackage=role\tentry to be deleted='%v'", d)
		}
	} else {
		// Write any missing App Roles to the Vault instance.
		for _, e := range entriesToBeWritten {
			e.(entry).Save(vault.ClientFromEnv())
		}

		// Delete any App Roles from the Vault instance.
		for _, e := range entriesToBeDeleted {
			e.(entry).Delete(vault.ClientFromEnv())
		}
	}
}

func asItems(xs []entry) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}
