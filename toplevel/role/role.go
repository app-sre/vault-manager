// Package role implements the application of a declarative configuration
// for Vault App Roles.
package role

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	"github.com/app-sre/vault-manager/toplevel/instance"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type entry struct {
	Name        string                 `yaml:"name"`
	Type        string                 `yaml:"type"`
	Mount       string                 `yaml:"mount"`
	Instance    instance.Instance      `yaml:"instance"`
	Options     map[string]interface{} `yaml:"options"`
	Description string                 `yaml:"description"`
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

	return e.Name == entry.Name &&
		e.Type == entry.Type &&
		e.Mount == entry.Mount &&
		vault.OptionsEqual(e.Options, entry.Options)
}

func (e entry) Save() {
	path := filepath.Join("auth", e.Mount, "role", e.Name)
	options := make(map[string]interface{})
	for k, v := range e.Options {
		// local_secret_ids can not be changed after creation so we skip this option
		if k == "local_secret_ids" {
			continue
			// initially unmarshalled as string or nil and require further processing
		} else {
			options[k] = v
		}
	}
	vault.WriteSecret(e.Instance.Address, path, options)
	log.WithField("path", path).WithField("type", e.Type).Info("[Vault Role] role is successfully written")
}

func (e entry) Delete() {
	path := filepath.Join("auth", e.Mount, "role", e.Name)
	vault.DeleteSecret(e.Instance.Address, path)
	log.WithField("path", path).WithField("type", e.Type).Info("[Vault Role] role is successfully deleted from Vault instance")
}

type config struct{}

var _ toplevel.Configuration = config{}

func init() {
	toplevel.RegisterConfiguration("vault_roles", config{})
}

// Apply ensures that an instance of Vault's roles are configured exactly
// as provided.
//
// This function exits the program if an error occurs.
func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {
	var entries []entry
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Role] failed to decode role configuration")
	}
	instancesToDesiredRoles := make(map[string][]entry)
	for _, e := range entries {
		instancesToDesiredRoles[e.Instance.Address] = append(instancesToDesiredRoles[e.Instance.Address], e)
	}

	// Get the existing auth backends
	instancesToExistingAuths := make(map[string]map[string]*api.MountOutput)
	for _, e := range entries {
		if _, exists := instancesToExistingAuths[e.Instance.Address]; !exists {
			instancesToExistingAuths[e.Instance.Address] = vault.ListAuthBackends(e.Instance.Address)
		}
	}

	// build list of all existing roles for each instance
	instancesToExistingRoles := make(map[string][]entry)
	for instance, existingAuthBackends := range instancesToExistingAuths {
		if existingAuthBackends != nil {
			for authBackend := range existingAuthBackends {
				// Get the secret with the existing App Roles.
				path := filepath.Join("auth", authBackend, "role")
				secret := vault.ListSecrets(instance, path)
				if secret != nil {
					roles := secret.Data["keys"].([]interface{})

					var mutex = &sync.Mutex{}
					bwg := utils.NewBoundedWaitGroup(threadPoolSize)

					// fill existing policies array in parallel
					for i := range roles {
						bwg.Add(1)

						go func(i int) {
							path := filepath.Join("auth", authBackend, "role", roles[i].(string))

							mutex.Lock()

							instancesToExistingRoles[instance] = append(instancesToExistingRoles[instance],
								entry{
									Name:    roles[i].(string),
									Type:    existingAuthBackends[authBackend].Type,
									Mount:   authBackend,
									Options: vault.ReadSecret(instance, path).Data,
								})

							defer bwg.Done()
							defer mutex.Unlock()
						}(i)
					}
					bwg.Wait()
				}
			}
		}
	}

	addOptionalOidcDefaults(instancesToDesiredRoles)
	err := pruneUnsupported(instancesToDesiredRoles)
	if err != nil {
		log.WithError(err).Fatal("[Vault Role] failed to determine vault version")
	}

	err = unmarshallOptionObjects(instancesToDesiredRoles)
	if err != nil {
		log.WithError(err).Fatal("[Vault Role] failed to unmarshal oidc options of desired role")
	}

	// perform reconcile operations for each instance
	for _, instance := range instance.InstanceAddresses {
		// Diff the local configuration with the Vault instance.
		entriesToBeWritten, entriesToBeDeleted, _ :=
			vault.DiffItems(asItems(instancesToDesiredRoles[instance]), asItems(instancesToExistingRoles[instance]))

		if dryRun == true {
			for _, w := range entriesToBeWritten {
				log.WithField("name", w.Key()).WithField("type", w.(entry).Type).Info("[Dry Run] [Vault Role] role to be written")
			}
			for _, d := range entriesToBeDeleted {
				log.WithField("name", d.Key()).WithField("type", d.(entry).Type).Info("[Dry Run] [Vault Role] role to be deleted")
			}
		} else {
			// Write any missing roles to the Vault instance.
			for _, e := range entriesToBeWritten {
				e.(entry).Save()
			}

			// Delete any roles from the Vault instance.
			for _, e := range entriesToBeDeleted {
				e.(entry).Delete()
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

// unmarshals select options attributes which are defined within schema as objects
// limitation within yaml unmarshal causes theses attributes to be initially unmarshalled as strings
func unmarshallOptionObjects(instancesToDesiredRoles map[string][]entry) error {
	for _, roles := range instancesToDesiredRoles {
		for _, role := range roles {
			if strings.ToLower(role.Type) == "oidc" {
				for k := range role.Options {
					if k == "bound_claims" || k == "claim_mappings" {
						converted, err := utils.UnmarshalJsonObj(k, role.Options[k])
						if err != nil {
							return err
						}
						// avoid assignment if result of unmarshal call is nil bc it will
						// set type of option[k] to map[string]interface{}
						// causing failure in reflect.deepequal check even when both are nil
						if converted == nil {
							continue
						}
						role.Options[k] = converted
					}
				}
			}
		}
	}
	return nil
}

// addOptionalOidcDefaults adds optional attributes and corresponding default values to desired oidc roles
// this circumvents defining every attribute within desired oidc roles
func addOptionalOidcDefaults(instancesToDesiredRoles map[string][]entry) {
	defaults := map[string]interface{}{
		"bound_audiences":      []string{},
		"bound_claims":         nil,
		"bound_claims_type":    "string",
		"bound_subject":        "",
		"claim_mappings":       nil,
		"clock_skew_leeway":    0,
		"expiration_leeway":    0,
		"groups_claim":         "",
		"max_age":              0,
		"not_before_leeway":    0,
		"oidc_scopes":          []string{},
		"verbose_oidc_logging": false,
	}
	for _, roles := range instancesToDesiredRoles {
		for _, role := range roles {
			if strings.ToLower(role.Type) == "oidc" {
				for k, v := range defaults {
					// denotes that attr was not included in definition and graphql assigned nil
					// proceed with assigning default value that api would assign if attribute was omitted
					if role.Options[k] == nil {
						role.Options[k] = v
					}
				}
			}
		}
	}
}

// remove attributes not supported in commercial but in fedramp variant
func pruneUnsupported(instancesToDesiredRoles map[string][]entry) error {
	for instance, roles := range instancesToDesiredRoles {
		current, err := version.NewVersion(vault.GetVaultVersion(instance))
		if err != nil {
			return err
		}
		// https://github.com/hashicorp/vault/blob/main/CHANGELOG.md#170
		threshold, err := version.NewVersion("1.7.0")
		if current.LessThan(threshold) {
			for _, role := range roles {
				if strings.ToLower(role.Type) == "oidc" {
					delete(role.Options, "max_age")
				}
			}
		}
	}
	return nil
}
