
path "app-sre/*" {
    capabilities = ["create", "read", "update", "delete", "list"]
}
path "app-interface/*" {
    capabilities = ["create", "read", "update", "delete", "list"]
}

# https://www.vaultproject.io/api/system/audit-hash#calculate-hash
path "/sys/audit-hash/file" {
    capabilities = ["create", "read", "update"]
}

#allow vault seal/unseal
path "/sys/seal" {
    policy = "sudo"
}
path "/sys/unseal" {
    policy = "sudo"
}
