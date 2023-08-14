# vault-manager
vault-manager is an automation tool for managing [hashicorp vault](https://github.com/hashicorp/vault) configurations based on [Vault GO API Client](https://github.com/hashicorp/vault/tree/master/api)

# how to use
```bash
docker run --rm -t \
           -v <PATH_TO_FILE_WITH_GRAPHQL_QUERY>:/query.graphql \
           -e GRAPHQL_QUERY_FILE=/query.graphql \
           -e GRAPHQL_SERVER=<GRAPHQL_SERVER_URL> \
           -e GRAPHQL_USERNAME=<GRAPHQL_USERNAME> \
           -e GRAPHQL_PASSWORD=<GRAPHQL_PASSWORD> \
           -e VAULT_ADDR=<VAULT_INSTANCE_URL> \
           -e VAULT_AUTHTYPE=approle \
           -e VAULT_ROLE_ID=<APPROLE_ROLE_ID> \
           -e VAULT_SECRET_ID=<APPROLE_SECRET_ID> \
           quay.io/app-sre/vault-manager:latest -dry-run
```
Note that running vault-manager with -dry-run flag will only print planned actions,
remove this flag to make changes enter into effect

## Flags
- `-dry-run`, default=false<br>
runs vault-manager in dry-run mode and only print planned actions
- `-thread-pool-size`, default=10<br>
Some operations are running in parallel to achieve the best performance,
so `-thread-pool-size` determine how many threads can be utilized

# Changing data.json used for testing
`data.json` within `tests/app-interface` is utilized by the qontract-server created for testing. If schema / query changes are made, this data bundle must be re-generated and committed with the PR. To re-generate: update `SCHEMAS_IMAGE_TAG` within `.env` (make sure to commit this change as well) and execute `make data` within `/tests/app-interface`

# Local Development

For local development, the script `/local-dev.sh` can be ran to configure necessary resources to mirror testing performed within PR check builds.  

Once the script completes, the following containers will be running:
* keycloak 
    * necessary for oidc testing
    * view `/tests/keycloak` for configuration files applied to the instance
* qontract-server 
    * view `/tests/app-interface/data/services/vault/config` for all resources being reconciled by tests
* primary vault instance
    * running on `localhost:8200`
* secondary vault instance
    * running on `localhost:8202`

From root of repo, run `source dev-env`

You can now execute run vault-manager against the local vault instances. Note that after a non `-dry-run`, the resources will be added to the vault instances. To reset, simply rerun `local-dev.sh`

Note: `--net=host` isn't supported for Mac([doc](https://docs.docker.com/network/drivers/host/)). So if you are developing from Mac, remove the flag from local-dev.sh and also remove key-cloak related `docker run` command.

### Example launch.json for VS Code:
```
{
    "version": "0.2.0",
    "configurations": [
      {
        "name": "Launch Package",
        "type": "go",
        "request": "launch",
        "mode": "auto",
        "program": "${workspaceFolder}/cmd/vault-manager/main.go",
        "args": ["--dry-run"],
        "env": {
          "VAULT_ADDR": "http://127.0.0.1:8200",
          "VAULT_TOKEN": "root",
          "VAULT_AUTHTYPE": "token",
          "GRAPHQL_SERVER": "http://localhost:4000/graphql",
          "GRAPHQL_QUERY_FILE": "/Users/olivia/SourceCode/app-sre/vault-manager/query.graphql"
        }
      }
    ]
  }
```

### Testing:

This project use BATS for integration test, using mentioned primary and secondary vault instance. You can debug them by point environment variable `GRAPHQL_QUERY_FILE` to the .graphql under /fixtures.


## Gotchas

### Approle output_path
You will notice that the first `-dry-run` execution after spinning up environment will fail stating a `specified output path does not match existing KV engines`. This is due to how the tests within `/tests/run-tests.sh` are executed.  
To resolve you can either:  

a) manually create the `app-interface` secret engine for both vault instances

b) remove `output_path` from the following files:  
* `/tests/app-interface/data/services/vault/config/roles/master/approles/vault-manager.yml`  
* `/tests/app-interface/data/services/vault/config/roles/secondary/approles/app-interface.yml`  
* update data.json following directions above **do not commit data.json with these attributes missing**

### Vault audit device 
Depending on local container runtime, permission issues when attempting to reconcile the vault audit devices may be encountered. If your development is not affecting logic within `/toplevel/audit.go`, you can remove the files within `/tests/app-interface/data/services/vault/config/audit-backends` and re-generate the data.json. **do not commit data.json with these attributes missing**
