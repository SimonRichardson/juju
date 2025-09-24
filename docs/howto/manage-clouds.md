(manage-clouds)=
# How to manage clouds

```{ibnote}
See also: {ref}`cloud`
```

This document shows how to manage your existing cloud(s) with Juju.

(add-a-cloud)=
## Add a cloud

```{ibnote}
See also: {ref}`list-of-supported-clouds`
```

The procedure for how to add a cloud definition to Juju depends on whether the cloud is a machine (traditional, non-Kubernetes) cloud or a Kubernetes cloud -- more below.

In either case, the cloud definition is saved to the directory defined in
- the {ref}`envvar-juju-data` environment variable or, if that is not defined,
    - `$XDG_DATA_HOME/juju` or, if that is not defined,
        - `~/.local/share/juju`

 in a file called `clouds.yaml`.

(add-a-machine-cloud)=
### Add a machine cloud

First, check the {ref}`list of supported machine clouds <list-of-supported-machine-clouds>` to see if your cloud is supported, or the cloud-specific doc linked from there to see if your cloud must meet any prerequisites.

Then, if your cloud is a public cloud or a localhost LXD cloud: Juju likely already knows about it, so you can skip this step; run `juju clouds` to verify.

Otherwise, to add a machine cloud to Juju, run the `add-cloud` command:

```text
juju add-cloud
```

This will start an interactive session where you'll be asked to choose a cloud type (from a given list), the name that you want to use for your cloud, the API endpoint, the authentication type, etc. Juju will use this to create a cloud definition.

<!--
, which it will save to the directory defined in the `JUJU_DATA` environment variable (default path: `~/.local/share/juju/`), in a file called `clouds.yaml`
-->

The command also has a manual mode where you can specify the desired cloud name and cloud definition file in-line; whether you want this definition to be known just to the Juju client or also to an existing controller (the latter creates what is known as a multi-cloud controller); etc.

```{ibnote}
See more: {ref}`command-juju-add-cloud`, {ref}`cloud-definition`
```

(add-a-kubernetes-cloud)=
### Add a Kubernetes cloud

First, check the {ref}`list of supported clouds <list-of-supported-kubernetes-clouds>` to see if your cloud is supported, or the cloud-specific doc linked from there to see if your cloud must meet any prerequisites.

Then, if you're using a localhost MicroK8s cloud installed from a strictly confined snap: Juju likely already knows about it, so you can skip this step; run `juju clouds` to verify.

Otherwise, to add a Kubernetes cloud to Juju:

1. Prepare your kubeconfig file.

2. Run the `add-k8s` command followed by the desired cloud name:

```{important}
**If you have a Juju 3.0+ CLI client installed from snap and you're using a public Kubernetes cloud (AKS, EKS, GKE):** <br>
Run this command with the 'raw' (not strictly confined) snap: `/snap/juju/current/bin/juju add-k8s <cloud name>`.

This is required because, starting with Juju 3.0, the `juju` CLI client snap is a strictly confined snap, whereas the public cloud CLIs are not (see [discussion](https://bugs.launchpad.net/juju/+bug/2007575)), and it is only necessary for this step -- for any other step you can go back to using the client from the strictly confined snap (so, you can keep typing just `juju`).

```

```text
juju add-k8s <cloud name>
```

Juju will check the default location for the kubeconfig file and  use the information in there to create a cloud definition.

The command also allows you to specify a non-default kubeconfig file path (via the `KUBECONFIG` environment variable); in the case where you have multiple cluster definitions and credentials in your kubeconfig file, which cluster and credential to use; what name you want to assign to your new cloud; whether you want to make this cloud known just to the client or also to an existing controller (the latter gives rise to what is known as a multi-cloud controller); etc.

```{ibnote}
See more: {ref}`command-juju-add-k8s`
```

## View all the known clouds

To get a list of all the clouds that your Juju client is currently aware of, run the `clouds` command with the `--client` and `-all` flags:

```text
juju clouds --client --all
```

````{dropdown} Example output

```text
You can bootstrap a new controller using one of these clouds...

Clouds available on the client:
Cloud        Regions  Default        Type     Credentials  Source    Description
aws          22       us-east-1      ec2      0            public    Amazon Web Services
aws-china    2        cn-north-1     ec2      0            public    Amazon China
aws-gov      2        us-gov-west-1  ec2      0            public    Amazon (USA Government)
azure        43       centralus      azure    0            public    Microsoft Azure
azure-china  4        chinaeast      azure    0            public    Microsoft Azure China
equinix      25       px             equinix  0            public
google       25       us-east1       gce      0            public    Google Cloud Platform
localhost    1        localhost      lxd      1            built-in  LXD Container Hypervisor
microk8s     1        localhost      k8s      1            built-in  A Kubernetes Cluster
oracle       4        us-phoenix-1   oci      0            public    Oracle Compute Cloud Service
```

````

In the output, each line represents a cloud that Juju can interact with -- the cloud name (that you will have to use to interact with the cloud), the number of cloud regions Juju is aware of, the default region (for the current Juju client), the type/API used to control it, the number of credentials associated with a cloud, the source of the cloud, and a brief description.

By omitting the flags, you will see a list of the clouds available on the client for which you have also registered the credentials. Alternatively, by passing other flags you can specify an output format or file, etc.

```{ibnote}
See more: {ref}`command-juju-clouds`
```

## View details about a cloud

To get more detail about a particular cloud, run the `show-cloud` command followed by the cloud name. For example:

```text
juju show-cloud azure
```

The command also has flags that allow you to specify whether you want this information from the client or a controller; whether you want the output to include the configuration options specific to the cloud; an output format or file; etc.

```{ibnote}
See more: {ref}`command-juju-show-cloud`
```

## Manage cloud regions

### View all the known regions

To see which regions Juju is aware of for any given cloud, use the `regions` command. For example, for the `aws` cloud, run:

```text
juju regions aws
```

````{dropdown} Example output
```text
Client Cloud Regions
us-east-1
us-east-2
us-west-1
us-west-2
ca-central-1
eu-west-1
eu-west-2
eu-west-3
eu-central-1
eu-north-1
eu-south-1
af-south-1
ap-east-1
ap-south-1
ap-southeast-1
ap-southeast-2
ap-southeast-3
ap-northeast-1
ap-northeast-2
ap-northeast-3
me-south-1
sa-east-1
```
````

The command also has flags that allow you to select a specific controller, choose an output format or file, etc.

```{ibnote}
See more: {ref}`command-juju-regions`
```


### Manage the default region

**Set the default region.** To set the default region for a cloud, run the `default-region` command followed by the name of the cloud and the name of the region that you want to start using as a default. For example:

```text
juju default-region aws eu-central-1
```

If at any point you want to reset this value, drop the region argument and pass the `--reset` flag.

```{ibnote}
See more: {ref}`command-juju-default-region`
```

**Get the default region.** To get the current default region for a cloud, run the `default-region` command followed by the name of the cloud. For example:

```text
juju default-region azure-china
```

```{ibnote}
See more: {ref}`command-juju-default-region`
```

## Manage cloud credentials

See {ref}`manage-credentials`.

## Update a cloud

The procedure for how to update a cloud on Juju depends on whether the cloud is public or private.


### Update a public cloud

To synchronise the Juju client with changes occurring on public clouds (e.g. cloud API changes, new cloud regions) or on Juju's side (e.g. support for a new cloud), run the `update-public-clouds` command:

```text
juju update-public-clouds
```

The command also allows you to specify whether you want this update to happen on the client or a controller.

```{ibnote}
See more: {ref}`command-juju-update-public-clouds`
```

### Update a private cloud

To update Juju's definition for a private cloud, run the `update-cloud` command followed by the cloud name and the `-f` flag followed by the path to the new cloud definition file. For example:

```text
juju update-cloud mymaas -f path/to/maas.yaml
```

The command also allows you to indicate whether the update should happen on the client or the controller; to to update the definition on a controller to match the one on the client; etc.

```{ibnote}
See more: {ref}`command-juju-update-cloud`
```

<!--
The definition of an existing cloud can be done locally or, since `v.2.5.3`, remotely (on a controller).

For the 'oracle' cloud, for instance, create a [YAML-formatted](http://www.yaml.org/spec/1.2/spec.html) file, say `oracle.yaml`, with contents like:

```yaml
clouds:
   oracle:
      type: oci
      config:
         compartment-id: <some value>
```

Here, the local (client cache) definition is modified:

```text
juju update-cloud --local oracle -f oracle.yaml
```

This will avoid having to include `--config compartment-id=<value` at controller-creation time (`bootstrap`).

Here, the remote definition is updated by specifying the controller:

```bash
juju update-cloud oracle -f oracle.yaml -c oracle-controller
```

If you specify a controller without supplying a YAML file then the remote cloud will be updated according to the client's current knowledge of that cloud.

-->


## Remove a cloud

```{ibnote}
See also: {ref}`removing-things`
```

Given a cloud definition that you've added explicitly to Juju via `add-cloud` or `add-k8s`, to remove that definition from Juju, run the `remove-cloud` command followed by the name of the cloud. For example:

```text
juju remove-cloud lxd-remote
```

The command also allows you to specify whether this operation should be performed on the client or a on a specific controller.

```{ibnote}
See more: {ref}`command-juju-remove-cloud`
```
