.PHONY: schemas bundle run data

include ../../.env

CONTAINER_ENGINE ?= $(shell which podman &>/dev/null && echo podman || echo docker)
CONTAINER_SELINUX_FLAG ?= :z
OUTPUT_DIR ?= $(shell pwd)
OUTPUT_DIR := $(shell realpath $(OUTPUT_DIR))
BUNDLE_FILENAME ?= data.json
PWD := $(shell pwd)
VALIDATOR_CONTAINER_NAME ?= qontract-validator
VALIDATOR_IMAGE_NAME ?= quay.io/app-sre/qontract-validator

schemas:
	@rm -rf schemas graphql-schemas
	$(eval ID := $(shell $(CONTAINER_ENGINE) create $(SCHEMAS_IMAGE):$(SCHEMAS_IMAGE_TAG)))
	@$(CONTAINER_ENGINE) cp $(ID):/schemas/. .
	@$(CONTAINER_ENGINE) rm $(ID) &>/dev/null

bundle:
	mkdir -p $(OUTPUT_DIR)
	@$(CONTAINER_ENGINE) run --rm \
		-v $(PWD)/schemas:/schemas:z \
		-v $(PWD)/graphql-schemas:/graphql:z \
		-v $(PWD)/data:/data:z \
		-v $(PWD)/resources:/resources:z \
		$(VALIDATOR_IMAGE):$(VALIDATOR_IMAGE_TAG) \
		qontract-bundler /schemas /graphql/schema.yml /data /resources placeholder placeholder > $(OUTPUT_DIR)/$(BUNDLE_FILENAME)
	@$(CONTAINER_ENGINE) run --rm --name $(VALIDATOR_CONTAINER_NAME) \
		-v $(OUTPUT_DIR):/bundle$(CONTAINER_SELINUX_FLAG) \
		$(VALIDATOR_IMAGE_NAME):$(VALIDATOR_IMAGE_TAG) \
		qontract-validator --only-errors /bundle/data.json

data: schemas bundle
