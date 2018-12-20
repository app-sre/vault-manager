// Package approle implements the application of a declarative configuration
// for Vault App Roles.
package approle

import (
	"path/filepath"

	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
)

const appRolePath = "auth/approle/role"

type entry struct {
	Name    string                 `yaml:"name"`
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

	if e.Name != entry.Name {
		return false
	}

	return vault.OptionsEqual(e.Options, entry.Options)
}

func (e entry) Save(client *api.Client) {
	path := filepath.Join(appRolePath, e.Name)
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
		logrus.WithError(err).WithField("path", path).Fatal("failed to write AppRole to Vault instance")
	}
	logrus.WithField("path", path).Info("successfully wrote AppRole")
}

func (e entry) Delete(client *api.Client) {
	path := filepath.Join(appRolePath, e.Name)
	_, err := client.Logical().Delete(path)
	if err != nil {
		logrus.WithError(err).WithField("path", path).Fatal("failed to delete AppRole from Vault instance")
	}
	logrus.WithField("path", path).Info("successfully deleted AppRole from Vault instance")
}

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("approle", config{})
}

// Apply ensures that an instance of Vault's AppRoles are configured exactly
// as provided.
//
// This function exits the program if an error occurs.
func (c config) Apply(entriesBytes []byte, dryRun bool) {
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		logrus.WithError(err).Fatal("failed to decode AppRole configuration")
	}

	// Get the secret with the existing App Roles.
	secret, err := vault.ClientFromEnv().Logical().List(appRolePath)
	if err != nil {
		logrus.WithError(err).Fatal("failed to list AppRoles from Vault instance")
	}

	// Build a list of all the existing entries.
	var existingRoles []entry
	if secret != nil {
		for _, roleName := range secret.Data["keys"].([]interface{}) {
			path := filepath.Join(appRolePath, roleName.(string))
			roleSecret, err := vault.ClientFromEnv().Logical().Read(path)
			if err != nil {
				logrus.WithError(err).WithField("path", path).Fatal("failed to read AppRole secret")
			}

			existingRoles = append(existingRoles, entry{
				Name:    roleName.(string),
				Options: roleSecret.Data,
			})
		}
	}

	// Diff the local configuration with the Vault instance.
	entriesToBeWritten, entriesToBeDeleted := vault.DiffItems(asItems(entries), asItems(existingRoles))

	if dryRun == true {
		for _, w := range entriesToBeWritten {
			logrus.Infof("[Dry Run]\tpackage=approle\tentry to be written='%v'", w)
		}
		for _, d := range entriesToBeDeleted {
			logrus.Infof("[Dry Run]\tpackage=approle\tentry to be deleted='%v'", d)
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
