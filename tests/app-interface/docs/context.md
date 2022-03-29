# App-Interface

This repository serves as a central coordination point for hosted services.
Many or all of the portions of the contract
defined herein will be handled through automation after changes are accepted to
this repository by the appropriate parties.

- [App-Interface](#app-interface)
  - [Overview](#overview)
  - [Components](#components)
  - [Workflow](#workflow)
  - [Local validation of datafile modifications / contract amendment](#local-validation-of-datafile-modifications--contract-amendment)
    - [JSON schema validation](#json-schema-validation)
    - [Running integrations locally with `--dry-run`](#running-integrations-locally-with---dry-run)
  - [Querying the App-interface](#querying-the-app-interface)

## Overview

This repository contains of a collection of files under the `data` folder.
Whatever is present inside that folder constitutes the contract.

These files can be `yaml` or `json` files, and they must validate against the some
[well-defined json schemas][schemas].

The path of the files do not have any effect on the integrations (automation
components that feed off the contract), but the contents of the files do. They
will all contain:

- `$schema`: which maps to a well defined schema [schema][schemas].
- `labels`: arbitrary labels that can be used to perform queries, etc.
- Additional data specific to the resource in question.

## Components

Main App-Interface contract components:

- <https://github.com/app-sre/app-interface-demo>: datafiles (schema
  implementations) that define the contract. JSON and GraphQL schemas of the datafiles.
- <https://github.com/app-sre/qontract-server>: The GraphQL component developed
  in this repository will make the datafiles queryable.

Additional components and tools:

- <https://github.com/app-sre/qontract-validator>: Python script that validates
  the datafiles against the json schemas hosted in `qontract-server`.
- <https://github.com/app-sre/qontract-reconcile>: automation component that
  reconciles the state of third-party services like `github`, with the contract
  definition.

The word _qontract_ comes from _queryable-contract_.

## Workflow

The main use case happens when an interested party wants to submit a contract
amendment. Some examples would be:

- Adding a new user and granting access to certain teams / services.
- Submitting a new application to be hosted by the Application SRE team.
- Modifying the SLO of an application.
- etc.

All contract amendments must be formally defined. Formal definitions are
expressed as json schemas. You can find the supported schemas here:
[schemas][schemas].

1. The interested party will:

- Fork the app-interface repository.
- Submit a PR with the desired contract amendment.

2. Automation will validate the amendment and generate a report of the desired
   changes, including changes that would be applied to third-party services like
   `OpenShift`, `Github`, etc, as a consequence of the amendment.

4. From the moment the PR is accepted, the amended contract will enter into
   effect.

## Local validation of datafile modifications / contract amendment

Before submitting a PR with a datafile modification / contract amendment, the
user has the option to validate the changes locally.

Two things can be checked: (1) JSON schema validation (2) run integrations with
`--dry-run` option.

Both scripts rely on a temporary directory which defaults to `temp/`, but can be
overridden with the env var `TEMP_DIR`. The contents of this directory will
contain the results of the manual pr check.

### JSON schema validation

Instructions to perform JSON schema validation on changes to the datafiles.

**Requirements**:

- docker (or podman)
- git

**Instructions**:

Run the actual validator by executing:

```sh
# make sure you are in the top dir of the `app-interface` git repo
source .env
make bundle validate # output is data.json
```

The output will be JSON document, so you can pipe it with `jq`, example:
`cat data.json | jq .`

### Running integrations locally with `--dry-run`

Instructions in [this document](/docs/app-sre/sop/running-integrations-manually.md).

## Querying the App-interface

The contract can be queried programmatically using a
[GraphQL](<https://graphql.org/learn/>) API.

The GraphQL endpoint is reachable here:
<http://localhost:4000/graphql>.

**IMPORTANT**: in order to use the GraphQL UI you need to click on the Settings
wheel icon (top-right corner) and replace `omit` with `include` in
`request.credentials`.
