.PHONY: test build-test-container build push

IMAGE_NAME := quay.io/app-sre/vault-manager
IMAGE_TAG := $(shell git rev-parse --short=7 HEAD)
DOCKER_CONF := $(CURDIR)/.docker

build:
	@docker build -t builder:$(IMAGE_TAG) -f Dockerfile.build .
	@docker container create --name extract_$(IMAGE_TAG) builder:$(IMAGE_TAG)
	@docker container cp extract_$(IMAGE_TAG):/go/src/github.com/app-sre/vault-manager/vault-manager vault-manager
	@docker container rm extract_$(IMAGE_TAG)
	@docker build -t $(IMAGE_NAME):latest .
	@docker tag $(IMAGE_NAME):latest $(IMAGE_NAME):$(IMAGE_TAG)

push:
	@docker --config=$(DOCKER_CONF) push $(IMAGE_NAME):latest
	@docker --config=$(DOCKER_CONF) push $(IMAGE_NAME):$(IMAGE_TAG)

build-test-container:
	@docker build -t vault-manager-test -f tests/Dockerfile.tests .

test: build build-test-container
	@docker run -ti --rm --net=host -v /var/run/docker.sock:/var/run/docker.sock vault-manager-test
