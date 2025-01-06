#!/bin/bash

set -e

source .env

export PODMAN_IGNORE_CGROUPSV1_WARNING=1

cleanup () {
    echo "cleaning"
    podman-compose -f compose.yml down --volumes --remove-orphans --timeout 0
    echo "podman environment cleaned"
}

# cleanup

# podman-compose -f compose.yml up -d

# debug
echo "RUNNING CONTAINERS:"
podman ps --filter "name=vault-manager-test"

# populate necessary vault access vars to primary
podman-compose exec primary-vault kv put secret/master rootToken=root
podman-compose exec primary-vault kv put secret/secondary root=root
podman-compose exec primary-vault kv put secret/oidc client-secret=my-special-client-secret
podman-compose exec primary-vault kv put secret/kubernetes cert=very-valid-cert

# populate oidc client secret in secondary
podman-compose exec secondary-vault kv put secret/oidc client-secret=my-special-client-secret
podman-compose exec secondary-vault kv put secret/kubernetes cert=very-valid-cert

# run test suite
for test in $(find bats/ -type f | grep .bats | grep -v roles | grep -v entities | grep -v groups | grep -v errors); do
    echo "running $test"
    podman-compose exec bats-testing bats --tap "$test"
    # hack so flags.bats has clean slate for audit resources when testing
    if [[ $test == "bats/audit/audit-devices.bats" ]]; then
        # need to execute this for both instances
        podman-compose exec primary-vault audit disable file
        podman-compose exec secondary-vault audit disable file
    fi
done

# roles is dependent on secret engines being enabled due to credential output
echo "running bats/roles/roles.bats"
bats --tap bats/roles/roles.bats

# entities is dependent on config generated by other tests
echo "running bats/entities/entities.bats"
bats --tap bats/entities/entities.bats

# groups is dependent on entities
echo "running bats/groups/groups.bats"
bats --tap bats/groups/groups.bats

# run error handling test now that vaults are fully configured
echo "running bats/errors/errors.bats"
bats --tap bats/errors/errors.bats

cleanup
