package main

import (
	"context"
	"flag"
	"github.com/app-sre/vault-manager/toplevel"
	"github.com/machinebox/graphql"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
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
	var force bool
	flag.BoolVar(&dryRun, "dry-run", false, "If true, will only print planned actions")
	flag.BoolVar(&force, "force", false, "If true, will force potentially unsafe write actions")
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

	// create a graphql client
	client := graphql.NewClient(graphqlServer)

	// make a request
	req := graphql.NewRequest(`
	{
	  vault_audit_backends {
		type
		path_ugly
		description
		options {
          ... on VaultAuditOptionsFile_v1 {
            file_path
    	  }
		}
	  }
      vault_auth_backends {
        path_ugly
        type
        description
        settings {
          config {
            ... on VaultAuthConfigGithub_v1 {
              organization
              base_url
              max_ttl
              ttl
            }
          }
        }
		policy_mappings {
		  github_team {
			team
		  }
		  policies {
			name
		  }
		}
      }
      vault_secret_engines {
        path_ugly
        type
        description
        options {
          ... on VaultSecretEngineOptionsKV_v1 {
            version
          }
        }
      }
	  vault_roles {
		name
		type
		mount
		options {
		  ... on VaultApproleOptions_v1 {
			bind_secret_id
			local_secret_ids
			period
			secret_id_num_uses
			secret_id_ttl
			token_max_ttl
			token_num_uses
			token_ttl
			token_type
			bound_cidr_list
			policies
			  secret_id_bound_cidrs
			  token_bound_cidrs
		  }
		}
	  }
      vault_policies {
		name
		rules
	  }
	}
	`)

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
