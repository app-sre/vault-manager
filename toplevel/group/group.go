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
	"github.com/app-sre/vault-manager/toplevel/instance"
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
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Service     string            `yaml:"service"`
	Instance    instance.Instance `yaml:"instance"`
	Policies    []vaultPolicy     `yaml:"vault_policies"`
}

type vaultPolicy struct {
	Name string `yaml:"name"`
}

type group struct {
	Name      string
	Id        string
	Type      string
	Instance  instance.Instance
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

func (g group) CreateOrUpdate(action string) error {
	path := filepath.Join("identity", g.Type, "name", g.Name)
	config := map[string]interface{}{
		"member_entity_ids": g.EntityIds,
		"policies":          g.Policies,
		"metadata":          g.Metadata,
	}
	err := vault.WriteSecret(g.Instance.Address, path, config)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{
		"path":     path,
		"type":     g.Type,
		"instance": g.Instance.Address,
	}).Infof("[Vault Identity] group successfully %s", action)
	return nil
}

func (g group) Delete() error {
	path := filepath.Join("identity", g.Type, "name", g.Name)
	err := vault.DeleteSecret(g.Instance.Address, path)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{
		"path":     path,
		"type":     g.Type,
		"instance": g.Instance.Address,
	}).Info("[Vault Identity] group successfully deleted")
	return nil
}

var _ vault.Item = group{}

func init() {
	toplevel.RegisterConfiguration("vault_groups", config{})
}

func (c config) Apply(entriesBytes []byte, dryRun bool, threadPoolSize int) {
	var users []user
	if err := yaml.Unmarshal(entriesBytes, &users); err != nil {
		log.WithError(err).Fatal("[Vault Identity] failed to decode entity configuration")
	}
	// perform processing/reconcile per instance
OUTER:
	for instanceAddr := range vault.InstanceAddresses {
		entityNamesToIds, err := getEntityNamesToIds(instanceAddr)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"instance": instanceAddr,
			}).Info("[Vault Identity] failed to parse existing entities as prereq for group reconcile")
			vault.AddInvalid(instanceAddr)
			continue
		}
		desired := processDesired(instanceAddr, users, entityNamesToIds)
		existing, err := getExistingGroups(instanceAddr, threadPoolSize)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"instance": instanceAddr,
			}).Info("[Vault Identity] failed to retrieve existing groups")
			vault.AddInvalid(instanceAddr)
			continue
		}
		sortSlices(desired)
		sortSlices(existing)

		toBeWritten, toBeDeleted, toBeUpdated := vault.DiffItems(groupsAsItems(desired), groupsAsItems(existing))
		if dryRun {
			dryRunOutput(instanceAddr, toBeWritten, "written")
			dryRunOutput(instanceAddr, toBeDeleted, "deleted")
			dryRunOutput(instanceAddr, toBeUpdated, "updated")
		} else {
			for _, w := range toBeWritten {
				err := w.(group).CreateOrUpdate("written")
				if err != nil {
					vault.AddInvalid(instanceAddr)
					continue OUTER // terminate remaining reconcile for instance that returned an error
				}
			}
			for _, d := range toBeDeleted {
				err := d.(group).Delete()
				if err != nil {
					vault.AddInvalid(instanceAddr)
					continue OUTER // terminate remaining reconcile for instance that returned an error
				}
			}
			for _, u := range toBeUpdated {
				err := u.(group).CreateOrUpdate("updated")
				if err != nil {
					vault.AddInvalid(instanceAddr)
					continue OUTER // terminate remaining reconcile for instance that returned an error
				}
			}
		}
	}
	// removes instances that generated errors from remaining reconciliation process
	// this is necessary due to dependencies between toplevels
	vault.RemoveInstanceFromReconciliation()
}

// processDesired accepts the yaml-marshalled result of the `vault_groups` graphql
// query and returns group objects
func processDesired(instanceAddr string, users []user, entityNamesToIds map[string]string) []group {
	desired := []group{}
	processedGroups := make(map[string]*group)
	for _, user := range users {
		for _, role := range user.Roles {
			for _, permission := range role.Permissions {
				if permission.Service == "vault" && permission.Instance.Address == instanceAddr {
					handleNewDesired(processedGroups, permission, role.Name, entityNamesToIds[user.Name])
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
			Instance:  permission.Instance,
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
func getExistingGroups(instanceAddr string, threadPoolSize int) ([]group, error) {
	raw, err := vault.ListGroups(instanceAddr)
	if err != nil {
		return nil, err
	}
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
			Instance: instance.Instance{
				Address: instanceAddr,
			},
		})
	}
	// make separate call for each group to retrieve necessary details
	bwg := utils.NewBoundedWaitGroup(threadPoolSize)

	ch := make(chan error)
	for i := range processed {
		bwg.Add(1)
		go getGroupDetails(&processed[i], ch, &bwg)
	}

	// separate thread to wait and close channel
	go func() {
		bwg.Wait()
		close(ch)
	}()

	// sit and wait for all getGroupDetails goroutines to return
	for err := range ch {
		if err != nil {
			return nil, err
		}
	}

	return processed, nil
}

// goroutine function
// makes request to vault instance and updates a particular group object
func getGroupDetails(g *group, ch chan<- error, wg *utils.BoundedWaitGroup) {
	defer wg.Done()
	info, err := vault.GetGroupInfo(g.Instance.Address, g.Name)
	if err != nil {
		ch <- err
		return
	}
	if info == nil {
		ch <- errors.New(fmt.Sprintf(
			"No information returned for group: %s", g.Name))
		return
	}
	if _, exists := info["member_entity_ids"]; !exists {
		ch <- errors.New(fmt.Sprintf(
			"Required `member_entity_ids` attribute not found for group: %s", g.Name))
		return
	}
	for _, id := range info["member_entity_ids"].([]interface{}) {
		g.EntityIds = append(g.EntityIds, id.(string))
	}
	if _, exists := info["policies"]; !exists {
		ch <- errors.New(fmt.Sprintf(
			"Required `policies` attribute not found for group: %s", g.Name))
		return
	}
	for _, policy := range info["policies"].([]interface{}) {
		g.Policies = append(g.Policies, policy.(string))
	}
	if _, exists := info["metadata"]; !exists {
		ch <- errors.New(fmt.Sprintf(
			"Required `metadata` attribute not found for group: %s", g.Name))
		return
	}
	if info["metadata"] != nil {
		g.Metadata = info["metadata"].(map[string]interface{})
	}
}

// processes result of ListEntites to build a map of entity names to Ids
// this map is used to determine what groups should contain which entities
func getEntityNamesToIds(instanceAddr string) (map[string]string, error) {
	var entityNamesToIds map[string]string
	raw, err := vault.ListEntities(instanceAddr)
	if err != nil {
		return nil, err
	}
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
func dryRunOutput(instanceAddr string, groups []vault.Item, action string) {
	for _, g := range groups {
		log.WithFields(log.Fields{
			"name":     g.Key(),
			"type":     g.KeyForType(),
			"instance": instanceAddr,
		}).Infof("[Dry Run] [Vault Identity] group to be %s", action)
	}
}
