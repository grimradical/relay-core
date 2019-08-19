# Nebula Tasks

The code in this repository supports the tailored task images created by
Puppet for use in Nebula workflows. Some of the code is used to support
APIs that the tasks can use to get metadata, secrets and other data needed
to run.

## Builds

The build system will check each directory inside `./cmd` and look for a
`build-info.sh` file. If one exists, it will source that file and plug it's
configuration into a docker image template `scripts/Dockerfile.template`. This
allows all required binaries to be build into their own docker images. By default
the final image is based on an alpine container specified in `scripts/Dockerfile.include`.
If a directory inside `./cmd` contains a `Dockerfile.include`, that will be used as
the final build image instead.

### build-info.sh

Current supported configuration variables are listed below.

- `DOCKER_CMD`: this is the name of the command in the image to execute as part of the
  `CMD` directive (i.e. `nebula-k8s-provisioner`).
- `DOCKER_REPO`: this is a docker hub repo path (i.e. `gcr.io/nebula-tasks/nebula-k8s-provisioner`).

### Dockerfile.include

The build system uses a multi-stage build approach and calls the build container `builder`.
The base builder is not swappable at the moment. This would be a nice to have, but since
all of the sub-projects in this repo are written in Go, a standard Go base image is the only
one needed. See the base Dockerfile.include for an example.

## Tasks

### projectnebula/slack-notification

A task that will send a message to a slack channel.

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `apiToken` | The slack legacy API token to use | None | True |
| `channel` | The channel to send the message to | None | True |
| `message` | The message to send | None | True |
| `username` | The username to use in slack | None | True |

### projectnebula/k8s-provisioner

A task that creates and manages Kubernetes clusters in cloud platforms.

Note: This task can cost you money. It will provision resources in your cloud platform account
which can incur charges for resource use.

Current supported platforms:
- Google Cloud Platform (GCP)
- AWS

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `provider` | The cloud provider to use. Currently "aws" or "gcp". | None | True |
| `project` | The GCP project ID. Not used with AWS. | None | True for GCP |
| `clusterName` | A name for your cluster. This must be a FQDN. You can use a root domain in route53 or GCP domain name service or you can set the domain to `k8s.local` if you don't want to use one of your roots. | None | True |
| `credentials` | A map of credentials used for platform authentication | None | True |
| `credentials.gcpServiceAccountFile` | The GCP service account json | None | True |
| `credentials.awsAccessKeyID` | The aws access key ID | None | True |
| `credentials.awsSecretAccessKey` | The aws secret access key | None | True |
| `credentials.sshPublicKey` | An ssh public key to install on the virtual machine instances that run the cluster | None | True for AWS |
| `stateStoreName` | a storage bucket name to store cluster state. This configuration will use the storage system of your cloud provider, so if you are using AWS, then s3 will be used. If you are using GCP then gs will be used. If the bucket exists, then the task will try to just use it, otherwise the task will attempt to create the bucket. Multiple clusters can use the same state storage as long as the clusterName's are different. | None | True |
| `masterCount` | A count of how many master nodes to provision. | 1 | False |
| `nodeCount` | A count of how many agent nodes to provision. | 3 | False |
| `zones` | A list of zones in the cloud platform to run node instances in. | None | True (at least one) |
| `region` | A platform region to use when provisioning a cluster. | None | True |

TODO:
- [ ] integrate outputs system by storing master certificates used in kubeconfig for descendant tasks
- [ ] add support for Digital Ocean (supported by kops directly as alpha)

### projectnebula/helm-deployer

A task that deploys a helm chart to a Kubernetes cluster.

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `credentials` | A map of cert credentials to use for accessing the tiller controller in the cluster | None | True |
| `credentials.ca` | The tiller CA file contents | None | True |
| `credentials.key` | The tiller Key file contents | None | True |
| `credentials.cert` | The tiller cert file contents | None | True |
| `values` | A map of values to use for the helm deployment call | None | True |
| `chart` | The repo/chart to use. If the `git` map is set, then the chart will be referenced from that repository instead of a remote chart repo. | None | True |
| `namespace` | The Kubernetes namespace to deploy the chart into. | None | True |
| `git` | A map of git configuration. See [git specification](#common-spec-git) | None | False |
| `cluster` | A map of kubectl configuration and credentials. See [cluster specification](#common-spec-cluster) | None | True |

TODO:
- [ ] Make `credentials` optional by running tiller temporarily inside of the task container. This allows us to run clusters without the need
  for tiller, which is notoriously complicated to run right (especially safely across multiple namespaces).
- [ ] Add a repositoryURL key that we will register with the helm command so we can allow chart installs from repos outside stable and incubator.
- [ ] make sure all keys are camelCased to be consistent and not contain _

### projectnebula/kubectl

A task that allows general kubectl use. This can largely take arbitrary kubectl commands.

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `cluster` | A map of kubectl configuration and credentials. See [cluster specification](#common-spec-cluster) | None | True |
| `command` | A command to pass to kubectl. e.g. `apply`. | None | True |
| `file` | A resource file to use. Setting this implies the need for `-f`. | None | False |
| `namespace` | The namespace to run the command under. | `default` | False |
| `git` | A map of git configuration. See [git specification](#common-spec-git) | None | False |

### projectnebula/terraform

A task that runs terraform provisioning.

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `provider` | The cloud provider to use. Currently "aws" or "gcp". | `gcp` | False |
| `credentials` | A map of platform credentials. See [credentials specification](#common-spec-credentials) | None | True |
| `workspace` | A name for the Terraform workspace to use. | `default` | False |
| `directory` | A directory containing Terraform mordules and resources. | `default` | False |
| `git` | A map of git configuration. See [git specification](#common-spec-git) | None | False |

### projectnebula/kaniko

A task that runs the Kaniko image builder.

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `destination` | A destination directory for the build. | None | True |
| `git` | A map of git configuration. See [git specification](#common-spec-git) | None | False |

### projectnebula/kustomize

A task that applies k8s kustomizations.

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `cluster` | A map of kubectl configuration and credentials. See [cluster specification](#common-spec-cluster) | None | True |
| `path` | A path to the kustomization resources. | None | True |
| `namespace` | The namespace to run the command under. | `default` | False |
| `git` | A map of git configuration. See [git specification](#common-spec-git) | None | False |

### projectnebula/ecs-provisioner

A task that provisions ECS clusters in AWS.

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `cluster` | A map of ECS cluster configuration. | None | True |
| `cluster.name` | A name for the cluster. | None | True |
| `cluster.region` | A region for the cluster. | None | True |
| `path` | A path to the workspace to use for provisioning. | None | True |
| `credentials` | A map of platform credentials. See [credentials specification](#common-spec-credentials) | None | True |
| `git` | A map of git configuration. See [git specification](#common-spec-git) | None | False |

### projectnebula/ecs-deployer

A task that deploys containers to ECS clusters in AWS.

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `path` | A path to the workspace to use for deploying. | None | True |
| `cluster` | A map of ECS cluster configuration. | None | True |
| `cluster.name` | A name for the cluster. | None | True |
| `cluster.region` | A region for the cluster. | None | True |
| `credentials` | A map of platform credentials. See [credentials specification](#common-spec-credentials) | None | True |
| `git` | A map of git configuration. See [git specification](#common-spec-git) | None | False |

### projectnebula/vault

A task that runs commands against vault instances.

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `token` | A token to use for accessing vault. | None | True |
| `url` | The URL to the vault cluster. | None | True |
| `command` | The command to pass to the vault CLI. | None | True |
| `args` | A list of arguments for the command. | None | True |
| `git` | A map of git configuration. See [git specification](#common-spec-git) | None | False |

### Common spec: `git`

A common specification for cloning a git repository.

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `git` | A map of git configuration. Will clone a repository to a directory called `git.name`. | None | |
| `git.ssh_key` | The ssh key to use when cloning the git repository. | None | True |
| `git.known_hosts` | SSH known hosts file. | None | True |
| `git.name` | A directory name for the git clone. | None | True |
| `git.repository` | The git repository URL. | None | True |

### Common spec: `cluster`

A common specification for adding Kubernetes cluster credentials.

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `cluster` | A map of configuration and credentials for accessing a Kubernetes cluster. | None | |
| `cluster.name` | A name for the Kubernetes cluster. Used for referencing it via kubectl contexts. | None | True |
| `cluster.url` | The URL to the Kubernetes cluster master. | None | True |
| `cluster.cadata` | A file containing the Kubernetes master CA contents. | None | True |
| `cluster.token` | A token for the Kubernetes master | None | True |

### Common spec: `credentials`

A common specification for GCP and AWS credentials.

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `credentials` | The cloud provider access credentials. | None | |
| `credentials.credentials.json` | The GCP service account file contents. | None | True for `gcp` |
| `credentials.credentials` | The AWS shared account file contents. | None | True for `aws` |

TODO:
- [ ] generalize this more. This is mostly related to cloud credentials, so maybe this section should just be called
  `cloudCredentials` instead. Or maybe `platformCredentials`. Or maybe `credentials.cloud.<platform>`.
