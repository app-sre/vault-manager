package identity

import (
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	log "github.com/sirupsen/logrus"

	"gopkg.in/yaml.v2"
)

type config struct{}

var _ toplevel.Configuration = config{}

type user struct {
	Name  string `yaml:"name"`
	Roles []role `yaml:"roles"`
}

type role struct {
	Name        string           `yaml:"name"`
	Permissions []oidcPermission `yaml:"oidc_permissions"`
}

type oidcPermission struct {
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Service     string        `yaml:"service"`
	Policies    []vaultPolicy `yaml:"vault_policies"`
}

type vaultPolicy struct {
	Name string `yaml:"name"`
}

type group struct {
	Name      string
	Type      string
	Metadata  map[string]interface{}
	Policies  []string
	EntityIds []string
}

var _ vault.Item = group{}

func (g group) Key() string {
	return g.Name
}

func (g group) KeyForType() string {
	return g.Type
}

func (g group) KeyForDescription() string {
	return fmt.Sprintf("%v", g.Metadata)
}

func (g group) Equals(i interface{}) bool {
	group, ok := i.(group)
	if !ok {
		return false
	}
	return g.Name == group.Name &&
		reflect.DeepEqual(g.Metadata, group.Metadata) &&
		reflect.DeepEqual(g.Policies, group.Policies) &&
		reflect.DeepEqual(g.EntityIds, group.EntityIds)
}

func init() {
	toplevel.RegisterConfiguration("vault_groups", config{})
}

func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {
	var entries []user
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Group] failed to decode entity configuration")
	}
	entityNamesToIds, err := getEntityNamesToIds()
	if err != nil {
		log.WithError(err).Fatal("[Vault Group] failed to parse existing entities")
	}
	fmt.Println(entityNamesToIds)
}

// processes result of ListEntites to build a map of entity names to Ids
// this map is used to determine what groups should contain which entities
func getEntityNamesToIds() (map[string]string, error) {
	var entityNamesToIds map[string]string
	raw := vault.ListEntities()
	if raw == nil {
		return nil, errors.New("No results retrieved by vault.ListEntites()")
	}
	if _, exists := raw["key_info"]; !exists {
		return nil, errors.New(
			"Required `key_info` attribute not found in response from vault.ListEntites()")
	}
	existingEntities, ok := raw["key_info"].(map[string]interface{})
	if !ok {
		return nil, errors.New(fmt.Sprintf(
			"Failed to convert `key_info` to map[string]interface{}"))
	}
	for id, v := range existingEntities {
		values, ok := v.(map[string]interface{})
		if !ok {
			return nil, errors.New(fmt.Sprintf(
				"Failed to convert value to map[string]interface{} for entity id: %s", id))
		}
		if _, exists := values["name"]; !exists {
			return nil, errors.New(fmt.Sprintf(
				"Required `name` attribute not found for entity id: %s", id))
		}
		if entityNamesToIds == nil {
			entityNamesToIds = make(map[string]string)
		}
		name := values["name"].(string)
		entityNamesToIds[name] = id
	}
	return entityNamesToIds, nil
}

// Sorts slices of strings within each group object
// Necessary for reflect.DeepEqual to be consistent in group.Equals()
func sortSlices(groups []group) {
	for _, group := range groups {
		sort.Strings(group.Policies)
		sort.Strings(group.EntityIds)
	}
}
