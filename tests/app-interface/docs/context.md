# App-Interface

This minimal version of App-Interface only contains necessary data definitions to support testing of the vault-manager bats test-suite (`/tests/bats`).  

### Utilization
An instance of qontract-server using the minimal data defined in `tests/app-interface/data` is deployed for each pr-check build. This instance is then queried by the various bats tests when evaluating vault-manager logic.

### Generate data.json
`data.json` must be re-generated and committed when alterations are made within `tests/app-interface/data`.  
To generate: execute `make data` within `tests/app-interface`
