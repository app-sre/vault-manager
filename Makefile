.PHONY: test build-test-container build push gotest gobuild

IMAGE_NAME := quay.io/app-sre/vault-manager
IMAGE_TAG := $(shell git rev-parse --short=7 HEAD)
DOCKER_CONF := $(CURDIR)/.docker
GOOS := $(shell go env GOOS)

gotest:
	CGO_ENABLED=0 GOOS=$(GOOS) go test ./...

gobuild: gotest
	CGO_ENABLED=0 GOOS=$(GOOS) go build -a -installsuffix cgo ./cmd/vault-manager

build:
	@docker build --no-cache -t $(IMAGE_NAME):$(IMAGE_TAG) .

push:
	@docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(IMAGE_NAME):latest
	@docker --config=$(DOCKER_CONF) push $(IMAGE_NAME):$(IMAGE_TAG)
	@docker --config=$(DOCKER_CONF) push $(IMAGE_NAME):latest

build-test-container:
	@docker build -t vault-manager-test -f tests/Dockerfile.tests .

test: build-test-container
	@docker --config=$(DOCKER_CONF) pull quay.io/app-sre/vault:1.5.4
	@docker --config=$(DOCKER_CONF) pull quay.io/app-sre/qontract-server:64b433b
	@docker --config=$(DOCKER_CONF) pull quay.io/keycloak/keycloak:17.0
	@docker --config=$(DOCKER_CONF) pull quay.io/tpate/keycloak-config-cli:latest
	@docker run -t \
	            --rm \
				--net=host \
	            -v /var/run/docker.sock:/var/run/docker.sock \
				-e HOST_PATH=$(shell pwd) \
	            -e GRAPHQL_SERVER=http://127.0.0.1:4000/graphql \
	            vault-manager-test
