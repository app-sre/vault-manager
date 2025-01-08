#!/bin/bash

source .env

export PODMAN_IGNORE_CGROUPSV1_WARNING=1

cleanup () {
    echo "cleaning"
    podman-compose -f tests/compose.yml down --volumes --remove-orphans --timeout 0
    echo "podman environment cleaned"
}

# debug
echo "RUNNING CONTAINERS:"
podman ps --filter "name=vault-manager-test"

# populate necessary vault access vars to primary
podman-compose -f tests/compose.yml exec primary-vault vault kv put -mount=secret master rootToken=root
podman-compose -f tests/compose.yml exec primary-vault vault kv put -mount=secret oidc client-secret=my-special-client-secret
podman-compose -f tests/compose.yml exec primary-vault vault kv put -mount=secret kubernetes cert=very-valid-cert

# populate oidc client secret in secondary
podman-compose -f tests/compose.yml exec secondary-vault vault kv put -mount=secret oidc client-secret=my-special-client-secret
podman-compose -f tests/compose.yml exec secondary-vault vault kv put -mount=secret kubernetes cert=very-valid-cert

# run test suite
for test in $(find tests/bats/ -type f | grep .bats | grep -v roles | grep -v entities | grep -v groups | grep -v errors); do
    echo "running $test"
    # podman-compose -f tests/compose.yml exec vault-manager-test bats --tap "$test"
    bats --tap "$test"
    # hack so flags.bats has clean slate for audit resources when testing
    if [[ $test == "tests/bats/audit/audit-devices.bats" ]]; then
        # need to execute this for both instances
        podman-compose -f tests/compose.yml exec primary-vault vault audit disable file
        podman-compose -f tests/compose.yml exec secondary-vault vault audit disable file
    fi
done

# roles is dependent on secret engines being enabled due to credential output
echo "running bats/roles/roles.bats"
podman-compose -f tests/compose.yml exec vault-manager-test bats --tap tests/bats/roles/roles.bats

# entities is dependent on config generated by other tests
echo "running bats/entities/entities.bats"
podman-compose -f tests/compose.yml exec vault-manager-test bats --tap tests/bats/entities/entities.bats

# groups is dependent on entities
echo "running bats/groups/groups.bats"
podman-compose -f tests/compose.yml exec vault-manager-test bats --tap tests/bats/groups/groups.bats

# run error handling test now that vaults are fully configured
echo "running bats/errors/errors.bats"
podman-compose -f tests/compose.yml exec vault-manager-test bats --tap tests/bats/errors/errors.bats

cleanup
