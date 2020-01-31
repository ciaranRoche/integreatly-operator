# Integreatly Operator

A Kubernetes Operator based on the Operator SDK for installing and reconciling Integreatly products.

### Project status: _alpha_ 

This is a proof of concept/alpha version. Most functionality is present but it is highly likely there are bugs and improvements needed.

### Installed products
Currently the operator installs the following products:
- AMQ Online
- AMQ Streams
- Codeready
- Fuse
- Launcher
- Nexus
- RHSSO (both a cluster instance and a user instance)
- 3scale
- Integreatly solution explorer

## Prerequisites
- [operator-sdk](https://github.com/operator-framework/operator-sdk) version v0.12.0.
- [go](https://golang.org/dl/) version 1.13.4+
- [moq](https://github.com/matryer/moq)
- [oc](https://docs.okd.io/3.11/cli_reference/get_started_cli.html#cli-reference-get-started-cli) version v3.11+
- Access to an Openshift v4.2.0+ cluster
- A user with administrative privileges in the OpenShift cluster
- AWS account with permissions to create S3 buckets

After installation, the following commands must be run to avoid a known [issue](https://github.com/matryer/moq/issues/98) related to the Moq package:
```
go get -u .
go install github.com/matryer/moq
```

## Local Setup

Download the integreatly-operator project:
```sh
mkdir -p $GOPATH/src/github.com/integr8ly
cd $GOPATH/src/github.com/integr8ly
git clone https://github.com/integr8ly/integreatly-operator
cd integreatly-operator
```

Some products require certain credentials to be present in the namespace before installation can proceed: 
* RHSSO requires Github OAuth credentials to create a Github Identity Provider for Launcher (see [here](https://github.com/integr8ly/installation/#51-create-github-oauth-to-enable-github-authorization-for-launcher) for creating a Github OAuth app) and Codeready

**Note:** If this secret isn't created, the integreatly preflight checks will fail

```sh
# The project name for the integreatly operator to watch 
export NAMESPACE="integreatly-test"

# RHSSO requires Github OAuth credentials to setup a Github identity provider
# for Fabric8 Launcher and Codeready
export GH_CLIENT_ID=<client id>
export GH_CLIENT_SECRET=<client secret>

# Bootstrap the project
make cluster/prepare/local
```


* 3scale requires AWS S3 bucket credentials for storage. The bucket should have all public access turned off.
Currently this secret (`threescale-blobstorage-<installation-name>`) is created with dummy credentials by the [cloud resource operator](https://github.com/integr8ly/cloud-resource-operator), in the namespace the integreatly operator is deployed into. In order for this feature to work, these credentials should be replaced:
    * _bucketName_: The name of the AWS bucket
    * _bucketRegion_: The AWS region where the bucket has been created
    * _credentialKeyID_: The AWS access key
    * _credentialSecretKey_: The AWS secret key

You can use this command to replace S3 credentials in 3Scale secret:
```sh
oc process -f deploy/s3-secret.yaml -p AWS_ACCESS_KEY_ID=<YOURID> -p AWS_SECRET_ACCESS_KEY=<YOURKEY> -p AWS_BUCKET=<YOURBUCKET> -p AWS_REGION=eu-west-1 -p NAMESPACE=<integreatly-operator-namespace> -p NAME=threescale-blobstorage-<installation-name> | oc replace -f -
```

* Backup jobs require AWS S3 bucket credentials for storage. A `backups-s3-credentials` Secret is created the same way as a 3Scale secret described above.

You can use this command to replace S3 credentials in backup secret:
```sh
oc process -f deploy/s3-secret.yaml -p AWS_ACCESS_KEY_ID=<YOURID> -p AWS_SECRET_ACCESS_KEY=<YOURKEY> -p AWS_BUCKET=<YOURBUCKET> -p AWS_REGION=eu-west-1 -p NAMESPACE=<integreatly-operator-namespace> | oc replace -f -
```


### Installation custom resource
An `Installation` custom resource can now be created which will kick of the installation of the integreatly products, once the operator is running:
```sh
# Create the installation custom resource definition
oc create -f deploy/crds/installation.crd.yaml   

# Create the installation custom resource
oc create -f deploy/crds/examples/installation.cr.yaml

# The operator can now be run locally
make code/run
```
*Note:* if an operator doesn't find Installation resource, it will create one (Name: `integreatly-operator`).

### Logging in to SSO 

In the OpenShift UI, in `Projects > integreatly-rhsso > Networking > Routes`, select the `sso` route to open up the SSO login page.

# Bootstrap the project

```sh
make cluster/prepare/local
```

### Configuring Github OAuth
Log in to RHSSO (see above) and click `Identity Providers` in the left sidebar. In the Github identity provider, find the Redirect URI and paste this URL into the Homepage URL and Authorization callback URL fields of your Github OAuth app. 

## Deploying to a Cluster with OLM
Make sure to export the variables above (see [local setup](#local-setup)), then run:

```sh
make cluster/prepare
```

Within a few minutes, the Integreatly operator should be visible in the OperatorHub (`Catalog > OperatorHub`). To create a new subscription, click on the Install button, choose to install the operator in the created namespace and keep the approval strategy on automatic.

Once the subscription shows a status of `installed`, a new Integreatly `Installation` custom resource can be created which will begin to install the supported products.

In `Catalog > Developer Catalog`, choose the Integreatly Installation and click create. An example installation CR can be found below:

```yml
apiVersion: integreatly.org/v1alpha1
kind: Installation
metadata:
  name: example-installation
spec:
  type: workshop
  namespacePrefix: integreatly-
  selfSignedCerts: true
```

## Set up dedicated admins 

To setup your cluster to have dedicated admins run the `./scripts/setup-htpass-idp.sh` script which creates htpasswd identity provider and creates users.

## Tests

Running unit tests:
```sh
make test/unit
```

## Release

Update the operator version in the following files:

* Update [version/version.go](version/version.go) (`Version = "<version>"`)

* Update `TAG` and `PREVIOUS_TAG` (the previous version) in the [Makefile](Makefile) 

* Update the operator image version in [deploy/operator.yaml](deploy/operator.yaml)
(`image: quay.io/integreatly/integreatly-operator:v<version>`)

* Generate a new CSV:
```sh
make gen/csv
```

Commit changes and open pull request. When the PR is accepted, create a new release tag:

```sh
git tag v<version> && git push upstream v<version>
```

## Debugging PR CI Failures
Prow is used to run various checks on every Pull Request made to the operator. These checks can fail for various reasons. Below is a list of steps to help debug failures in Prow.
### Prerequisites
* Ensure you are a member of Openshift Origin, follow [this guide](https://mojo.redhat.com/docs/DOC-1081313#jive_content_id_Github_Access) and when you get as far as `sending the email request`, use the `the openshift org so I can generate an api.ci key` option. Once a member of this organisation you will gain access to the resources used to perform Prow checks.
### e2e check
The following steps will help debug failing e2e tests.
* Visit [this cluster](https://api.ci.openshift.org/console/catalog) and log in via your GitHub. 
* Here you will have access to Namespace used to for running the checks on your Pull Request.
* In this Namespace you will see an `e2e` pod
* There are numerous containers running in this pod. The first one of interest is the `setup` container. Here we can see in the log the cluster being provisioned to execute the e2e tests against. Once this cluster has provisioned, a `url` to the cluster will be output.
* You will also see output in the logs the location to the cluster credentials.
* To retrieve the cluster credentials, you can simply `cat` the location in the `test` container.
* Logging into the cluster with these credentials you will have `admin` access allowing you to debug the issue while the tests are being executed.
* *NOTE* This cluster will be deprovisioned once the `e2e timeout` has exceeded. 