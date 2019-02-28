package main

import (
	"context"
	"flag"
	"github.com/app-sre/vault-manager/toplevel"
	"github.com/machinebox/graphql"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"sort"

	// Register top-level configurations.
	_ "github.com/app-sre/vault-manager/toplevel/audit"
	_ "github.com/app-sre/vault-manager/toplevel/auth"
	_ "github.com/app-sre/vault-manager/toplevel/policy"
	_ "github.com/app-sre/vault-manager/toplevel/role"
	_ "github.com/app-sre/vault-manager/toplevel/secretsengine"
)

type TopLevelConfig struct {
	Name     string
	Priority int
}

type ByPriority []TopLevelConfig

func (a ByPriority) Len() int {
	return len(a)
}
func (a ByPriority) Less(i, j int) bool {
	return a[i].Priority < a[j].Priority
}
func (a ByPriority) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func main() {
	var dryRun bool
	flag.BoolVar(&dryRun, "dry-run", false, "If true, will only print planned actions")
	flag.Parse()

	cfg, err := getConfig()
	if err != nil {
		logrus.WithError(err).Fatal("failed to parse config")
	}

	topLevelConfigs := []TopLevelConfig{}

	for key := range cfg {
		c := TopLevelConfig{key, resolveConfigPriority(key)}
		topLevelConfigs = append(topLevelConfigs, c)
	}

	// sort configs by priority
	sort.Sort(ByPriority(topLevelConfigs))

	for _, config := range topLevelConfigs {
		// Marshal the contents of this object back into bytes so that it can be
		// unmarshaled into a specific type in the application.
		dataBytes, err := yaml.Marshal(cfg[config.Name])
		if err != nil {
			logrus.WithField("name", config.Name).Fatal("failed to remarshal configuration")
		}
		toplevel.Apply(config.Name, dataBytes, dryRun)
	}
}

type config map[string]interface{}

func getConfig() (config, error) {
	graphqlServer := os.Getenv("GRAPHQL_SERVER")
	if graphqlServer == "" {
		graphqlServer = "http://localhost:4000/graphql"
	}

	graphqlQueryFile := os.Getenv("GRAPHQL_QUERY_FILE")
	if graphqlQueryFile == "" {
		graphqlQueryFile = "query.graphql"
	}

	// create a graphql client
	client := graphql.NewClient(graphqlServer)

	// read graphql query from file
	query, err := ioutil.ReadFile(graphqlQueryFile)
	if err != nil {
		logrus.WithField("path", graphqlQueryFile).Fatal("failed to read graphql query file")
	}

	// make a request
	req := graphql.NewRequest(string(query))

	// define a Context for the request
	ctx := context.Background()

	var response map[string]interface{}

	// execute query and capture the response
	if err := client.Run(ctx, req, &response); err != nil {
		return config{}, errors.Wrap(err, "failed to query graphql server")
	}

	return response, nil
}

func resolveConfigPriority(s string) int {
	var priority int
	switch s {
	case "vault_policies":
		priority = 1
	case "vault_audit_backends":
		priority = 2
	case "vault_secret_engines":
		priority = 3
	case "vault_auth_backends":
		priority = 4
	case "vault_roles":
		priority = 5
	default:
		priority = 0
	}
	return priority
}
