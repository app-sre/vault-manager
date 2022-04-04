package group

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/app-sre/vault-manager/pkg/utils"
	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	log "github.com/sirupsen/logrus"

	"gopkg.in/yaml.v2"
)

type config struct{}

var _ toplevel.Configuration = config{}

type user struct {
	Name  string `yaml:"org_username"`
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
	Id        string
	Type      string
	Metadata  map[string]interface{}
	Policies  []string
	EntityIds []string
}

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

func (g group) CreateOrUpdate(action string) {
	path := filepath.Join("identity", g.Type, "name", g.Name)
	config := map[string]interface{}{
		"member_entity_ids": g.EntityIds,
		"policies":          g.Policies,
		"metadata":          g.Metadata,
	}
	vault.WriteSecret(path, config)
	log.WithField("path", path).WithField("type", g.Type).Info(
		fmt.Sprintf("[Vault Identity] group successfully %s", action))
}

func (g group) Delete() {
	path := filepath.Join("identity", g.Type, "name", g.Name)
	vault.DeleteSecret(path)
	log.WithField("path", path).WithField("type", g.Type).Info(
		"[Vault Identity] group successfully deleted")
}

var _ vault.Item = group{}

func init() {
	toplevel.RegisterConfiguration("vault_groups", config{})
}

func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {
	var entries []user
	if err := yaml.Unmarshal(entriesBytes, &entries); err != nil {
		log.WithError(err).Fatal("[Vault Identity] failed to decode entity configuration")
	}
	entityNamesToIds, err := getEntityNamesToIds()
	if err != nil {
		log.WithError(err).Fatal("[Vault Identity] failed to parse existing entities")
	}

	desired := processDesired(entries, entityNamesToIds)
	existing, err := getExistingGroups(threadPoolSize)
	sortSlices(desired)
	sortSlices(existing)

	toBeWritten, toBeDeleted, toBeUpdated := vault.DiffItems(groupsAsItems(desired), groupsAsItems(existing))
	if dryRun {
		dryRunOutput(toBeWritten, "written")
		dryRunOutput(toBeDeleted, "deleted")
		dryRunOutput(toBeUpdated, "updated")
	} else {
		for _, w := range toBeWritten {
			w.(group).CreateOrUpdate("written")
		}
		for _, d := range toBeDeleted {
			d.(group).Delete()
		}
		for _, u := range toBeUpdated {
			u.(group).CreateOrUpdate("updated")
		}
	}
}

// processDesired accepts the yaml-marshalled result of the `vault_groups` graphql
// query and returns group objects
func processDesired(entries []user, entityNamesToIds map[string]string) []group {
	desired := []group{}
	processedGroups := make(map[string]*group)
	for _, entry := range entries {
		if entry.Roles != nil {
			for _, role := range entry.Roles {
				if role.Permissions != nil {
					for _, permission := range role.Permissions {
						if permission.Service == "vault" {
							handleNewDesired(processedGroups, permission, role.Name, entityNamesToIds[entry.Name])
						}
					}
				}
			}
		}
	}
	for _, v := range processedGroups {
		desired = append(desired, *v)
	}
	return desired
}

// either creates or updates a desired group
// helper function for processDesired
func handleNewDesired(processedGroups map[string]*group, permission oidcPermission, roleName string, entityId string) {
	policies := []string{}
	for _, policy := range permission.Policies {
		policies = append(policies, policy.Name)
	}
	if _, exists := processedGroups[roleName]; !exists {
		processedGroups[roleName] = &group{
			Name:      roleName,
			Type:      "group",
			EntityIds: []string{entityId}, // note that this could potentially be empty
			Policies:  policies,
			Metadata: map[string]interface{}{
				permission.Name: permission.Description,
			},
		}
	} else {
		processedGroups[roleName].EntityIds = append(processedGroups[roleName].EntityIds, entityId)
		processedGroups[roleName].Metadata[permission.Name] = permission.Description
		// avoid adding duplicate policies that already exist on another permission associated w/ role
		existingPolicies := make(map[string]bool)
		for _, policy := range processedGroups[roleName].Policies {
			existingPolicies[policy] = true
		}
		for _, policy := range policies {
			if _, exists := existingPolicies[policy]; !exists {
				processedGroups[roleName].Policies = append(processedGroups[roleName].Policies, policy)
			}
		}
	}
}

// returns list of existing vault groups
func getExistingGroups(threadPoolSize int) ([]group, error) {
	raw := vault.ListGroups()
	if raw == nil {
		return nil, nil
	}
	processed := []group{}
	if _, exists := raw["key_info"]; !exists {
		return nil, errors.New(
			"Required `key_info` attribute not found in response from vault.ListGroups()")
	}
	existingGroups, ok := raw["key_info"].(map[string]interface{})
	if !ok {
		return nil, errors.New(fmt.Sprintf(
			"Failed to convert `key_info` to map[string]interface{}"))
	}
	for id, v := range existingGroups {
		values, ok := v.(map[string]interface{})
		if !ok {
			return nil, errors.New(fmt.Sprintf(
				"Failed to convert value to map[string]interface{} for entity id: %s", id))
		}
		if _, exists := values["name"]; !exists {
			return nil, errors.New(fmt.Sprintf(
				"Required `name` attribute not found for entity id: %s", id))
		}
		name := values["name"].(string)

		processed = append(processed, group{
			Name: name,
			Id:   id,
			Type: "group",
		})
	}
	// make separate call for each group to retrieve necessary details
	bwg := utils.NewBoundedWaitGroup(threadPoolSize)
	for i := range processed {
		bwg.Add(1)
		getGroupDetails(&processed[i], &bwg)
	}
	return processed, nil
}

// goroutine function
func getGroupDetails(g *group, bwg *utils.BoundedWaitGroup) {
	defer bwg.Done()
	info := vault.GetGroupInfo(g.Name)
	if info == nil {
		log.WithError(errors.New(fmt.Sprintf(
			"No information returned for group: %s", g.Name))).Fatal()
	}
	if _, exists := info["member_entity_ids"]; !exists {
		log.WithError(errors.New(fmt.Sprintf(
			"Required `member_entity_ids` attribute not found for group: %s", g.Name))).Fatal()
	}
	for _, id := range info["member_entity_ids"].([]interface{}) {
		g.EntityIds = append(g.EntityIds, id.(string))
	}
	if _, exists := info["policies"]; !exists {
		log.WithError(errors.New(fmt.Sprintf(
			"Required `policies` attribute not found for group: %s", g.Name))).Fatal()
	}
	for _, policy := range info["policies"].([]interface{}) {
		g.Policies = append(g.Policies, policy.(string))
	}
	if _, exists := info["metadata"]; !exists {
		log.WithError(errors.New(fmt.Sprintf(
			"Required `metadata` attribute not found for group: %s", g.Name))).Fatal()
	}
	if info["metadata"] != nil {
		g.Metadata = info["metadata"].(map[string]interface{})
	}
}

// processes result of ListEntites to build a map of entity names to Ids
// this map is used to determine what groups should contain which entities
func getEntityNamesToIds() (map[string]string, error) {
	var entityNamesToIds map[string]string
	raw := vault.ListEntities()
	if raw == nil {
		return make(map[string]string), nil
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

func groupsAsItems(groups []group) []vault.Item {
	items := []vault.Item{}
	for _, group := range groups {
		items = append(items, group)
	}
	return items
}

// reusable func to output updates on writes, deletes, and updates for groups
func dryRunOutput(groups []vault.Item, action string) {
	for _, g := range groups {
		log.WithField("name", g.Key()).WithField("type", g.(group).Type).Info(
			fmt.Sprintf("[Dry Run] [Vault Identity] group to be %s", action))
	}
}
