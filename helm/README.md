# vault-manager
A Helm chart to generate OpenShift templates for vault-manager.

## Generate OpenShift templates

To generate OpenShift templates from this Helm chart:

1. Update `qontract-reconcile/templates/template.yaml` as required
4. `make generate`

## Install Helm (v3)

https://github.com/helm/helm/releases

## Usage

Refer to [values.yaml](./vault-manager/values.yaml) for default options.
One configuration is produced for the standard commercial environment and one for
FedRamp using [Google Chat logging](https://github.com/app-sre/qontract-reconcile/tree/master/helm#logging).
