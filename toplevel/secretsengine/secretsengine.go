// Package secretsengine implements the application of a declarative configuration
// for Vault Secrets Engines.
//
// Secrets Engines used to be referred to as "mounts".
package secretsengine

import (
	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"strings"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
)

type entry struct {
	Path        string            `yaml:"_path"`
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
		log.WithField("package", "secrets-engine").WithError(err).WithField("path", e.Path).WithField("type", e.Type).Fatal("failed to enable secrets-engine")
	}
	log.WithField("package", "secrets-engine").WithField("path", e.Path).WithField("type", e.Type).Info("successfully enabled secrets-engine")
}

func (e entry) disable(client *api.Client) {
	if err := client.Sys().Unmount(e.Path); err != nil {
		log.WithField("package", "secrets-engine").WithError(err).WithField("path", e.Path).WithField("type", e.Type).Fatal("failed to disable secrets-engine")
	}
	log.WithField("package", "secrets-engine").WithField("path", e.Path).WithField("type", e.Type).Info("successfully disabled secrets-engine")
}

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("vault_secret_engines", config{})
}

// Apply ensures that an instance of Vault's secrets engine are configured
// exactly as provided.
//
// This function exits the program if an error occurs.
func (c config) Apply(entriesBytes []byte, dryRun bool) {
	// Unmarshal the list of configured secrets engines.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithField("package", "secrets-engine").WithError(err).Fatal("failed to decode secrets engines configuration")
	}

	// List the existing secrets engines.
	existingMounts, err := vault.Client().Sys().ListMounts()
	if err != nil {
		log.WithField("package", "secrets-engine").WithError(err).Fatal("failed to list Mounts from Vault instance")
	}

	// Build a list of all the existing entries.
	existingSecretsEngines := make([]entry, 0)
	if existingSecretsEngines != nil {
		for path, engine := range existingMounts {
			existingSecretsEngines = append(existingSecretsEngines, entry{
				Path:        path,
				Type:        engine.Type,
				Description: engine.Description,
				Options:     engine.Options,
			})
		}
	}

	toBeWritten, toBeDeleted := vault.DiffItems(asItems(entries), asItems(existingSecretsEngines))

	if dryRun == true {
		for _, w := range toBeWritten {
			log.WithField("package", "secrets-engine").WithField("path", w.Key()).WithField("type", w.(entry).Type).Info("[Dry Run] secrets-engine to be enabled")
		}
		for _, d := range toBeDeleted {
			if !isDefaultMount(d.Key()) {
				log.WithField("package", "secrets-engine").WithField("path", d.Key()).WithField("type", d.(entry).Type).Infof("[Dry Run] secrets-engine to be disabled")
			}
		}
	} else {
		// TODO(riuvshin): implement tuning
		for _, e := range toBeWritten {
			e.(entry).enable(vault.Client())
		}

		for _, e := range toBeDeleted {
			ent := e.(entry)
			if !isDefaultMount(ent.Path) {
				ent.disable(vault.Client())
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
