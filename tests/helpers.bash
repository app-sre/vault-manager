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