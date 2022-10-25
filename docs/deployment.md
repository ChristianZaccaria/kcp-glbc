# Deployment

This document describes the process of deploying GLBC using the `./utils/deploy.sh` script directly. 
This requires you to have access to a KCP instance to deploy GLBC to, if you want to try it using a locally running KCP instance and KinD clusters, please refer to the [`local deployment`](local_deployment.md) docs.

**Prerequisite**

* KCP version 0.9

**Useful Links**

* [KCP](https://github.com/kcp-dev/kcp)
* [KCP Docs](https://github.com/kcp-dev/kcp/blob/main/docs)
* [KCP Locations and Scheduling](https://github.com/kcp-dev/kcp/blob/main/docs/content/en/main/concepts/locations-and-scheduling.md)

## GLBC

Deploy the GLBC using a default configuration (config/deploy/local).

```
./utils/deploy.sh -w <target workspace i.e. root:myglbc>
```
N.B. You must be targeting a KCP instance in order to install GLBC.

## GLBC Sync Target

The deployment script will create a `glbc` sync target in the target workspace, however, for GLBC and it's dependencies to deploy the [`KCP syncer`](https://github.com/kcp-dev/kcp/blob/main/docs/content/en/main/concepts/syncer.md) resources must be applied to a physical cluster.
The exact command will be output as part of the deploy script, but will look something like:

```
# targeting your physical cluster run:

kubectl apply -f ./config/deploy/sync-targets/kcp-cluster-1-syncer.yaml
```

Verify the workload cluster becomes ready:
```
# targeting your kcp cluster run:

./bin/kubectl-kcp ws root:<target workspace>
 $ kubectl get synctargets -o wide
NAME            LOCATION        READY   SYNCED API RESOURCES   KEY                                      AGE
kcp-cluster-1   kcp-cluster-1   True                           832Kocbfr9pZCD62hs7bt3No2aylrt9lYwTJYa   46m
```
N.B. It can take a couple of minutes for it go into a "ready" state.

Verify GLBC deployments:
```
./bin/kubectl-kcp ws root:<target workspace>
kubectl get deployments --all-namespaces
NAMESPACE      NAME                          READY   UP-TO-DATE   AVAILABLE   AGE
cert-manager   cert-manager                  1/1     1            1           38m
kcp-glbc       kcp-glbc-controller-manager   1/1     1            1           37m
```

## Configuration

### AWS Credentials (Optional) 

A secret  `secret/kcp-glbc-aws-credentials` containing AWS access key and secret. This is only required if `GLBC_DNS_PROVIDER` is set to `aws`.
The credentials must have permissions to create/update/delete records in the hosted zone set in `AWS_DNS_PUBLIC_ZONE_ID`, and the
domain set in `GLBC_DOMAIN` corresponds to the public zone id. An empty secret is created by default during installation, 
but can be replaced with:

```
kubectl -n kcp-glbc create secret generic kcp-glbc-aws-credentials \
--from-literal=AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
--from-literal=AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
```

### TLS Issuer provider (Optional) 

A TLS Issuer provider supported by cert-manager and created via KCP before running the GLBC controller is required only if the genaration of TLS certs (GLBC_TLS_PROVIDED) for the GLBC is enabled. 

A reference to the TLS certificate issuer resource can be passed when starting the GLBC using the tag `--glbc-tls-provider` or the environment variables `GLBC_TLS_PROVIDER`

Refer to the [cert-manager repo](https://github.com/cert-manager/cert-manager#cert-manager) to learn more about the supported providers and how to create a cert issuer.

There is also a script that generates a let's encrypt issuer against KCP that can be triggered using the command below:

```
go run ./utils/certman-issuer/ 
--glbc-kubeconfig <Path to GLBC kubeconfig> 
--glbc-tls-provider <The TLS certificate issuer, one of [glbc-ca, le-staging, le-production]> 
--region <the region we should target with AWS clients>
--issuer-namespace <namespace where the issuer resource will be created, the namespace should match with the namespace where the glbc is deployed>
```

### GLBC Controller Options (Optional)

A config map `configmap/kcp-glbc-controller-config` containing GLBC configuration options. A config map is created by 
default during installation containing the default values, but can be replaced by editing the config map:

```
kubectl -n kcp-glbc edit configmap kcp-glbc-controller-config
```

| Annotation                    | Description | Default value |
|-------------------------------| ----------- | ------------- |
| `AWS_DNS_PUBLIC_ZONE_ID`      |  AWS hosted zone id where route53 records will be created (default is dev.hcpapps.net) | Z08652651232L9P84LRSB |
| `GLBC_ADVANCED_SCHEDULING`    | Enable advanced scheduling | false |
| `GLBC_DNS_PROVIDER`           |  The dns provider to use, one of [aws, fake] | fake |
| `GLBC_DOMAIN`                 |  The domain to use when exposing ingresses via glbc | dev.hcpapps.net |
| `GLBC_ENABLE_CUSTOM_HOSTS`    | Allow custom hosts in glbc managed ingresses | false |
| `GLBC_EXPORT`                 | The name of the glbc api export to use | glbc-root-kuadrant |
| `GLBC_LOGICAL_CLUSTER_TARGET` | logical cluster to target | `*` |
| `GLBC_TLS_PROVIDED`           | Generate TLS certs for glbc managed hosts | false |
| `GLBC_TLS_PROVIDER`           | The TLS certificate issuer | glbc-ca |
| `GLBC_WORKSPACE`              | The GLBC workspace| root:kuadrant |
| `HCG_LE_EMAIL`                | Email address to use during LE cert requests | kuadrant-dev@redhat.com |
| `NAMESPACE`                   | Target namespace of cert-manager resources (issuers, certificates) | kcp-glbc |

### Applying configuration changes

Any of the described configurations can be modified after the initial creation of the resources, the deployment will however 
need to be restarted after each change in order for them to come into effect.

`kubectl rollout restart deployment/kcp-glbc-controller-manager -n kcp-glbc`


## Configuring for remote KCP

ToDo 
