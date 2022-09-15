package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/app-sre/vault-manager/pkg/vault"
	"github.com/app-sre/vault-manager/toplevel"
	"github.com/machinebox/graphql"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	// Register top-level configurations.
	_ "github.com/app-sre/vault-manager/toplevel/audit"
	_ "github.com/app-sre/vault-manager/toplevel/auth"
	_ "github.com/app-sre/vault-manager/toplevel/entity"
	_ "github.com/app-sre/vault-manager/toplevel/group"
	_ "github.com/app-sre/vault-manager/toplevel/policy"
	_ "github.com/app-sre/vault-manager/toplevel/role"
	_ "github.com/app-sre/vault-manager/toplevel/secretsengine"
)

type TopLevelConfig struct {
	Name     string
	Priority int
}

type ByPriority []TopLevelConfig

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp: true,
		DisableColors:    true,
	})
	log.SetOutput(os.Stdout)
}
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
	var runOnce bool
	var threadPoolSize int
	flag.BoolVar(&dryRun, "dry-run", false, "If true, will only print planned actions")
	flag.IntVar(&threadPoolSize, "thread-pool-size", 10, "Some operations are running in parallel"+
		" to achieve the best performance, so -thread-pool-size determine how many threads can be utilized, default is 10")
	flag.BoolVar(&runOnce, "run-once", true, "If true, program will skip loop and exit after first reconcile attempt")
	flag.Parse()

	var sleepDuration time.Duration
	if !runOnce {
		sleep, _ := os.LookupEnv("RECONCILE_SLEEP_TIME")
		if sleep == "" {
			log.Fatalln("`RECONCILE_SLEEP_TIME` must be set when `run-once` flag is false")
		}
		sleepDur, err := time.ParseDuration(sleep)
		if err != nil {
			log.Fatalln(err)
		}
		sleepDuration = sleepDur

		port, _ := os.LookupEnv("METRICS_SERVER_PORT")
		if port == "" {
			log.Fatalln("`METRICS_SERVER_PORT` must be set when `run-once` flag is false")
		}
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	}

	for {
		cfg, err := getConfig()
		if err != nil {
			log.WithError(err).Fatal("failed to parse config")
		}

		// initialize vault clients and gather list of instance addresses for reconciliation
		instanceAddresses := initInstances(cfg, threadPoolSize)

		// remove disabled toplevels
		if disabled, _ := os.LookupEnv("DISABLE_IDENTITY"); disabled == "true" {
			delete(cfg, "vault_entities")
			delete(cfg, "vault_groups")
		}

		topLevelConfigs := []TopLevelConfig{}

		for key := range cfg {
			c := TopLevelConfig{key, resolveConfigPriority(key)}
			topLevelConfigs = append(topLevelConfigs, c)
		}

		// sort configs by priority
		sort.Sort(ByPriority(topLevelConfigs))

		// perform reconcile process per instance
		for _, address := range instanceAddresses {
			for _, config := range topLevelConfigs {
				// Marshal the contents of this object back into bytes so that it can be
				// unmarshaled into a specific type in the application.
				dataBytes, err := yaml.Marshal(cfg[config.Name])
				if err != nil {
					log.WithField("name", config.Name).Fatal("failed to remarshal configuration")
				}
				err = toplevel.Apply(config.Name, address, dataBytes, dryRun, threadPoolSize)
				if err != nil {
					fmt.Println(fmt.Sprintf("SKIPPING REMAINING RECONCILIATION FOR %s", address))
					break
				}
			}
		}

		if runOnce {
			return
		} else {
			time.Sleep(sleepDuration)
		}
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
		graphqlQueryFile = "/query.graphql"
	}

	graphqlUsername := os.Getenv("GRAPHQL_USERNAME")

	graphqlPassword := os.Getenv("GRAPHQL_PASSWORD")

	// create a graphql client
	client := graphql.NewClient(graphqlServer)

	// read graphql query from file
	query, err := ioutil.ReadFile(graphqlQueryFile)
	if err != nil {
		log.WithField("path", graphqlQueryFile).Fatal("failed to read graphql query file")
	}

	// make a request
	req := graphql.NewRequest(string(query))

	// set basic auth header
	if graphqlUsername != "" && graphqlPassword != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(graphqlUsername+":"+graphqlPassword)))
	}

	// define a Context for the request
	ctx := context.Background()

	var response map[string]interface{}

	// execute query and capture the response
	if err := client.Run(ctx, req, &response); err != nil {
		return config{}, errors.Wrap(err, "failed to query graphql server")
	}

	return response, nil
}

// gathers instances referenced across all applicable file definitions and initializes the clients
// clients are set as private global witihn client.go
// return is list of strings containing addresses of vault instances
func initInstances(cfg config, threadPoolSize int) []string {
	const INSTANCE_KEY = "vault_instances"
	dataBytes, err := yaml.Marshal(cfg[INSTANCE_KEY])
	if err != nil {
		log.WithField("name", INSTANCE_KEY).Fatal("failed to remarshal instance configuration")
	}
	// do not include `vault_instances` in standard top-level reconcile loop
	delete(cfg, INSTANCE_KEY)
	return vault.GetInstances(dataBytes, threadPoolSize)
}

func resolveConfigPriority(s string) int {
	var priority int
	switch s {
	case "vault_instances":
		priority = 1
	case "vault_policies":
		priority = 2
	case "vault_audit_backends":
		priority = 3
	case "vault_secret_engines":
		priority = 4
	case "vault_auth_backends":
		priority = 5
	case "vault_roles":
		priority = 6
	case "vault_entities":
		priority = 7
	case "vault_groups":
		priority = 8
	default:
		priority = 0
	}
	return priority
}
