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
           -e DISABLE_IDENTITY=true \
           quay.io/app-sre/vault-manager:latest -dry-run
```
Note that running vault-manager with -dry-run flag will only print planned actions,
remove this flag to make changes enter into effect

Note that `DISABLE_IDENTITY` is currently required for commercial usage.

## Flags
- `-dry-run`, default=false<br>
runs vault-manager in dry-run mode and only print planned actions
- `-thread-pool-size`, default=10<br>
Some operations are running in parallel to achieve the best performance,
so `-thread-pool-size` determine how many threads can be utilized
