# netplan-operator

## Technical requirements
* go version 1.16+
* An operator-sdk binary installed locally
* Make sure your user is authorized with cluster-admin permissions.

## Step 1 : Setting up your project

initialize a boilerplate project structure with the following:

```bash
# we'll use a domain of netplan.io
# so all API groups will be <group>.netplan.io
operator-sdk init --domain netplan.io --repo github.com/saeed-mcu/netplan-operator
```

## Step 2 : Defining an API
```bash
operator-sdk create api --group network --version v1 --kind NetplanConfig --resource --controller
```
Chnage the code and logic for reconciler and type:
```
make generate
make manifest
# Apply CRD into Cluster
kustomize build config/crd/ | kubectl apply -f -
# run controller
go run ./cmd/main.go
```
