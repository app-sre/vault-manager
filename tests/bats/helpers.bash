setup() {
  export VAULT_ADDR=http://127.0.0.1:8200
  export VAULT_TOKEN=root
  export VAULT_AUTHTYPE=token

  docker run -d --name="vault-dev-server" --cap-add=IPC_LOCK -e 'VAULT_DEV_ROOT_TOKEN_ID=root' -p 8200:8200 vault:0.11.1
  until $(curl --output /dev/null --silent --head --fail http://127.0.0.1:8200); do
    printf '.'
    sleep 1
  done
}

teardown() {
  docker rm -f vault-dev-server
}

# rerun vault-manager to ensure that nothing happens on further runs
rerun_check() {
    run vault-manager
    [ "$status" -eq 0 ]
    # check vault-manager output
    [[ "${output}" == "" ]]
}

# write the given string to the console.
decho() {
    echo "# DEBUG: " $@ >&3
}

check_vault_secret() {
    run vault $1 $2
    [ "$status" -eq 0 ]
    [[ "${output}" == *"$3"* ]]
}

check_vault_secret_not_exist() {
    run vault $1 $2
    [ "$status" -eq 0 ]
    [[ "${output}" != *"$3"* ]]
}