# KCP Global Load Balancer

![build status badge](https://github.com/kcp-dev/kcp-glbc/actions/workflows/ci.yaml/badge.svg)

The KCP Global Load Balancer Controller (GLBC) solves multi cluster ingress use cases when leveraging KCP to provide transparent multi cluster deployments. 

The main use case it solves currently is providing you with a single host that can be used to access your workload and bring traffic to the correct physical clusters. The GLBC manages the DNS for this host and provides you with a valid TLS certificate. If your workload moves/is moved or expands contracts across clusters, GLBC will ensure that the DNS for this host is correct and traffic will continue to reach your workload.

It also offers the ability to setup a health check for your workload. When this health check fails for a particular cluster, the unhealthy cluster will be removed from the DNS response.

The GLBCs main API is the kubernetes Ingress object. GLBC watches for Ingress objects and mutates them adding in the GLBC managed host and tls certificate. 

In the future we will also add the same functionality for Gateway API and OpenShift Route. 

## Getting Started

To get started with the GLBC, we recommend you follow our [Getting Started tutorial](/docs/getting_started/tutorial.md).

## Development

### Interacting directly with WorkloadClusters

Prefix `kubectl` with the appropriate kubeconfig file from the tmp directory.
For example:

```bash
KUBECONFIG=./tmp/kcp-cluster-1.kubeconfig kubectl get deployments,services,ingress --all-namespaces
```

### Testing

The e2e tests can be executed locally by running the following commands:

#### Terminal 1

Start KCP and create KinD clusters
```bash
make local-setup
```

#### Terminal 2

Run the controller with the same environment as CI ([e2e](.github/workflows/e2e.yaml)):
```bash
(export $(cat ./config/deploy/local/kcp-glbc/controller-config.env.ci | xargs) && \
KUBECONFIG=./tmp/kcp.kubeconfig ./bin/kcp-glbc)
```

#### Terminal 3

Run the e2e tests:

```bash
(export $(cat ./config/deploy/local/kcp-glbc/controller-config.env.ci | xargs) && \
export KUBECONFIG="$(pwd)"/.kcp/admin.kubeconfig && \
export CLUSTERS_KUBECONFIG_DIR="$(pwd)/tmp" && \
make e2e)
```

Run the performance tests:

```bash
(export $(cat ./config/deploy/local/kcp-glbc/controller-config.env.ci | xargs) && \
export KUBECONFIG="$(pwd)"/.kcp/admin.kubeconfig &&
make performance TEST_WORKSPACE_COUNT=1 TEST_DNSRECORD_COUNT=1 TEST_INGRESS_COUNT=1)
```

Run the smoke tests:

```bash
(export $(cat ./config/deploy/local/kcp-glbc/controller-config.env.ci | xargs) && \
export KUBECONFIG="$(pwd)"/.kcp/admin.kubeconfig &&
make smoke)
```

Alternatively, You can run the KCP GLBC and/or the tests from your IDE / debugger.

### Local development on a mac
MacOS does not default to allowing you to communicate with ports running in docker containers from the host machine. To bypass this limitation we can use `docker-mac-net-connect`:
```
# Install via Homebrew
$ brew install chipmk/tap/docker-mac-net-connect

# Run the service and register it to launch at boot
$ sudo brew services start chipmk/tap/docker-mac-net-connect
```

This is done automatically as part of the `make local-setup` but will require presenting a sudo password to start the service if it is not configured to autostart.

N.B. This does not remove the requirement to have the DNS records created in a valid DNS service (e.g. route53 in AWS).

