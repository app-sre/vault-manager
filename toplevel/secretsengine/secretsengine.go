// Package secretsengine implements the application of a declarative configuration
// for Vault Secrets Engines.
//
// Secrets Engines used to be referred to as "mounts".
package secretsengine

import (
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
)

type entry struct {
	Path        string            `yaml:"path"`
	Type        string            `yaml:"type"`
	Description string            `yaml:"description"`
	Options     map[string]string `yaml:"options"`
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
		e.Type == entry.Type &&
		e.Description == entry.Description &&
		vault.OptionsEqual(e.ambiguousOptions(), entry.ambiguousOptions())
}

func (e entry) ambiguousOptions() map[string]interface{} {
	opts := make(map[string]interface{}, len(e.Options))
	for k, v := range e.Options {
		opts[k] = v
	}
	return opts
}

func (e entry) enable(client *api.Client) {
	if err := client.Sys().Mount(e.Path, &api.MountInput{
		Type:        e.Type,
		Description: e.Description,
		Options:     e.Options,
	}); err != nil {
		logrus.WithError(err).WithField("path", e.Path).Fatal("failed to enable mount")
	}
	logrus.WithField("path", e.Path).Info("successfully enabled mount")
}

func (e entry) disable(client *api.Client) {
	if err := client.Sys().Unmount(e.Path); err != nil {
		logrus.WithError(err).WithField("path", e.Path).Fatal("failed to disable mount")
	}
	logrus.WithField("path", e.Path).Info("successfully disabled mount")
}

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("secrets-engines", config{})
}

// Apply ensures that an instance of Vault's secrets engine are configured
// exactly as provided.
//
// This function exits the program if an error occurs.
func (c config) Apply(entriesBytes []byte, dryRun bool) {
	// Unmarshal the list of configured secrets engines.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		logrus.WithError(err).Fatal("failed to decode secrets engines configuration")
	}

	// List the existing secrets engines.
	existingMounts, err := vault.ClientFromEnv().Sys().ListMounts()
	if err != nil {
		logrus.WithError(err).Fatal("failed to list Mounts from Vault instance")
	}

	// Build a list of all the existing entries.
	existingSecretsEngines := make([]entry, 0)
	for path, engine := range existingMounts {
		existingSecretsEngines = append(existingSecretsEngines, entry{
			Path:        path,
			Type:        engine.Type,
			Description: engine.Description,
			Options:     engine.Options,
		})
	}

	toBeWritten, toBeDeleted := vault.DiffItems(asItems(entries), asItems(existingSecretsEngines))

	if dryRun == true {
		for _, w := range toBeWritten {
			logrus.Infof("[Dry Run]\tpackage=secrets-engine\tentry to be written='%v'", w)
		}
		for _, d := range toBeDeleted {
			if !isDefaultMount(d.Key()) {
				logrus.Infof("[Dry Run]\tpackage=secrets-engine\tentry to be deleted='%v'", d)
			}
		}
	} else {
		// TODO(riuvshin): implement tuning
		for _, e := range toBeWritten {
			e.(entry).enable(vault.ClientFromEnv())
		}

		for _, e := range toBeDeleted {
			ent := e.(entry)
			if !isDefaultMount(ent.Path) {
				ent.disable(vault.ClientFromEnv())
			}
		}
	}
}

func isDefaultMount(path string) bool {
	switch {
	case strings.HasPrefix(path, "cubbyhole/"),
		strings.HasPrefix(path, "identity/"),
		strings.HasPrefix(path, "secret/"),
		strings.HasPrefix(path, "sys/"):
		return true
	default:
		return false
	}
}

func asItems(xs []entry) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}
