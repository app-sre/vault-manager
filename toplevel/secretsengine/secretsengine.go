// Package secretsengine implements the application of a declarative configuration
// for Vault Secrets Engines.
//
// Secrets Engines used to be referred to as "mounts".
package secretsengine

import (
	"strings"

	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
)

type entry struct {
	Path        string            `yaml:"_path"`
	Type        string            `yaml:"type"`
	Instance    vault.Instance    `yaml:"instance"`
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

func (e entry) KeyForDescription() string {
	return e.Description
}

func (e entry) KeyForType() string {
	return e.Type
}

func (e entry) ambiguousOptions() map[string]interface{} {
	opts := make(map[string]interface{}, len(e.Options))
	for k, v := range e.Options {
		opts[k] = v
	}
	return opts
}

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("vault_secret_engines", config{})
}

// TODO(dwelch) refactor into multiple functions
// Apply ensures that an instance of Vault's secrets engine are configured
// exactly as provided.
func (c config) Apply(address string, entriesBytes []byte, dryRun bool, threadPoolSize int) error {
	// Unmarshal the list of configured secrets engines.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Secrets engine] failed to decode secrets engines configuration")
	}
	instancesToDesiredEngines := make(map[string][]entry)
	for _, e := range entries {
		instancesToDesiredEngines[e.Instance.Address] = append(instancesToDesiredEngines[e.Instance.Address], e)
	}

	enabledSecretEngines, err := vault.ListSecretsEngines(address)
	if err != nil {
		return err
	}

	existingSecretEngines := []entry{}
	for path, engine := range enabledSecretEngines {
		existingSecretEngines = append(existingSecretEngines, entry{
			Path:        path,
			Type:        engine.Type,
			Description: engine.Description,
			Options:     engine.Options,
		})
	}
	toBeWritten, toBeDeleted, toBeUpdated :=
		vault.DiffItems(asItems(instancesToDesiredEngines[address]), asItems(existingSecretEngines))

	if dryRun == true {
		for _, w := range toBeWritten {
			log.WithFields(log.Fields{
				"path":     w.Key(),
				"type":     w.(entry).Type,
				"instance": address,
			}).Info("[Dry Run] [Vault Secrets engine] secrets-engine to be enabled")
		}
		for _, u := range toBeUpdated {
			log.WithFields(log.Fields{
				"path":     u.Key(),
				"type":     u.(entry).Type,
				"instance": address,
			}).Info("[Dry Run] [Vault Secrets engine] secrets-engine to be updated")
		}
		for _, d := range toBeDeleted {
			if !isDefaultMount(d.Key()) {
				log.WithFields(log.Fields{
					"path":     d.Key(),
					"type":     d.(entry).Type,
					"instance": address,
				}).Info("[Dry Run] [Vault Secrets engine] secrets-engine to be disabled")
			}
		}
	} else {
		// TODO(riuvshin): implement tuning
		for _, e := range toBeWritten {
			ent := e.(entry)
			err := vault.EnableSecretsEngine(address, ent.Path, &api.MountInput{
				Type:        ent.Type,
				Description: ent.Description,
				Options:     ent.Options,
			})
			if err != nil {
				return err
			}
		}

		for _, e := range toBeUpdated {
			ent := e.(entry)
			err := vault.UpdateSecretsEngine(address, ent.Path, api.MountConfigInput{
				// vault.UpdateSecretsEngine(ent.Path, &api.MountInput{
				Description: &ent.Description,
			})
			if err != nil {
				return err
			}
		}

		for _, e := range toBeDeleted {
			ent := e.(entry)
			if !isDefaultMount(ent.Path) {
				err := vault.DisableSecretsEngine(address, ent.Path)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
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
