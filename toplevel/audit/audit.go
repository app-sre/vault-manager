// Package audit implements the application of a declarative configuration
// for Vault Audit Devices.
package audit

import (
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	"github.com/app-sre/vault-manager/toplevel/instance"

	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type entry struct {
	Path        string            `yaml:"_path"`
	Type        string            `yaml:"type"`
	Description string            `yaml:"description"`
	Instance    instance.Instance `yaml:"instance"`
	Options     map[string]string `yaml:"options"`
}

var _ vault.Item = entry{}

func (e entry) Key() string {
	return e.Path
}

func (e entry) KeyForType() string {
	return e.Type
}

func (e entry) KeyForDescription() string {
	return e.Description
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

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("vault_audit_backends", config{})
}

// Apply ensures that an instance of Vault's Audit Devices are configured
// exactly as provided.
//
// This function exits the program if an error occurs.
func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Audit] failed to decode audit device configuration")
	}
	instancesToDesiredAudits := make(map[string][]entry)
	for _, e := range entries {
		instancesToDesiredAudits[e.Instance.Address] = append(instancesToDesiredAudits[e.Instance.Address], e)
	}

	// call to vault api for each instance to obtain raw enabled audit info
	instancesToEnabledAudits := make(map[string]map[string]*api.Audit)
	for _, e := range entries {
		if _, exists := instancesToEnabledAudits[e.Instance.Address]; !exists {
			instancesToEnabledAudits[e.Instance.Address] = vault.ListAuditDevices(e.Instance.Address)
		}
	}

	// Build a list of all the existing audits for each instance
	instancesToExistingAudits := make(map[string][]entry)
	for instance, enabledAudits := range instancesToEnabledAudits {
		if enabledAudits != nil {
			for k := range enabledAudits {
				instancesToExistingAudits[instance] = append(instancesToExistingAudits[instance], entry{
					Path:        enabledAudits[k].Path,
					Type:        enabledAudits[k].Type,
					Description: enabledAudits[k].Description,
					Options:     enabledAudits[k].Options,
				})
			}
		}
	}

	// perform reconcile operations for each instance
	for _, instance := range instance.InstanceAddresses {
		// Diff the local configuration with the Vault instance.
		toBeWritten, toBeDeleted, _ :=
			vault.DiffItems(asItems(instancesToDesiredAudits[instance]), asItems(instancesToExistingAudits[instance]))

		if dryRun == true {
			for _, w := range toBeWritten {
				log.WithField("path", w.Key()).Infof("[Dry Run] [Vault Audit] audit device to be enabled")
			}
			for _, d := range toBeDeleted {
				log.WithField("path", d.Key()).Infof("[Dry Run] [Vault Audit] audit device to be disabled")
			}
		} else {
			// Write any missing Audit Devices to the Vault instance.
			for _, e := range toBeWritten {
				ent := e.(entry)
				vault.EnableAduitDevice(instance, ent.Path, &api.EnableAuditOptions{
					Type:        ent.Type,
					Description: ent.Description,
					Options:     ent.Options,
				})
			}
			// Delete any Audit Devices from the Vault instance.
			for _, e := range toBeDeleted {
				vault.DisableAuditDevice(instance, e.(entry).Path)
			}
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
