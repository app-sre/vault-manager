// Package auth implements the application of a declarative configuration
// for Vault authentication backends.
package auth

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
)

type entry struct {
	Path        string                 `yaml:"path"`
	Type        string                 `yaml:"type"`
	Description string                 `yaml:"description"`
	Config      map[string]interface{} `yaml:"config"`
}

var _ vault.Item = entry{}

func (e entry) Key() string {
	return e.Path
}

func (e entry) Equals(i interface{}) bool {
	entry, ok := i.(entry)
	if !ok {
		return false
	}

	return vault.EqualPathNames(e.Path, entry.Path) &&
		e.Type == entry.Type
}

func (e entry) enable(client *api.Client) {
	if err := client.Sys().EnableAuthWithOptions(e.Path, &api.EnableAuthOptions{
		Type:        e.Type,
		Description: e.Description,
	}); err != nil {
		logrus.WithError(err).WithField("path", e.Path).Fatal("failed to enable auth backend")
	}
	logrus.WithFields(logrus.Fields{
		"path": e.Path,
		"type": e.Type,
	}).Info("successfully enabled auth backend")
}

func (e entry) disable(client *api.Client) {
	if err := client.Sys().DisableAuth(e.Path); err != nil {
		logrus.WithError(err).WithField("path", e.Path).Fatal("failed to disable auth backend")
	}
	logrus.WithField("path", e.Path).Info("successfully disabled auth backend")
}

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("auth", config{})
}

// Apply ensures that an instance of Vault's authentication backends are
// configured exactly as provided.
//
// This function exits the program if an error occurs.
func (c config) Apply(entriesBytes []byte, dryRun bool) {
	// Unmarshal the list of configured auth backends.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		logrus.WithError(err).Fatal("failed to decode authentication backend configuration")
	}

	// Get the existing enabled auth backends.
	existingAuthMounts, err := vault.ClientFromEnv().Sys().ListAuth()
	if err != nil {
		logrus.WithError(err).Fatal("failed to list authentication backends from Vault instance")
	}

	// Build a list of all the existing entries.
	existingBackends := make([]entry, 0)
	for path, backend := range existingAuthMounts {
		existingBackends = append(existingBackends, entry{
			Path:        path,
			Type:        backend.Type,
			Description: backend.Description,
		})
	}

	toBeWritten, toBeDeleted := vault.DiffItems(asItems(entries), asItems(existingBackends))

	if dryRun == true {
		for _, w := range toBeWritten {
			logrus.Infof("[Dry Run]\tpackage=auth\tentry to be written='%v'", w)
		}

		for _, e := range entries {
			if e.Config != nil {
				path := filepath.Join("auth", e.Path, "config")
				if !vault.DataInSecret(e.Config, path, vault.ClientFromEnv()) {
					logrus.Infof("[Dry Run]\tpackage=auth\tentry to be written path='%v' config='%v'", path, e.Config)
				}
			}
		}

		for _, d := range toBeDeleted {
			if d.Key() == "token/" {
				continue
			}
			logrus.Infof("[Dry Run]\tpackage=auth\tentry to be deleted='%v'", d)
		}
	} else {
		// TODO(riuvshin): implement auth tuning
		for _, e := range toBeWritten {
			e.(entry).enable(vault.ClientFromEnv())
		}

		// configure auth mounts
		for _, e := range entries {
			if e.Config != nil {
				path := filepath.Join("auth", e.Path, "config")
				if !vault.DataInSecret(e.Config, path, vault.ClientFromEnv()) {
					_, err := vault.ClientFromEnv().Logical().Write(path, e.Config)
					if err != nil {
						log.Fatal(err)
					}
					logrus.WithField("path", path).WithField("type", e.Type).Info("auth mount successfully configured")
				}
			}
		}

		for _, e := range toBeDeleted {
			ent := e.(entry)
			if strings.HasPrefix(ent.Path, "token/") {
				continue
			}
			ent.disable(vault.ClientFromEnv())
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
