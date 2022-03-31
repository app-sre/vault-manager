#!/bin/bash

set -e

export KEYCLOAK_USER=admin
export KEYCLOAK_PASSWORD=admin

export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=root
export VAULT_AUTHTYPE=token

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
  -v $HOST_PATH/$(pwd)/keycloak:/config \
  $KEYCLOAK_CLI_IMAGE:$KEYCLOAK_CLI_IMAGE_TAG

# spin up qontract-server, using existing data.json file
docker run -d --rm \
  --net=host \
  --name=$QONTRACT_SERVER_NAME \
  -v $HOST_PATH/$(pwd)/app-interface:/bundle:z \
  -p 4000:4000 \
  -e LOAD_METHOD=fs \
  -e DATAFILES_FILE=/bundle/data.json \
  $QONTRACT_SERVER_IMAGE:$QONTRACT_SERVER_IMAGE_TAG
container_alive "http://127.0.0.1:4000" $CONTAINER_HEALTH_TIMEOUT_DEFAULT $QONTRACT_SERVER_NAME

# spin up vault server
docker run -d --name=$VAULT_NAME \
  --net=host \
  --cap-add=IPC_LOCK \
  -e 'VAULT_DEV_ROOT_TOKEN_ID=root' \
  -p 8200:8200 \
  -v /tmp/:/var/log/vault/:Z \
  $VAULT_IMAGE:$VAULT_IMAGE_TAG
container_alive "http://127.0.0.1:8200" $CONTAINER_HEALTH_TIMEOUT_DEFAULT $VAULT_NAME

for test in $(find bats/ -type f | grep .bats); do
    echo "running $test"
    bats --tap $test
    # hack so flags.bats has clean slate for audit resources when testing
    if [[ $test == "bats/audit/audit-devices.bats" ]]; then
        vault audit disable file
    fi
done

cleanup
