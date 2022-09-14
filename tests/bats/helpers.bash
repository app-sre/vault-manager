# rerun vault-manager to ensure that nothing happens on further runs
rerun_check() {
    run vault-manager -metrics=false
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
