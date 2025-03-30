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
```

## Run Operator
### Run locally outside the cluster (for development purposes)
Apply CRD into cluster and run controller:
```bash
kustomize build config/crd/ | kubectl apply -f -
go run ./cmd/main.go
```
Then apply your CR.

### Run as a Deployment inside the cluster

This is essentially just calling docker build (with an added dependency to make a test). The Makefile command uses this IMG variable to define the tag for the compiled image.
```bash
export IMG=my-reg.io/sample/netplan-operator:v0.1
make docker-build
```

Oush to docker registry
```bash
make docker push
```

With the Operator image accessible (and the public image name defined in an environment variable or modified in Makefile), all that is required to run the Operator in a cluster now:
```bash
make deploy
```
