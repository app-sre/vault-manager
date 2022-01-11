// Package policy implements the application of a declarative configuration
// for Vault policies.
package policy

import (
	"sync"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("vault_policies", config{})
}

type entry struct {
	Name        string `yaml:"name"`
	Rules       string `yaml:"rules"`
	Description string `yaml:"description"`
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

	return e.Name == entry.Name && e.Rules == entry.Rules
}

func (e entry) KeyForDescription() string {
	return e.Description
}

func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {
	// Unmarshal the list of configured secrets engines.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Policy] failed to decode policies configuration")
	}

	// List the existing policies.
	existingPolicyNames := vault.ListVaultPolicies()

	// Build a list of all the existing entries.
	existingPolicies := make([]entry, 0)
	if existingPolicyNames != nil {

		var mutex = &sync.Mutex{}

		bwg := utils.NewBoundedWaitGroup(threadPoolSize)

		// fill existing policies array in parallel
		for i := range existingPolicyNames {

			bwg.Add(1)

			go func(i int) {

				name := existingPolicyNames[i]

				policy := vault.GetVaultPolicy(name)

				mutex.Lock()

				existingPolicies = append(existingPolicies, entry{Name: name, Rules: policy})

				defer bwg.Done()
				defer mutex.Unlock()
			}(i)
		}
		bwg.Wait()
	}

	// Diff the local configuration with the Vault instance.
	toBeWritten, toBeDeleted, _ := vault.DiffItems(asItems(entries), asItems(existingPolicies))

	if dryRun == true {
		for _, w := range toBeWritten {
			log.Infof("[Dry Run] [Vault Policy] policy to be written='%v'", w.Key())
		}
		for _, d := range toBeDeleted {
			if isDefaultPolicy(d.Key()) {
				continue
			}

			log.Infof("[Dry Run] [Vault Policy] policy to be deleted='%v'", d.Key())
		}
	} else {
		// Write any missing policies to the Vault instance.
		for _, e := range toBeWritten {
			ent := e.(entry)
			vault.PutVaultPolicy(ent.Name, ent.Rules)
		}

		// Delete any policies from the Vault instance.
		for _, e := range toBeDeleted {
			ent := e.(entry)
			if isDefaultPolicy(ent.Name) {
				continue
			}
			vault.DeleteVaultPolicy(ent.Name)
		}
	}
}

func isDefaultPolicy(name string) bool {
	return name == "root" || name == "default"
}

func asItems(xs []entry) (items []vault.Item) {
	items = make([]vault.Item, 0)
	for _, x := range xs {
		items = append(items, x)
	}

	return
}
