#! /bin/bash

set -e

source .env

CONTAINER_HEALTH_TIMEOUT_DEFAULT=60

cleanup () {
  echo "cleaning"
  if docker ps -a --format "table {{.Names}}" | grep -qw $KEYCLOAK_CLI_NAME; then
    docker rm -f $KEYCLOAK_CLI_NAME
  fi
  if docker ps -a --format "table {{.Names}}" | grep -qw $KEYCLOAK_NAME; then
    docker rm -f $KEYCLOAK_NAME
  fi
  if docker ps -a --format "table {{.Names}}" | grep -qw $QONTRACT_SERVER_NAME; then
    docker rm -f $QONTRACT_SERVER_NAME
  fi
  if docker ps -a --format "table {{.Names}}" | grep -qw $VAULT_NAME; then
    docker rm -f $VAULT_NAME
  fi
  if docker ps -a --format "table {{.Names}}" | grep -qw $VAULT_NAME_SECONDARY; then
    docker rm -f $VAULT_NAME_SECONDARY
  fi
  echo "Docker environment cleaned"
}

container_alive () {
  echo ""
  echo "testing connectivity with container $3"
  echo ""

  idx=0
  until $(curl --output /dev/null --silent --head --fail $1); do
    printf '.'
    sleep 1
    if [[ $idx == $2 ]]; then
      cleanup
      exit 1
    fi
    ((++idx))
  done

  echo ""
  echo "connectivty established with $3"
  echo ""
}

audit_perms() {
  # allow container to write to audit log location
  # which by default only has root permissions
  AUDIT_LOC=/var/log/vault

  docker exec -u 0:0 $1 mkdir -p $AUDIT_LOC
  docker exec -u 0:0 $1 chown vault:vault $AUDIT_LOC
}

cleanup

# spin up keycloak server
docker run -d --name=$KEYCLOAK_NAME \
  --net=host \
  --cap-add=IPC_LOCK \
  -e KEYCLOAK_ADMIN=$KEYCLOAK_USER -e KEYCLOAK_ADMIN_PASSWORD=$KEYCLOAK_PASSWORD \
  -p 8180:8180 \
  $KEYCLOAK_IMAGE:$KEYCLOAK_IMAGE_TAG \
  start-dev \
  --http-port 8180 \
  --http-relative-path /auth
container_alive "http://127.0.0.1:8180/auth" 120 $KEYCLOAK_NAME

# run keycloak-cli container to apply realm, client, and user config to keycloak server
docker run --name $KEYCLOAK_CLI_NAME \
  --net=host \
  -e KEYCLOAK_URL="http://localhost:8180/auth" \
  -e KEYCLOAK_USER=$KEYCLOAK_USER \
  -e KEYCLOAK_PASSWORD=$KEYCLOAK_PASSWORD \
  -e KEYCLOAK_AVAILABILITYCHECK_ENABLED=true \
  -e IMPORT_FILES='/config/*' \
  -v $(pwd)/tests/keycloak:/config:z \
  $KEYCLOAK_CLI_IMAGE:$KEYCLOAK_CLI_IMAGE_TAG

# spin up qontract-server, using existing data.json file
docker run -d --rm \
  --net=host \
  --name=$QONTRACT_SERVER_NAME \
  -v $(pwd)/tests/app-interface:/bundle \
  -p 4000:4000 \
  -e LOAD_METHOD=fs \
  -e DATAFILES_FILE=/bundle/data.json \
  $QONTRACT_SERVER_IMAGE:$QONTRACT_SERVER_IMAGE_TAG
container_alive "http://127.0.0.1:4000" $CONTAINER_HEALTH_TIMEOUT_DEFAULT $QONTRACT_SERVER_NAME

# spin up primary vault server
docker run -d --name=$VAULT_NAME \
  --net=host \
  --cap-add=IPC_LOCK \
  -e 'VAULT_DEV_ROOT_TOKEN_ID=root' \
  -p 8200:8200 \
  $VAULT_IMAGE:$VAULT_IMAGE_TAG
container_alive "http://127.0.0.1:8200" $CONTAINER_HEALTH_TIMEOUT_DEFAULT $VAULT_NAME

audit_perms $VAULT_NAME

# populate necessary vault access vars to master
vault kv put secret/master rootToken=root
vault kv put secret/secondary root=root
vault kv put secret/oidc client-secret=my-special-client-secret
vault kv put secret/kubernetes cert=very-valid-cert

# spin up secondary vault server
docker run -d --name=$VAULT_NAME_SECONDARY \
  --net=host \
  --cap-add=IPC_LOCK \
  -e 'VAULT_DEV_ROOT_TOKEN_ID=root' \
  -e 'VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8202' \
  -p 8202:8202 \
  $VAULT_IMAGE:$VAULT_SECONDARY_IMAGE_TAG
container_alive "http://127.0.0.1:8202" $CONTAINER_HEALTH_TIMEOUT_DEFAULT $VAULT_NAME_SECONDARY

audit_perms $VAULT_NAME_SECONDARY

# populate oidc client secret and kubernetes cert in secondary
export VAULT_ADDR=http://127.0.0.1:8202
vault kv put secret/oidc client-secret=my-special-client-secret
vault kv put secret/kubernetes cert=very-valid-cert
export VAULT_ADDR=http://127.0.0.1:8200
