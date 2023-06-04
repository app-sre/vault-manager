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

const toplevelName = "vault_policies"

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration(toplevelName, config{})
}

type entry struct {
	Name        string         `yaml:"name"`
	Rules       string         `yaml:"rules"`
	Type        string         `yaml:"type"`
	Instance    vault.Instance `yaml:"instance"`
	Description string         `yaml:"description"`
}

var _ vault.Item = entry{}

func (e entry) Key() string {
	return e.Name
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

	return e.Name == entry.Name && e.Rules == entry.Rules
}

// TODO(dwelch): refactor into multiple functions
func (c config) Apply(address string, entriesBytes []byte, dryRun bool, threadPoolSize int) error {
	// Unmarshal the list of configured secrets engines.
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Policy] failed to decode policies configuration")
	}
	instancesToDesiredPolicies := make(map[string][]entry)
	for _, e := range entries {
		instancesToDesiredPolicies[e.Instance.Address] = append(instancesToDesiredPolicies[e.Instance.Address], e)
	}

	existingPolicyNames, err := vault.ListVaultPolicies(address)
	if err != nil {
		return err
	}

	// Build a list of all the existing policies for each instance
	existingPolicies := []entry{}
	var mutex = &sync.Mutex{}
	bwg := utils.NewBoundedWaitGroup(threadPoolSize)
	ch := make(chan error)

	// fill existing policies array in parallel
	for i := range existingPolicyNames {
		bwg.Add(1)

		go func(i int, ch chan<- error) {
			defer bwg.Done()

			name := existingPolicyNames[i]
			policy, err := vault.GetVaultPolicy(address, name)
			if err != nil {
				ch <- err
				return
			}

			mutex.Lock()
			defer mutex.Unlock()
			existingPolicies = append(existingPolicies, entry{Name: name, Rules: policy})
		}(i, ch)
	}

	go func() {
		bwg.Wait()
		close(ch)
	}()

	for e := range ch {
		if e != nil {
			return e
		}
	}

	desired := instancesToDesiredPolicies[address]
	desiredItems := asItems((desired))

	validateUniquenessError := vault.ValidateUniqueness(desiredItems, toplevelName)
	if validateUniquenessError != nil {
		log.Fatalln(validateUniquenessError)
	}

	// Diff the local configuration with the Vault instance.
	toBeWritten, toBeDeleted, _ :=
		vault.DiffItems(desiredItems, asItems(existingPolicies))

	if dryRun == true {
		for _, w := range toBeWritten {
			log.WithField("instance", address).Infof("[Dry Run] [Vault Policy] policy to be written='%v'", w.Key())
		}
		for _, d := range toBeDeleted {
			if isDefaultPolicy(d.Key()) {
				continue
			}
			log.WithField("instance", address).Infof("[Dry Run] [Vault Policy] policy to be deleted='%v'", d.Key())
		}
		toplevel.UpdatePolicies(toBeWritten, toBeDeleted)
	} else {
		// Write any missing policies to the Vault instance.
		for _, e := range toBeWritten {
			ent := e.(entry)
			err := vault.PutVaultPolicy(address, ent.Name, ent.Rules)
			if err != nil {
				return err
			}
		}
		// Delete any policies from the Vault instance.
		for _, e := range toBeDeleted {
			ent := e.(entry)
			if isDefaultPolicy(ent.Name) {
				continue
			}
			err := vault.DeleteVaultPolicy(address, ent.Name)
			if err != nil {
				return err
			}
		}
	}

	return nil
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
